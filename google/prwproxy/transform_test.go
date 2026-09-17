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
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/prometheus/prometheus/model/labels"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
)

// series is a decoded, human readable view of a writev2.TimeSeries, used to
// keep the test expectations readable.
type series struct {
	name    string
	typ     writev2.Metadata_MetricType
	samples []writev2.Sample
}

func request(t *testing.T, in ...series) *writev2.Request {
	t.Helper()

	syms := writev2.NewSymbolTable()
	req := &writev2.Request{}
	for _, s := range in {
		lset := labels.FromStrings(labels.MetricName, s.name, "job", "test")
		req.Timeseries = append(req.Timeseries, writev2.TimeSeries{
			LabelsRefs: syms.SymbolizeLabels(lset, nil),
			Samples:    s.samples,
			Metadata:   writev2.Metadata{Type: s.typ},
		})
	}
	req.Symbols = syms.Symbols()
	return req
}

func decode(t *testing.T, req *writev2.Request) []series {
	t.Helper()

	b := labels.NewScratchBuilder(0)
	out := make([]series, 0, len(req.Timeseries))
	for _, ts := range req.Timeseries {
		lset, err := ts.ToLabels(&b, req.Symbols)
		require.NoError(t, err)
		out = append(out, series{
			name:    lset.Get(labels.MetricName),
			typ:     ts.Metadata.Type,
			samples: ts.Samples,
		})
	}
	return out
}

func testTransformer(t *testing.T) *Transformer {
	t.Helper()
	return NewTransformer(TransformConfig{
		HandleUnknown:        true,
		UnknownGaugeSuffix:   DefaultUnknownGaugeSuffix,
		UnknownCounterSuffix: DefaultUnknownCounterSuffix,
		SynthesizeST:         true,
	}, nil)
}

func TestTransform_UnknownSplit(t *testing.T) {
	tr := testTransformer(t)

	// First scrape: the counter stream only establishes the reference point, so
	// only the gauge stream is emitted.
	got := decode(t, tr.Transform(request(t, series{
		name:    "mysql_slow_queries",
		typ:     writev2.Metadata_METRIC_TYPE_UNSPECIFIED,
		samples: []writev2.Sample{{Value: 10, Timestamp: 1000}},
	})))
	require.Equal(t, []series{
		{
			name:    "mysql_slow_queries/unknown",
			typ:     writev2.Metadata_METRIC_TYPE_GAUGE,
			samples: []writev2.Sample{{Value: 10, Timestamp: 1000}},
		},
	}, got)

	// Second scrape: both streams, the counter re-based against ST=1000.
	got = decode(t, tr.Transform(request(t, series{
		name:    "mysql_slow_queries",
		typ:     writev2.Metadata_METRIC_TYPE_UNSPECIFIED,
		samples: []writev2.Sample{{Value: 15, Timestamp: 2000}},
	})))
	require.Equal(t, []series{
		{
			name:    "mysql_slow_queries/unknown",
			typ:     writev2.Metadata_METRIC_TYPE_GAUGE,
			samples: []writev2.Sample{{Value: 15, Timestamp: 2000}},
		},
		{
			name:    "mysql_slow_queries/unknown:counter",
			typ:     writev2.Metadata_METRIC_TYPE_COUNTER,
			samples: []writev2.Sample{{Value: 5, Timestamp: 2000, StartTimestamp: 1000}},
		},
	}, got)

	// Third scrape resets the underlying value; the counter stream must restart
	// from a fresh ST, the gauge stream is unaffected.
	got = decode(t, tr.Transform(request(t, series{
		name:    "mysql_slow_queries",
		typ:     writev2.Metadata_METRIC_TYPE_UNSPECIFIED,
		samples: []writev2.Sample{{Value: 2, Timestamp: 3000}},
	})))
	require.Equal(t, []series{
		{
			name:    "mysql_slow_queries/unknown",
			typ:     writev2.Metadata_METRIC_TYPE_GAUGE,
			samples: []writev2.Sample{{Value: 2, Timestamp: 3000}},
		},
		{
			name:    "mysql_slow_queries/unknown:counter",
			typ:     writev2.Metadata_METRIC_TYPE_COUNTER,
			samples: []writev2.Sample{{Value: 2, Timestamp: 3000, StartTimestamp: 2999}},
		},
	}, got)
}

