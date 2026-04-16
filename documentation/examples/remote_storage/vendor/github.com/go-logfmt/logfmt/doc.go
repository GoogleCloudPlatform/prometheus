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

// Package logfmt implements utilities to marshal and unmarshal data in the
// logfmt format. The logfmt format records key/value pairs in a way that
// balances readability for humans and simplicity of computer parsing. It is
// most commonly used as a more human friendly alternative to JSON for
// structured logging.
package logfmt
