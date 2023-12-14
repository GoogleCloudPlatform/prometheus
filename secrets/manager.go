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
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v2"
)

var (
	failedSecretConfigs = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "prometheus_sp_failed_secret_configs",
			Help: "Current number of secret configurations that failed to load.",
		},
	)
	secretsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "prometheus_sp_secrets_total",
			Help: "Current number of secrets.",
		},
	)
)

func yamlSerialize(obj any) ([]byte, error) {
	if obj == nil {
		return []byte{}, nil
	}
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	if err := encoder.Encode(obj); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func yamlEqual(x, y any) (bool, error) {
	yamlX, err := yamlSerialize(x)
	if err != nil {
		return false, err
	}
	yamlY, err := yamlSerialize(y)
	if err != nil {
		return false, err
	}
	return bytes.Equal(yamlX, yamlY), nil
}

// SecretConfig maps a secret name references to a secret provider configuration.
type SecretConfig[T any] struct {
	Name   string `yaml:"name"`
	Config T      `yaml:"config"`
}

type secretEntry[T any] struct {
	config T
	secret Secret
}

// ProviderManager manages secret providers.
type ProviderManager[T any] struct {
	ctx      context.Context
	cancelFn func()
	provider Provider[T]
	config   Config[T]
	secrets  map[string]*secretEntry[T]
}

// NewProviderManager creates a new secret manager with the provided options.
func NewProviderManager[T any](ctx context.Context, reg prometheus.Registerer) ProviderManager[T] {
	if reg != nil {
		reg.MustRegister(failedSecretConfigs)
		reg.MustRegister(secretsTotal)
	}
	return ProviderManager[T]{
		ctx:      ctx,
		cancelFn: func() {},
		secrets:  make(map[string]*secretEntry[T]),
	}
}

// ApplyConfig applies the new secrets, diffing each one with the last configuration to apply the
// relevant update.
func (m *ProviderManager[T]) ApplyConfig(ctx context.Context, providerConfig Config[T], configs []SecretConfig[T]) error {
	// If no secrets are provided, cancel any existing secret provider.
	if len(configs) == 0 {
		m.cancelFn()
		m.provider = nil
		m.cancelFn = func() {}
		m.secrets = map[string]*secretEntry[T]{}
		m.config = nil
		return nil
	}

	eq, err := yamlEqual(m.config, providerConfig)
	if err != nil {
		return err
	}

	defer func() {
		m.config = providerConfig
	}()

	if !eq || m.provider == nil {
		ctx, cancel := context.WithCancel(m.ctx)
		provider, err := providerConfig.NewProvider(ctx, ProviderOptions{})
		if err != nil {
			cancel()
			return err
		}

		m.cancelFn()
		m.provider = provider
		m.cancelFn = cancel
		m.secrets = map[string]*secretEntry[T]{}
	}
	return m.updateSecrets(ctx, configs)
}

func (m *ProviderManager[T]) updateSecrets(ctx context.Context, configs []SecretConfig[T]) error {
	var errs []error

	// Do a first pass to check for errors and disable those secrets.
	secretNamesEnabled := make(map[string]bool)
	for _, secret := range configs {
		if enabled, ok := secretNamesEnabled[secret.Name]; ok {
			if !enabled {
				continue
			}
			errs = append(errs, fmt.Errorf("duplicate secret key %q", secret.Name))
			secretNamesEnabled[secret.Name] = false
		} else {
			secretNamesEnabled[secret.Name] = true
		}
	}

	secretsFinal := map[string]*secretEntry[T]{}
	for i := range configs {
		secretIncoming := &configs[i]
		if enabled := secretNamesEnabled[secretIncoming.Name]; !enabled {
			continue
		}
		// First check if we've registered this secret before.
		if secretPrevious, ok := m.secrets[secretIncoming.Name]; ok {
			// Track all the secrets we saw. The leftover are later removed.
			delete(m.secrets, secretIncoming.Name)

			// If the config didn't change, we skip this one.
			eq, err := yamlEqual(&secretPrevious.config, &secretIncoming.Config)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if eq {
				secretsFinal[secretIncoming.Name] = secretPrevious
				continue
			}

			// The config changed, so update it.
			s, err := m.provider.Update(ctx, &secretPrevious.config, &secretIncoming.Config)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			secretPrevious.secret = s
			secretsFinal[secretIncoming.Name] = secretPrevious
			continue
		} else {
			// We've never seen this secret before, so add it.
			s, err := m.provider.Add(ctx, &secretIncoming.Config)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			secretsFinal[secretIncoming.Name] = &secretEntry[T]{
				config: secretIncoming.Config,
				secret: s,
			}
		}
	}
	for _, secretUnused := range m.secrets {
		if err := m.provider.Remove(ctx, &secretUnused.config); err != nil {
			errs = append(errs, err)
		}
	}

	m.secrets = secretsFinal

	total := len(secretNamesEnabled)
	success := len(m.secrets)
	failedSecretConfigs.Set(float64(total - success))
	secretsTotal.Set(float64(total))
	return errors.Join(errs...)
}

// Fetch implements Secret.Fetch.
func (m *ProviderManager[T]) Fetch(ctx context.Context, name string) (string, error) {
	secret, ok := m.secrets[name]
	if !ok {
		return "", fmt.Errorf("secret %q not found", name)
	}
	return secret.secret.Fetch(ctx)
}

// Close cancels the manager, stopping all secret providers.
func (m *ProviderManager[T]) Close(reg prometheus.Registerer) {
	m.cancelFn()
	if reg != nil {
		reg.Unregister(failedSecretConfigs)
		reg.Unregister(secretsTotal)
	}
}

func (m *ProviderManager[T]) secretCount() int {
	return len(m.secrets)
}