func TestTransform_UnknownSplitDisabled(t *testing.T) {
	tr := NewTransformer(TransformConfig{SynthesizeST: true}, nil)

	in := request(t, series{
		name:    "mysql_slow_queries",
		typ:     writev2.Metadata_METRIC_TYPE_UNSPECIFIED,
		samples: []writev2.Sample{{Value: 10, Timestamp: 1000}},
	})
	require.Equal(t, decode(t, in), decode(t, tr.Transform(in)))
}

func TestTransform_NoSuffixes(t *testing.T) {
	// With empty suffixes both streams keep the original name and only differ by
	// type, which is what the OTel `transform` processor recipe produces and what
	// a receiver that derives the suffix from the type itself expects.
	tr := NewTransformer(TransformConfig{HandleUnknown: true, SynthesizeST: true}, nil)

	_ = tr.Transform(request(t, series{
		name:    "up_ish",
		typ:     writev2.Metadata_METRIC_TYPE_UNSPECIFIED,
		samples: []writev2.Sample{{Value: 1, Timestamp: 1000}},
	}))
	got := decode(t, tr.Transform(request(t, series{
		name:    "up_ish",
		typ:     writev2.Metadata_METRIC_TYPE_UNSPECIFIED,
		samples: []writev2.Sample{{Value: 3, Timestamp: 2000}},
	})))
	require.Equal(t, []series{
		{name: "up_ish", typ: writev2.Metadata_METRIC_TYPE_GAUGE, samples: []writev2.Sample{{Value: 3, Timestamp: 2000}}},
		{name: "up_ish", typ: writev2.Metadata_METRIC_TYPE_COUNTER, samples: []writev2.Sample{{Value: 2, Timestamp: 2000, StartTimestamp: 1000}}},
	}, got)
}

func TestTransform_CounterMissingST(t *testing.T) {
	tr := testTransformer(t)

	// First sample is only a reference point, so the series is dropped entirely.
	got := decode(t, tr.Transform(request(t, series{
		name:    "http_requests_total",
		typ:     writev2.Metadata_METRIC_TYPE_COUNTER,
		samples: []writev2.Sample{{Value: 100, Timestamp: 1000}},
	})))
	require.Empty(t, got)

	got = decode(t, tr.Transform(request(t, series{
		name:    "http_requests_total",
		typ:     writev2.Metadata_METRIC_TYPE_COUNTER,
		samples: []writev2.Sample{{Value: 130, Timestamp: 2000}},
	})))
	require.Equal(t, []series{
		{
			name:    "http_requests_total",
			typ:     writev2.Metadata_METRIC_TYPE_COUNTER,
			samples: []writev2.Sample{{Value: 30, Timestamp: 2000, StartTimestamp: 1000}},
		},
	}, got)
}

func TestTransform_PassThrough(t *testing.T) {
	tr := testTransformer(t)

	in := request(t,
		// Counter that already carries an ST must not be touched.
		series{
			name:    "http_requests_total",
			typ:     writev2.Metadata_METRIC_TYPE_COUNTER,
			samples: []writev2.Sample{{Value: 100, Timestamp: 2000, StartTimestamp: 500}},
		},
		series{
			name:    "go_goroutines",
			typ:     writev2.Metadata_METRIC_TYPE_GAUGE,
			samples: []writev2.Sample{{Value: 42, Timestamp: 2000}},
		},
	)
	require.Equal(t, decode(t, in), decode(t, tr.Transform(in)))
}

