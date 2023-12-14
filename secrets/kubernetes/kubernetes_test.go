// Copyright 2023 The Prometheus Authors
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

package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/prometheus/secrets"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func toSecretConfig(s *corev1.Secret, key string) *SecretConfig {
	return &SecretConfig{
		Namespace: s.Namespace,
		Name:      s.Name,
		Key:       key,
	}
}

func deleteKey(s *corev1.Secret, key string) {
	delete(s.Data, key)
	delete(s.StringData, key)
}

func updateKey(s *corev1.Secret, key, value string) {
	if _, ok := s.Data[key]; ok {
		s.Data[key] = []byte(value)
		return
	}
	if _, ok := s.StringData[key]; ok {
		s.StringData[key] = value
		return
	}
	panic(fmt.Errorf("invalid key %q in secret: %s/%s", key, s.Namespace, s.Name))
}

func copyKey(s *corev1.Secret, keyFrom, keyTo string) {
	if _, ok := s.Data[keyFrom]; ok {
		s.Data[keyTo] = s.Data[keyFrom]
		return
	}
	if _, ok := s.StringData[keyFrom]; ok {
		s.StringData[keyTo] = s.StringData[keyFrom]
		return
	}
	panic(fmt.Errorf("invalid key %q in secret: %s/%s", keyFrom, s.Namespace, s.Name))
}

type testCase struct {
	description string
	test        func(ctx context.Context, c *fake.Clientset, provider secrets.Provider[*SecretConfig])
}

type entry struct {
	key   string
	value string
}

type testCategory struct {
	name    string
	secret  *corev1.Secret
	entries []entry
}

func requireFetchEquals(t testing.TB, ctx context.Context, s secrets.Secret, expected string) {
	var err error
	pollErr := wait.PollUntilContextTimeout(ctx, time.Millisecond, 10*time.Second, true, func(ctx context.Context) (bool, error) {
		var val string
		val, err = s.Fetch(ctx)
		if err != nil {
			err = fmt.Errorf("expected nil error but received: %w", err)
			return false, nil
		}
		if expected != val {
			err = fmt.Errorf("expected value %q but received %q", expected, val)
			return false, nil
		}
		return true, nil
	})
	if pollErr != nil {
		if errors.Is(pollErr, context.DeadlineExceeded) && err != nil {
			pollErr = err
		}
		t.Fatal(pollErr)
	}
}

func requireFetchFail(t testing.TB, ctx context.Context, s secrets.Secret, expected error) {
	var err error
	pollErr := wait.PollUntilContextTimeout(ctx, time.Millisecond, 10*time.Second, true, func(ctx context.Context) (bool, error) {
		var val string
		val, err = s.Fetch(ctx)
		if val != "" {
			err = fmt.Errorf("expected empty value but received %q", val)
			return false, nil
		}
		if expected.Error() != err.Error() {
			err = fmt.Errorf("expected error %q but received %q", expected, err)
			return false, nil
		}
		return true, nil
	})
	if pollErr != nil {
		if errors.Is(pollErr, context.DeadlineExceeded) && err != nil {
			pollErr = err
		}
		t.Fatal(pollErr)
	}
}

