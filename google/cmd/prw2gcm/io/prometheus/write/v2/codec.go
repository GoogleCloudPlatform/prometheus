// Copyright 2024 Google LLC
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

// Copyright 2024 Prometheus Team
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

package writev2

import (
	"fmt"
	"strings"
)

// LabelsToString formats labels into a PromQL-like string without panicking on malformed symbol references.
func (m *TimeSeries) LabelsToString(symbols []string) string {
	if m == nil {
		return "{}"
	}
	lookup := func(ref uint32) string {
		if uint64(ref) >= uint64(len(symbols)) {
			return fmt.Sprintf("<invalid ref %d>", ref)
		}
		return symbols[ref]
	}

	var (
		name string
		lbls []string
	)
	for i := 0; i+1 < len(m.LabelsRefs); i += 2 {
		n, v := lookup(m.LabelsRefs[i]), lookup(m.LabelsRefs[i+1])
		if n == "__name__" {
			name = v
			continue
		}
		// TODO: Quote UTF-8.
		lbls = append(lbls, fmt.Sprintf("%v=%q", n, v))
	}
	if len(m.LabelsRefs)%2 != 0 {
		lbls = append(lbls, fmt.Sprintf("<dangling ref %d>", m.LabelsRefs[len(m.LabelsRefs)-1]))
	}

	b := strings.Builder{}
	if name != "" {
		b.WriteString(name)
	}
	b.WriteString("{")
	b.WriteString(strings.Join(lbls, ", "))
	b.WriteString("}")
	return b.String()
}
