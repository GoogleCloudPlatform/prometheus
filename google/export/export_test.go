// Copyright 2020 Google LLC
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

package export

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	monitoring_pb "cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/go-kit/log"
	"github.com/gogo/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	gax "github.com/googleapis/gax-go/v2"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	gcmconfig "github.com/prometheus/prometheus/google/config"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb/record"
	"github.com/stretchr/testify/require"
	monitoredres_pb "google.golang.org/genproto/googleapis/api/monitoredres"
	timestamp_pb "google.golang.org/protobuf/types/known/timestamppb"
	metric_pb "google.golang.org/genproto/googleapis/api/metric"
	"k8s.io/apimachinery/pkg/util/wait"
)

func TestBatchAdd(t *testing.T) {
	b := newBatch(nil, DefaultShardCount, 100, nil, "")

	if !b.empty() {
		t.Fatalf("batch unexpectedly not empty")
	}
	// Add 99 samples per project across 10 projects. The batch should not be full at
	// any point and never be empty after adding the first sample.
	for i := range 10 {
		for range 99 {
			if b.full() {
				t.Fatalf("batch unexpectedly full")
			}
			b.add(&monitoring_pb.TimeSeries{
				Resource: &monitoredres_pb.MonitoredResource{
					Labels: map[string]string{
						KeyProjectID: fmt.Sprintf("project-%d", i),
					},
				},
			})
			if b.empty() {
				t.Fatalf("batch unexpectedly empty")
			}
		}
	}
	if b.full() {
		t.Fatalf("batch unexpectedly full")
	}

	// Adding one more sample to one of the projects should make the batch be full.
	b.add(&monitoring_pb.TimeSeries{
		Resource: &monitoredres_pb.MonitoredResource{
			Labels: map[string]string{
				KeyProjectID: fmt.Sprintf("project-%d", 5),
			},
		},
	})
	if !b.full() {
		t.Fatalf("batch unexpectedly not full")
	}
}

func TestBatchFillFromShardsAndSend(t *testing.T) {
	// Fill the batch from 100 shards with samples across 100 projects.
	var shards []*shard
	for range 100 {
		shards = append(shards, newShard(10000))
	}
	for i := range 10000 {
		shards[i%100].enqueue(uint64(i), &monitoring_pb.TimeSeries{
			Resource: &monitoredres_pb.MonitoredResource{
				Labels: map[string]string{
					KeyProjectID: fmt.Sprintf("project-%d", i%100),
				},
			},
		})
	}

	b := newBatch(nil, DefaultShardCount, 101, nil, "")

	for _, s := range shards {
		s.fill(b)

		if !s.pending {
			t.Fatalf("shard unexpectedly not pending after fill")
		}
	}

	var mtx sync.Mutex
	receivedSamples := 0

	// When sending the batch we should see the right number of samples and all shards we pass should
	// be notified at the end.
	sendOne := func(_ context.Context, req *monitoring_pb.CreateTimeSeriesRequest, _ ...gax.CallOption) error {
		mtx.Lock()
		receivedSamples += len(req.TimeSeries)
		mtx.Unlock()
		return nil
	}
	b.send(t.Context(), sendOne)

	if want := 10000; receivedSamples != want {
		t.Fatalf("unexpected number of received samples (want=%d, got=%d)", want, receivedSamples)
	}
	for _, s := range shards {
		if s.pending {
			t.Fatalf("shard unexpectedtly pending after send")
		}
	}
}