func TestTransform_DoesNotMutateInput(t *testing.T) {
	tr := testTransformer(t)

	in := request(t, series{
		name:    "mysql_slow_queries",
		typ:     writev2.Metadata_METRIC_TYPE_UNSPECIFIED,
		samples: []writev2.Sample{{Value: 10, Timestamp: 1000}},
	})
	before := decode(t, in)
	_ = tr.Transform(in)
	_ = tr.Transform(in)
	require.Equal(t, before, decode(t, in))
}

func TestTransform_OutOfOrderAndDuplicateTimestamps(t *testing.T) {
	tr := testTransformer(t)

	// 1. Anchor sample at t=2000, v=100 (dropped to establish ST reference).
	got := decode(t, tr.Transform(request(t, series{
		name:    "http_requests_total",
		typ:     writev2.Metadata_METRIC_TYPE_COUNTER,
		samples: []writev2.Sample{{Value: 100, Timestamp: 2000}},
	})))
	require.Empty(t, got)
	require.Equal(t, 0.0, testutil.ToFloat64(tr.metrics.outOfOrderSamples))

	// 2. Out-of-order sample at t=1500, v=90.
	// Without ordering validation, v=90 < 100 would falsely trigger a counter
	// reset and corrupt the start timestamp state. With validation, it is dropped.
	got = decode(t, tr.Transform(request(t, series{
		name:    "http_requests_total",
		typ:     writev2.Metadata_METRIC_TYPE_COUNTER,
		samples: []writev2.Sample{{Value: 90, Timestamp: 1500}},
	})))
	require.Empty(t, got)
	require.Equal(t, 1.0, testutil.ToFloat64(tr.metrics.outOfOrderSamples))

	// 3. Duplicate timestamp sample at t=2000, v=105 (also dropped).
	got = decode(t, tr.Transform(request(t, series{
		name:    "http_requests_total",
		typ:     writev2.Metadata_METRIC_TYPE_COUNTER,
		samples: []writev2.Sample{{Value: 105, Timestamp: 2000}},
	})))
	require.Empty(t, got)
	require.Equal(t, 2.0, testutil.ToFloat64(tr.metrics.outOfOrderSamples))

	// 4. Subsequent valid in-order sample at t=3000, v=125.
	// Must be re-based against the uncorrupted original anchor (v=100, ST=2000).
	got = decode(t, tr.Transform(request(t, series{
		name:    "http_requests_total",
		typ:     writev2.Metadata_METRIC_TYPE_COUNTER,
		samples: []writev2.Sample{{Value: 125, Timestamp: 3000}},
	})))
	require.Equal(t, []series{
		{
			name:    "http_requests_total",
			typ:     writev2.Metadata_METRIC_TYPE_COUNTER,
			samples: []writev2.Sample{{Value: 25, Timestamp: 3000, StartTimestamp: 2000}},
		},
	}, got)
	require.Equal(t, 2.0, testutil.ToFloat64(tr.metrics.outOfOrderSamples))
}

func TestTransform_ConcurrentAccessValidation(t *testing.T) {
	tr := testTransformer(t)

	// Establish series state and hold its per-series mutex to simulate an
	// in-flight request for the same series.
	lset := labels.FromStrings(labels.MetricName, "http_requests_total", "job", "test")
	st := tr.stateFor(lset.Hash(), time.Now())
	st.mtx.Lock()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = tr.Transform(request(t, series{
			name:    "http_requests_total",
			typ:     writev2.Metadata_METRIC_TYPE_COUNTER,
			samples: []writev2.Sample{{Value: 100, Timestamp: 1000}},
		}))
	}()

	// Give the goroutine a moment to hit TryLock() and block on Lock(),
	// then release the lock so it completes cleanly.
	time.Sleep(20 * time.Millisecond)
	st.mtx.Unlock()
	wg.Wait()

	require.Equal(t, 1.0, testutil.ToFloat64(tr.metrics.concurrentSeriesAccess))
}
