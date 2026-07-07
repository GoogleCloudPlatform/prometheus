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
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb/chunks"
	"github.com/prometheus/prometheus/tsdb/record"
)

type fixedLease struct {
	start time.Time
	end   time.Time
}

func (l *fixedLease) Range() (time.Time, time.Time, bool) {
	return l.start, l.end, true
}
func (l *fixedLease) Run(ctx context.Context) {
	<-ctx.Done()
}
func (l *fixedLease) OnLeaderChange(func()) {}

func setupHistogramBenchDataV2(numHistograms int, numSeriesPerHist int, grouped bool) (seriesMap, MetadataFunc, [][]record.RefSample) {
	sMap := make(seriesMap)
	metadataMap := make(metricMetadataMap)

	buckets := []string{"0.005", "0.01", "0.025", "0.05", "0.1", "0.25", "0.5", "1", "2.5", "5", "10", "+Inf"}
	samplesPerSeries := len(buckets) + 2

	totalSeries := numHistograms * numSeriesPerHist
	totalPromSeries := totalSeries * samplesPerSeries
	batch0 := make([]record.RefSample, 0, totalPromSeries)
	batch1 := make([]record.RefSample, 0, totalPromSeries)

	for h := 0; h < numHistograms; h++ {
		metricName := fmt.Sprintf("http_request_duration_seconds_%d", h)
		metadataMap[metricName] = MetricMetadata{Type: model.MetricTypeHistogram, Help: "Histogram help text"}

		for s := 0; s < numSeriesPerHist; s++ {
			seriesIdx := h*numSeriesPerHist + s
			baseRef := storage.SeriesRef(seriesIdx*samplesPerSeries + 1)
			lset := labels.FromStrings(
				"job", "api-server",
				"instance", fmt.Sprintf("10.0.0.%d:8080", seriesIdx%256),
				"handler", fmt.Sprintf("/api/v1/resource_%d", seriesIdx),
			)

			sMap[baseRef] = labels.NewBuilder(lset).Set("__name__", metricName+"_sum").Labels()
			sMap[baseRef+1] = labels.NewBuilder(lset).Set("__name__", metricName+"_count").Labels()
			for bIdx, le := range buckets {
				sMap[baseRef+storage.SeriesRef(2+bIdx)] = labels.NewBuilder(lset).Set("__name__", metricName+"_bucket").Set("le", le).Labels()
			}
		}
	}

	metadata := testMetadataFunc(metadataMap)

	if grouped {
		for h := 0; h < numHistograms; h++ {
			for s := 0; s < numSeriesPerHist; s++ {
				seriesIdx := h*numSeriesPerHist + s
				baseRef := storage.SeriesRef(seriesIdx*samplesPerSeries + 1)
				batch0 = append(batch0, record.RefSample{Ref: chunks.HeadSeriesRef(baseRef), T: 1000, V: float64(100 * (seriesIdx + 1))})
				batch0 = append(batch0, record.RefSample{Ref: chunks.HeadSeriesRef(baseRef + 1), T: 1000, V: float64(10 * (seriesIdx + 1))})
				for bIdx := range buckets {
					batch0 = append(batch0, record.RefSample{Ref: chunks.HeadSeriesRef(baseRef + storage.SeriesRef(2+bIdx)), T: 1000, V: float64((bIdx + 1) * (seriesIdx + 1))})
				}

				batch1 = append(batch1, record.RefSample{Ref: chunks.HeadSeriesRef(baseRef), T: 2000, V: float64(200 * (seriesIdx + 1))})
				batch1 = append(batch1, record.RefSample{Ref: chunks.HeadSeriesRef(baseRef + 1), T: 2000, V: float64(20 * (seriesIdx + 1))})
				for bIdx := range buckets {
					batch1 = append(batch1, record.RefSample{Ref: chunks.HeadSeriesRef(baseRef + storage.SeriesRef(2+bIdx)), T: 2000, V: float64(2 * (bIdx + 1) * (seriesIdx + 1))})
				}
			}
		}
	} else {
		for h := 0; h < numHistograms; h++ {
			for s := 0; s < numSeriesPerHist; s++ {
				seriesIdx := h*numSeriesPerHist + s
				baseRef := storage.SeriesRef(seriesIdx*samplesPerSeries + 1)
				batch0 = append(batch0, record.RefSample{Ref: chunks.HeadSeriesRef(baseRef), T: 1000, V: float64(100 * (seriesIdx + 1))})
				batch1 = append(batch1, record.RefSample{Ref: chunks.HeadSeriesRef(baseRef), T: 2000, V: float64(200 * (seriesIdx + 1))})
			}
			for s := 0; s < numSeriesPerHist; s++ {
				seriesIdx := h*numSeriesPerHist + s
				baseRef := storage.SeriesRef(seriesIdx*samplesPerSeries + 1)
				batch0 = append(batch0, record.RefSample{Ref: chunks.HeadSeriesRef(baseRef + 1), T: 1000, V: float64(10 * (seriesIdx + 1))})
				batch1 = append(batch1, record.RefSample{Ref: chunks.HeadSeriesRef(baseRef + 1), T: 2000, V: float64(20 * (seriesIdx + 1))})
			}
			for bIdx := range buckets {
				for s := 0; s < numSeriesPerHist; s++ {
					seriesIdx := h*numSeriesPerHist + s
					baseRef := storage.SeriesRef(seriesIdx*samplesPerSeries + 1)
					batch0 = append(batch0, record.RefSample{Ref: chunks.HeadSeriesRef(baseRef + storage.SeriesRef(2+bIdx)), T: 1000, V: float64((bIdx + 1) * (seriesIdx + 1))})
					batch1 = append(batch1, record.RefSample{Ref: chunks.HeadSeriesRef(baseRef + storage.SeriesRef(2+bIdx)), T: 2000, V: float64(2 * (bIdx + 1) * (seriesIdx + 1))})
				}
			}
		}
	}

	return sMap, metadata, [][]record.RefSample{batch0, batch1}
}