func TestSampleInRange(t *testing.T) {
	cases := []struct {
		interval   monitoring_pb.TimeInterval
		start, end time.Time
		want       bool
	}{
		{
			interval: monitoring_pb.TimeInterval{
				EndTime: &timestamp_pb.Timestamp{Seconds: 100},
			},
			start: time.Unix(100, 0),
			end:   time.Unix(100, 0),
			want:  true,
		}, {
			interval: monitoring_pb.TimeInterval{
				EndTime: &timestamp_pb.Timestamp{Seconds: 100},
			},
			start: time.Unix(90, 0),
			end:   time.Unix(100, 0),
			want:  true,
		}, {
			interval: monitoring_pb.TimeInterval{
				EndTime: &timestamp_pb.Timestamp{Seconds: 101},
			},
			start: time.Unix(90, 0),
			end:   time.Unix(100, 0),
			want:  false,
		}, {
			interval: monitoring_pb.TimeInterval{
				StartTime: &timestamp_pb.Timestamp{Seconds: 90},
				EndTime:   &timestamp_pb.Timestamp{Seconds: 100},
			},
			start: time.Unix(90, 0),
			end:   time.Unix(100, 0),
			want:  true,
		}, {
			interval: monitoring_pb.TimeInterval{
				StartTime: &timestamp_pb.Timestamp{Seconds: 89},
				EndTime:   &timestamp_pb.Timestamp{Seconds: 100},
			},
			start: time.Unix(90, 0),
			end:   time.Unix(100, 0),
			want:  false,
		}, {
			interval: monitoring_pb.TimeInterval{
				StartTime: &timestamp_pb.Timestamp{Seconds: 90},
				EndTime:   &timestamp_pb.Timestamp{Seconds: 101},
			},
			start: time.Unix(90, 0),
			end:   time.Unix(100, 0),
			want:  false,
		}, {
			interval: monitoring_pb.TimeInterval{
				StartTime: &timestamp_pb.Timestamp{Seconds: 89},
				EndTime:   &timestamp_pb.Timestamp{Seconds: 101},
			},
			start: time.Unix(90, 0),
			end:   time.Unix(100, 0),
			want:  false,
		},
	}
	//nolint:govet
	for _, c := range cases {
		p := &monitoring_pb.TimeSeries{
			Points: []*monitoring_pb.Point{
				{Interval: &c.interval},
			},
		}
		if ok := sampleInRange(p, c.start, c.end); ok != c.want {
			t.Errorf("expected sample in range %v, got %v", c.want, ok)
		}
	}
}

