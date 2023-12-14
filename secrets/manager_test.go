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
	"errors"
	"fmt"
	"testing"
)

type testSecret struct {
	Foo string `yaml:"foo"`
}

func makeTestSecretData(config, value string, id int) string {
	return fmt.Sprintf("%s-%d-foo: %s", config, id, value)
}

type testConfig struct {
	Prefix string `yaml:"prefix"`
	// Don't serialize this. Use this ID to determine if the config was changed.
	configID int
}

func (p *testConfig) Name() string {
	return "prefix"
}

func (p *testConfig) NewProvider(ctx context.Context, opts ProviderOptions) (Provider[testSecret], error) {
	provider := &testProvider{
		config: p.Prefix,
		id:     p.configID,
	}
	p.configID += 1
	return provider, nil
}

const invalidTestSecretConfigValue = "blue"

var errInvalidTestSecretConfig = errors.New("invalid secret config")

type testProvider struct {
	config string
	id     int
}

func (p *testProvider) Add(ctx context.Context, config *testSecret) (Secret, error) {
	if config.Foo == "" {
		return nil, errInvalidTestSecretConfig
	}
	secret := SecretFn(func(ctx context.Context) (string, error) {
		if config.Foo == invalidTestSecretConfigValue {
			return "", errInvalidTestSecretConfig
		}
		return makeTestSecretData(p.config, config.Foo, p.id), nil
	})
	return &secret, nil
}

func (p *testProvider) Update(ctx context.Context, configBefore, configAfter *testSecret) (Secret, error) {
	return p.Add(ctx, configAfter)
}

func (p *testProvider) Remove(ctx context.Context, config *testSecret) error {
	return nil
}

type dataOrError struct {
	data string
	err  error
}

func assertApplyConfig(ctx context.Context, t testing.TB, description string, manager *ProviderManager[testSecret], providerConfig *testConfig, config []SecretConfig[testSecret], secrets map[string]dataOrError, expectedErr error) {
	configCopy := config
	if err := manager.ApplyConfig(ctx, providerConfig, configCopy); err != nil {
		if expectedErr == nil || (!errors.Is(err, expectedErr) && err.Error() != expectedErr.Error()) {
			t.Fatalf("expected %s error %q but got: %s", description, expectedErr, err)
		}
	} else if expectedErr != nil {
		t.Fatalf("expected %s error %q but got none", description, expectedErr)
	}
	for name, value := range secrets {
		data, err := manager.Fetch(context.Background(), name)
		if err != nil {
			if value.err != nil {
				if errors.Is(err, value.err) {
					continue
				}
				t.Fatalf("expected error %q for %s secret %q but got: %s", value.err, description, name, err)
			}
			t.Fatalf("unexpected error for %s secret %q: %s", description, name, err)
		}
		if data != value.data {
			t.Fatalf("expected data %q for %s secret %q but got: %s", value.data, description, name, data)
		}
	}
	if manager.secretCount() != len(secrets) {
		t.Errorf("expected %s %d secrets but found %d", description, len(secrets), manager.secretCount())
	}
}

func testValues(config *testConfig) ([]SecretConfig[testSecret], map[string]dataOrError) {
	return []SecretConfig[testSecret]{
			{
				Name: "abc",
				Config: testSecret{
					Foo: "green",
				},
			},
			{
				Name: "xyz",
				Config: testSecret{
					Foo: "orange",
				},
			},
		}, map[string]dataOrError{
			"abc": {
				data: makeTestSecretData(config.Prefix, "green", config.configID),
			},
			"xyz": {
				data: makeTestSecretData(config.Prefix, "orange", config.configID),
			},
		}
}

func updateSecret(configs []SecretConfig[testSecret], secrets map[string]dataOrError, index int, value string, result dataOrError) {
	configs[index].Config.Foo = value
	if value == "" {
		delete(secrets, configs[index].Name)
		return
	}
	secrets[configs[index].Name] = result
}

func updateSecretInvalid(configs []SecretConfig[testSecret], secrets map[string]dataOrError, index int) {
	updateSecret(configs, secrets, index, invalidTestSecretConfigValue, dataOrError{
		err: errInvalidTestSecretConfig,
	})
}

func updateSecretError(configs []SecretConfig[testSecret], secrets map[string]dataOrError, index int) {
	updateSecret(configs, secrets, index, "", dataOrError{})
}

