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

// Package prwproxy implements a minimal, stateful Prometheus Remote Write 2.0
// proxy that adapts vanilla OSS Prometheus output to the stricter Google Cloud
// Monitoring (Monarch) data model.
//
// It is meant to run as a sidecar next to Prometheus:
//
//	prometheus --(PRW2)--> prwproxy --(PRW2)--> GCM PRW2 endpoint
//
// See go/gmp:unknown-types (b/551762955) for the design.
package prwproxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang/snappy"
	"github.com/prometheus/client_golang/prometheus"

	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
)

const (
	contentTypeHeader     = "Content-Type"
	contentEncodingHeader = "Content-Encoding"
	rwVersionHeader       = "X-Prometheus-Remote-Write-Version"

	prw2ContentType = "application/x-protobuf;proto=io.prometheus.write.v2.Request"
	rwVersion2      = "2.0.0"

	// DefaultMaxBodySize is the maximum accepted (compressed) request body.
	DefaultMaxBodySize = 32 << 20 // 32 MiB.
	// DefaultMaxSeriesPerRequest is the maximum number of time series GCM
	// accepts in a single write request.
	DefaultMaxSeriesPerRequest = 200
)

// writtenHeaders are the PRW2 response headers reporting how much of a request
// was written, see
// https://prometheus.io/docs/specs/prw/remote_write_spec_2_0/#required-written-response-headers.
var writtenHeaders = [...]string{
	"X-Prometheus-Remote-Write-Samples-Written",
	"X-Prometheus-Remote-Write-Histograms-Written",
	"X-Prometheus-Remote-Write-Exemplars-Written",
}

// Config configures the Proxy.
type Config struct {
	// ForwardURL is the downstream PRW2 endpoint, e.g.
	// https://monitoring.googleapis.com/v1/projects/$PROJECT_ID/location/global/prometheus/api/v1/write
	ForwardURL string
	// MaxBodySize limits the accepted compressed request body size.
	MaxBodySize int64
	// MaxSeriesPerRequest limits the number of time series in a single
	// forwarded request. Transformed requests with more series (splitting
	// untyped series can double their number) are forwarded as multiple
	// requests.
	MaxSeriesPerRequest int

	Transform TransformConfig
}

// Proxy is an http.Handler accepting Prometheus Remote Write 2.0 requests,
// transforming them (see Transformer) and forwarding them upstream.
type Proxy struct {
	cfg         Config
	logger      *slog.Logger
	client      *http.Client
	transformer *Transformer

	requests      *prometheus.CounterVec
	splitRequests prometheus.Counter
	duration      prometheus.Histogram
}

// New returns a ready to use Proxy. The passed client is used for the
// downstream requests and is expected to carry any required authentication
// (see storage/remote/googleiam).
func New(cfg Config, client *http.Client, logger *slog.Logger, reg prometheus.Registerer) (*Proxy, error) {
	if cfg.ForwardURL == "" {
		return nil, errors.New("forward URL must be set")
	}
	if cfg.MaxBodySize <= 0 {
		cfg.MaxBodySize = DefaultMaxBodySize
	}
	if cfg.MaxSeriesPerRequest <= 0 {
		cfg.MaxSeriesPerRequest = DefaultMaxSeriesPerRequest
	}
	if client == nil {
		client = http.DefaultClient
	}
	if logger == nil {
		logger = slog.Default()
	}

	p := &Proxy{
		cfg:         cfg,
		logger:      logger,
		client:      client,
		transformer: NewTransformer(cfg.Transform, reg),
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "prwproxy_requests_total",
			Help: "Number of remote write requests handled, by outcome.",
		}, []string{"result"}),
		splitRequests: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prwproxy_requests_split_total",
			Help: "Number of remote write requests that had more series than the per request maximum after transformation and were forwarded as multiple requests.",
		}),
		duration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "prwproxy_request_duration_seconds",
			Help:    "Latency of handling (transform + forward) a remote write request.",
			Buckets: prometheus.DefBuckets,
		}),
	}
	if reg != nil {
		reg.MustRegister(p.requests, p.splitRequests, p.duration)
	}
	return p, nil
}

// Transformer exposes the underlying transformer, mostly to run
// GarbageCollect.
func (p *Proxy) Transformer() *Transformer { return p.transformer }

