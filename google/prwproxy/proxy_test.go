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
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/golang/snappy"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
)

// testBackend is a fake PRW2 receiver capturing what the proxy forwarded.
type testBackend struct {
	*httptest.Server

	// status is returned for requests not covered by statuses.
	status int
	// statuses are returned for the first len(statuses) requests, in order.
	statuses []int
	// maxSeries, if positive, makes the backend reject requests with more
	// series as a whole, like GCM does.
	maxSeries int

	received []*writev2.Request
	headers  []http.Header
}

func newTestBackend(t *testing.T) *testBackend {
	t.Helper()

	b := &testBackend{status: http.StatusNoContent}
	b.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		compressed, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		raw, err := snappy.Decode(nil, compressed)
		require.NoError(t, err)

		req := &writev2.Request{}
		require.NoError(t, req.Unmarshal(raw))

		n := len(b.received)
		b.received = append(b.received, req)
		b.headers = append(b.headers, r.Header.Clone())

		if b.maxSeries > 0 && len(req.Timeseries) > b.maxSeries {
			http.Error(w, fmt.Sprintf("Request has %d timeseries. Maximum allowed is %d.", len(req.Timeseries), b.maxSeries), http.StatusBadRequest)
			return
		}
		status := b.status
		if n < len(b.statuses) {
			status = b.statuses[n]
		}
		if status/100 != 2 {
			http.Error(w, fmt.Sprintf("request %d failed", n), status)
			return
		}

		var samples, histograms, exemplars int
		for _, ts := range req.Timeseries {
			samples += len(ts.Samples)
			histograms += len(ts.Histograms)
			exemplars += len(ts.Exemplars)
		}
		w.Header().Set("X-Prometheus-Remote-Write-Samples-Written", strconv.Itoa(samples))
		w.Header().Set("X-Prometheus-Remote-Write-Histograms-Written", strconv.Itoa(histograms))
		w.Header().Set("X-Prometheus-Remote-Write-Exemplars-Written", strconv.Itoa(exemplars))
		w.WriteHeader(status)
	}))
	t.Cleanup(b.Close)
	return b
}

func post(t *testing.T, p *Proxy, req *writev2.Request) *httptest.ResponseRecorder {
	t.Helper()

	raw, err := req.OptimizedMarshal(nil)
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodPost, "/api/v1/write", bytes.NewReader(snappy.Encode(nil, raw)))
	r.Header.Set(contentTypeHeader, prw2ContentType)
	r.Header.Set(contentEncodingHeader, "snappy")
	r.Header.Set(rwVersionHeader, rwVersion2)

	w := httptest.NewRecorder()
	p.ServeHTTP(w, r)
	return w
}

func newTestProxy(t *testing.T, forwardURL string, opts ...func(*Config)) *Proxy {
	t.Helper()

	cfg := Config{
		ForwardURL: forwardURL,
		Transform: TransformConfig{
			HandleUnknown:        true,
			UnknownGaugeSuffix:   DefaultUnknownGaugeSuffix,
			UnknownCounterSuffix: DefaultUnknownCounterSuffix,
			SynthesizeST:         true,
		},
	}
	for _, o := range opts {
		o(&cfg)
	}
	p, err := New(cfg, nil, nil, nil)
	require.NoError(t, err)
	return p
}

