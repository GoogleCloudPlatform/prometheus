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

package v4

import (
	"encoding/hex"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
)

type credentialValueProvider interface {
	Get() (credentials.Value, error)
}

// StreamSigner implements signing of event stream encoded payloads
type StreamSigner struct {
	region  string
	service string

	credentials credentialValueProvider

	prevSig []byte
}

// NewStreamSigner creates a SigV4 signer used to sign Event Stream encoded messages
func NewStreamSigner(region, service string, seedSignature []byte, credentials *credentials.Credentials) *StreamSigner {
	return &StreamSigner{
		region:      region,
		service:     service,
		credentials: credentials,
		prevSig:     seedSignature,
	}
}

// GetSignature takes an event stream encoded headers and payload and returns a signature
func (s *StreamSigner) GetSignature(headers, payload []byte, date time.Time) ([]byte, error) {
	credValue, err := s.credentials.Get()
	if err != nil {
		return nil, err
	}

	sigKey := deriveSigningKey(s.region, s.service, credValue.SecretAccessKey, date)

	keyPath := buildSigningScope(s.region, s.service, date)

	stringToSign := buildEventStreamStringToSign(headers, payload, s.prevSig, keyPath, date)

	signature := hmacSHA256(sigKey, []byte(stringToSign))
	s.prevSig = signature

	return signature, nil
}

func buildEventStreamStringToSign(headers, payload, prevSig []byte, scope string, date time.Time) string {
	return strings.Join([]string{
		"AWS4-HMAC-SHA256-PAYLOAD",
		formatTime(date),
		scope,
		hex.EncodeToString(prevSig),
		hex.EncodeToString(hashSHA256(headers)),
		hex.EncodeToString(hashSHA256(payload)),
	}, "\n")
}
