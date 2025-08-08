// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type projectIDCtxKeyType struct{}

var (
	projectIDCtxKey projectIDCtxKeyType
	projectIDRe     = regexp.MustCompile("^[a-z-0-9]+$")
)

func detectPOSTMethodProjectID(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}

		// Retrieve and sanitize the PROJECT_ID from the URL path like with
		// the '/v1/projects/PROJECT_ID/location/global/prometheus/api/v1/write' format.
		// This follows similar format to Google Prometheus HTTP API.
		urlPath := r.URL.Path
		if !strings.HasPrefix(urlPath, pathPrefix) {
			http.NotFound(w, r)
			return
		}
		urlPath = strings.TrimPrefix(urlPath, pathPrefix)

		if !strings.HasSuffix(urlPath, pathSuffix) {
			http.NotFound(w, r)
			return
		}

		projectID := strings.TrimSuffix(urlPath, pathSuffix)
		if !projectIDRe.MatchString(projectID) {
			http.Error(w, fmt.Sprintf("PROJECT_ID detected from the '%sPROJECT_ID%s' path has unsupported value, got %q, expected value that matches %q; URL path %q", pathPrefix, pathSuffix, projectID, projectIDRe.String(), r.URL.Path), http.StatusNotFound)
			return
		}
		ctx := context.WithValue(r.Context(), projectIDCtxKey, projectID)
		handler.ServeHTTP(w, r.WithContext(ctx))
	}
}

func getProjectID(ctx context.Context) string {
	ret, ok := ctx.Value(projectIDCtxKey).(string)
	if !ok {
		return "unknown"
	}
	return ret
}

func instrument(reg prometheus.Registerer, handlerName string, handler http.Handler) http.HandlerFunc {
	reg = prometheus.WrapRegistererWith(prometheus.Labels{"handler": handlerName}, reg)

	requestDuration := promauto.With(reg).NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "http_request_duration_seconds",
			Help: "Tracks the latencies for HTTP requests.",

			NativeHistogramBucketFactor: 1.1,
		},
		[]string{"method", "code"},
	)
	requestSize := promauto.With(reg).NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "http_request_size_bytes",
			Help: "Tracks the size of HTTP requests.",

			// Custom buckets, so key metric is visible in the text format (for testing and local debugging).
			Buckets: []float64{0, 200, 1024, 2048, 10240},

			NativeHistogramBucketFactor: 1.1,
		},
		[]string{"method", "code"},
	)
	requestsTotal := promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Tracks the number of HTTP requests.",
		}, []string{"method", "code"},
	)
	responseSize := promauto.With(reg).NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "http_response_size_bytes",
			Help: "Tracks the size of HTTP responses.",

			NativeHistogramBucketFactor: 1.1,
		},
		[]string{"method", "code"},
	)

	base := promhttp.InstrumentHandlerRequestSize(
		requestSize,
		promhttp.InstrumentHandlerCounter(
			requestsTotal,
			promhttp.InstrumentHandlerResponseSize(
				responseSize,
				promhttp.InstrumentHandlerDuration(
					requestDuration,
					http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
						handler.ServeHTTP(writer, r)
					}),
				),
			),
		),
	)
	return base.ServeHTTP
}
