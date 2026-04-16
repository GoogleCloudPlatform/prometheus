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

package request

import (
	"io"
	"net/http"
	"net/url"
)

func copyHTTPRequest(r *http.Request, body io.ReadCloser) *http.Request {
	req := new(http.Request)
	*req = *r
	req.URL = &url.URL{}
	*req.URL = *r.URL
	req.Body = body

	req.Header = http.Header{}
	for k, v := range r.Header {
		for _, vv := range v {
			req.Header.Add(k, vv)
		}
	}

	return req
}