func TestExporter_wrapMetadata(t *testing.T) {
	cases := []struct {
		desc   string
		mf     MetadataFunc
		metric string
		want   MetricMetadata
		wantOK bool
	}{
		{
			desc:   "nil MetadataFunc always defaults to gauge",
			mf:     nil,
			metric: "some_metric",
			want:   MetricMetadata{MetricFamily: "some_metric", Type: model.MetricTypeGauge},
			wantOK: true,
		}, {
			desc:   "nil MetadataFunc preserves synthetic metric metadata",
			mf:     nil,
			metric: "up",
			want: MetricMetadata{
				MetricFamily: "up",
				Type:         model.MetricTypeGauge,
				Help:         "Up indicates whether the last target scrape was successful.",
			},
			wantOK: true,
		}, {
			desc: "synthetic metric metadata precedence",
			mf: func(string) (MetricMetadata, bool) {
				return MetricMetadata{
					MetricFamily: "up",
					Type:         model.MetricTypeCounter,
				}, false
			},
			metric: "up",
			want: MetricMetadata{
				MetricFamily: "up",
				Type:         model.MetricTypeGauge,
				Help:         "Up indicates whether the last target scrape was successful.",
			},
			wantOK: true,
		}, {
			desc: "regular metadata is returned as is",
			mf: func(string) (MetricMetadata, bool) {
				return MetricMetadata{
					MetricFamily: "some_metric",
					Type:         model.MetricTypeCounter,
					Help:         "useful help",
				}, true
			},
			metric: "some_metric",
			want: MetricMetadata{
				MetricFamily: "some_metric",
				Type:         model.MetricTypeCounter,
				Help:         "useful help",
			},
			wantOK: true,
		}, {
			desc: "info metadata is returned as is",
			mf: func(string) (MetricMetadata, bool) {
				return MetricMetadata{
					MetricFamily: "some_info_metric",
					Type:         model.MetricTypeInfo,
					Help:         "info help",
				}, true
			},
			metric: "some_info_metric",
			want: MetricMetadata{
				MetricFamily: "some_info_metric",
				Type:         model.MetricTypeInfo,
				Help:         "info help",
			},
			wantOK: true,
		}, {
			desc: "stateset metadata is returned as is",
			mf: func(string) (MetricMetadata, bool) {
				return MetricMetadata{
					MetricFamily: "some_stateset_metric",
					Type:         model.MetricTypeStateset,
					Help:         "stateset help",
				}, true
			},
			metric: "some_stateset_metric",
			want: MetricMetadata{
				MetricFamily: "some_stateset_metric",
				Type:         model.MetricTypeStateset,
				Help:         "stateset help",
			},
			wantOK: true,
		}, {
			desc: "not found metadata defaults to untyped",
			mf: func(string) (MetricMetadata, bool) {
				return MetricMetadata{}, false
			},
			metric: "some_metric",
			want: MetricMetadata{
				MetricFamily: "some_metric",
				Type:         model.MetricTypeUnknown,
			},
			wantOK: true,
		}, {
			desc: "not found metadata returns false if base name has metadata (_sum)",
			mf: func(m string) (MetricMetadata, bool) {
				if m == "foo" {
					return MetricMetadata{MetricFamily: "foo", Type: model.MetricTypeSummary}, true
				}
				return MetricMetadata{}, false
			},
			metric: "foo_sum",
			want:   MetricMetadata{},
			wantOK: false,
		}, {
			desc: "not found metadata returns false if base name has metadata (_bucket)",
			mf: func(m string) (MetricMetadata, bool) {
				if m == "foo" {
					return MetricMetadata{MetricFamily: "foo", Type: model.MetricTypeSummary}, true
				}
				return MetricMetadata{}, false
			},
			metric: "foo_bucket",
			want:   MetricMetadata{},
			wantOK: false,
		}, {
			desc: "not found metadata returns false if base name has metadata (_count)",
			mf: func(m string) (MetricMetadata, bool) {
				if m == "foo" {
					return MetricMetadata{MetricFamily: "foo", Type: model.MetricTypeSummary}, true
				}
				return MetricMetadata{}, false
			},
			metric: "foo_count",
			want:   MetricMetadata{},
			wantOK: false,
		},
	}

	exporterOpts := ExporterOpts{DisableAuth: true}
	exporterOpts.DefaultUnsetFields()
	e, err := New(t.Context(), log.NewJSONLogger(log.NewSyncWriter(os.Stderr)), nil, exporterOpts, NopLease())
	if err != nil {
		t.Fatal(err)
	}
	require.NoError(t, e.ApplyConfig(&config.Config{}))

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			got, ok := e.wrapMetadata(c.mf)(c.metric)
			if ok != c.wantOK {
				t.Fatalf("MetadataFunc unexpectedly ok=%v, want ok=%v", ok, c.wantOK)
			}
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Fatalf("unexpected metadata (-want,+got): %s", diff)
			}
		})
	}
}

type testMetricService struct {
	monitoring_pb.MetricServiceServer // Inherit all interface methods
	samples                           []*monitoring_pb.TimeSeries
	sync.Mutex
}

func (srv *testMetricService) CreateTimeSeries(_ context.Context, req *monitoring_pb.CreateTimeSeriesRequest, _ ...gax.CallOption) error {
	srv.Lock()
	defer srv.Unlock()
	srv.samples = append(srv.samples, req.TimeSeries...)
	return nil
}

func (srv *testMetricService) Close() error {
	return nil
}

func (srv *testMetricService) clear() {
	srv.Lock()
	defer srv.Unlock()

	srv.samples = []*monitoring_pb.TimeSeries{}
}