func TestProviderConfig(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	testCases := []struct {
		description string
		testFn      func(t testing.TB, manager *ProviderManager[testSecret])
	}{
		{
			description: "no change invalid",
			testFn: func(t testing.TB, manager *ProviderManager[testSecret]) {
				providerConfigInitial := &testConfig{
					Prefix:   "i",
					configID: 1,
				}
				config, secrets := testValues(providerConfigInitial)
				updateSecretError(config, secrets, 0)
				assertApplyConfig(ctx, t, "initial", manager, providerConfigInitial, config, secrets, errInvalidTestSecretConfig)
				if providerConfigInitial.configID != 2 {
					t.Fatalf("Initial provider not called")
				}

				providerConfigFinal := &testConfig{
					Prefix:   "i",
					configID: 1,
				}
				assertApplyConfig(ctx, t, "final", manager, providerConfigFinal, config, secrets, errInvalidTestSecretConfig)
				if e, a := 1, providerConfigFinal.configID; e != a {
					t.Fatalf("Final provider config has wrong value, expected %d actual %d", e, a)
				}
			},
		},
		{
			description: "no change valid",
			testFn: func(t testing.TB, manager *ProviderManager[testSecret]) {
				providerConfigInitial := &testConfig{
					Prefix:   "i",
					configID: 1,
				}
				config, secrets := testValues(providerConfigInitial)
				assertApplyConfig(ctx, t, "initial", manager, providerConfigInitial, config, secrets, nil)
				if providerConfigInitial.configID != 2 {
					t.Fatalf("Initial provider not called")
				}

				providerConfigFinal := &testConfig{
					Prefix:   "i",
					configID: 1,
				}
				assertApplyConfig(ctx, t, "final", manager, providerConfigFinal, config, secrets, nil)
				if e, a := 1, providerConfigFinal.configID; e != a {
					t.Fatalf("Final provider config has wrong value, expected %d actual %d", e, a)
				}
			},
		},
		{
			description: "no config change invalid",
			testFn: func(t testing.TB, manager *ProviderManager[testSecret]) {
				providerConfigInitial := &testConfig{
					Prefix:   "i",
					configID: 1,
				}
				config, secrets := testValues(providerConfigInitial)
				updateSecretError(config, secrets, 0)
				assertApplyConfig(ctx, t, "initial", manager, providerConfigInitial, config, secrets, errInvalidTestSecretConfig)
				if providerConfigInitial.configID != 2 {
					t.Fatalf("Initial provider not called")
				}

				providerConfigFinal := &testConfig{
					Prefix:   "i",
					configID: 10,
				}
				assertApplyConfig(ctx, t, "final", manager, providerConfigFinal, config, secrets, errInvalidTestSecretConfig)
				if e, a := 10, providerConfigFinal.configID; e != a {
					t.Fatalf("Final provider config has wrong value, expected %d actual %d", e, a)
				}
			},
		},
		{
			description: "no config change valid",
			testFn: func(t testing.TB, manager *ProviderManager[testSecret]) {
				providerConfigInitial := &testConfig{
					Prefix:   "i",
					configID: 1,
				}
				config, secrets := testValues(providerConfigInitial)
				assertApplyConfig(ctx, t, "initial", manager, providerConfigInitial, config, secrets, nil)
				if providerConfigInitial.configID != 2 {
					t.Fatalf("Initial provider not called")
				}

				providerConfigFinal := &testConfig{
					Prefix:   "i",
					configID: 10,
				}
				assertApplyConfig(ctx, t, "final", manager, providerConfigFinal, config, secrets, nil)
				if e, a := 10, providerConfigFinal.configID; e != a {
					t.Fatalf("Final provider config has wrong value, expected %d actual %d", e, a)
				}
			},
		},
		{
			description: "config change invalid",
			testFn: func(t testing.TB, manager *ProviderManager[testSecret]) {
				providerConfigInitial := &testConfig{
					Prefix:   "i",
					configID: 1,
				}
				config, secrets := testValues(providerConfigInitial)
				updateSecretError(config, secrets, 0)
				assertApplyConfig(ctx, t, "initial", manager, providerConfigInitial, config, secrets, errInvalidTestSecretConfig)
				if providerConfigInitial.configID != 2 {
					t.Fatalf("Initial provider not called")
				}

				providerConfigFinal := &testConfig{
					Prefix:   "j",
					configID: 10,
				}
				config, secrets = testValues(providerConfigFinal)
				updateSecretError(config, secrets, 0)
				assertApplyConfig(ctx, t, "final", manager, providerConfigFinal, config, secrets, errInvalidTestSecretConfig)
				if e, a := 11, providerConfigFinal.configID; e != a {
					t.Fatalf("Final provider config has wrong value, expected %d actual %d", e, a)
				}
			},
		},
		{
			description: "config change valid",
			testFn: func(t testing.TB, manager *ProviderManager[testSecret]) {
				providerConfigInitial := &testConfig{
					Prefix:   "i",
					configID: 1,
				}
				config, secrets := testValues(providerConfigInitial)
				assertApplyConfig(ctx, t, "initial", manager, providerConfigInitial, config, secrets, nil)
				if providerConfigInitial.configID != 2 {
					t.Fatalf("Initial provider not called")
				}

				providerConfigFinal := &testConfig{
					Prefix:   "j",
					configID: 10,
				}
				config, secrets = testValues(providerConfigFinal)
				assertApplyConfig(ctx, t, "final", manager, providerConfigFinal, config, secrets, nil)
				if e, a := 11, providerConfigFinal.configID; e != a {
					t.Fatalf("Final provider config has wrong value, expected %d actual %d", e, a)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			manager := NewProviderManager[testSecret](ctx, nil)
			defer manager.Close(nil)
			tc.testFn(t, &manager)
		})
	}
}

func TestSecretConfig(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	getProviderConfig := func() *testConfig {
		return &testConfig{
			Prefix:   "i",
			configID: 1,
		}
	}
	assertConfigState := func(ctx context.Context, t testing.TB, description string, manager *ProviderManager[testSecret], config []SecretConfig[testSecret], secrets map[string]dataOrError) {
		assertApplyConfig(ctx, t, description, manager, getProviderConfig(), config, secrets, nil)
	}
	commonTestValues := func() ([]SecretConfig[testSecret], map[string]dataOrError) {
		return testValues(getProviderConfig())
	}

	testCases := []struct {
		description string
		testFn      func(t testing.TB, manager *ProviderManager[testSecret])
	}{
		{
			description: "no change none",
			testFn: func(t testing.TB, manager *ProviderManager[testSecret]) {
				configs, secrets := []SecretConfig[testSecret]{}, map[string]dataOrError{}
				assertConfigState(ctx, t, "initial", manager, configs, secrets)
				assertConfigState(ctx, t, "final", manager, configs, secrets)
			},
		},
		{
			description: "no change valid",
			testFn: func(t testing.TB, manager *ProviderManager[testSecret]) {
				configs, secrets := commonTestValues()
				assertConfigState(ctx, t, "initial", manager, configs, secrets)
				assertConfigState(ctx, t, "final", manager, configs, secrets)
			},
		},
		{
			description: "no change invalid",
			testFn: func(t testing.TB, manager *ProviderManager[testSecret]) {
				configs, secrets := commonTestValues()
				updateSecretInvalid(configs, secrets, 0)
				assertConfigState(ctx, t, "initial", manager, configs, secrets)
				assertConfigState(ctx, t, "final", manager, configs, secrets)
			},
		},
		{
			description: "add invalid to none",
			testFn: func(t testing.TB, manager *ProviderManager[testSecret]) {
				configs, secrets := []SecretConfig[testSecret]{}, map[string]dataOrError{}
				assertConfigState(ctx, t, "initial", manager, configs, secrets)

				configs = append(configs, SecretConfig[testSecret]{
					Name: "123",
					Config: testSecret{
						Foo: invalidTestSecretConfigValue,
					},
				})
				secrets["123"] = dataOrError{
					err: errInvalidTestSecretConfig,
				}
				assertConfigState(ctx, t, "final", manager, configs, secrets)
			},
		},
		{
			description: "add valid to none",
			testFn: func(t testing.TB, manager *ProviderManager[testSecret]) {
				configs, secrets := []SecretConfig[testSecret]{}, map[string]dataOrError{}
				assertConfigState(ctx, t, "initial", manager, configs, secrets)

				configs = append(configs, SecretConfig[testSecret]{
					Name: "123",
					Config: testSecret{
						Foo: "red",
					},
				})
				secrets["123"] = dataOrError{
					data: makeTestSecretData("i", "red", 1),
				}
				assertConfigState(ctx, t, "final", manager, configs, secrets)
			},
		},
		{
			description: "add invalid to some",
			testFn: func(t testing.TB, manager *ProviderManager[testSecret]) {
				configs, secrets := commonTestValues()
				assertConfigState(ctx, t, "initial", manager, configs, secrets)

				configs = append(configs, SecretConfig[testSecret]{
					Name: "123",
					Config: testSecret{
						Foo: invalidTestSecretConfigValue,
					},
				})
				secrets["123"] = dataOrError{
					err: errInvalidTestSecretConfig,
				}
				assertConfigState(ctx, t, "final", manager, configs, secrets)
			},
		},
		{
			description: "add duplicate to some",
			testFn: func(t testing.TB, manager *ProviderManager[testSecret]) {
				configs, secrets := commonTestValues()
				assertConfigState(ctx, t, "initial", manager, configs, secrets)

				delete(secrets, configs[0].Name)
				delete(secrets, configs[1].Name)
				configs[1].Name = configs[0].Name
				assertApplyConfig(ctx, t, "final", manager, getProviderConfig(), configs, secrets, fmt.Errorf("duplicate secret key %q", configs[0].Name))
			},
		},
		{
			description: "add valid to some",
			testFn: func(t testing.TB, manager *ProviderManager[testSecret]) {
				configs, secrets := commonTestValues()
				assertConfigState(ctx, t, "initial", manager, configs, secrets)

				configs = append(configs, SecretConfig[testSecret]{
					Name: "123",
					Config: testSecret{
						Foo: "red",
					},
				})
				secrets["123"] = dataOrError{
					data: makeTestSecretData("i", "red", 1),
				}
				assertConfigState(ctx, t, "final", manager, configs, secrets)
			},
		},
		{
			description: "update invalid",
			testFn: func(t testing.TB, manager *ProviderManager[testSecret]) {
				configs, secrets := commonTestValues()
				updateSecretInvalid(configs, secrets, 0)
				assertConfigState(ctx, t, "initial", manager, configs, secrets)

				configs[0].Config.Foo = "red"
				secrets[configs[0].Name] = dataOrError{
					data: makeTestSecretData("i", "red", 1),
				}
				assertConfigState(ctx, t, "final", manager, configs, secrets)
			},
		},
		{
			description: "update valid",
			testFn: func(t testing.TB, manager *ProviderManager[testSecret]) {
				configs, secrets := commonTestValues()
				assertConfigState(ctx, t, "initial", manager, configs, secrets)

				configs[0].Config.Foo = "red"
				secrets[configs[0].Name] = dataOrError{
					data: makeTestSecretData("i", "red", 1),
				}
				assertConfigState(ctx, t, "final", manager, configs, secrets)
			},
		},
		{
			description: "replace invalid",
			testFn: func(t testing.TB, manager *ProviderManager[testSecret]) {
				configs, secrets := commonTestValues()
				updateSecretInvalid(configs, secrets, 0)
				assertConfigState(ctx, t, "initial", manager, configs, secrets)

				delete(secrets, configs[0].Name)
				configs[0] = SecretConfig[testSecret]{
					Name: "123",
					Config: testSecret{
						Foo: "red",
					},
				}
				secrets["123"] = dataOrError{
					data: makeTestSecretData("i", "red", 1),
				}
				assertConfigState(ctx, t, "final", manager, configs, secrets)
			},
		},
		{
			description: "replace valid",
			testFn: func(t testing.TB, manager *ProviderManager[testSecret]) {
				configs, secrets := commonTestValues()
				assertConfigState(ctx, t, "initial", manager, configs, secrets)

				delete(secrets, configs[0].Name)
				configs[0] = SecretConfig[testSecret]{
					Name: "123",
					Config: testSecret{
						Foo: "red",
					},
				}
				secrets["123"] = dataOrError{
					data: makeTestSecretData("i", "red", 1),
				}
				assertConfigState(ctx, t, "final", manager, configs, secrets)
			},
		},
		{
			description: "remove invalid",
			testFn: func(t testing.TB, manager *ProviderManager[testSecret]) {
				configs, secrets := commonTestValues()
				updateSecretInvalid(configs, secrets, 0)
				assertConfigState(ctx, t, "initial", manager, configs, secrets)

				delete(secrets, configs[0].Name)
				configs = configs[1:]
				assertConfigState(ctx, t, "final", manager, configs, secrets)
			},
		},
		{
			description: "remove valid",
			testFn: func(t testing.TB, manager *ProviderManager[testSecret]) {
				configs, secrets := commonTestValues()
				assertConfigState(ctx, t, "initial", manager, configs, secrets)

				delete(secrets, configs[0].Name)
				configs = configs[1:]
				assertConfigState(ctx, t, "final", manager, configs, secrets)
			},
		},
		{
			description: "clear all",
			testFn: func(t testing.TB, manager *ProviderManager[testSecret]) {
				configs, secrets := commonTestValues()
				updateSecretInvalid(configs, secrets, 0)
				assertConfigState(ctx, t, "initial", manager, configs, secrets)

				configs, secrets = []SecretConfig[testSecret]{}, map[string]dataOrError{}
				assertConfigState(ctx, t, "final", manager, configs, secrets)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			manager := NewProviderManager[testSecret](ctx, nil)
			defer manager.Close(nil)
			tc.testFn(t, &manager)
		})
	}
}