func TestProxy_TransformsAndForwards(t *testing.T) {
	backend := newTestBackend(t)
	p := newTestProxy(t, backend.URL)

	unknown := func(v float64, ts int64) *writev2.Request {
		return request(t, series{
			name:    "rabbitmq_queue_messages",
			typ:     writev2.Metadata_METRIC_TYPE_UNSPECIFIED,
			samples: []writev2.Sample{{Value: v, Timestamp: ts}},
		})
	}

	require.Equal(t, http.StatusNoContent, post(t, p, unknown(10, 1000)).Code)
	w := post(t, p, unknown(15, 2000))
	require.Equal(t, http.StatusNoContent, w.Code)
	// Downstream response headers are proxied back to Prometheus. The written
	// stats count the transformed series, here a gauge and a counter sample.
	require.Equal(t, "2", w.Header().Get("X-Prometheus-Remote-Write-Samples-Written"))

	require.Len(t, backend.received, 2)
	for _, h := range backend.headers {
		require.Equal(t, prw2ContentType, h.Get(contentTypeHeader))
		require.Equal(t, "snappy", h.Get(contentEncodingHeader))
		require.Equal(t, rwVersion2, h.Get(rwVersionHeader))
	}

	require.Equal(t, []series{
		{
			name:    "rabbitmq_queue_messages/unknown",
			typ:     writev2.Metadata_METRIC_TYPE_GAUGE,
			samples: []writev2.Sample{{Value: 10, Timestamp: 1000}},
		},
	}, decode(t, backend.received[0]))

	require.Equal(t, []series{
		{
			name:    "rabbitmq_queue_messages/unknown",
			typ:     writev2.Metadata_METRIC_TYPE_GAUGE,
			samples: []writev2.Sample{{Value: 15, Timestamp: 2000}},
		},
		{
			name:    "rabbitmq_queue_messages/unknown:counter",
			typ:     writev2.Metadata_METRIC_TYPE_COUNTER,
			samples: []writev2.Sample{{Value: 5, Timestamp: 2000, StartTimestamp: 1000}},
		},
	}, decode(t, backend.received[1]))
}

func TestProxy_PropagatesDownstreamFailure(t *testing.T) {
	backend := newTestBackend(t)
	backend.status = http.StatusTooManyRequests
	p := newTestProxy(t, backend.URL)

	w := post(t, p, request(t, series{
		name:    "go_goroutines",
		typ:     writev2.Metadata_METRIC_TYPE_GAUGE,
		samples: []writev2.Sample{{Value: 1, Timestamp: 1000}},
	}))
	// Prometheus needs the original status code to keep its retry semantics.
	require.Equal(t, http.StatusTooManyRequests, w.Code)
}

// TestProxy_SplitsRequestsOverSeriesLimit reproduces requests GCM rejected
// with "Request has 297 timeseries. Maximum allowed is 200.": a Prometheus
// batch of 200 samples (max_samples_per_send: 200) without metadata, with 97
// untyped floats and 103 untyped native histograms. Each float is split into a
// gauge and a counter stream, the histograms are forwarded as is.
func TestProxy_SplitsRequestsOverSeriesLimit(t *testing.T) {
	backend := newTestBackend(t)
	backend.maxSeries = DefaultMaxSeriesPerRequest
	p := newTestProxy(t, backend.URL)

	batch := func(v float64, ts int64) *writev2.Request {
		syms := writev2.NewSymbolTable()
		req := &writev2.Request{}
		for i := range 97 {
			lset := labels.FromStrings(labels.MetricName, fmt.Sprintf("mysql_global_status_%d", i), "job", "mysql")
			req.Timeseries = append(req.Timeseries, writev2.TimeSeries{
				LabelsRefs: syms.SymbolizeLabels(lset, nil),
				Samples:    []writev2.Sample{{Value: v, Timestamp: ts}},
			})
		}
		for i := range 103 {
			lset := labels.FromStrings(labels.MetricName, fmt.Sprintf("http_request_duration_seconds_%d", i), "job", "app")
			req.Timeseries = append(req.Timeseries, writev2.TimeSeries{
				LabelsRefs: syms.SymbolizeLabels(lset, nil),
				Histograms: []writev2.Histogram{writev2.FromIntHistogram(ts, &histogram.Histogram{
					Count:           uint64(v),
					Sum:             v / 10,
					ZeroThreshold:   0.001,
					PositiveSpans:   []histogram.Span{{Offset: 0, Length: 1}},
					PositiveBuckets: []int64{int64(v)},
				})},
			})
		}
		req.Symbols = syms.Symbols()
		return req
	}

	// First scrape: the counter streams only establish their reference points,
	// so 97 gauges and 103 histograms fit into one request.
	w := post(t, p, batch(10, 1000))
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	require.Len(t, backend.received, 1)
	require.Len(t, backend.received[0].Timeseries, 200)

	// Second scrape: 97 gauges, 97 counters and 103 histograms are 297 series.
	w = post(t, p, batch(15, 2000))
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	require.Len(t, backend.received, 3)
	require.Len(t, backend.received[1].Timeseries, 200)
	require.Len(t, backend.received[2].Timeseries, 97)
	require.Equal(t, 1.0, testutil.ToFloat64(p.splitRequests))

	// The written stats cover both requests.
	require.Equal(t, "194", w.Header().Get("X-Prometheus-Remote-Write-Samples-Written"))
	require.Equal(t, "103", w.Header().Get("X-Prometheus-Remote-Write-Histograms-Written"))

	// Nothing is lost or reordered.
	var want []series
	for i := range 97 {
		name := fmt.Sprintf("mysql_global_status_%d", i)
		want = append(want,
			series{
				name:    name + "/unknown",
				typ:     writev2.Metadata_METRIC_TYPE_GAUGE,
				samples: []writev2.Sample{{Value: 15, Timestamp: 2000}},
			},
			series{
				name:    name + "/unknown:counter",
				typ:     writev2.Metadata_METRIC_TYPE_COUNTER,
				samples: []writev2.Sample{{Value: 5, Timestamp: 2000, StartTimestamp: 1000}},
			},
		)
	}
	for i := range 103 {
		want = append(want, series{name: fmt.Sprintf("http_request_duration_seconds_%d", i)})
	}
	require.Equal(t, want, append(decode(t, backend.received[1]), decode(t, backend.received[2])...))
}