func TestExporter_drainBacklog(t *testing.T) {
	exporterOpts := ExporterOpts{DisableAuth: true}
	exporterOpts.DefaultUnsetFields()
	e, err := New(t.Context(), log.NewJSONLogger(log.NewSyncWriter(os.Stderr)), nil, exporterOpts, NopLease())
	if err != nil {
		t.Fatalf("Creating Exporter failed: %s", err)
	}
	metricServer := testMetricService{}
	e.metricClient = &metricServer

	e.SetLabelsByIDFunc(func(storage.SeriesRef) labels.Labels {
		return labels.FromStrings("project_id", "test", "location", "test")
	})

	{
		// Export now requires at least one ApplyConfig call, otherwise it's noop, test it.
		for i := range 100 {
			// Noop.
			e.Export(nil, []record.RefSample{
				{Ref: 1, T: int64(i), V: float64(i)},
			}, nil)
		}

		metricServer.Lock()
		got := len(metricServer.samples)
		metricServer.Unlock()
		if got != 0 {
			t.Fatalf("got %d, want zero, because ApplyConfig was not called", got)
		}
	}
	require.NoError(t, e.ApplyConfig(&config.Config{}))

	// Fill a single shard with samples.
	wantSamples := 50
	for i := range wantSamples {
		e.Export(nil, []record.RefSample{
			{Ref: 1, T: int64(i), V: float64(i)},
		}, nil)
	}

	//nolint:errcheck
	go e.Run()
	// As our samples are all for the same series, each batch can only contain a single sample.
	// The exporter waits for the batch delay duration before sending it.
	// We sleep for an appropriate multiple of it to allow it to drain the shard.
	ctxTimeout, cancel := context.WithTimeout(t.Context(), 60*batchDelayMax)
	defer cancel()

	pollErr := wait.PollUntilContextCancel(ctxTimeout, batchDelayMax, false, func(_ context.Context) (bool, error) {
		metricServer.Lock()
		defer metricServer.Unlock()

		// Check that we received all samples that went in.
		if got, want := len(metricServer.samples), wantSamples; got != want {
			err = fmt.Errorf("got %d, want %d", got, want)
			return false, nil
		}
		return true, nil
	})
	if pollErr != nil {
		if wait.Interrupted(pollErr) && err != nil {
			pollErr = err
		}
		t.Fatalf("did not get samples: %s", pollErr)
	}
}

func TestApplyConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second*30)
	defer cancel()

	exporterOpts := ExporterOpts{
		DisableAuth: true, // Don't error on lack of default creds for metric client on New.
		ProjectID:   "project_abc",
		Matchers: func() (ret Matchers) {
			// Initial matchers, imitating flag setting (or EXTRA ARGS).
			require.NoError(t, ret.Set(`{ref=~"(1|2)"}`))
			return ret
		}(),
	}
	exporterOpts.DefaultUnsetFields()

	e, err := New(ctx, log.NewJSONLogger(log.NewSyncWriter(os.Stderr)), nil, exporterOpts, NopLease())
	if err != nil {
		t.Fatalf("create exporter: %s", err)
	}

	// Each case will export 3 samples with ref=<ref> label and one external label overridden.
	e.SetLabelsByIDFunc(func(ref storage.SeriesRef) labels.Labels {
		return labels.FromStrings("location", "us-central1-c", "ref", fmt.Sprint(ref))
	})
	exportSamplesFn := func() {
		e.Export(nil, []record.RefSample{{Ref: 1, T: int64(0), V: float64(0)}}, nil)
		e.Export(nil, []record.RefSample{{Ref: 2, T: int64(0), V: float64(0)}}, nil)
		e.Export(nil, []record.RefSample{{Ref: 3, T: int64(0), V: float64(0)}}, nil)
	}

	metricServer := testMetricService{}
	e.metricClient = &metricServer

	// In our Prometheus fork, GCM is executed before the reloader in the run group.
	go func() {
		if err := e.Run(); err != nil {
			t.Errorf("Run exporter: %s", err)
		}
	}()

	type gcmSampleLabels struct {
		resource []string
		metric   []string
	}

	resourceWithProject := func(projectID string) []string {
		return []string{"cluster", "", "instance", "", "job", "", "location", "us-central1-c", "namespace", "", "project_id", projectID}
	}

	// NOTE: All test cases share the same exporter to ensure subsequent ApplyConfigs are
	// updating options correctly.
	for _, tcase := range []struct {
		name string

		toApply                       *config.Config
		expectedSamplesSeries         []gcmSampleLabels // expected samples on GCM side.
		expectClientReload            bool
		expectedCacheEntriesRefreshed []storage.SeriesRef
	}{
		{
			name:    "no change",
			toApply: &config.Config{},
			expectedSamplesSeries: []gcmSampleLabels{
				// No 3 ref due to initial matchers.
				{metric: []string{"ref", "1"}, resource: resourceWithProject("project_abc")},
				{metric: []string{"ref", "2"}, resource: resourceWithProject("project_abc")},
			},
		},
		{
			name: "compression change should trigger client recreation",
			toApply: &config.Config{
				GoogleCloud: gcmconfig.GoogleCloudConfig{
					Export: gcmconfig.GoogleCloudExportConfig{
						Compression: "gzip",
					},
				},
			},
			expectClientReload: true,
			expectedSamplesSeries: []gcmSampleLabels{
				{metric: []string{"ref", "1"}, resource: resourceWithProject("project_abc")},
				{metric: []string{"ref", "2"}, resource: resourceWithProject("project_abc")},
			},
		},
		{
			name: "ExternalLabels change should trigger cache refresh",
			toApply: &config.Config{
				GlobalConfig: config.GlobalConfig{
					ExternalLabels: labels.FromStrings(KeyProjectID, "project-test"),
				},
				GoogleCloud: gcmconfig.GoogleCloudConfig{
					Export: gcmconfig.GoogleCloudExportConfig{
						Compression: "gzip",
					},
				},
			},
			expectedCacheEntriesRefreshed: []storage.SeriesRef{1, 2},
			expectedSamplesSeries: []gcmSampleLabels{
				{metric: []string{"ref", "1"}, resource: resourceWithProject("project-test")},
				{metric: []string{"ref", "2"}, resource: resourceWithProject("project-test")},
			},
		},
		{
			name: "Noop filtering change (without enable flag)",
			toApply: &config.Config{
				GlobalConfig: config.GlobalConfig{
					ExternalLabels: labels.FromStrings(KeyProjectID, "project-test"),
				},
				GoogleCloud: gcmconfig.GoogleCloudConfig{
					Export: gcmconfig.GoogleCloudExportConfig{
						Compression: "gzip",
						Match:       []string{`{ref=~"(0|3)"}`, `{ref="2"}`}, // Skip 1.
					},
				},
			},
			expectedSamplesSeries: []gcmSampleLabels{
				{metric: []string{"ref", "1"}, resource: resourceWithProject("project-test")},
				{metric: []string{"ref", "2"}, resource: resourceWithProject("project-test")},
			},
		},
		{
			name: "Filtering change",
			toApply: &config.Config{
				GlobalConfig: config.GlobalConfig{
					ExternalLabels: labels.FromStrings(KeyProjectID, "project-test"),
				},
				GoogleCloud: gcmconfig.GoogleCloudConfig{
					Export: gcmconfig.GoogleCloudExportConfig{
						Compression: "gzip",
						EnableMatch: proto.Bool(true),
						Match:       []string{`{ref=~"(0|3)"}`, `{ref="2"}`}, // Skip 1.
					},
				},
			},
			expectedCacheEntriesRefreshed: []storage.SeriesRef{3}, // Expect refresh for series that're now matching.
			expectedSamplesSeries: []gcmSampleLabels{
				{metric: []string{"ref", "2"}, resource: resourceWithProject("project-test")},
				{metric: []string{"ref", "3"}, resource: resourceWithProject("project-test")},
			},
		},
		{
			name: "No change, same config",
			toApply: &config.Config{
				GlobalConfig: config.GlobalConfig{
					ExternalLabels: labels.FromStrings(KeyProjectID, "project-test"),
				},
				GoogleCloud: gcmconfig.GoogleCloudConfig{
					Export: gcmconfig.GoogleCloudExportConfig{
						Compression: "gzip",
						EnableMatch: proto.Bool(true),
						Match:       []string{`{ref=~"(0|3)"}`, `{ref="2"}`}, // Skip 1.
					},
				},
			},
			expectedSamplesSeries: []gcmSampleLabels{
				{metric: []string{"ref", "2"}, resource: resourceWithProject("project-test")},
				{metric: []string{"ref", "3"}, resource: resourceWithProject("project-test")},
			},
		},
		{
			name: "Filtering change (noop again)",
			toApply: &config.Config{
				GlobalConfig: config.GlobalConfig{
					ExternalLabels: labels.FromStrings(KeyProjectID, "project-test"),
				},
				GoogleCloud: gcmconfig.GoogleCloudConfig{
					Export: gcmconfig.GoogleCloudExportConfig{
						Compression: "gzip",
						// EnableMatch is now nil.
					},
				},
			},
			expectedCacheEntriesRefreshed: []storage.SeriesRef{1}, // Expect refresh for series that're now matching.
			expectedSamplesSeries: []gcmSampleLabels{
				// Noop means we go back to initial state of filtering.
				{metric: []string{"ref", "1"}, resource: resourceWithProject("project-test")},
				{metric: []string{"ref", "2"}, resource: resourceWithProject("project-test")},
			},
		},
		{
			name: "Filtering reset",
			toApply: &config.Config{
				GlobalConfig: config.GlobalConfig{
					ExternalLabels: labels.FromStrings(KeyProjectID, "project-test"),
				},
				GoogleCloud: gcmconfig.GoogleCloudConfig{
					Export: gcmconfig.GoogleCloudExportConfig{
						Compression: "gzip",
						EnableMatch: proto.Bool(false), // Explicit false means match all.
					},
				},
			},
			expectedCacheEntriesRefreshed: []storage.SeriesRef{3}, // Expect refresh for series that're now matching.
			expectedSamplesSeries: []gcmSampleLabels{
				{metric: []string{"ref", "1"}, resource: resourceWithProject("project-test")},
				{metric: []string{"ref", "2"}, resource: resourceWithProject("project-test")},
				{metric: []string{"ref", "3"}, resource: resourceWithProject("project-test")},
			},
		},
		{
			name: "Filtering change (back)",
			toApply: &config.Config{
				GlobalConfig: config.GlobalConfig{
					ExternalLabels: labels.FromStrings(KeyProjectID, "project-test"),
				},
				GoogleCloud: gcmconfig.GoogleCloudConfig{
					Export: gcmconfig.GoogleCloudExportConfig{
						Compression: "gzip",
						EnableMatch: proto.Bool(true),
						Match:       []string{`{ref=~"(0|3)"}`, `{ref="2"}`}, // Skip 1.
					},
				},
			},
			expectedSamplesSeries: []gcmSampleLabels{
				{metric: []string{"ref", "2"}, resource: resourceWithProject("project-test")},
				{metric: []string{"ref", "3"}, resource: resourceWithProject("project-test")},
			},
		},
		{
			name: "Filtering change (drop all)",
			toApply: &config.Config{
				GlobalConfig: config.GlobalConfig{
					ExternalLabels: labels.FromStrings(KeyProjectID, "project-test"),
				},
				GoogleCloud: gcmconfig.GoogleCloudConfig{
					Export: gcmconfig.GoogleCloudExportConfig{
						Compression: "gzip",
						EnableMatch: proto.Bool(true),
						Match:       []string{`{ref="1", ref="2"}`}, // Impossible match; drop all!
					},
				},
			},
			expectedSamplesSeries: []gcmSampleLabels{},
		},
		{
			name: "Filtering change (force empty)",
			toApply: &config.Config{
				GlobalConfig: config.GlobalConfig{
					ExternalLabels: labels.FromStrings(KeyProjectID, "project-test"),
				},
				GoogleCloud: gcmconfig.GoogleCloudConfig{
					Export: gcmconfig.GoogleCloudExportConfig{
						Compression: "gzip",
						EnableMatch: proto.Bool(true),
					},
				},
			},
			expectedCacheEntriesRefreshed: []storage.SeriesRef{1, 2, 3}, // Expect refresh for series that're now matching.
			expectedSamplesSeries: []gcmSampleLabels{
				{metric: []string{"ref", "1"}, resource: resourceWithProject("project-test")},
				{metric: []string{"ref", "2"}, resource: resourceWithProject("project-test")},
				{metric: []string{"ref", "3"}, resource: resourceWithProject("project-test")},
			},
		},
	} {
		t.Run(tcase.name, func(t *testing.T) {
			defer metricServer.clear()

			if tcase.expectClientReload {
				e.newMetricClient = func(_ context.Context, opts ExporterOpts) (metricServiceClient, error) {
					t.Helper()
					require.Equal(t, tcase.toApply.GoogleCloud.Export.Compression, opts.Compression)
					return &metricServer, nil
				}
			} else {
				e.newMetricClient = func(_ context.Context, opts ExporterOpts) (metricServiceClient, error) {
					t.Helper()
					t.Fatal("unexpected newMetricClient call, client should not be recreated")
					return nil, nil
				}
			}

			require.NoError(t, e.ApplyConfig(tcase.toApply))
			// Verify if cache was reset, if expected.
			// Run in closure for lock defer to work on require failures.
			func() {
				e.seriesCache.mtx.Lock()
				defer e.seriesCache.mtx.Unlock()
				for i, entry := range e.seriesCache.entries {
					if slices.Contains(tcase.expectedCacheEntriesRefreshed, i) {
						require.Zero(t, entry.nextRefresh, entry.lset)
						continue
					}
					if entry.dropped {
						// Dropped series is N/A.
						continue
					}
					require.NotZero(t, entry.nextRefresh, entry.lset)
				}
			}()

			// Export 3 samples (ref=1, ref=2, ref=3)
			exportSamplesFn()

			// TODO: This test is not very reliable for detecting if we send more samples than tcase.expectedSamplesSeries, find way to ensure this.
			// For now we wait a bit (0.5s)
			time.Sleep(10 * batchDelayMax)

			var err error
			pollErr := wait.PollUntilContextCancel(ctx, batchDelayMax, false, func(_ context.Context) (bool, error) {
				t.Helper()

				metricServer.Lock()
				defer metricServer.Unlock()
				switch len(metricServer.samples) {
				case len(tcase.expectedSamplesSeries):
					// Good.
				default:
					// Sometimes there's a small delay from the thread that sends the new
					// samples, so let's wait.
					err = fmt.Errorf("expected %d samples but got %d", len(tcase.expectedSamplesSeries), len(metricServer.samples))
					return false, nil
				}

				// samples might have been seen out of order (series-wise). Sort it for easier testing.
				sort.Slice(metricServer.samples, func(i, j int) bool {
					return strings.Compare(metricServer.samples[i].Metric.String(), metricServer.samples[j].Metric.String()) < 0
				})
				for i, sample := range metricServer.samples {
					require.Equal(t, labels.FromStrings(tcase.expectedSamplesSeries[i].resource...).Map(), sample.Resource.Labels)
					require.Equal(t, labels.FromStrings(tcase.expectedSamplesSeries[i].metric...).Map(), sample.Metric.Labels)
				}
				return true, nil
			})
			if pollErr != nil {
				if wait.Interrupted(pollErr) && err != nil {
					pollErr = err
				}
				t.Fatalf("did not get samples: %s", pollErr)
			}
		})
	}

	// Extra check if series cache clear works fine after our dynamic matchers resource releases
	e.seriesCache.clear()

}

