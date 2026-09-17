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
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/snappy"
	"github.com/stretchr/testify/require"

	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
)

// testBackend is a fake PRW2 receiver capturing what the proxy forwarded.
type testBackend struct {
	*httptest.Server

	status   int
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

		b.received = append(b.received, req)
		b.headers = append(b.headers, r.Header.Clone())

		w.Header().Set("X-Prometheus-Remote-Write-Samples-Written", "1")
		w.WriteHeader(b.status)
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

func newTestProxy(t *testing.T, forwardURL string) *Proxy {
	t.Helper()

	p, err := New(Config{
		ForwardURL: forwardURL,
		Transform: TransformConfig{
			HandleUnknown:        true,
			UnknownGaugeSuffix:   DefaultUnknownGaugeSuffix,
			UnknownCounterSuffix: DefaultUnknownCounterSuffix,
			SynthesizeST:         true,
		},
	}, nil, nil, nil)
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
	// Downstream response headers are proxied back to Prometheus.
	require.Equal(t, "1", w.Header().Get("X-Prometheus-Remote-Write-Samples-Written"))

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

func TestProxy_RejectsNonPRW2(t *testing.T) {
	p := newTestProxy(t, "http://localhost:1/write")

	r := httptest.NewRequest(http.MethodPost, "/api/v1/write", bytes.NewReader(nil))
	r.Header.Set(contentTypeHeader, "application/x-protobuf")
	w := httptest.NewRecorder()
	p.ServeHTTP(w, r)
	require.Equal(t, http.StatusUnsupportedMediaType, w.Code)
}
