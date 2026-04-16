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

// Package restjson provides RESTful JSON serialization of AWS
// requests and responses.
package restjson

//go:generate go run -tags codegen ../../../private/model/cli/gen-protocol-tests ../../../models/protocol_tests/input/rest-json.json build_test.go
//go:generate go run -tags codegen ../../../private/model/cli/gen-protocol-tests ../../../models/protocol_tests/output/rest-json.json unmarshal_test.go

import (
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/private/protocol/jsonrpc"
	"github.com/aws/aws-sdk-go/private/protocol/rest"
)

// BuildHandler is a named request handler for building restjson protocol
// requests
var BuildHandler = request.NamedHandler{
	Name: "awssdk.restjson.Build",
	Fn:   Build,
}

// UnmarshalHandler is a named request handler for unmarshaling restjson
// protocol requests
var UnmarshalHandler = request.NamedHandler{
	Name: "awssdk.restjson.Unmarshal",
	Fn:   Unmarshal,
}

// UnmarshalMetaHandler is a named request handler for unmarshaling restjson
// protocol request metadata
var UnmarshalMetaHandler = request.NamedHandler{
	Name: "awssdk.restjson.UnmarshalMeta",
	Fn:   UnmarshalMeta,
}

// Build builds a request for the REST JSON protocol.
func Build(r *request.Request) {
	rest.Build(r)

	if t := rest.PayloadType(r.Params); t == "structure" || t == "" {
		if v := r.HTTPRequest.Header.Get("Content-Type"); len(v) == 0 {
			r.HTTPRequest.Header.Set("Content-Type", "application/json")
		}
		jsonrpc.Build(r)
	}
}

// Unmarshal unmarshals a response body for the REST JSON protocol.
func Unmarshal(r *request.Request) {
	if t := rest.PayloadType(r.Data); t == "structure" || t == "" {
		jsonrpc.Unmarshal(r)
	} else {
		rest.Unmarshal(r)
	}
}

// UnmarshalMeta unmarshals response headers for the REST JSON protocol.
func UnmarshalMeta(r *request.Request) {
	rest.UnmarshalMeta(r)
}
