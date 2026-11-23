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

package writev2

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTimeSeriesLabelsToString(t *testing.T) {
	cases := []struct {
		name    string
		ts      *TimeSeries
		symbols []string
		want    string
	}{
		{
			name:    "nil_timeseries",
			ts:      nil,
			symbols: []string{""},
			want:    "{}",
		},
		{
			name:    "valid_labels",
			ts:      &TimeSeries{LabelsRefs: []uint32{1, 2, 3, 4}},
			symbols: []string{"", "__name__", "up", "job", "api"},
			want:    `up{job="api"}`,
		},
		{
			name:    "oob_refs_and_empty_symbols",
			ts:      &TimeSeries{LabelsRefs: []uint32{5, math.MaxUint32}},
			symbols: nil,
			want:    `{<invalid ref 5>="<invalid ref 4294967295>"}`,
		},
		{
			name:    "odd_labels_refs",
			ts:      &TimeSeries{LabelsRefs: []uint32{1, 2, 3}},
			symbols: []string{"", "__name__", "up", "job"},
			want:    `up{<dangling ref 3>}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.ts.LabelsToString(tc.symbols))
		})
	}
}
