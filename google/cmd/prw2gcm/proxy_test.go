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

//go:build gcme2e
// +build gcme2e

package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	gcm "cloud.google.com/go/monitoring/apiv3/v2"
	"github.com/oklog/ulid"
	"github.com/prometheus/client_golang/exp/api/remote"
	"github.com/prometheus/client_golang/prometheus"
	writev2 "github.com/prometheus/prometheus/google/cmd/prw2gcm/io/prometheus/write/v2"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/model/value"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	apihttp "google.golang.org/api/transport/http"
)

// gcmServiceAccountOrFail gets the Google SA JSON content from GCM_SECRET
// environment variable or fails.
func gcmServiceAccountOrFail(t testing.TB) []byte {
	saJSON := []byte(os.Getenv("GCM_SECRET"))
	if len(saJSON) == 0 {
		t.Fatal("gcmServiceAccountOrFail: no GCM_SECRET env var provided, can't run the test")
	}
	return saJSON
}

// NewRoundTripper creates a round tripper that adds Google Cloud Monitoring authorization to calls
// using either a credentials file or the default credentials.
// Literal copy of https://github.com/prometheus/prometheus/blob/main/storage/remote/googleiam/googleiam.go
// TODO(bwplotka): Import directly once this fork will contain this change.
func NewRoundTripper(gcmJSON []byte, next http.RoundTripper) (http.RoundTripper, error) {
	if next == nil {
		next = http.DefaultTransport
	}

	const scopes = "https://www.googleapis.com/auth/monitoring.write"
	ctx := context.Background()
	opts := []option.ClientOption{
		option.WithScopes(scopes),
		option.WithCredentialsJSON(gcmJSON),
	}
	return apihttp.NewTransport(ctx, next, opts...)
}

func generateTestID(t *testing.T) string {
	return fmt.Sprintf("%v: %v", t.Name(), ulid.MustNew(ulid.Now(), rand.New(rand.NewSource(time.Now().UnixNano()))).String())
}

