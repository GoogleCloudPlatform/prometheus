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

package shareddefaults

const (
	// ECSCredsProviderEnvVar is an environmental variable key used to
	// determine which path needs to be hit.
	ECSCredsProviderEnvVar = "AWS_CONTAINER_CREDENTIALS_RELATIVE_URI"
)

// ECSContainerCredentialsURI is the endpoint to retrieve container
// credentials. This can be overridden to test to ensure the credential process
// is behaving correctly.
var ECSContainerCredentialsURI = "http://169.254.170.2"