func TestProvider(t *testing.T) {
	validEmptySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns1",
			Name:      "s1",
		},
	}
	validBinarySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns1",
			Name:      "s2",
		},
		Data: map[string][]byte{
			"k1": []byte("Hello world!"),
			"k2": []byte("xyz"),
			"k3": []byte("abc"),
		},
	}
	validStringSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns2",
			Name:      "s1",
		},
		StringData: map[string]string{
			"foo":   "bar",
			"alpha": "bravo",
		},
	}
	validMixedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns3",
			Name:      "s2",
		},
		Data: map[string][]byte{
			"red": []byte("green"),
		},
		StringData: map[string]string{
			"orange": "blue",
		},
	}

	testCasesFor := func(main testCategory, others ...testCategory) []testCase {
		testCases := []testCase{
			{
				description: fmt.Sprintf("remove untracked %s secret", main.name),
				test: func(ctx context.Context, c *fake.Clientset, provider secrets.Provider[*SecretConfig]) {
					err := provider.Remove(ctx, toSecretConfig(main.secret, main.entries[0].key))
					require.NoError(t, err)
				},
			},
			{
				description: fmt.Sprintf("remove tracked %s secret", main.name),
				test: func(ctx context.Context, c *fake.Clientset, provider secrets.Provider[*SecretConfig]) {
					s, err := provider.Add(ctx, toSecretConfig(main.secret, main.entries[0].key))
					require.NoError(t, err)
					requireFetchEquals(t, ctx, s, main.entries[0].value)

					err = provider.Remove(ctx, toSecretConfig(main.secret, main.entries[0].key))
					require.NoError(t, err)

					// Attempt to remove twice. Second time does nothing.
					err = provider.Remove(ctx, toSecretConfig(main.secret, main.entries[0].key))
					require.NoError(t, err)
				},
			},
			{
				description: fmt.Sprintf("valid %s delete key", main.name),
				test: func(ctx context.Context, c *fake.Clientset, provider secrets.Provider[*SecretConfig]) {
					key := main.entries[0].key
					s, err := provider.Add(ctx, toSecretConfig(main.secret, key))
					require.NoError(t, err)
					requireFetchEquals(t, ctx, s, main.entries[0].value)

					secret := main.secret.DeepCopy()
					deleteKey(secret, key)
					_, err = c.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
					require.NoError(t, err)

					requireFetchFail(t, ctx, s, fmt.Errorf("secret %s/%s does not contain key: %s", main.secret.Namespace, main.secret.Name, key))

					err = provider.Remove(ctx, toSecretConfig(main.secret, key))
					require.NoError(t, err)
				},
			},
			{
				description: fmt.Sprintf("valid %s delete secret", main.name),
				test: func(ctx context.Context, c *fake.Clientset, provider secrets.Provider[*SecretConfig]) {
					key := main.entries[0].key
					s, err := provider.Add(ctx, toSecretConfig(main.secret, key))
					require.NoError(t, err)
					requireFetchEquals(t, ctx, s, main.entries[0].value)

					err = c.CoreV1().Secrets(main.secret.Namespace).Delete(ctx, main.secret.Name, metav1.DeleteOptions{})
					require.NoError(t, err)

					requireFetchFail(t, ctx, s, fmt.Errorf("secret %s/%s not found", main.secret.Namespace, main.secret.Name))

					err = provider.Remove(ctx, toSecretConfig(main.secret, key))
					require.NoError(t, err)
				},
			},
			{
				description: fmt.Sprintf("valid %s update value", main.name),
				test: func(ctx context.Context, c *fake.Clientset, provider secrets.Provider[*SecretConfig]) {
					key := main.entries[0].key
					s, err := provider.Add(ctx, toSecretConfig(main.secret, key))
					require.NoError(t, err)
					requireFetchEquals(t, ctx, s, main.entries[0].value)

					secret := main.secret.DeepCopy()
					updateKey(secret, key, "Goodbye")
					_, err = c.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
					require.NoError(t, err)

					requireFetchEquals(t, ctx, s, "Goodbye")

					err = provider.Remove(ctx, toSecretConfig(main.secret, key))
					require.NoError(t, err)
				},
			},
			{
				description: fmt.Sprintf("valid %s update valid key", main.name),
				test: func(ctx context.Context, c *fake.Clientset, provider secrets.Provider[*SecretConfig]) {
					keyFrom := main.entries[0].key
					s, err := provider.Add(ctx, toSecretConfig(main.secret, keyFrom))
					require.NoError(t, err)
					requireFetchEquals(t, ctx, s, main.entries[0].value)

					keyTo := main.entries[1].key
					s, err = provider.Update(ctx, toSecretConfig(main.secret, keyFrom), toSecretConfig(main.secret, keyTo))
					require.NoError(t, err)
					requireFetchEquals(t, ctx, s, main.entries[1].value)

					// Update original key.
					secret := main.secret.DeepCopy()
					updateKey(secret, keyFrom, "Goodbye")
					_, err = c.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
					require.NoError(t, err)

					requireFetchEquals(t, ctx, s, main.entries[1].value)

					// Update new key.
					updateKey(secret, keyTo, "Sayonara")
					_, err = c.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
					require.NoError(t, err)

					requireFetchEquals(t, ctx, s, "Sayonara")

					err = provider.Remove(ctx, toSecretConfig(main.secret, keyTo))
					require.NoError(t, err)
				},
			},
			{
				description: fmt.Sprintf("valid %s update invalid key", main.name),
				test: func(ctx context.Context, c *fake.Clientset, provider secrets.Provider[*SecretConfig]) {
					key := main.entries[0].key
					s, err := provider.Add(ctx, toSecretConfig(main.secret, key))
					require.NoError(t, err)
					requireFetchEquals(t, ctx, s, main.entries[0].value)

					s, err = provider.Update(ctx, toSecretConfig(main.secret, key), toSecretConfig(main.secret, "x"))
					require.NoError(t, err)
					requireFetchFail(t, ctx, s, fmt.Errorf("secret %s/%s does not contain key: x", main.secret.Namespace, main.secret.Name))

					err = provider.Remove(ctx, toSecretConfig(main.secret, key))
					require.NoError(t, err)
				},
			},
			{
				description: fmt.Sprintf("valid %s update invalid secret", main.name),
				test: func(ctx context.Context, c *fake.Clientset, provider secrets.Provider[*SecretConfig]) {
					key := main.entries[0].key
					s, err := provider.Add(ctx, toSecretConfig(main.secret, key))
					require.NoError(t, err)
					requireFetchEquals(t, ctx, s, main.entries[0].value)

					s, err = provider.Update(ctx, toSecretConfig(main.secret, key), &SecretConfig{Namespace: "x", Name: "y", Key: "z"})
					require.NoError(t, err)
					requireFetchFail(t, ctx, s, fmt.Errorf("secret x/y not found"))

					err = provider.Remove(ctx, &SecretConfig{Namespace: "x", Name: "y", Key: "z"})
					require.NoError(t, err)
				},
			},
			{
				description: fmt.Sprintf("invalid %s create key", main.name),
				test: func(ctx context.Context, c *fake.Clientset, provider secrets.Provider[*SecretConfig]) {
					key := "kn"
					s, err := provider.Add(ctx, toSecretConfig(main.secret, key))
					require.NoError(t, err)
					requireFetchFail(t, ctx, s, fmt.Errorf("secret %s/%s does not contain key: %s", main.secret.Namespace, main.secret.Name, key))

					secret := main.secret.DeepCopy()
					copyKey(secret, main.entries[0].key, key)
					updateKey(secret, key, "Goodbye")
					_, err = c.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
					require.NoError(t, err)

					requireFetchEquals(t, ctx, s, "Goodbye")

					err = provider.Remove(ctx, toSecretConfig(main.secret, key))
					require.NoError(t, err)
				},
			},
			{
				description: fmt.Sprintf("invalid %s create secret", main.name),
				test: func(ctx context.Context, c *fake.Clientset, provider secrets.Provider[*SecretConfig]) {
					err := c.CoreV1().Secrets(main.secret.Namespace).Delete(ctx, main.secret.Name, metav1.DeleteOptions{})
					require.NoError(t, err)

					key := main.entries[0].key
					s, err := provider.Add(ctx, toSecretConfig(main.secret, key))
					require.NoError(t, err)
					requireFetchFail(t, ctx, s, fmt.Errorf("secret %s/%s not found", main.secret.Namespace, main.secret.Name))

					secret := main.secret.DeepCopy()
					updateKey(secret, key, "Goodbye")
					_, err = c.CoreV1().Secrets(secret.Namespace).Create(ctx, secret, metav1.CreateOptions{})
					require.NoError(t, err)

					requireFetchEquals(t, ctx, s, "Goodbye")

					err = provider.Remove(ctx, toSecretConfig(main.secret, key))
					require.NoError(t, err)
				},
			},
			{
				description: fmt.Sprintf("invalid %s update valid key", main.name),
				test: func(ctx context.Context, c *fake.Clientset, provider secrets.Provider[*SecretConfig]) {
					keyFrom := "kn1"
					s, err := provider.Add(ctx, toSecretConfig(main.secret, keyFrom))
					require.NoError(t, err)
					requireFetchFail(t, ctx, s, fmt.Errorf("secret %s/%s does not contain key: %s", main.secret.Namespace, main.secret.Name, keyFrom))

					keyTo := "kn2"
					s, err = provider.Update(ctx, toSecretConfig(main.secret, keyFrom), toSecretConfig(main.secret, keyTo))
					require.NoError(t, err)
					requireFetchFail(t, ctx, s, fmt.Errorf("secret %s/%s does not contain key: %s", main.secret.Namespace, main.secret.Name, keyTo))

					err = provider.Remove(ctx, toSecretConfig(main.secret, keyTo))
					require.NoError(t, err)
				},
			},
			{
				description: fmt.Sprintf("invalid update valid %s secret", main.name),
				test: func(ctx context.Context, c *fake.Clientset, provider secrets.Provider[*SecretConfig]) {
					s, err := provider.Add(ctx, &SecretConfig{Namespace: "x", Name: "y", Key: "z"})
					require.NoError(t, err)
					requireFetchFail(t, ctx, s, errors.New("secret x/y not found"))

					keyTo := main.entries[0].key
					s, err = provider.Update(ctx, &SecretConfig{Namespace: "x", Name: "y", Key: "z"}, toSecretConfig(main.secret, keyTo))
					require.NoError(t, err)
					requireFetchEquals(t, ctx, s, main.entries[0].value)

					err = provider.Remove(ctx, toSecretConfig(main.secret, keyTo))
					require.NoError(t, err)
				},
			},
			{
				description: fmt.Sprintf("invalid %s update invalid key", main.name),
				test: func(ctx context.Context, c *fake.Clientset, provider secrets.Provider[*SecretConfig]) {
					keyFrom := "kn"
					s, err := provider.Add(ctx, toSecretConfig(main.secret, keyFrom))
					require.NoError(t, err)
					requireFetchFail(t, ctx, s, fmt.Errorf("secret %s/%s does not contain key: %s", main.secret.Namespace, main.secret.Name, keyFrom))

					keyTo := main.entries[0].key
					s, err = provider.Update(ctx, toSecretConfig(main.secret, keyFrom), toSecretConfig(main.secret, keyTo))
					require.NoError(t, err)
					requireFetchEquals(t, ctx, s, main.entries[0].value)

					err = provider.Remove(ctx, toSecretConfig(main.secret, keyTo))
					require.NoError(t, err)
				},
			},
			{
				description: fmt.Sprintf("invalid %s update invalid secret", main.name),
				test: func(ctx context.Context, c *fake.Clientset, provider secrets.Provider[*SecretConfig]) {
					s, err := provider.Add(ctx, &SecretConfig{Namespace: "x", Name: "y", Key: "z"})
					require.NoError(t, err)
					requireFetchFail(t, ctx, s, errors.New("secret x/y not found"))

					s, err = provider.Update(ctx, &SecretConfig{Namespace: "x", Name: "y", Key: "z"}, &SecretConfig{Namespace: "a", Name: "b", Key: "c"})
					require.NoError(t, err)
					requireFetchFail(t, ctx, s, errors.New("secret a/b not found"))

					err = provider.Remove(ctx, &SecretConfig{Namespace: "a", Name: "b", Key: "c"})
					require.NoError(t, err)
				},
			},
		}
		for _, other := range others {
			testCases = append(
				testCases,
				testCase{
					description: fmt.Sprintf("valid %s update valid %s secret", main.name, other.name),
					test: func(ctx context.Context, c *fake.Clientset, provider secrets.Provider[*SecretConfig]) {
						keyFrom := main.entries[0].key
						s, err := provider.Add(ctx, toSecretConfig(main.secret, keyFrom))
						require.NoError(t, err)
						requireFetchEquals(t, ctx, s, main.entries[0].value)

						keyTo := other.entries[0].key
						s, err = provider.Update(ctx, toSecretConfig(main.secret, keyFrom), toSecretConfig(other.secret, keyTo))
						require.NoError(t, err)
						requireFetchEquals(t, ctx, s, other.entries[0].value)

						// Update original secret.
						secret := main.secret.DeepCopy()
						updateKey(secret, keyFrom, "Goodbye")
						_, err = c.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
						require.NoError(t, err)

						requireFetchEquals(t, ctx, s, other.entries[0].value)

						// Update new secret.
						secret = other.secret.DeepCopy()
						updateKey(secret, keyTo, "Sayonara")
						_, err = c.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
						require.NoError(t, err)

						requireFetchEquals(t, ctx, s, "Sayonara")

						err = provider.Remove(ctx, toSecretConfig(other.secret, keyTo))
						require.NoError(t, err)
					},
				},
			)
		}
		return testCases
	}
	typeBinary := testCategory{
		name:   "binary",
		secret: validBinarySecret,
		entries: []entry{
			{"k1", "Hello world!"},
			{"k2", "xyz"},
		},
	}
	typeString := testCategory{
		name:   "string",
		secret: validStringSecret,
		entries: []entry{
			{"foo", "bar"},
			{"alpha", "bravo"},
		},
	}
	typeMixedString := testCategory{
		name:   "mixed (string)",
		secret: validMixedSecret,
		entries: []entry{
			{"orange", "blue"},
			{"red", "green"},
		},
	}
	typeMixedBinary := testCategory{
		name:   "mixed (binary)",
		secret: validMixedSecret,
		entries: []entry{
			{"red", "green"},
			{"orange", "blue"},
		},
	}

	testCases := []testCase{
		{
			description: "add secret with no keys",
			test: func(ctx context.Context, c *fake.Clientset, provider secrets.Provider[*SecretConfig]) {
				err := provider.Remove(ctx, toSecretConfig(validEmptySecret, "k1"))
				require.NoError(t, err)
			},
		},
	}
	testCases = append(testCases, testCasesFor(typeBinary, typeString, typeMixedBinary, typeMixedString)...)
	testCases = append(testCases, testCasesFor(typeString, typeBinary, typeMixedBinary, typeMixedString)...)
	testCases = append(testCases, testCasesFor(typeMixedBinary, typeBinary, typeString, typeMixedString)...)
	testCases = append(testCases, testCasesFor(typeMixedString, typeBinary, typeString, typeMixedBinary)...)

	ctx := context.Background()

	providerTypes := []struct {
		name        string
		constructor func(c kubernetes.Interface) (secrets.Provider[*SecretConfig], func() bool, error)
	}{
		{
			name: "demand",
			constructor: func(c kubernetes.Interface) (secrets.Provider[*SecretConfig], func() bool, error) {
				provider, err := newOnDemandProvider(c)
				return provider, func() bool { return true }, err
			},
		},
		{
			name: "watch",
			constructor: func(c kubernetes.Interface) (secrets.Provider[*SecretConfig], func() bool, error) {
				provider, err := newWatchProvider(ctx, log.NewNopLogger(), c)
				return provider, func() bool { return provider.isClean() }, err
			},
		},
	}
	for _, providerType := range providerTypes {
		t.Run(providerType.name, func(t *testing.T) {
			for _, tc := range testCases {
				// Deep copy resources to ensure all tests are independent.
				c := fake.NewSimpleClientset(
					validEmptySecret.DeepCopy(),
					validBinarySecret.DeepCopy(),
					validStringSecret.DeepCopy(),
					validMixedSecret.DeepCopy(),
				)

				provider, isClean, err := providerType.constructor(c)
				require.NoError(t, err)

				t.Run(tc.description, func(t *testing.T) {
					tc.test(ctx, c, provider)
					require.Equal(t, true, isClean())
				})
			}
		})
	}
}
