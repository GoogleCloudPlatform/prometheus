// Copyright 2026 The Prometheus Authors
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

package export

import (
	"fmt"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb/chunks"
	"github.com/prometheus/prometheus/tsdb/record"
)

func setupHistogramBenchData(numSeries int, grouped bool) (seriesMap, MetadataFunc, [][]record.RefSample) {
	sMap := make(seriesMap)
	metadata := testMetadataFunc(metricMetadataMap{
		"http_request_duration_seconds": {Type: model.MetricTypeHistogram, Help: "Histogram help text"},
	})

	buckets := []string{"0.005", "0.01", "0.025", "0.05", "0.1", "0.25", "0.5", "1", "2.5", "5", "10", "+Inf"}
	samplesPerSeries := len(buckets) + 2

	batch0 := make([]record.RefSample, 0, numSeries*samplesPerSeries)
	batch1 := make([]record.RefSample, 0, numSeries*samplesPerSeries)

	for s := 0; s < numSeries; s++ {
		baseRef := storage.SeriesRef(s*samplesPerSeries + 1)
		lset := labels.FromStrings("job", "api-server", "instance", fmt.Sprintf("10.0.0.%d:8080", s%256), "handler", fmt.Sprintf("/api/v1/resource_%d", s))

		sMap[baseRef] = labels.NewBuilder(lset).Set("__name__", "http_request_duration_seconds_sum").Labels()
		sMap[baseRef+1] = labels.NewBuilder(lset).Set("__name__", "http_request_duration_seconds_count").Labels()
		for bIdx, le := range buckets {
			sMap[baseRef+storage.SeriesRef(2+bIdx)] = labels.NewBuilder(lset).Set("__name__", "http_request_duration_seconds_bucket").Set("le", le).Labels()
		}
	}

	if grouped {
		for s := 0; s < numSeries; s++ {
			baseRef := storage.SeriesRef(s*samplesPerSeries + 1)
			batch0 = append(batch0, record.RefSample{Ref: chunks.HeadSeriesRef(baseRef), T: 1000, V: float64(100 * (s + 1))})
			batch0 = append(batch0, record.RefSample{Ref: chunks.HeadSeriesRef(baseRef + 1), T: 1000, V: float64(10 * (s + 1))})
			for bIdx := range buckets {
				batch0 = append(batch0, record.RefSample{Ref: chunks.HeadSeriesRef(baseRef + storage.SeriesRef(2+bIdx)), T: 1000, V: float64((bIdx + 1) * (s + 1))})
			}

			batch1 = append(batch1, record.RefSample{Ref: chunks.HeadSeriesRef(baseRef), T: 2000, V: float64(200 * (s + 1))})
			batch1 = append(batch1, record.RefSample{Ref: chunks.HeadSeriesRef(baseRef + 1), T: 2000, V: float64(20 * (s + 1))})
			for bIdx := range buckets {
				batch1 = append(batch1, record.RefSample{Ref: chunks.HeadSeriesRef(baseRef + storage.SeriesRef(2+bIdx)), T: 2000, V: float64(2 * (bIdx + 1) * (s + 1))})
			}
		}
	} else {
		for s := 0; s < numSeries; s++ {
			baseRef := storage.SeriesRef(s*samplesPerSeries + 1)
			batch0 = append(batch0, record.RefSample{Ref: chunks.HeadSeriesRef(baseRef), T: 1000, V: float64(100 * (s + 1))})
			batch1 = append(batch1, record.RefSample{Ref: chunks.HeadSeriesRef(baseRef), T: 2000, V: float64(200 * (s + 1))})
		}
		for s := 0; s < numSeries; s++ {
			baseRef := storage.SeriesRef(s*samplesPerSeries + 1)
			batch0 = append(batch0, record.RefSample{Ref: chunks.HeadSeriesRef(baseRef + 1), T: 1000, V: float64(10 * (s + 1))})
			batch1 = append(batch1, record.RefSample{Ref: chunks.HeadSeriesRef(baseRef + 1), T: 2000, V: float64(20 * (s + 1))})
		}
		for bIdx := range buckets {
			for s := 0; s < numSeries; s++ {
				baseRef := storage.SeriesRef(s*samplesPerSeries + 1)
				batch0 = append(batch0, record.RefSample{Ref: chunks.HeadSeriesRef(baseRef + storage.SeriesRef(2+bIdx)), T: 1000, V: float64((bIdx + 1) * (s + 1))})
				batch1 = append(batch1, record.RefSample{Ref: chunks.HeadSeriesRef(baseRef + storage.SeriesRef(2+bIdx)), T: 2000, V: float64(2 * (bIdx + 1) * (s + 1))})
			}
		}
	}

	return sMap, metadata, [][]record.RefSample{batch0, batch1}
}

func benchHistograms(b *testing.B, numSeries int, grouped bool) {
	sMap, metadata, batches := setupHistogramBenchData(numSeries, grouped)
	externalLabels := labels.FromStrings("project_id", "test-project")
	cache := newSeriesCache(nil, nil, MetricTypePrefix)
	cache.getLabelsByRef = func(ref storage.SeriesRef) labels.Labels {
		return sMap[ref]
	}

	// Run initial batch to populate reset timestamps.
	b0 := newSampleBuilder(cache)
	batch := batches[0]
	for len(batch) > 0 {
		_, tail, err := b0.next(metadata, externalLabels, batch, nil)
		if err != nil {
			b.Fatal(err)
		}
		batch = tail
	}
	b0.close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sb := newSampleBuilder(cache)
		batch := batches[1]
		for len(batch) > 0 {
			_, tail, err := sb.next(metadata, externalLabels, batch, nil)
			if err != nil {
				b.Fatal(err)
			}
			batch = tail
		}
		sb.close()
	}
}

func BenchmarkSampleBuilder_HistogramsGrouped_10(b *testing.B)    { benchHistograms(b, 10, true) }
func BenchmarkSampleBuilder_HistogramsGrouped_100(b *testing.B)   { benchHistograms(b, 100, true) }
func BenchmarkSampleBuilder_HistogramsGrouped_500(b *testing.B)   { benchHistograms(b, 500, true) }
func BenchmarkSampleBuilder_HistogramsUngrouped_10(b *testing.B)  { benchHistograms(b, 10, false) }
func BenchmarkSampleBuilder_HistogramsUngrouped_100(b *testing.B) { benchHistograms(b, 100, false) }
func BenchmarkSampleBuilder_HistogramsUngrouped_500(b *testing.B) { benchHistograms(b, 500, false) }
