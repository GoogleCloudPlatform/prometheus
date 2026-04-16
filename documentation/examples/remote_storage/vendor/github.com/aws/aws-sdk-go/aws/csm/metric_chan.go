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

package csm

import (
	"sync/atomic"
)

const (
	runningEnum = iota
	pausedEnum
)

var (
	// MetricsChannelSize of metrics to hold in the channel
	MetricsChannelSize = 100
)

type metricChan struct {
	ch     chan metric
	paused *int64
}

func newMetricChan(size int) metricChan {
	return metricChan{
		ch:     make(chan metric, size),
		paused: new(int64),
	}
}

func (ch *metricChan) Pause() {
	atomic.StoreInt64(ch.paused, pausedEnum)
}

func (ch *metricChan) Continue() {
	atomic.StoreInt64(ch.paused, runningEnum)
}

func (ch *metricChan) IsPaused() bool {
	v := atomic.LoadInt64(ch.paused)
	return v == pausedEnum
}

// Push will push metrics to the metric channel if the channel
// is not paused
func (ch *metricChan) Push(m metric) bool {
	if ch.IsPaused() {
		return false
	}

	select {
	case ch.ch <- m:
		return true
	default:
		return false
	}
}
