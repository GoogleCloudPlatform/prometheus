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

	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
)

// splitRequest splits req into requests with at most maxSeries time series
// each, keeping the order of the series. Each of them gets its own symbols
// table with only the symbols its series reference. A request that fits is
// returned as is.
//
// This is needed because Transform can grow a request past the downstream
// limit. For example, Prometheus sends one series per sample, so a batch of
// 200 samples with 97 untyped floats becomes 297 series once the untyped series
// are split into a gauge and a counter stream, which GCM rejects with "Request
// has 297 timeseries. Maximum allowed is 200.".
func splitRequest(req *writev2.Request, maxSeries int) ([]*writev2.Request, error) {
	if maxSeries <= 0 || len(req.Timeseries) <= maxSeries {
		return []*writev2.Request{req}, nil
	}

	out := make([]*writev2.Request, 0, (len(req.Timeseries)+maxSeries-1)/maxSeries)
	for start := 0; start < len(req.Timeseries); start += maxSeries {
		sub, err := subRequest(req, start, min(start+maxSeries, len(req.Timeseries)))
		if err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, nil
}

// subRequest returns a request with the series req.Timeseries[start:end],
// re-symbolized against a new symbols table.
func subRequest(req *writev2.Request, start, end int) (*writev2.Request, error) {
	m := &symbolRemapper{from: req.Symbols, to: writev2.NewSymbolTable()}
	out := make([]writev2.TimeSeries, 0, end-start)
	for i := start; i < end; i++ {
		// Samples and histograms don't reference symbols, so a shallow copy
		// can share them with req.
		ts := req.Timeseries[i]
		ts.LabelsRefs = m.refs(ts.LabelsRefs)
		ts.Metadata.HelpRef = m.ref(ts.Metadata.HelpRef)
		ts.Metadata.UnitRef = m.ref(ts.Metadata.UnitRef)
		if len(ts.Exemplars) > 0 {
			exemplars := make([]writev2.Exemplar, len(ts.Exemplars))
			for j, e := range ts.Exemplars {
				e.LabelsRefs = m.refs(e.LabelsRefs)
				exemplars[j] = e
			}
			ts.Exemplars = exemplars
		}
		if m.err != nil {
			return nil, fmt.Errorf("series %d: %w", i, m.err)
		}
		out = append(out, ts)
	}
	return &writev2.Request{Symbols: m.to.Symbols(), Timeseries: out}, nil
}

// symbolRemapper re-points references into the symbols table from at the
// symbols table to, adding the referenced symbols to it. The first invalid
// reference is recorded in err, after which the returned references must not
// be used.
type symbolRemapper struct {
	from []string
	to   writev2.SymbolsTable
	err  error
}

func (m *symbolRemapper) ref(ref uint32) uint32 {
	if int(ref) >= len(m.from) {
		if m.err == nil {
			m.err = fmt.Errorf("symbol reference %d outside of symbols table (size %d)", ref, len(m.from))
		}
		return 0
	}
	return m.to.Symbolize(m.from[ref])
}

func (m *symbolRemapper) refs(refs []uint32) []uint32 {
	if len(refs) == 0 {
		return nil
	}
	out := make([]uint32, len(refs))
	for i, ref := range refs {
		out[i] = m.ref(ref)
	}
	return out
}
