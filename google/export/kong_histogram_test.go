// Copyright 2026 Google LLC
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

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb/record"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKongHistogramInconsistencies(t *testing.T) {
	metricName := "kong_upstream_latency_ms"
	lset := labels.FromStrings("__name__", metricName+"_bucket", "le", "100", "route", "users")
	extLabels := labels.FromStrings("project_id", "test-project", "location", "us-east1", "cluster", "kong-cluster")

	series := map[storage.SeriesRef]labels.Labels{
		1: labels.FromStrings("job", "kong", "instance", "kong-1", "__name__", metricName+"_bucket", "le", "100", "route", "users"),
		2: labels.FromStrings("job", "kong", "instance", "kong-1", "__name__", metricName+"_bucket", "le", "+Inf", "route", "users"),
		3: labels.FromStrings("job", "kong", "instance", "kong-1", "__name__", metricName+"_count", "route", "users"),
		4: labels.FromStrings("job", "kong", "instance", "kong-1", "__name__", metricName+"_sum", "route", "users"),
		5: labels.FromStrings("job", "kong", "instance", "kong-1", "__name__", metricName+"_bucket", "le", "25", "route", "users"),
	}

	metaFunc := func(name string) (MetricMetadata, bool) {
		if name == metricName {
			return MetricMetadata{
				Metric: metricName,
				Type:   model.MetricTypeHistogram,
				Help:   "Latency histogram",
			}, true
		}
		return MetricMetadata{}, false
	}

	cases := []struct {
		name        string
		scrapes     [][]record.RefSample
		wantSkipped []bool
		wantCount   []int64
		wantBuckets [][]int64
		wantRT      []int64
	}{
		{
			// Kong omits zero-count buckets from shared dictionaries.
			// When latency drops mid-stream, le="25" appears dynamically for the first time.
			// Without normalization, getResetAdjusted returns ok=false and skips the distribution.
			// With normalization, resetValue is initialized to 0 and the sample is exported.
			name: "dynamic_zero_bucket_appearance",
			scrapes: [][]record.RefSample{
				{ // Scrape 1 (T=1000s): Initial scrape without le="25"
					{Ref: 1, T: 1000000, V: 10},
					{Ref: 2, T: 1000000, V: 10},
					{Ref: 3, T: 1000000, V: 10},
					{Ref: 4, T: 1000000, V: 500},
				},
				{ // Scrape 2 (T=1030s): Normal scrape
					{Ref: 1, T: 1030000, V: 20},
					{Ref: 2, T: 1030000, V: 20},
					{Ref: 3, T: 1030000, V: 20},
					{Ref: 4, T: 1030000, V: 1000},
				},
				{ // Scrape 3 (T=1060s): Dynamic appearance of zero bucket le="25"
					{Ref: 5, T: 1060000, V: 5},
					{Ref: 1, T: 1060000, V: 25},
					{Ref: 2, T: 1060000, V: 25},
					{Ref: 3, T: 1060000, V: 25},
					{Ref: 4, T: 1060000, V: 1100},
				},
			},
			wantSkipped: []bool{true, false, false},
			wantCount:   []int64{0, 10, 15},
			wantBuckets: [][]int64{nil, {10, 0}, {5, 10, 0}},
			wantRT:      []int64{0, 1000000, 1000000},
		},
		{
			// Kong retrieves keys alphabetically (_bucket, _count, _sum) with coroutine.yield()
			// called between fetches. Incoming requests mid-scrape increment _count and _sum
			// after _bucket has already been read.
			name: "mid_scrape_yielding_desynchronization",
			scrapes: [][]record.RefSample{
				{ // Scrape 1 (T=1000s): Initial scrape
					{Ref: 5, T: 1000000, V: 2},
					{Ref: 1, T: 1000000, V: 10},
					{Ref: 2, T: 1000000, V: 10},
					{Ref: 3, T: 1000000, V: 10},
					{Ref: 4, T: 1000000, V: 500},
				},
				{ // Scrape 2 (T=1030s): Mid-scrape request increments _count=21 while buckets reflect 20
					{Ref: 5, T: 1030000, V: 5},
					{Ref: 1, T: 1030000, V: 20},
					{Ref: 2, T: 1030000, V: 20},
					{Ref: 3, T: 1030000, V: 21},
					{Ref: 4, T: 1030000, V: 1050},
				},
			},
			wantSkipped: []bool{true, false},
			wantCount:   []int64{0, 10},
			wantBuckets: [][]int64{nil, {3, 7, 0}},
			wantRT:      []int64{0, 1000000},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cache := newSeriesCache(nil, nil, "prometheus.googleapis.com/")
			cache.getLabelsByRef = func(ref storage.SeriesRef) labels.Labels {
				return series[ref]
			}
			cache.setMatchers(Matchers{})

			for i, batch := range tc.scrapes {
				for _, s := range batch {
					cache.get(s, extLabels, metaFunc)
				}

				b := newSampleBuilder(cache)
				dist, rt, tail, err := b.buildDistribution(metricName, lset, batch, nil, extLabels, metaFunc)
				require.NoError(t, err)
				assert.Empty(t, tail)

				if tc.wantSkipped[i] {
					assert.Nil(t, dist, "Scrape %d should be skipped", i+1)
				} else {
					require.NotNil(t, dist, "Scrape %d should not be skipped", i+1)
					assert.Equal(t, tc.wantCount[i], dist.Count, "Scrape %d count mismatch", i+1)
					assert.Equal(t, tc.wantBuckets[i], dist.BucketCounts, "Scrape %d bucket counts mismatch", i+1)
					assert.Equal(t, tc.wantRT[i], rt, "Scrape %d reset timestamp mismatch", i+1)
				}
				b.close()
			}
		})
	}
}

func TestHistogramResetsCleanup(t *testing.T) {
	cache := newSeriesCache(nil, nil, "prometheus.googleapis.com/")
	cache.histogramResets[12345] = 1000000
	cache.histogramResets[67890] = 1000000

	// Add an active entry for hash 12345.
	cache.entries[1] = &seriesCacheEntry{
		lastUsed: cache.now().Unix(),
		metadata: MetricMetadata{Type: model.MetricTypeHistogram},
		protos:   cachedProtos{cumulative: hashedSeries{hash: 12345}},
	}

	err := cache.garbageCollect(time.Minute)
	require.NoError(t, err)

	assert.Contains(t, cache.histogramResets, uint64(12345))
	assert.NotContains(t, cache.histogramResets, uint64(67890))

	cache.clear()
	assert.Empty(t, cache.histogramResets)
}