// Run garbage collects stale per-series state until ctx is done.
func (p *Proxy) Run(ctx context.Context) {
	interval := p.cfg.Transform.StaleAfter / 2
	if interval <= 0 {
		interval = DefaultStaleAfter / 2
	}
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			if dropped := p.transformer.GarbageCollect(now); dropped > 0 {
				p.logger.Debug("garbage collected stale series state", "dropped", dropped)
			}
		}
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	result, code, err := p.serve(w, r)
	p.requests.WithLabelValues(result).Inc()
	p.duration.Observe(time.Since(start).Seconds())
	if err != nil {
		p.logger.Error("handling remote write request", "err", err, "result", result)
		http.Error(w, err.Error(), code)
	}
}

func (p *Proxy) serve(w http.ResponseWriter, r *http.Request) (result string, code int, _ error) {
	if r.Method != http.MethodPost {
		return "method_not_allowed", http.StatusMethodNotAllowed, fmt.Errorf("expected POST, got %v", r.Method)
	}
	// We only understand PRW2; PRW1 has no metadata or start timestamps, so
	// there is nothing useful this proxy could do with it.
	if ct := r.Header.Get(contentTypeHeader); !isPRW2(ct) {
		return "unsupported_content_type", http.StatusUnsupportedMediaType,
			fmt.Errorf("expected content type %q, got %q; configure Prometheus with protobuf_message: io.prometheus.write.v2.Request", prw2ContentType, ct)
	}
	if enc := r.Header.Get(contentEncodingHeader); enc != "" && enc != "snappy" {
		return "unsupported_encoding", http.StatusUnsupportedMediaType, fmt.Errorf("expected snappy encoding, got %q", enc)
	}

	compressed, err := io.ReadAll(io.LimitReader(r.Body, p.cfg.MaxBodySize))
	if err != nil {
		return "read_error", http.StatusBadRequest, fmt.Errorf("reading body: %w", err)
	}
	raw, err := snappy.Decode(nil, compressed)
	if err != nil {
		return "decompress_error", http.StatusBadRequest, fmt.Errorf("snappy decoding body: %w", err)
	}
	var req writev2.Request
	if err := req.Unmarshal(raw); err != nil {
		return "unmarshal_error", http.StatusBadRequest, fmt.Errorf("unmarshalling PRW2 request: %w", err)
	}

	out, commits := p.transformer.transform(&req)

	// Transform can grow a request past the downstream limit, so forward it in
	// multiple parts if needed.
	outs, err := splitRequest(out, p.cfg.MaxSeriesPerRequest)
	if err != nil {
		return "split_error", http.StatusBadRequest, fmt.Errorf("splitting PRW2 request: %w", err)
	}
	if len(outs) > 1 {
		p.splitRequests.Inc()
	}
	chunks := make([]forwardChunk, 0, len(outs))
	var endIdx int
	for i, o := range outs {
		outRaw, err := o.OptimizedMarshal(nil)
		if err != nil {
			return "marshal_error", http.StatusInternalServerError, fmt.Errorf("marshalling PRW2 request: %w", err)
		}
		endIdx += len(o.Timeseries)
		var c []seriesCommit
		if i == len(outs)-1 {
			c = commits
		} else {
			n := 0
			for n < len(commits) && commits[n].outIdx < endIdx {
				n++
			}
			c, commits = commits[:n], commits[n:]
		}
		chunks = append(chunks, forwardChunk{
			body:    snappy.Encode(nil, outRaw),
			commits: c,
		})
	}

	resp, err := p.forwardAll(r.Context(), chunks)
	if err != nil {
		return "forward_error", http.StatusBadGateway, fmt.Errorf("forwarding to %v: %w", p.cfg.ForwardURL, err)
	}

	// Pass the downstream answer (including the PRW2 written-stats headers and
	// any retriable status code) straight back to Prometheus, so its remote
	// write queue keeps its usual retry/backoff behaviour.
	//
	// NOTE(bwplotka): The written-stats refer to the transformed request, so
	// they can exceed what Prometheus sent us. Prometheus only logs them.
	for k, vs := range resp.header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.code)
	if resp.code/100 != 2 {
		_, _ = w.Write(resp.body)
		return "forward_status_" + strconv.Itoa(resp.code), resp.code, nil
	}
	return "success", http.StatusOK, nil
}