func TestDisabledExporter(t *testing.T) {
	// Since on certain environments (e.g. Google-developer machines), we can't emulate a non-GCE
	// environment, we instead set invalid an invalid credential path to emulate no credentials.
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "does-not-exist.json")

	exporterOpts := ExporterOpts{}
	exporterOpts.DefaultUnsetFields()

	// The default exporter will look for authentication.
	if _, err := New(t.Context(), log.NewJSONLogger(log.NewSyncWriter(os.Stderr)), nil, exporterOpts, NopLease()); err == nil {
		t.Fatal("Expected error but got none")
	}

	// When we disable the exporter, it doesn't matter if we have credentials or not.
	exporterOpts.Disable = true
	e, err := New(t.Context(), log.NewJSONLogger(log.NewSyncWriter(os.Stderr)), nil, exporterOpts, NopLease())
	if err != nil {
		t.Fatalf("Run exporter: %s", err)
	}

	// In our Prometheus fork, GCM is executed before the reloader in the run group.
	go func() {
		if err := e.Run(); err != nil {
			t.Errorf("Run exporter: %s", err)
		}
	}()

	if err := e.ApplyConfig(&config.Config{}); err != nil {
		t.Fatalf("Initial apply: %s", err)
	}
	e.Export(nil, []record.RefSample{{Ref: 1, T: int64(0), V: float64(0)}}, nil)

	// Allow samples to be sent to the void. If we don't panic, we're good.
	time.Sleep(batchDelayMax)
}