func TestProxy_SplitRequestFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		// statuses the backend returns for the three downstream requests.
		statuses []int

		wantCode     int
		wantBody     string
		wantReceived int
		wantWritten  string
	}{
		{
			name:         "non-retriable failure does not stop the remaining requests",
			statuses:     []int{http.StatusBadRequest, http.StatusNoContent, http.StatusNoContent},
			wantCode:     http.StatusBadRequest,
			wantBody:     "request 0 failed\n",
			wantReceived: 3,
			wantWritten:  "2",
		},
		{
			name:         "first non-retriable failure is returned",
			statuses:     []int{http.StatusNoContent, http.StatusUnprocessableEntity, http.StatusBadRequest},
			wantCode:     http.StatusUnprocessableEntity,
			wantBody:     "request 1 failed\n",
			wantReceived: 3,
			wantWritten:  "1",
		},
		{
			name:         "retriable failure stops the remaining requests",
			statuses:     []int{http.StatusNoContent, http.StatusServiceUnavailable, http.StatusNoContent},
			wantCode:     http.StatusServiceUnavailable,
			wantBody:     "request 1 failed\n",
			wantReceived: 2,
			wantWritten:  "1",
		},
		{
			name:         "retriable failure takes precedence over a non-retriable one",
			statuses:     []int{http.StatusBadRequest, http.StatusTooManyRequests, http.StatusNoContent},
			wantCode:     http.StatusTooManyRequests,
			wantBody:     "request 1 failed\n",
			wantReceived: 2,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backend := newTestBackend(t)
			backend.statuses = tc.statuses
			p := newTestProxy(t, backend.URL, func(c *Config) { c.MaxSeriesPerRequest = 1 })

			w := post(t, p, request(t,
				series{name: "go_goroutines", typ: writev2.Metadata_METRIC_TYPE_GAUGE, samples: []writev2.Sample{{Value: 42, Timestamp: 1000}}},
				series{name: "go_threads", typ: writev2.Metadata_METRIC_TYPE_GAUGE, samples: []writev2.Sample{{Value: 12, Timestamp: 1000}}},
				series{name: "process_open_fds", typ: writev2.Metadata_METRIC_TYPE_GAUGE, samples: []writev2.Sample{{Value: 30, Timestamp: 1000}}},
			))
			require.Equal(t, tc.wantCode, w.Code)
			require.Equal(t, tc.wantBody, w.Body.String())
			require.Len(t, backend.received, tc.wantReceived)
			require.Equal(t, tc.wantWritten, w.Header().Get("X-Prometheus-Remote-Write-Samples-Written"))
		})
	}
}

func TestProxy_RejectsNonPRW2(t *testing.T) {
	p := newTestProxy(t, "http://localhost:1/write")

	r := httptest.NewRequest(http.MethodPost, "/api/v1/write", bytes.NewReader(nil))
	r.Header.Set(contentTypeHeader, "application/x-protobuf")
	w := httptest.NewRecorder()
	p.ServeHTTP(w, r)
	require.Equal(t, http.StatusUnsupportedMediaType, w.Code)
}