// forwardChunk is an encoded downstream request together with the synthesis
// state updates that should be committed once it has been accepted (or
// rejected as non-retriable).
type forwardChunk struct {
	body    []byte
	commits []seriesCommit
}

// forwardResponse is what the proxy keeps from a downstream response.
type forwardResponse struct {
	code   int
	header http.Header
	// body is only read for non-2xx responses.
	body []byte
}

// forwardAll forwards the encoded requests one after another, so samples of a
// series spread across them keep arriving in timestamp order. Per-series
// synthesis state for a chunk is committed only when the chunk succeeds (2xx)
// or fails with a non-retriable status, so a Prometheus retry after a 5xx, 429
// or transport error re-synthesizes any uncommitted chunks while skipping
// samples that were already committed.
//
// It returns the response to pass back to Prometheus, which retries or drops
// the whole batch based on it:
//
//   - The first failure asking to retry later (5xx or 429). The remaining
//     requests are not sent, as Prometheus retries the whole batch (429 only
//     with retry_on_http_429) and more load won't help the backend recover.
//   - Otherwise the first non-retriable failure. The remaining requests are
//     still sent, so one bad series doesn't drop unrelated data.
//   - Otherwise the last response.
//
// The PRW2 written-stats headers of the returned response are replaced with
// their sums over all downstream responses.
func (p *Proxy) forwardAll(ctx context.Context, chunks []forwardChunk) (forwardResponse, error) {
	var (
		ret     forwardResponse
		failed  bool
		written writtenStats
	)
	for _, chunk := range chunks {
		resp, err := p.forward(ctx, chunk.body)
		if err != nil {
			return forwardResponse{}, err
		}
		written.add(resp.header)

		switch {
		case resp.code/100 == 2:
			p.transformer.commit(chunk.commits)
			if !failed {
				ret = resp
			}
		case isRetriable(resp.code):
			written.set(resp.header)
			return resp, nil
		default:
			p.transformer.commit(chunk.commits)
			if !failed {
				ret, failed = resp, true
			}
		}
	}
	written.set(ret.header)
	return ret, nil
}

func (p *Proxy) forward(ctx context.Context, body []byte) (forwardResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.ForwardURL, bytes.NewReader(body))
	if err != nil {
		return forwardResponse{}, err
	}
	req.Header.Set(contentTypeHeader, prw2ContentType)
	req.Header.Set(contentEncodingHeader, "snappy")
	req.Header.Set(rwVersionHeader, rwVersion2)
	req.Header.Set("User-Agent", "prwproxy")

	resp, err := p.client.Do(req)
	if err != nil {
		return forwardResponse{}, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	ret := forwardResponse{code: resp.StatusCode, header: resp.Header}
	if resp.StatusCode/100 != 2 {
		ret.body, _ = io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	}
	return ret, nil
}

// isRetriable reports whether a failed remote write request should be retried
// later.
func isRetriable(code int) bool {
	return code/100 == 5 || code == http.StatusTooManyRequests
}

// writtenStats sums the PRW2 written-stats headers over multiple responses.
type writtenStats struct {
	sums [len(writtenHeaders)]int
	seen [len(writtenHeaders)]bool
}

func (s *writtenStats) add(h http.Header) {
	for i, name := range writtenHeaders {
		v := h.Get(name)
		if v == "" {
			continue
		}
		s.seen[i] = true
		if n, err := strconv.Atoi(v); err == nil {
			s.sums[i] += n
		}
	}
}

// set sets the sums in h. Headers no response had are left unset, so a single
// response is passed through unchanged.
func (s *writtenStats) set(h http.Header) {
	for i, name := range writtenHeaders {
		if s.seen[i] {
			h.Set(name, strconv.Itoa(s.sums[i]))
		}
	}
}

func isPRW2(contentType string) bool {
	// Content types are compared ignoring optional whitespace, e.g.
	// "application/x-protobuf; proto=io.prometheus.write.v2.Request".
	return strings.EqualFold(strings.ReplaceAll(contentType, " ", ""), prw2ContentType)
}
