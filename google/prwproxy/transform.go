// Copyright 2025 Google LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prwproxy

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/stsynthesis"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
)

const (
	// DefaultUnknownGaugeSuffix and DefaultUnknownCounterSuffix mirror the metric
	// type suffixes the GMP Prometheus fork appends to GCM metric types for
	// untyped ("unknown") metrics, see google/export/series_cache.go. Keeping the
	// same suffixes means users migrating from the fork to vanilla Prometheus +
	// this proxy keep querying the exact same GCM metric types.
	DefaultUnknownGaugeSuffix   = "/unknown"
	DefaultUnknownCounterSuffix = "/unknown:counter"

	// DefaultStaleAfter is how long per-series synthesis state is kept around
	// after the last sample was seen for that series.
	DefaultStaleAfter = 10 * time.Minute
)

// TransformConfig configures Transformer.
type TransformConfig struct {
	// HandleUnknown enables splitting untyped (METRIC_TYPE_UNSPECIFIED) series
	// into an explicit gauge stream and an explicit cumulative counter stream.
	HandleUnknown bool
	// UnknownGaugeSuffix and UnknownCounterSuffix are appended to __name__ of the
	// respective streams. Set to "" to leave the name untouched (useful if the
	// receiving endpoint derives the suffix from the metric type itself).
	UnknownGaugeSuffix   string
	UnknownCounterSuffix string

	// SynthesizeST enables start timestamp synthesis for cumulative series
	// (counters and histograms) that arrive without one.
	SynthesizeST bool

	// StaleAfter is the TTL of the per-series synthesis state.
	StaleAfter time.Duration
}

// Transformer rewrites PRW2 requests so they satisfy the stricter Google Cloud
// Monitoring (Monarch) data model:
//
//  1. Cumulative series without a start timestamp (ST) get one synthesized,
//     re-basing values against the first observed sample. This reuses the exact
//     same logic the Prometheus scrape loop uses (model/stsynthesis).
//
//  2. Untyped ("unknown") series are split into two explicitly typed streams: a
//     gauge and a monotonic cumulative counter (with a synthesized ST). This
//     mirrors both what the GMP Prometheus fork does today and the equivalent
//     OpenTelemetry Collector `transform` processor recipe
//     (copy_metric -> convert_gauge_to_sum("cumulative", true)).
//
// Transformer is stateful: it holds one stsynthesis.Cache per cumulative series
// it had to synthesize an ST for.
//
// Concurrency and ordering model:
//
// prwproxy is designed to run as a 1:1 sidecar next to a single Prometheus
// instance (single Prometheus -> single prwproxy). In Prometheus's remote write
// QueueManager, series are consistently sharded by label hash (hash % numShards),
// and each shard goroutine reads samples from the WAL and sends HTTP batches
// sequentially in timestamp order.
//
// Consequently, for any given series (label hash):
//  1. Requests are expected to arrive sequentially (no concurrent requests for
//     the same series).
//  2. Samples within and across requests are expected to arrive in strictly
//     increasing timestamp order (Timestamp > lastTimestamp).
//
// Transformer validates both expectations at runtime:
//   - Each seriesState has a mutex (mtx). In synthesize(), we first attempt
//     mtx.TryLock(). If contention is detected (TryLock returns false), we
//     increment prwproxy_series_concurrent_access_total and fall back to
//     mtx.Lock() to serialize access and prevent data races on stsynthesis.Cache.
//   - Each seriesState tracks lastTs (the timestamp of the last synthesized
//     sample/histogram). Any sample arriving with Timestamp <= lastTs is
//     dropped and increments prwproxy_samples_out_of_order_total. This prevents
//     out-of-order or duplicate samples from falsely triggering counter reset
//     heuristics in stsynthesis.Cache or producing invalid cumulative intervals
//     where StartTimestamp >= Timestamp.
type Transformer struct {
	cfg TransformConfig

	mtx   sync.Mutex
	state map[uint64]*seriesState

	metrics *transformMetrics
}

type seriesState struct {
	mtx       sync.Mutex
	cache     *stsynthesis.Cache
	lastSeen  time.Time
	lastTs    int64
	hasLastTs bool
}

