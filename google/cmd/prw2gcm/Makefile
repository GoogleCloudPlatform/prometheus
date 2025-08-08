# Copyright 2025 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https:#www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

.PHONY: proto
proto: ## Regenerate Go from proto
proto: $(BUF)
	@echo ">> regenerating Prometheus proto"
	@$(BUF) generate
	@# For some reasons buf generates this unused import, kill it manually for now and reformat.
	@find io/prometheus/write/v2/ -type f -exec sed -i '' 's/_ "github.com\/gogo\/protobuf\/gogoproto"#g' {} \;
	@go fmt ./io/prometheus/write/v2/...

BUF = buf
$(BUF):
	@go install github.com/bufbuild/buf/cmd/buf@v1.39.0

GCM_SECRET?=
.PHONY: test
test:  ## Run special proxy unit tests that will use GCM if GCM_SECRET is present.
ifneq ($(GCM_SECRET),)
	go test -tags gcme2e -v ./...
else
	@echo "Secret not provided, skipping!"
endif
