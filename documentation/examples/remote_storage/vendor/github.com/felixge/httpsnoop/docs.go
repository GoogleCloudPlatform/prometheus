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

// Package httpsnoop provides an easy way to capture http related metrics (i.e.
// response time, bytes written, and http status code) from your application's
// http.Handlers.
//
// Doing this requires non-trivial wrapping of the http.ResponseWriter
// interface, which is also exposed for users interested in a more low-level
// API.
package httpsnoop

//go:generate go run codegen/main.go