type transformMetrics struct {
	seriesTracked          prometheus.GaugeFunc
	unknownSeriesSplit     prometheus.Counter
	unknownNotSplittable   prometheus.Counter
	stSynthesized          prometheus.Counter
	samplesDropped         prometheus.Counter
	decodeErrors           prometheus.Counter
	concurrentSeriesAccess prometheus.Counter
	outOfOrderSamples      prometheus.Counter
}

// NewTransformer returns a Transformer. Pass a nil registerer to skip metric
// registration.
func NewTransformer(cfg TransformConfig, reg prometheus.Registerer) *Transformer {
	if cfg.StaleAfter <= 0 {
		cfg.StaleAfter = DefaultStaleAfter
	}
	t := &Transformer{
		cfg:   cfg,
		state: map[uint64]*seriesState{},
	}
	t.metrics = &transformMetrics{
		seriesTracked: prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "prwproxy_tracked_series",
			Help: "Number of series the proxy currently keeps start timestamp synthesis state for.",
		}, func() float64 {
			t.mtx.Lock()
			defer t.mtx.Unlock()
			return float64(len(t.state))
		}),
		unknownSeriesSplit: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prwproxy_unknown_series_split_total",
			Help: "Number of untyped series that were split into a gauge and a counter stream.",
		}),
		unknownNotSplittable: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prwproxy_unknown_series_not_splittable_total",
			Help: "Number of untyped series that could not be split (e.g. native histogram samples or missing __name__) and were forwarded as-is.",
		}),
		stSynthesized: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prwproxy_start_timestamps_synthesized_total",
			Help: "Number of samples that got a synthesized start timestamp.",
		}),
		samplesDropped: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prwproxy_samples_dropped_total",
			Help: "Number of samples dropped because they were the first observation of a series and are only used as the synthesis reference point.",
		}),
		decodeErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prwproxy_series_decode_errors_total",
			Help: "Number of series that could not be decoded and were forwarded untouched.",
		}),
		concurrentSeriesAccess: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prwproxy_series_concurrent_access_total",
			Help: "Number of times concurrent requests attempted to synthesize the same series simultaneously, violating the single-writer sequential assumption.",
		}),
		outOfOrderSamples: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prwproxy_samples_out_of_order_total",
			Help: "Number of samples dropped because their timestamp was not strictly greater than the previous sample for the same series.",
		}),
	}
	if reg != nil {
		reg.MustRegister(
			t.metrics.seriesTracked,
			t.metrics.unknownSeriesSplit,
			t.metrics.unknownNotSplittable,
			t.metrics.stSynthesized,
			t.metrics.samplesDropped,
			t.metrics.decodeErrors,
			t.metrics.concurrentSeriesAccess,
			t.metrics.outOfOrderSamples,
		)
	}
	return t
}

// Transform returns a new request with the transformations described on
// Transformer applied. The input request is not modified.
func (t *Transformer) Transform(req *writev2.Request) *writev2.Request {
	var (
		syms = newSymbolAppender(req.Symbols)
		out  = make([]writev2.TimeSeries, 0, len(req.Timeseries))
		b    = labels.NewScratchBuilder(0)
		now  = time.Now()
	)

	for _, ts := range req.Timeseries {
		lset, err := ts.ToLabels(&b, req.Symbols)
		if err != nil {
			t.metrics.decodeErrors.Inc()
			out = append(out, ts)
			continue
		}

		switch ts.Metadata.Type {
		case writev2.Metadata_METRIC_TYPE_UNSPECIFIED:
			if !t.cfg.HandleUnknown {
				out = append(out, ts)
				continue
			}
			out = t.splitUnknown(out, ts, lset, syms, now)

		case writev2.Metadata_METRIC_TYPE_COUNTER, writev2.Metadata_METRIC_TYPE_HISTOGRAM:
			// Avoid allocating (and keeping) per-series state for the common case
			// of a series that already carries a start timestamp.
			if !t.cfg.SynthesizeST || !needsST(ts) {
				out = append(out, ts)
				continue
			}
			ts = t.synthesize(ts, lset.Hash(), now)
			if len(ts.Samples) == 0 && len(ts.Histograms) == 0 {
				// Everything was the first (reference) observation, nothing to send.
				continue
			}
			out = append(out, ts)

		default:
			out = append(out, ts)
		}
	}

	return &writev2.Request{Symbols: syms.symbols, Timeseries: out}
}

