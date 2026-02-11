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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb/record"
	metric_pb "google.golang.org/genproto/googleapis/api/metric"
	monitoredres_pb "google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestSeriesCache_populate_Info(t *testing.T) {
	cache := newSeriesCache(nil, nil, MetricTypePrefix)

	// Mock getLabelsByRef to return labels for our info metric.
	ref := storage.SeriesRef(1)
	metricName := "test_info"
	lset := labels.FromStrings("__name__", metricName, "job", "test_job", "instance", "test_instance", "version", "1.2.3")
	cache.getLabelsByRef = func(r storage.SeriesRef) labels.Labels {
		if r == ref {
			return lset
		}
		return labels.EmptyLabels()
	}

	// Prepare entry.
	entry := &seriesCacheEntry{}

	// Metadata function returning Info type.
	mdFunc := func(m string) (MetricMetadata, bool) {
		if m == metricName {
			return MetricMetadata{
				Metric: metricName,
				Type:   model.MetricTypeInfo,
				Help:   "info help",
			}, true
		}
		return MetricMetadata{}, false
	}

	// Populate cache. Provide required external labels (project_id, location).
	externalLabels := labels.FromStrings("project_id", "my-project", "location", "us-central1")
	err := cache.populate(ref, entry, externalLabels, mdFunc)
	if err != nil {
		t.Fatalf("populate failed: %v", err)
	}

	// Verify the cache entry.
	if entry.protos.gauge.proto == nil {
		t.Fatal("expected gauge proto to be populated for Info metric")
	}
	if entry.protos.cumulative.proto != nil {
		t.Error("expected cumulative proto to be nil for Info metric")
	}

	p := entry.protos.gauge.proto

	// Check MetricKind
	if p.MetricKind != metric_pb.MetricDescriptor_GAUGE {
		t.Errorf("expected MetricKind GAUGE, got %v", p.MetricKind)
	}
	// Check ValueType
	if p.ValueType != metric_pb.MetricDescriptor_DOUBLE {
		t.Errorf("expected ValueType DOUBLE, got %v", p.ValueType)
	}
	// Check Type (name)
	expectedType := "prometheus.googleapis.com/test_info/gauge"
	if p.Metric.Type != expectedType {
		t.Errorf("expected Metric Type %q, got %q", expectedType, p.Metric.Type)
	}
	// Check Description
	if p.Description != "info help" {
		t.Errorf("expected Description 'info help', got %q", p.Description)
	}
}

func TestExtractResource(t *testing.T) {
	cases := []struct {
		doc            string
		externalLabels labels.Labels
		seriesLabels   labels.Labels
		wantResource   *monitoredres_pb.MonitoredResource
		wantLabels     labels.Labels
		wantOk         bool
	}{
		{
			doc: "everything contained in series labels",
			seriesLabels: labels.FromMap(map[string]string{
				"project_id": "p1",
				"location":   "l1",
				"cluster":    "c1",
				"namespace":  "n1",
				"job":        "j1",
				"instance":   "i1",
				"key":        "v1",
			}),
			wantResource: &monitoredres_pb.MonitoredResource{
				Type: "prometheus_target",
				Labels: map[string]string{
					"project_id": "p1",
					"location":   "l1",
					"cluster":    "c1",
					"namespace":  "n1",
					"job":        "j1",
					"instance":   "i1",
				},
			},
			wantLabels: labels.FromStrings("key", "v1"),
			wantOk:     true,
		},
		{
			doc: "partially contained in series labels",
			seriesLabels: labels.FromMap(map[string]string{
				"project_id": "p1",
				"location":   "l1",
				"namespace":  "n1",
				"instance":   "i1",
				"key":        "v1",
			}),
			wantResource: &monitoredres_pb.MonitoredResource{
				Type: "prometheus_target",
				Labels: map[string]string{
					"project_id": "p1",
					"location":   "l1",
					"cluster":    "",
					"namespace":  "n1",
					"job":        "",
					"instance":   "i1",
				},
			},
			wantLabels: labels.FromStrings("key", "v1"),
			wantOk:     true,
		}, {
			doc: "some target and metric labels through external labels",
			externalLabels: labels.FromMap(map[string]string{
				"project_id": "p1",
				"location":   "l1",
				"cluster":    "c1",
				"key1":       "v1",
			}),
			seriesLabels: labels.FromMap(map[string]string{
				"cluster":   "c2",
				"namespace": "n1",
				"job":       "j1",
				"instance":  "i1",
				"key2":      "v2",
			}),
			wantResource: &monitoredres_pb.MonitoredResource{
				Type: "prometheus_target",
				Labels: map[string]string{
					"project_id": "p1",
					"location":   "l1",
					"cluster":    "c2",
					"namespace":  "n1",
					"job":        "j1",
					"instance":   "i1",
				},
			},
			wantLabels: labels.FromStrings("key1", "v1", "key2", "v2"),
			wantOk:     true,
		}, {
			doc: "location must be set",
			seriesLabels: labels.FromMap(map[string]string{
				"project_id": "p1",
				"cluster":    "c1",
				"namespace":  "n1",
				"job":        "j1",
				"key":        "v1",
			}),
			wantLabels: labels.EmptyLabels(),
			wantOk:     false,
		}, {
			doc: "project_id must be set",
			seriesLabels: labels.FromMap(map[string]string{
				"location":  "l1",
				"cluster":   "c1",
				"namespace": "n1",
				"job":       "j1",
				"key":       "v1",
			}),
			wantLabels: labels.EmptyLabels(),
			wantOk:     false,
		},
	}
	for _, c := range cases {
		t.Run(c.doc, func(t *testing.T) {
			resource, lset, err := extractResource(c.externalLabels, c.seriesLabels)
			if c.wantOk && err != nil {
				t.Errorf("expected no error but got: %s", err)
			}
			if !c.wantOk && err == nil {
				t.Errorf("expected error but got none")
			}
			if diff := cmp.Diff(c.wantResource, resource, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected resource (-want, +got): %s", diff)
			}
			if diff := cmp.Diff(c.wantLabels.String(), lset.String()); diff != "" {
				t.Errorf("unexpected labels (-want, +got): %s", diff)
			}
		})
	}
}

