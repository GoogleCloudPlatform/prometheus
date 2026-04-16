// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build appengine
// +build appengine

// This file contains the safe implementations of otherwise unsafe-using code.

package xxhash

// Sum64String computes the 64-bit xxHash digest of s.
func Sum64String(s string) uint64 {
	return Sum64([]byte(s))
}

// WriteString adds more data to d. It always returns len(s), nil.
func (d *Digest) WriteString(s string) (n int, err error) {
	return d.Write([]byte(s))
}
