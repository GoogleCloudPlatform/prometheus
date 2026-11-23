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

package main

import (
	"math"
	"net/http"
	"testing"

	monitoring_pb "cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	writev2 "github.com/prometheus/prometheus/google/cmd/prw2gcm/io/prometheus/write/v2"
	"github.com/stretchr/testify/require"
)

var _ gcmQueue = (*recordingQueue)(nil)

type recordingQueue struct {
	series  []*monitoring_pb.TimeSeries
	flushes int
}

func (q *recordingQueue) Enqueue(ts *monitoring_pb.TimeSeries) {
	q.series = append(q.series, ts)
}

func (q *recordingQueue) Flush() {
	q.flushes++
}

func TestConvertInvalidSymbolRefs(t *testing.T) {
	validMeta := &writev2.Metadata{
		Type: writev2.Metadata_METRIC_TYPE_GAUGE,
	}
	validSample := []*writev2.Sample{{Timestamp: 1000, Value: 1}}

	cases := []struct {
		name string
		req  *writev2.Request
	}{
		{
			name: "oob_label_name_ref",
			req: &writev2.Request{
				Symbols: []string{""},
				Timeseries: []*writev2.TimeSeries{{
					LabelsRefs: []uint32{5, 0},
					Metadata:   validMeta,
					Samples:    validSample,
				}},
			},
		},
		{
			name: "oob_label_value_ref",
			req: &writev2.Request{
				Symbols: []string{"", "__name__"},
				Timeseries: []*writev2.TimeSeries{{
					LabelsRefs: []uint32{1, 99},
					Metadata:   validMeta,
					Samples:    validSample,
				}},
			},
		},
		{
			name: "max_uint32_label_ref",
			req: &writev2.Request{
				Symbols: []string{"", "__name__", "up"},
				Timeseries: []*writev2.TimeSeries{{
					LabelsRefs: []uint32{math.MaxUint32, 2},
					Metadata:   validMeta,
					Samples:    validSample,
				}},
			},
		},
		{
			name: "empty_symbols_table",
			req: &writev2.Request{
				Symbols: nil,
				Timeseries: []*writev2.TimeSeries{{
					LabelsRefs: []uint32{0, 0},
					Metadata:   validMeta,
					Samples:    validSample,
				}},
			},
		},
		{
			name: "odd_labels_refs_len_1",
			req: &writev2.Request{
				Symbols: []string{"", "__name__"},
				Timeseries: []*writev2.TimeSeries{{
					LabelsRefs: []uint32{1},
					Metadata:   validMeta,
					Samples:    validSample,
				}},
			},
		},
		{
			name: "odd_labels_refs_len_3",
			req: &writev2.Request{
				Symbols: []string{"", "__name__", "up"},
				Timeseries: []*writev2.TimeSeries{{
					LabelsRefs: []uint32{1, 2, 1},
					Metadata:   validMeta,
					Samples:    validSample,
				}},
			},
		},
		{
			name: "oob_help_ref",
			req: &writev2.Request{
				Symbols: []string{"", "__name__", "up"},
				Timeseries: []*writev2.TimeSeries{{
					LabelsRefs: []uint32{1, 2},
					Metadata: &writev2.Metadata{
						Type:    writev2.Metadata_METRIC_TYPE_GAUGE,
						HelpRef: 9999,
					},
					Samples: validSample,
				}},
			},
		},
		{
			name: "oob_unit_ref",
			req: &writev2.Request{
				Symbols: []string{"", "__name__", "up"},
				Timeseries: []*writev2.TimeSeries{{
					LabelsRefs: []uint32{1, 2},
					Metadata: &writev2.Metadata{
						Type:    writev2.Metadata_METRIC_TYPE_GAUGE,
						UnitRef: math.MaxUint32,
					},
					Samples: validSample,
				}},
			},
		},
		{
			name: "nil_timeseries",
			req: &writev2.Request{
				Symbols:    []string{""},
				Timeseries: []*writev2.TimeSeries{nil},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := &recordingQueue{}
			stats, err := Convert(t.Context(), tc.req, q)
			require.Error(t, err)
			require.Equal(t, http.StatusBadRequest, httpCodeFromErrorOr500(err))
			require.Zero(t, stats.Samples)
			require.Empty(t, q.series)
		})
	}
}

func TestConvertValidAndUnevenSamples(t *testing.T) {
	req := &writev2.Request{
		Symbols: []string{"", "__name__", "up", "requests", "help text", "seconds"},
		Timeseries: []*writev2.TimeSeries{
			{
				LabelsRefs: []uint32{1, 2},
				Metadata: &writev2.Metadata{
					Type:    writev2.Metadata_METRIC_TYPE_GAUGE,
					HelpRef: 4,
					UnitRef: 5,
				},
				Samples: []*writev2.Sample{
					{Timestamp: 1000, Value: 1},
					{Timestamp: 2000, Value: 2},
				},
			},
			{
				LabelsRefs: []uint32{1, 3},
				Metadata: &writev2.Metadata{
					Type: writev2.Metadata_METRIC_TYPE_GAUGE,
				},
				Samples: []*writev2.Sample{
					{Timestamp: 1000, Value: 10},
				},
			},
		},
	}

	q := &recordingQueue{}
	stats, err := Convert(t.Context(), req, q)
	require.NoError(t, err)
	require.Equal(t, 3, stats.Samples)
	require.Len(t, q.series, 3)
	require.Equal(t, 2, q.flushes)
	require.Equal(t, "help text", q.series[0].Description)
	require.Equal(t, "seconds", q.series[0].Unit)
}
