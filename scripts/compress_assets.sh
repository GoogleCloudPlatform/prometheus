#!/usr/bin/env bash
# Copyright 2026 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#
# compress static assets

set -euo pipefail

cd web/ui
cp embed.go.tmpl embed.go

GZIP_OPTS="-fk"
# gzip option '-k' may not always exist in the latest gzip available on different distros.
if ! gzip -k -h &>/dev/null; then GZIP_OPTS="-f"; fi

find static -type f -name '*.gz' -delete
find static -type f -exec gzip $GZIP_OPTS '{}' \; -print0 | xargs -0 -I % echo %.gz | xargs echo //go:embed >> embed.go
echo var EmbedFS embed.FS >> embed.go
