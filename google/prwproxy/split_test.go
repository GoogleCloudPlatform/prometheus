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
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
)

// transformedRequest returns a request like Transform produces for a scrape
// with an untyped metric, a histogram and a counter with exemplars, and a gauge.
func transformedRequest() *writev2.Request {
	syms := writev2.NewSymbolTable()
	// Transform renames untyped series, which leaves their original name in the
	// symbols table without any series referencing it.
	syms.Symbolize("mysql_slow_queries")

	lbls := func(name string) []uint32 {
		return syms.SymbolizeLabels(labels.FromStrings(labels.MetricName, name, "instance", "db:9104"), nil)
	}
	traceID := func(id string) []uint32 {
		return syms.SymbolizeLabels(labels.FromStrings("trace_id", id), nil)
	}
	slowQueriesHelp := syms.Symbolize("Number of slow queries.")

	req := &writev2.Request{
		Timeseries: []writev2.TimeSeries{
			{
				LabelsRefs: lbls("mysql_slow_queries/unknown"),
				Samples:    []writev2.Sample{{Value: 15, Timestamp: 2000}},
				Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE, HelpRef: slowQueriesHelp},
			},
			{
				LabelsRefs: lbls("mysql_slow_queries/unknown:counter"),
				Samples:    []writev2.Sample{{Value: 5, Timestamp: 2000, StartTimestamp: 1000}},
				Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_COUNTER, HelpRef: slowQueriesHelp},
			},
			{
				LabelsRefs: lbls("http_request_duration_seconds"),
				Histograms: []writev2.Histogram{writev2.FromIntHistogram(2000, &histogram.Histogram{
					Count:           3,
					Sum:             0.9,
					ZeroThreshold:   0.001,
					PositiveSpans:   []histogram.Span{{Offset: 0, Length: 2}},
					PositiveBuckets: []int64{1, 1},
				})},
				Exemplars: []writev2.Exemplar{{LabelsRefs: traceID("4bf92f3577b34da6"), Value: 0.3, Timestamp: 1990}},
				Metadata: writev2.Metadata{
					Type:    writev2.Metadata_METRIC_TYPE_HISTOGRAM,
					HelpRef: syms.Symbolize("Request latency."),
					UnitRef: syms.Symbolize("seconds"),
				},
			},
			{
				LabelsRefs: lbls("http_requests_total"),
				Samples:    []writev2.Sample{{Value: 100, Timestamp: 2000, StartTimestamp: 500}},
				Exemplars:  []writev2.Exemplar{{LabelsRefs: traceID("00f067aa0ba902b7"), Value: 1, Timestamp: 1995}},
				Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_COUNTER, HelpRef: syms.Symbolize("Number of requests.")},
			},
			{
				LabelsRefs: lbls("go_goroutines"),
				Samples:    []writev2.Sample{{Value: 42, Timestamp: 2000}},
				Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE, HelpRef: syms.Symbolize("Number of goroutines.")},
			},
		},
	}
	req.Symbols = syms.Symbols()
	return req
}

// decodedSeries is a writev2.TimeSeries with all symbol references resolved.
type decodedSeries struct {
	labels     labels.Labels
	metadata   metadata.Metadata
	exemplars  []exemplar.Exemplar
	samples    []writev2.Sample
	histograms []writev2.Histogram
}

func desymbolize(t *testing.T, reqs ...*writev2.Request) []decodedSeries {
	t.Helper()

	var (
		b   = labels.NewScratchBuilder(0)
		out []decodedSeries
	)
	for _, req := range reqs {
		for _, ts := range req.Timeseries {
			lset, err := ts.ToLabels(&b, req.Symbols)
			require.NoError(t, err)
			md, err := ts.ToMetadata(req.Symbols)
			require.NoError(t, err)

			d := decodedSeries{labels: lset, metadata: md, samples: ts.Samples, histograms: ts.Histograms}
			for _, e := range ts.Exemplars {
				ex, err := e.ToExemplar(&b, req.Symbols)
				require.NoError(t, err)
				d.exemplars = append(d.exemplars, ex)
			}
			out = append(out, d)
		}
	}
	return out
}

func TestSplitRequest(t *testing.T) {
	in := transformedRequest()

	for _, tc := range []struct {
		maxSeries int
		wantSizes []int
	}{
		{maxSeries: 0, wantSizes: []int{5}},
		{maxSeries: 5, wantSizes: []int{5}},
		{maxSeries: 200, wantSizes: []int{5}},
		{maxSeries: 4, wantSizes: []int{4, 1}},
		{maxSeries: 2, wantSizes: []int{2, 2, 1}},
		{maxSeries: 1, wantSizes: []int{1, 1, 1, 1, 1}},
	} {
		t.Run(fmt.Sprintf("max_series=%d", tc.maxSeries), func(t *testing.T) {
			got, err := splitRequest(in, tc.maxSeries)
			require.NoError(t, err)

			sizes := make([]int, 0, len(got))
			for _, req := range got {
				sizes = append(sizes, len(req.Timeseries))
			}
			require.Equal(t, tc.wantSizes, sizes)
			if len(got) == 1 {
				// A request that fits is forwarded as is.
				require.Same(t, in, got[0])
				return
			}
			// Nothing is lost, changed or reordered.
			require.Equal(t, desymbolize(t, in), desymbolize(t, got...))
		})
	}
}

func TestSplitRequest_MinimalSymbols(t *testing.T) {
	got, err := splitRequest(transformedRequest(), 2)
	require.NoError(t, err)
	require.Len(t, got, 3)

	// Each request only carries the symbols its own series reference.
	for i, want := range [][]string{
		{"", "__name__", "instance", "db:9104", "mysql_slow_queries/unknown", "mysql_slow_queries/unknown:counter", "Number of slow queries."},
		{"", "__name__", "instance", "db:9104", "http_request_duration_seconds", "Request latency.", "seconds", "trace_id", "4bf92f3577b34da6", "http_requests_total", "Number of requests.", "00f067aa0ba902b7"},
		{"", "__name__", "instance", "db:9104", "go_goroutines", "Number of goroutines."},
	} {
		require.ElementsMatch(t, want, got[i].Symbols, "request %d", i)
		// PRW2 requires the empty string to be the first symbol.
		require.Empty(t, got[i].Symbols[0], "request %d", i)
	}
}

func TestSplitRequest_InvalidSymbolReference(t *testing.T) {
	in := transformedRequest()
	in.Timeseries[3].Exemplars[0].LabelsRefs[1] = uint32(len(in.Symbols))

	_, err := splitRequest(in, 2)
	require.EqualError(t, err, fmt.Sprintf("series 3: symbol reference %d outside of symbols table (size %d)", len(in.Symbols), len(in.Symbols)))
}
