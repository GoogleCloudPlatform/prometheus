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

// SymbolsTable implements table for easy symbol use.
type SymbolsTable struct {
	strings    []string
	symbolsMap map[string]uint32
}

// NewSymbolTable returns a symbol table.
func NewSymbolTable() SymbolsTable {
	return SymbolsTable{
		// Empty string is required as a first element.
		symbolsMap: map[string]uint32{"": 0},
		strings:    []string{""},
	}
}

// Symbolize adds (if not added before) a string to the symbols table,
// while returning its reference number.
func (t *SymbolsTable) Symbolize(str string) uint32 {
	if ref, ok := t.symbolsMap[str]; ok {
		return ref
	}
	ref := uint32(len(t.strings))
	t.strings = append(t.strings, str)
	t.symbolsMap[str] = ref
	return ref
}

// Symbols returns computes symbols table to put in e.g. Request.Symbols.
// As per spec, order does not matter.
func (t *SymbolsTable) Symbols() []string {
	return t.strings
}

// Reset clears symbols table.
func (t *SymbolsTable) Reset() {
	// NOTE: Make sure to keep empty symbol.
	t.strings = t.strings[:1]
	for k := range t.symbolsMap {
		if k == "" {
			continue
		}
		delete(t.symbolsMap, k)
	}
}
