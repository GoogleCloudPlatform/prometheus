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

package sdkuri

import (
	"path"
	"strings"
)

// PathJoin will join the elements of the path delimited by the "/"
// character. Similar to path.Join with the exception the trailing "/"
// character is preserved if present.
func PathJoin(elems ...string) string {
	if len(elems) == 0 {
		return ""
	}

	hasTrailing := strings.HasSuffix(elems[len(elems)-1], "/")
	str := path.Join(elems...)
	if hasTrailing && str != "/" {
		str += "/"
	}

	return str
}
