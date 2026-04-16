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

//go:build !go1.6
// +build !go1.6

package sdkrand

import "math/rand"

// Read backfills Go 1.6's math.Rand.Reader for Go 1.5
func Read(r *rand.Rand, p []byte) (n int, err error) {
	// Copy of Go standard libraries math package's read function not added to
	// standard library until Go 1.6.
	var pos int8
	var val int64
	for n = 0; n < len(p); n++ {
		if pos == 0 {
			val = r.Int63()
			pos = 7
		}
		p[n] = byte(val)
		val >>= 8
		pos--
	}

	return n, err
}