// splitUnknown implements the equivalent of the OpenTelemetry Collector recipe:
//
//	copy_metric(...)                        where metric.type == "unknown"
//	convert_gauge_to_sum("cumulative", true) on the copy
//
// The gauge stream keeps raw values, the counter stream is re-based against a
// synthesized start timestamp so it is a valid Monarch cumulative.
func (t *Transformer) splitUnknown(out []writev2.TimeSeries, ts writev2.TimeSeries, lset labels.Labels, syms *symbolAppender, now time.Time) []writev2.TimeSeries {
	name := lset.Get(labels.MetricName)
	// Native histograms are never untyped in practice, and there is no meaningful
	// "gauge" representation for them here, so don't touch them.
	if name == "" || len(ts.Histograms) > 0 {
		t.metrics.unknownNotSplittable.Inc()
		return append(out, ts)
	}
	t.metrics.unknownSeriesSplit.Inc()

	// 1. Gauge stream: raw values, no ST.
	gauge := cloneSeries(ts)
	gauge.Metadata.Type = writev2.Metadata_METRIC_TYPE_GAUGE
	// Exemplars belong to the cumulative stream, drop them here to avoid
	// double-reporting them to the backend.
	gauge.Exemplars = nil
	for i := range gauge.Samples {
		gauge.Samples[i].StartTimestamp = 0
	}
	renameSeries(&gauge, syms, name+t.cfg.UnknownGaugeSuffix)
	out = append(out, gauge)

	// 2. Cumulative counter stream, with a synthesized ST.
	counter := cloneSeries(ts)
	counter.Metadata.Type = writev2.Metadata_METRIC_TYPE_COUNTER
	renameSeries(&counter, syms, name+t.cfg.UnknownCounterSuffix)
	counter = t.synthesize(counter, lset.Hash(), now)
	if len(counter.Samples) > 0 {
		out = append(out, counter)
	}
	return out
}

// needsST reports whether any sample of ts is missing a start timestamp.
func needsST(ts writev2.TimeSeries) bool {
	for _, s := range ts.Samples {
		if s.StartTimestamp == 0 {
			return true
		}
	}
	for _, h := range ts.Histograms {
		if h.StartTimestamp == 0 {
			return true
		}
	}
	return false
}

// synthesize fills in start timestamps for all samples/histograms of ts that
// don't have one, re-basing their values against the first observed sample.
// Samples that only established the reference point or arrived out of order are dropped.
func (t *Transformer) synthesize(ts writev2.TimeSeries, key uint64, now time.Time) writev2.TimeSeries {
	st := t.stateFor(key, now)
	if !st.mtx.TryLock() {
		t.metrics.concurrentSeriesAccess.Inc()
		st.mtx.Lock()
	}
	defer st.mtx.Unlock()

	if len(ts.Samples) > 0 {
		kept := make([]writev2.Sample, 0, len(ts.Samples))
		for _, s := range ts.Samples {
			if s.StartTimestamp != 0 {
				kept = append(kept, s)
				continue
			}
			if st.hasLastTs && s.Timestamp <= st.lastTs {
				t.metrics.outOfOrderSamples.Inc()
				continue
			}
			st.lastTs = s.Timestamp
			st.hasLastTs = true

			v, start, skip := st.cache.SynthesizeFloat(s.Value, s.Timestamp)
			if skip {
				t.metrics.samplesDropped.Inc()
				continue
			}
			s.Value = v
			s.StartTimestamp = start
			t.metrics.stSynthesized.Inc()
			kept = append(kept, s)
		}
		ts.Samples = kept
	}

	if len(ts.Histograms) > 0 {
		kept := make([]writev2.Histogram, 0, len(ts.Histograms))
		for _, h := range ts.Histograms {
			if h.StartTimestamp != 0 {
				kept = append(kept, h)
				continue
			}
			if st.hasLastTs && h.Timestamp <= st.lastTs {
				t.metrics.outOfOrderSamples.Inc()
				continue
			}
			st.lastTs = h.Timestamp
			st.hasLastTs = true

			adjusted, skip := synthesizeHistogram(st.cache, h)
			if skip {
				t.metrics.samplesDropped.Inc()
				continue
			}
			t.metrics.stSynthesized.Inc()
			kept = append(kept, adjusted)
		}
		ts.Histograms = kept
	}
	return ts
}