func TestMatchers_Equals(t *testing.T) {
	var m Matchers
	require.NoError(t, m.Set(`{name="foo"}`))
	require.NoError(t, m.Set(`{name=~"foo.1"}`))
	require.NoError(t, m.Set(`{name="foo",bar="x"}`))

	var same Matchers
	require.NoError(t, same.Set(`{name="foo"}`))
	require.NoError(t, same.Set(`{name=~"foo.1"}`))
	require.NoError(t, same.Set(`{name="foo",    bar="x"}`))

	require.True(t, m.Equals(same))
	require.True(t, same.Equals(m))

	var diff1 Matchers
	require.False(t, m.Equals(diff1))
	require.False(t, diff1.Equals(m))

	var diff2 Matchers
	require.NoError(t, diff2.Set(`{name="foo"}`))
	require.False(t, m.Equals(diff2))
	require.False(t, diff2.Equals(m))

	var diff3 Matchers
	require.NoError(t, diff3.Set(`{name="foo"}`))
	require.NoError(t, diff3.Set(`{name=~"foo.1"}`))
	require.False(t, m.Equals(diff3))
	require.False(t, diff3.Equals(m))

	var diff4 Matchers
	require.NoError(t, diff4.Set(`{name="foo"}`))
	require.NoError(t, diff4.Set(`{name="foo",bar="x"}`))
	require.NoError(t, diff4.Set(`{name=~"foo.1"}`))
	require.False(t, m.Equals(diff4))
	require.False(t, diff4.Equals(m))

	var diff5 Matchers
	require.NoError(t, diff5.Set(`{name="foo1"}`))
	require.NoError(t, diff5.Set(`{name=~"foo.1"}`))
	require.NoError(t, diff5.Set(`{name="foo",bar="x"}`))
	require.False(t, m.Equals(diff5))
	require.False(t, diff5.Equals(m))

	var diff6 Matchers
	require.NoError(t, diff6.Set(`{name="foo"}`))
	require.NoError(t, diff6.Set(`{name=~"foo.1"}`))
	require.NoError(t, diff6.Set(`{bar="x",name="foo"}`))
	require.False(t, m.Equals(diff6))
	require.False(t, diff6.Equals(m))
}

