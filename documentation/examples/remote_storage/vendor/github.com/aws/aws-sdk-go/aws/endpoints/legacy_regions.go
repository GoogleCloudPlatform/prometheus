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

package endpoints

var legacyGlobalRegions = map[string]map[string]struct{}{
	"sts": {
		"ap-northeast-1": {},
		"ap-south-1":     {},
		"ap-southeast-1": {},
		"ap-southeast-2": {},
		"ca-central-1":   {},
		"eu-central-1":   {},
		"eu-north-1":     {},
		"eu-west-1":      {},
		"eu-west-2":      {},
		"eu-west-3":      {},
		"sa-east-1":      {},
		"us-east-1":      {},
		"us-east-2":      {},
		"us-west-1":      {},
		"us-west-2":      {},
	},
	"s3": {
		"us-east-1": {},
	},
}
