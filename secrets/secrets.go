// Copyright 2024 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package secrets

import (
	"context"

	"github.com/go-kit/log"
)

// Secret represents a sensitive value.
type Secret interface {
	// Fetch fetches the secret content.
	Fetch(ctx context.Context) (string, error)
}

// SecretFn wraps a function to make it a Secret.
type SecretFn func(ctx context.Context) (string, error)

// Fetch implements Secret.Fetch.
func (fn *SecretFn) Fetch(ctx context.Context) (string, error) {
	return (*fn)(ctx)
}

// Provider is a secret provider.
type Provider[T any] interface {
	// Add returns the secret fetcher for the given configuration.
	Add(ctx context.Context, config *T) (Secret, error)

	// Update returns the secret fetcher for the new configuration.
	Update(ctx context.Context, configBefore, configAfter *T) (Secret, error)

	// Remove ensures that the secret fetcher for the configuration is invalid.
	Remove(ctx context.Context, config *T) error
}

// ProviderOptions provides options for a Provider.
type ProviderOptions struct {
	Logger log.Logger
}

// Config provides the configuration and constructor for a Provider.
type Config[T any] interface {
	// Name returns the name of the secret provider.
	Name() string

	// NewProvider creates a new Provider with the options.
	NewProvider(ctx context.Context, opts ProviderOptions) (Provider[T], error)
}