func TestSeriesCache_garbageCollect(t *testing.T) {
	cache := newSeriesCache(nil, nil, MetricTypePrefix)
	// Always return empty labels. This will cause cache entries to be added but not populated,
	// which we don't need to test garbage collection.
	cache.getLabelsByRef = func(storage.SeriesRef) labels.Labels { return labels.EmptyLabels() }

	// Fake now second timestamp.
	now := int64(100000)
	cache.now = func() time.Time { return time.Unix(now, 0) }

	// Populate some cache entries. Timestamps are converted to milliseconds.
	cache.get(record.RefSample{Ref: 1, T: (now - 100) * 1000}, labels.EmptyLabels(), nil)
	cache.get(record.RefSample{Ref: 2, T: (now - 101) * 1000}, labels.EmptyLabels(), nil)

	if err := cache.garbageCollect(100 * time.Second); err != nil {
		t.Fatal(err)
	}

	// Entry for series 1 should remain while 2 got dropped.
	if len(cache.entries) != 1 {
		t.Errorf("Expected exactly one cache entry left, but cache is %v", cache.entries)
	}
	if _, ok := cache.entries[1]; !ok {
		t.Errorf("Expected cache entry for series 1 but cache is %v", cache.entries)
	}
}

func TestSeriesCache_setMatchers(t *testing.T) {
	cache := newSeriesCache(nil, nil, MetricTypePrefix)
	cache.getLabelsByRef = func(i storage.SeriesRef) labels.Labels {
		switch i {
		case storage.SeriesRef(1):
			return labels.FromStrings("ref", "1")
		case storage.SeriesRef(2):
			return labels.FromStrings("ref", "2")
		}
		t.Fatal("expected either 1 or 2 ref, got", i)
		return nil
	}

	// Fake now second timestamp.
	now := int64(100000)
	cache.now = func() time.Time { return time.Unix(now, 0) }

	for _, tcase := range []struct {
		matchers                           Matchers
		expected1Dropped, expected2Dropped bool
	}{
		{
			expected1Dropped: false,
			expected2Dropped: false,
		},
		{
			matchers:         Matchers{{labels.MustNewMatcher(labels.MatchEqual, "ref", "2")}},
			expected1Dropped: true,
			expected2Dropped: false,
		},
		{
			matchers:         Matchers{{labels.MustNewMatcher(labels.MatchEqual, "ref", "1")}},
			expected1Dropped: false,
			expected2Dropped: true,
		},
		{
			expected1Dropped: false,
			expected2Dropped: false,
		},
	} {
		t.Run("", func(t *testing.T) {
			cache.setMatchers(tcase.matchers)

			e1, _ := cache.get(record.RefSample{Ref: 1, T: (now - 100) * 1000}, labels.EmptyLabels(), nil)
			e2, _ := cache.get(record.RefSample{Ref: 2, T: (now - 101) * 1000}, labels.EmptyLabels(), nil)
			if len(cache.entries) != 2 {
				t.Fatalf("Expected exactly 2 cache entries, but cache is %v", cache.entries)
			}

			if got, want := e1.dropped, tcase.expected1Dropped; got != want {
				t.Errorf("Expected cache entry for series 1 dropped: %v, but cache is %v", tcase.expected1Dropped, e1)
			}
			if got, want := e2.dropped, tcase.expected2Dropped; got != want {
				t.Errorf("Expected cache entry for series 2 dropped: %v, but cache is %v", tcase.expected2Dropped, e2)
			}
		})
	}
}
