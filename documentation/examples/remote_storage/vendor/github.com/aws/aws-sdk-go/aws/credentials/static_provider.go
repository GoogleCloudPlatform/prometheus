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

package credentials

import (
	"github.com/aws/aws-sdk-go/aws/awserr"
)

// StaticProviderName provides a name of Static provider
const StaticProviderName = "StaticProvider"

var (
	// ErrStaticCredentialsEmpty is emitted when static credentials are empty.
	ErrStaticCredentialsEmpty = awserr.New("EmptyStaticCreds", "static credentials are empty", nil)
)

// A StaticProvider is a set of credentials which are set programmatically,
// and will never expire.
type StaticProvider struct {
	Value
}

// NewStaticCredentials returns a pointer to a new Credentials object
// wrapping a static credentials value provider. Token is only required
// for temporary security credentials retrieved via STS, otherwise an empty
// string can be passed for this parameter.
func NewStaticCredentials(id, secret, token string) *Credentials {
	return NewCredentials(&StaticProvider{Value: Value{
		AccessKeyID:     id,
		SecretAccessKey: secret,
		SessionToken:    token,
	}})
}

// NewStaticCredentialsFromCreds returns a pointer to a new Credentials object
// wrapping the static credentials value provide. Same as NewStaticCredentials
// but takes the creds Value instead of individual fields
func NewStaticCredentialsFromCreds(creds Value) *Credentials {
	return NewCredentials(&StaticProvider{Value: creds})
}

// Retrieve returns the credentials or error if the credentials are invalid.
func (s *StaticProvider) Retrieve() (Value, error) {
	if s.AccessKeyID == "" || s.SecretAccessKey == "" {
		return Value{ProviderName: StaticProviderName}, ErrStaticCredentialsEmpty
	}

	if len(s.Value.ProviderName) == 0 {
		s.Value.ProviderName = StaticProviderName
	}
	return s.Value, nil
}

// IsExpired returns if the credentials are expired.
//
// For StaticProvider, the credentials never expired.
func (s *StaticProvider) IsExpired() bool {
	return false
}