func synthesizeHistogram(c *stsynthesis.Cache, h writev2.Histogram) (writev2.Histogram, bool) {
	if h.IsFloatHistogram() {
		fh, start, skip := c.SynthesizeFloatHistogram(h.ToFloatHistogram(), h.Timestamp)
		if skip {
			return h, true
		}
		out := writev2.FromFloatHistogram(h.Timestamp, fh)
		out.StartTimestamp = start
		out.CustomValues = h.CustomValues
		return out, false
	}
	ih, start, skip := c.SynthesizeHistogram(h.ToIntHistogram(), h.Timestamp)
	if skip {
		return h, true
	}
	out := writev2.FromIntHistogram(h.Timestamp, ih)
	out.StartTimestamp = start
	out.CustomValues = h.CustomValues
	return out, false
}

// stateFor returns (and creates if needed) the synthesis state for a series.
//
// TODO(bwplotka): The key is a labels hash, so a (very unlikely) collision makes
// two series share synthesis state. Before this goes beyond a prototype we
// should either store the label set or use a collision-resistant key.
func (t *Transformer) stateFor(key uint64, now time.Time) *seriesState {
	t.mtx.Lock()
	defer t.mtx.Unlock()

	s, ok := t.state[key]
	if !ok {
		s = &seriesState{cache: &stsynthesis.Cache{}}
		t.state[key] = s
	}
	s.lastSeen = now
	return s
}

// GarbageCollect drops synthesis state for series not seen for StaleAfter. It
// returns the number of dropped entries.
func (t *Transformer) GarbageCollect(now time.Time) int {
	t.mtx.Lock()
	defer t.mtx.Unlock()

	var dropped int
	for k, s := range t.state {
		if now.Sub(s.lastSeen) > t.cfg.StaleAfter {
			delete(t.state, k)
			dropped++
		}
	}
	return dropped
}

// cloneSeries deep copies the parts of ts that the transformations mutate.
func cloneSeries(ts writev2.TimeSeries) writev2.TimeSeries {
	out := ts
	out.LabelsRefs = append([]uint32(nil), ts.LabelsRefs...)
	out.Samples = append([]writev2.Sample(nil), ts.Samples...)
	out.Histograms = append([]writev2.Histogram(nil), ts.Histograms...)
	return out
}

// renameSeries points the __name__ label of ts at newName.
func renameSeries(ts *writev2.TimeSeries, syms *symbolAppender, newName string) {
	for i := 0; i+1 < len(ts.LabelsRefs); i += 2 {
		if syms.symbols[ts.LabelsRefs[i]] != labels.MetricName {
			continue
		}
		if syms.symbols[ts.LabelsRefs[i+1]] == newName {
			return
		}
		ts.LabelsRefs[i+1] = syms.ref(newName)
		return
	}
}

// symbolAppender lets us add new symbols to an existing PRW2 symbols table
// without invalidating any of the existing references (they are plain indices,
// so appending is always safe).
type symbolAppender struct {
	symbols []string
	index   map[string]uint32
}

func newSymbolAppender(symbols []string) *symbolAppender {
	s := &symbolAppender{
		symbols: symbols,
		index:   make(map[string]uint32, len(symbols)),
	}
	for i, sym := range symbols {
		if _, ok := s.index[sym]; !ok {
			s.index[sym] = uint32(i)
		}
	}
	return s
}

func (s *symbolAppender) ref(str string) uint32 {
	if ref, ok := s.index[str]; ok {
		return ref
	}
	ref := uint32(len(s.symbols))
	s.symbols = append(s.symbols, str)
	s.index[str] = ref
	return ref
}
