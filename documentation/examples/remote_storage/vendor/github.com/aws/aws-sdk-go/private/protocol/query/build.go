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

// Package query provides serialization of AWS query requests, and responses.
package query

//go:generate go run -tags codegen ../../../private/model/cli/gen-protocol-tests ../../../models/protocol_tests/input/query.json build_test.go

import (
	"net/url"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/private/protocol/query/queryutil"
)

// BuildHandler is a named request handler for building query protocol requests
var BuildHandler = request.NamedHandler{Name: "awssdk.query.Build", Fn: Build}

// Build builds a request for an AWS Query service.
func Build(r *request.Request) {
	body := url.Values{
		"Action":  {r.Operation.Name},
		"Version": {r.ClientInfo.APIVersion},
	}
	if err := queryutil.Parse(body, r.Params, false); err != nil {
		r.Error = awserr.New(request.ErrCodeSerialization, "failed encoding Query request", err)
		return
	}

	if !r.IsPresigned() {
		r.HTTPRequest.Method = "POST"
		r.HTTPRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
		r.SetBufferBody([]byte(body.Encode()))
	} else { // This is a pre-signed request
		r.HTTPRequest.Method = "GET"
		r.HTTPRequest.URL.RawQuery = body.Encode()
	}
}
