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
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client/metadata"
	"github.com/aws/aws-sdk-go/aws/request"
)

// A Config provides configuration to a service client instance.
type Config struct {
	Config         *aws.Config
	Handlers       request.Handlers
	PartitionID    string
	Endpoint       string
	SigningRegion  string
	SigningName    string
	ResolvedRegion string

	// States that the signing name did not come from a modeled source but
	// was derived based on other data. Used by service client constructors
	// to determine if the signin name can be overridden based on metadata the
	// service has.
	SigningNameDerived bool
}

// ConfigProvider provides a generic way for a service client to receive
// the ClientConfig without circular dependencies.
type ConfigProvider interface {
	ClientConfig(serviceName string, cfgs ...*aws.Config) Config
}

// ConfigNoResolveEndpointProvider same as ConfigProvider except it will not
// resolve the endpoint automatically. The service client's endpoint must be
// provided via the aws.Config.Endpoint field.
type ConfigNoResolveEndpointProvider interface {
	ClientConfigNoResolveEndpoint(cfgs ...*aws.Config) Config
}

// A Client implements the base client request and response handling
// used by all service clients.
type Client struct {
	request.Retryer
	metadata.ClientInfo

	Config   aws.Config
	Handlers request.Handlers
}

// New will return a pointer to a new initialized service client.
func New(cfg aws.Config, info metadata.ClientInfo, handlers request.Handlers, options ...func(*Client)) *Client {
	svc := &Client{
		Config:     cfg,
		ClientInfo: info,
		Handlers:   handlers.Copy(),
	}

	switch retryer, ok := cfg.Retryer.(request.Retryer); {
	case ok:
		svc.Retryer = retryer
	case cfg.Retryer != nil && cfg.Logger != nil:
		s := fmt.Sprintf("WARNING: %T does not implement request.Retryer; using DefaultRetryer instead", cfg.Retryer)
		cfg.Logger.Log(s)
		fallthrough
	default:
		maxRetries := aws.IntValue(cfg.MaxRetries)
		if cfg.MaxRetries == nil || maxRetries == aws.UseServiceDefaultRetries {
			maxRetries = DefaultRetryerMaxNumRetries
		}
		svc.Retryer = DefaultRetryer{NumMaxRetries: maxRetries}
	}

	svc.AddDebugHandlers()

	for _, option := range options {
		option(svc)
	}

	return svc
}

// NewRequest returns a new Request pointer for the service API
// operation and parameters.
func (c *Client) NewRequest(operation *request.Operation, params interface{}, data interface{}) *request.Request {
	return request.New(c.Config, c.ClientInfo, c.Handlers, c.Retryer, operation, params, data)
}

// AddDebugHandlers injects debug logging handlers into the service to log request
// debug information.
func (c *Client) AddDebugHandlers() {
	c.Handlers.Send.PushFrontNamed(LogHTTPRequestHandler)
	c.Handlers.Send.PushBackNamed(LogHTTPResponseHandler)
}
