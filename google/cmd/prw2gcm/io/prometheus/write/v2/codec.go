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

// LabelsToString return labels into PromQL-like readable string.
func (m *TimeSeries) LabelsToString(symbols []string) string {
	var (
		name string
		lbls []string
	)
	for i := 0; i < len(m.LabelsRefs); i += 2 {
		n, v := symbols[m.LabelsRefs[i]], symbols[m.LabelsRefs[i+1]]
		if n == "__name__" {
			name = v
			continue
		}
		// TODO: Quote UTF-8.
		lbls = append(lbls, fmt.Sprintf("%v=%q", n, v))
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