func TestDebugLogGRPCRequest(t *testing.T) {
	var buf bytes.Buffer
	logger := log.NewLogfmtLogger(&buf)

	var matchers Matchers
	require.NoError(t, matchers.Set(`{__name__="up"}`))

	req := &monitoring_pb.CreateTimeSeriesRequest{
		Name: "projects/test-proj",
		TimeSeries: []*monitoring_pb.TimeSeries{
			{
				Resource: &monitoredres_pb.MonitoredResource{
					Type: "prometheus_target",
					Labels: map[string]string{
						"project_id": "test-proj",
					},
				},
				Metric: &metric_pb.Metric{
					Type: "prometheus.googleapis.com/up/gauge",
					Labels: map[string]string{
						"instance": "localhost:9090",
					},
				},
			},
			{
				Resource: &monitoredres_pb.MonitoredResource{
					Type: "prometheus_target",
					Labels: map[string]string{
						"project_id": "test-proj",
					},
				},
				Metric: &metric_pb.Metric{
					Type: "prometheus.googleapis.com/other_metric/gauge",
					Labels: map[string]string{
						"instance": "localhost:9090",
					},
				},
			},
		},
	}

	// First test: no matchers set should not log.
	logDebugGRPCRequest(logger, nil, "prometheus.googleapis.com", req)
	require.Empty(t, buf.String())

	// Second test: matchers set should log only the matching series.
	logDebugGRPCRequest(logger, matchers, "prometheus.googleapis.com", req)
	out := buf.String()
	require.Contains(t, out, "gRPC CreateTimeSeries request matching debug matchers")
	require.Contains(t, out, "series_count=1")
	require.Contains(t, out, "prometheus.googleapis.com/up/gauge")
	require.NotContains(t, out, "other_metric")
}