func benchExportHistograms(b *testing.B, numHistograms int, numSeriesPerHist int, grouped bool) {
	sMap, metadata, batches := setupHistogramBenchDataV2(numHistograms, numSeriesPerHist, grouped)

	opts := ExporterOpts{
		ProjectID: "test-project",
		Location:  "us-central1",
		Cluster:   "test-cluster",
	}
	opts.DefaultUnsetFields()

	// Use a fixed lease that is always active but covers a time range in the past
	// (0 to 0). Since our samples have T=1000 and T=2000, they will be out of range
	// and won't be enqueued, avoiding the queueing overhead and background sends.
	lease := &fixedLease{
		start: time.Unix(0, 0),
		end:   time.Unix(0, 0),
	}

	ctx := context.Background()
	exporter, err := New(ctx, log.NewNopLogger(), nil, opts, lease)
	if err != nil {
		b.Fatal(err)
	}
	exporter.SetLabelsByIDFunc(func(ref storage.SeriesRef) labels.Labels {
		return sMap[ref]
	})

	if err := exporter.ApplyConfig(&config.Config{}); err != nil {
		b.Fatal(err)
	}

	// Run initial batch to populate reset timestamps.
	exporter.Export(metadata, batches[0], nil)

	b.ResetTimer()
	b.ReportAllocs()
	var t int64 = 2000
	old := testutil.ToFloat64(samplesBuilt)
	for b.Loop() {
		for i := range batches[1] {
			batches[1][i].T = t
		}
		exporter.Export(metadata, batches[1], nil)
		if diff := testutil.ToFloat64(samplesBuilt) - old; diff != float64(numHistograms*numSeriesPerHist) {
			b.Fatalf("unexpected number of samples, got %v, expected %v", diff, numHistograms*numSeriesPerHist)
		}
		old = testutil.ToFloat64(samplesBuilt)
		t += 1000
	}
}

/*
	export bench=after && go test ./google/export/... \
		 -run '^$' -bench '^BenchmarkExport_Histograms' \
		 -benchtime 2s -count 6 -cpu 2 -benchmem -timeout 999m \
	 | tee ${bench}.txt
*/
func BenchmarkExport_Histograms(b *testing.B) {
	numHistograms := 10
	for _, tc := range []struct {
		name      string
		numSeries int
		grouped   bool
	}{
		{name: "Grouped", numSeries: 1, grouped: true},
		{name: "Grouped", numSeries: 100, grouped: true},
		{name: "Grouped", numSeries: 1000, grouped: true},
		{name: "Ungrouped", numSeries: 1, grouped: false},
		{name: "Ungrouped", numSeries: 100, grouped: false},
		{name: "Ungrouped", numSeries: 1000, grouped: false},
	} {
		b.Run(fmt.Sprintf("case=%s_%d_series", tc.name, tc.numSeries), func(b *testing.B) {
			benchExportHistograms(b, numHistograms, tc.numSeries, tc.grouped)
		})
	}
}
