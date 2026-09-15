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
)

// Config configures the Proxy.
type Config struct {
	// ForwardURL is the downstream PRW2 endpoint, e.g.
	// https://monitoring.googleapis.com/v1/projects/$PROJECT_ID/location/global/prometheus/api/v1/write
	ForwardURL string
	// MaxBodySize limits the accepted compressed request body size.
	MaxBodySize int64

	Transform TransformConfig
}

// Proxy is an http.Handler accepting Prometheus Remote Write 2.0 requests,
// transforming them (see Transformer) and forwarding them upstream.
type Proxy struct {
	cfg         Config
	logger      *slog.Logger
	client      *http.Client
	transformer *Transformer

	requests *prometheus.CounterVec
	duration prometheus.Histogram
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
		duration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "prwproxy_request_duration_seconds",
			Help:    "Latency of handling (transform + forward) a remote write request.",
			Buckets: prometheus.DefBuckets,
		}),
	}
	if reg != nil {
		reg.MustRegister(p.requests, p.duration)
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

	out := p.transformer.Transform(&req)

	outRaw, err := out.OptimizedMarshal(nil)
	if err != nil {
		return "marshal_error", http.StatusInternalServerError, fmt.Errorf("marshalling PRW2 request: %w", err)
	}

	resp, err := p.forward(r.Context(), snappy.Encode(nil, outRaw))
	if err != nil {
		return "forward_error", http.StatusBadGateway, fmt.Errorf("forwarding to %v: %w", p.cfg.ForwardURL, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	// Pass the downstream answer (including the PRW2 written-stats headers and
	// any retriable status code) straight back to Prometheus, so its remote
	// write queue keeps its usual retry/backoff behaviour.
	//
	// NOTE(bwplotka): The written-stats refer to the transformed request, so
	// they can exceed what Prometheus sent us. Prometheus only logs them.
	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		_, _ = w.Write(body)
		return "forward_status_" + strconv.Itoa(resp.StatusCode), resp.StatusCode, nil
	}
	return "success", http.StatusOK, nil
}

func (p *Proxy) forward(ctx context.Context, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.ForwardURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set(contentTypeHeader, prw2ContentType)
	req.Header.Set(contentEncodingHeader, "snappy")
	req.Header.Set(rwVersionHeader, rwVersion2)
	req.Header.Set("User-Agent", "prwproxy")
	return p.client.Do(req)
}

func isPRW2(contentType string) bool {
	// Content types are compared ignoring optional whitespace, e.g.
	// "application/x-protobuf; proto=io.prometheus.write.v2.Request".
	return strings.EqualFold(strings.ReplaceAll(contentType, " ", ""), prw2ContentType)
}