// TestProxyGCM tests basic proxy handling, some of it against the production GCM service.
// GCM_SECRET environment variable is required, with the GCP secret with the
// https://www.googleapis.com/auth/monitoring.write scope.
func TestProxyGCM(t *testing.T) {
	gcmSA := gcmServiceAccountOrFail(t)

	slog.SetLogLoggerLevel(slog.LevelDebug)
	// Setup server side, explicitly without credentials -- we test Prometheus auth setup here.
	p, err := newProxy(t.Context(), proxyOpts{
		logger:             slog.Default(),
		forwardCredentials: true,
		defaultUserAgent:   "prw2gcm/test",
	})
	require.NoError(t, err)

	mux := http.NewServeMux()
	mux.Handle(p.Handler(prometheus.NewRegistry()))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// Client side, enhanced with the Prometheus GCM remote Write round tripper.
	client := srv.Client()
	rt, err := NewRoundTripper(gcmSA, client.Transport)
	require.NoError(t, err)
	client.Transport = rt

	ctx := t.Context()
	// We infer project id from credentials too.
	creds, err := google.CredentialsFromJSON(ctx, gcmSA, gcm.DefaultAuthScopes()...)
	require.NoError(t, err)

	api, err := remote.NewAPI(
		// NOTE: NewAPI adds /api/v1/write path by default.
		fmt.Sprintf("%s/v1/projects/%s/location/global/prometheus", srv.URL, creds.ProjectID),
		remote.WithAPIHTTPClient(client),
	)
	require.NoError(t, err)

	t.Run("v1 should be rejected", func(t *testing.T) {
		stats, err := api.Write(ctx, remote.WriteV1MessageType, &writev2.Request{})
		fmt.Println(err)
		require.Error(t, err)
		require.True(t, stats.NoDataWritten())
	})
	t.Run("wrong path", func(t *testing.T) {
		wrongPathAPI, err := remote.NewAPI(fmt.Sprintf("%s/v1/wrong", srv.URL))
		require.NoError(t, err)
		stats, err := wrongPathAPI.Write(ctx, remote.WriteV1MessageType, &writev2.Request{})
		fmt.Println(err)
		require.Error(t, err)
		require.True(t, stats.NoDataWritten())
	})
	t.Run("empty v2", func(t *testing.T) {
		stats, err := api.Write(ctx, remote.WriteV2MessageType, &writev2.Request{})
		require.Error(t, err)
		require.True(t, stats.NoDataWritten())
	})
	// NOTE: Detailed, sample specific tests are not needed here, given the
	// detailed google/internal/promqle2etest tests.
	t.Run("gauge and counter", func(t *testing.T) {
		testID := generateTestID(t)
		ts := time.Now().Add(-1 * time.Hour)

		s := writev2.NewSymbolTable()
		r := &writev2.Request{
			Timeseries: []*writev2.TimeSeries{
				{
					LabelsRefs: []uint32{
						s.Symbolize("__name__"), s.Symbolize("proxy_test_counter_total"),
						// Some target labels.
						s.Symbolize("project_id"), s.Symbolize(creds.ProjectID),
						s.Symbolize("location"), s.Symbolize("europe-west3-a"),
						s.Symbolize("cluster"), s.Symbolize("prom-github-action"),
						s.Symbolize("job"), s.Symbolize("TestProxyGCM"),
						s.Symbolize("instance"), s.Symbolize(testID),
						// Other.
						s.Symbolize("repo"), s.Symbolize("github.com/GoogleCloudPlatform/prometheus"),
					},
					Metadata: &writev2.Metadata{
						Type:    writev2.Metadata_METRIC_TYPE_COUNTER,
						HelpRef: s.Symbolize("Test counter used by prw2gcm test"),
						UnitRef: s.Symbolize("seconds"),
					},
					Samples: []*writev2.Sample{
						{
							CreatedTimestamp: timestamp.FromTime(ts),
							Timestamp:        timestamp.FromTime(ts.Add(10 * time.Minute)),
							Value:            10,
						},
						{
							CreatedTimestamp: timestamp.FromTime(ts),
							Timestamp:        timestamp.FromTime(ts.Add(11 * time.Minute)),
							Value:            100,
						},
						{
							CreatedTimestamp: timestamp.FromTime(ts),
							Timestamp:        timestamp.FromTime(ts.Add(12 * time.Minute)),
							Value:            math.Float64frombits(value.StaleNaN), // Should be skipped.
						},
						{
							CreatedTimestamp: timestamp.FromTime(ts.Add(13 * time.Minute)),
							Timestamp:        timestamp.FromTime(ts.Add(14 * time.Minute)),
							Value:            50,
						},
					},
				},
				{
					LabelsRefs: []uint32{
						s.Symbolize("__name__"), s.Symbolize("proxy_test_gauge"),
						// Some target labels.
						s.Symbolize("project_id"), s.Symbolize(creds.ProjectID),
						s.Symbolize("location"), s.Symbolize("europe-west3-a"),
						s.Symbolize("cluster"), s.Symbolize("prom-github-action"),
						s.Symbolize("job"), s.Symbolize("TestProxyGCM"),
						s.Symbolize("instance"), s.Symbolize(testID),
						// Other.
						s.Symbolize("repo"), s.Symbolize("github.com/GoogleCloudPlatform/prometheus"),
					},
					Metadata: &writev2.Metadata{
						Type:    writev2.Metadata_METRIC_TYPE_GAUGE,
						HelpRef: s.Symbolize("Test gauge used by prw2gcm test"),
						UnitRef: s.Symbolize("seconds"),
					},
					Samples: []*writev2.Sample{
						{
							Timestamp: timestamp.FromTime(ts.Add(10 * time.Minute)),
							Value:     10,
						},
						{
							Timestamp: timestamp.FromTime(ts.Add(11 * time.Minute)),
							Value:     5,
						},
						{
							Timestamp: timestamp.FromTime(ts.Add(12 * time.Minute)),
							Value:     math.Float64frombits(value.StaleNaN), // Should be skipped.
						},
						{
							CreatedTimestamp: timestamp.FromTime(ts.Add(13 * time.Minute)),
							Timestamp:        timestamp.FromTime(ts.Add(13 * time.Minute)),
							Value:            1124.155,
						},
					},
				},
			},
		}
		r.Symbols = s.Symbols()
		stats, err := api.Write(ctx, remote.WriteV2MessageType, r)
		require.NoError(t, err)
		require.Equal(t, 6, stats.Samples)
	})
	t.Run("no ct", func(t *testing.T) {
		testID := generateTestID(t)
		ts := time.Now().Add(-1 * time.Hour)

		s := writev2.NewSymbolTable()
		r := &writev2.Request{
			Timeseries: []*writev2.TimeSeries{
				{
					LabelsRefs: []uint32{
						s.Symbolize("__name__"), s.Symbolize("proxy_test_counter_total"),
						// Some target labels.
						s.Symbolize("project_id"), s.Symbolize(creds.ProjectID),
						s.Symbolize("location"), s.Symbolize("europe-west3-a"),
						s.Symbolize("cluster"), s.Symbolize("prom-github-action"),
						s.Symbolize("job"), s.Symbolize("TestProxyGCM"),
						s.Symbolize("instance"), s.Symbolize(testID),
						// Other.
						s.Symbolize("repo"), s.Symbolize("github.com/GoogleCloudPlatform/prometheus"),
					},
					Metadata: &writev2.Metadata{
						Type:    writev2.Metadata_METRIC_TYPE_COUNTER,
						HelpRef: s.Symbolize("Test counter used by prw2gcm test"),
						UnitRef: s.Symbolize("seconds"),
					},
					Samples: []*writev2.Sample{
						{
							CreatedTimestamp: timestamp.FromTime(ts),
							Timestamp:        timestamp.FromTime(ts.Add(10 * time.Minute)),
							Value:            10,
						},
						{
							// No CT!
							Timestamp: timestamp.FromTime(ts.Add(11 * time.Minute)),
							Value:     100,
						},
						{
							CreatedTimestamp: timestamp.FromTime(ts),
							Timestamp:        timestamp.FromTime(ts.Add(12 * time.Minute)),
							Value:            math.Float64frombits(value.StaleNaN), // Should be skipped.
						},
						{
							CreatedTimestamp: timestamp.FromTime(ts.Add(13 * time.Minute)),
							Timestamp:        timestamp.FromTime(ts.Add(14 * time.Minute)),
							Value:            50,
						},
					},
				},
			},
		}
		r.Symbols = s.Symbols()
		stats, err := api.Write(ctx, remote.WriteV2MessageType, r)
		require.Error(t, err)
		// Despite error, we should send 2 samples.
		require.Equal(t, 2, stats.Samples)
	})
}
