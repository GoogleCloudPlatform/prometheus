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

//go:build !go1.8
// +build !go1.8

package aws

import (
	"net/url"
	"strings"
)

// URLHostname will extract the Hostname without port from the URL value.
//
// Copy of Go 1.8's net/url#URL.Hostname functionality.
func URLHostname(url *url.URL) string {
	return stripPort(url.Host)

}

// stripPort is copy of Go 1.8 url#URL.Hostname functionality.
// https://golang.org/src/net/url/url.go
func stripPort(hostport string) string {
	colon := strings.IndexByte(hostport, ':')
	if colon == -1 {
		return hostport
	}
	if i := strings.IndexByte(hostport, ']'); i != -1 {
		return strings.TrimPrefix(hostport[:i], "[")
	}
	return hostport[:colon]
}
