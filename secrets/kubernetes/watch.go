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

package kubernetes

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/go-kit/log"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	"github.com/prometheus/prometheus/secrets"
)

// WatchSPConfig configures access to the Kubernetes API server.
type WatchSPConfig struct {
	ClientConfig
}

// Name returns the name of the Config.
func (*WatchSPConfig) Name() string { return "kubernetes_watch" }

// NewDiscoverer returns a Discoverer for the Config.
func (c *WatchSPConfig) NewProvider(ctx context.Context, opts secrets.ProviderOptions) (secrets.Provider[SecretConfig], error) {
	client, err := c.ClientConfig.client()
	if err != nil {
		return nil, err
	}
	return newWatchProvider(ctx, opts.Logger, client)
}

type watcher struct {
	mu       sync.Mutex
	w        watch.Interface
	refCount uint
	s        *corev1.Secret
}

func newWatcher(ctx context.Context, logger log.Logger, client kubernetes.Interface, config *SecretConfig) (*watcher, error) {
	val := &watcher{
		refCount: 1,
	}

	var err error
	val.w, err = client.CoreV1().Secrets(config.Namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector:       fields.OneTermEqualSelector(metav1.ObjectNameField, config.Name).String(),
		AllowWatchBookmarks: false,
	})
	if err != nil {
		return val, fmt.Errorf("unable to watch secret %s/%s: %w", config.Namespace, config.Name, err)
	}

	// We could wait for the first watch event, but it doesn't notify us if the resource doesn't exist.
	val.s, err = client.CoreV1().Secrets(config.Namespace).Get(ctx, config.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return val, fmt.Errorf("unable to fetch secret %s/%s: %w", config.Namespace, config.Name, err)
	}
	go func() {
		for {
		channelLoop:
			for {
				select {
				case e, ok := <-val.w.ResultChan():
					if !ok {
						break channelLoop
					}
					val.update(logger, e)
				case <-ctx.Done():
					// Run within separate function so we can use defer.
					func() {
						val.mu.Lock()
						defer val.mu.Unlock()
						val.w.Stop()
					}()
					return
				}
			}

			// Run within separate function so we can use defer.
			finished := func() bool {
				// Check in case the channel cancelled intentionally.
				if val.refCount == 0 {
					return true
				}

				// Pseudo-arbitrarily jitter the length of the most common scrape interval.
				time.Sleep(1*time.Second + time.Duration(rand.Intn(30)))

				// Lock the watcher so it doesn't cancel before we restart.
				val.mu.Lock()
				defer val.mu.Unlock()

				// Check again in case the watcher cancelled while we were waiting for the mutex.
				if val.refCount == 0 {
					return true
				}
				val.w, err = client.CoreV1().Secrets(config.Namespace).Watch(ctx, metav1.ListOptions{
					FieldSelector:       fields.OneTermEqualSelector(metav1.ObjectNameField, config.Name).String(),
					AllowWatchBookmarks: false,
				})
				if err != nil {
					//nolint:errcheck
					logger.Log("msg", "unable to restart watch for secret", "err", err, "namespace", config.Namespace, "name", config.Name)
				}
				return false
			}()
			if finished {
				return
			}
		}
	}()

	return val, nil
}

func (w *watcher) update(logger log.Logger, e watch.Event) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if e.Type == "" && e.Object == nil {
		return
	}
	switch e.Type {
	case watch.Modified, watch.Added:
		secret := e.Object.(*corev1.Secret)
		w.s = secret
	case watch.Deleted:
		w.s = nil
	case watch.Bookmark:
		// Disabled explicitly when creating the watch interface.
	case watch.Error:
		//nolint:errcheck
		logger.Log("msg", "watch error event", "namespace", w.s.Namespace, "name", w.s.Name)
	}
}

func (w *watcher) secret(config *SecretConfig) secrets.Secret {
	fn := secrets.SecretFn(func(ctx context.Context) (string, error) {
		w.mu.Lock()
		defer w.mu.Unlock()
		if w.s == nil {
			return "", fmt.Errorf("secret %s/%s not found", config.Namespace, config.Name)
		}
		return getKey(w.s, config.Key)
	})
	return &fn
}

type watchProvider struct {
	ctx                context.Context
	client             kubernetes.Interface
	secretKeyToWatcher map[string]*watcher
	logger             log.Logger
}

func newWatchProvider(ctx context.Context, logger log.Logger, client kubernetes.Interface) (*watchProvider, error) {
	return &watchProvider{
		ctx:                ctx,
		client:             client,
		secretKeyToWatcher: map[string]*watcher{},
		logger:             logger,
	}, nil
}

// Add adds a new secret to the provider, starting a new watch if the secret is not already watched.
func (p *watchProvider) Add(ctx context.Context, config *SecretConfig) (secrets.Secret, error) {
	keyStr := config.objectKey().String()
	val, ok := p.secretKeyToWatcher[keyStr]
	if ok {
		val.refCount++
		return nil, nil
	}

	var err error
	val, err = newWatcher(ctx, p.logger, p.client, config)
	if err != nil {
		return nil, err
	}

	p.secretKeyToWatcher[keyStr] = val
	return val.secret(config), nil
}

// Update updates the secret, restarting the watch if the key changes.
func (p *watchProvider) Update(ctx context.Context, configBefore, configAfter *SecretConfig) (secrets.Secret, error) {
	secretBefore := configBefore.objectKey()
	secretAfter := configAfter.objectKey()
	if secretBefore == secretAfter {
		// If we're using the same secret with a different key, just remap your current watch.
		val := p.secretKeyToWatcher[secretAfter.String()]
		if val == nil {
			return nil, fmt.Errorf("secret %s/%s not found", configAfter.Namespace, configAfter.Name)
		}
		return val.secret(configAfter), nil
	}
	if err := p.Remove(ctx, configBefore); err != nil {
		return nil, err
	}
	return p.Add(ctx, configAfter)
}

// Remove removes the secret, stopping the watch if no other keys for the same secret are watched.
func (p *watchProvider) Remove(ctx context.Context, config *SecretConfig) error {
	key := config.objectKey().String()
	val := p.secretKeyToWatcher[key]
	if val == nil {
		return nil
	}

	delete(p.secretKeyToWatcher, key)

	// Lock the watcher so it doesn't restart, and cancel intentionally.
	val.mu.Lock()
	defer val.mu.Unlock()

	val.refCount--
	if val.refCount > 0 {
		return nil
	}
	val.w.Stop()
	return nil
}

func (p *watchProvider) isClean() bool {
	return len(p.secretKeyToWatcher) == 0
}
