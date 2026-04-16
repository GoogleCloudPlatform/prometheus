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

package client

import (
	"encoding/json"
	"time"
)

type (
	// Identifier is an identifier value.
	Identifier string

	// StringValue is a string literal.
	StringValue string

	// RegexValue is a regexp literal.
	RegexValue string

	// NumberValue is a number literal.
	NumberValue float64

	// IntegerValue is an integer literal.
	IntegerValue int64

	// BooleanValue is a boolean literal.
	BooleanValue bool

	// TimeValue is a time literal.
	TimeValue time.Time

	// DurationValue is a duration literal.
	DurationValue time.Duration
)

func (v Identifier) MarshalJSON() ([]byte, error) {
	m := map[string]string{"identifier": string(v)}
	return json.Marshal(m)
}

func (v StringValue) MarshalJSON() ([]byte, error) {
	m := map[string]string{"string": string(v)}
	return json.Marshal(m)
}

func (v RegexValue) MarshalJSON() ([]byte, error) {
	m := map[string]string{"regex": string(v)}
	return json.Marshal(m)
}

func (v NumberValue) MarshalJSON() ([]byte, error) {
	m := map[string]float64{"number": float64(v)}
	return json.Marshal(m)
}

func (v IntegerValue) MarshalJSON() ([]byte, error) {
	m := map[string]int64{"integer": int64(v)}
	return json.Marshal(m)
}

func (v BooleanValue) MarshalJSON() ([]byte, error) {
	m := map[string]bool{"boolean": bool(v)}
	return json.Marshal(m)
}

func (v TimeValue) MarshalJSON() ([]byte, error) {
	t := time.Time(v)
	m := map[string]string{"string": t.Format(time.RFC3339Nano)}
	return json.Marshal(m)
}

func (v DurationValue) MarshalJSON() ([]byte, error) {
	m := map[string]int64{"duration": int64(v)}
	return json.Marshal(m)
}
