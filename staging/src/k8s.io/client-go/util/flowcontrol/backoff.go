/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package flowcontrol

import (
	"math/rand"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/utils/integer"
)

// TODO(vllry) consider allowing this to be customized in NewBackOff.
const DefaultJitterRatio float64 = 0.20 // 20%

type backoffEntry struct {
	backoff    time.Duration
	lastUpdate time.Time
	randomness *rand.Rand // Make random seed be per item, to avoid the same client line up the jitter.
}

// jitter takes a duration, and decreases or increases it within the jitterRatio.
// EG jitterRatio = 0.2 implies a result between 0.9*duration, and 1.1*duration
func (b *backoffEntry) jitter(duration time.Duration, jitterRatio float64) time.Duration {
	jitter := (0.5 - b.randomness.Float64()) * (float64(duration) * jitterRatio) // Centre jitter.
	duration += time.Duration(jitter)
	return duration
}

type Backoff struct {
	sync.RWMutex
	Clock            clock.Clock
	defaultDuration  time.Duration
	maxDuration      time.Duration
	perItemBackoff   map[string]*backoffEntry
	randomSeedSource func() int64 // Allow a different random source for real and fake Backoff.
}

func NewFakeBackOff(initial, max time.Duration, tc *clock.FakeClock) *Backoff {
	return &Backoff{
		perItemBackoff:  map[string]*backoffEntry{},
		Clock:           tc,
		defaultDuration: initial,
		maxDuration:     max,
		randomSeedSource: func() int64 { // Fixed seed.
			return 0
		},
	}
}

func NewBackOff(initial, max time.Duration) *Backoff {
	return &Backoff{
		perItemBackoff:  map[string]*backoffEntry{},
		Clock:           clock.RealClock{},
		defaultDuration: initial,
		maxDuration:     max,
		randomSeedSource: func() int64 {
			return time.Now().UTC().UnixNano()
		},
	}
}

// Get the current backoff Duration
func (p *Backoff) Get(id string) time.Duration {
	p.RLock()
	defer p.RUnlock()
	var delay time.Duration
	entry, ok := p.perItemBackoff[id]
	if ok {
		delay = entry.backoff
	}
	return delay
}

// move backoff to the next mark, capping at maxDuration
func (p *Backoff) Next(id string, eventTime time.Time) {
	p.Lock()
	defer p.Unlock()
	entry, ok := p.perItemBackoff[id]
	if !ok || hasExpired(eventTime, entry.lastUpdate, p.maxDuration) {
		entry = p.initEntryUnsafe(id)
	} else {
		// Exponential backoff, with a cap. Includes jitter from previous iterations.
		baseDelay := time.Duration(integer.Int64Min(int64(entry.backoff*2), int64(p.maxDuration)))
		entry.backoff = entry.jitter(baseDelay, DefaultJitterRatio)
	}
	entry.lastUpdate = p.Clock.Now()
}

// Reset forces clearing of all backoff data for a given key.
func (p *Backoff) Reset(id string) {
	p.Lock()
	defer p.Unlock()
	delete(p.perItemBackoff, id)
}

// Returns True if the elapsed time since eventTime is smaller than the current backoff window
func (p *Backoff) IsInBackOffSince(id string, eventTime time.Time) bool {
	p.RLock()
	defer p.RUnlock()
	entry, ok := p.perItemBackoff[id]
	if !ok {
		return false
	}
	if hasExpired(eventTime, entry.lastUpdate, p.maxDuration) {
		return false
	}
	return p.Clock.Since(eventTime) < entry.backoff
}

// Returns True if time since lastupdate is less than the current backoff window.
func (p *Backoff) IsInBackOffSinceUpdate(id string, eventTime time.Time) bool {
	p.RLock()
	defer p.RUnlock()
	entry, ok := p.perItemBackoff[id]
	if !ok {
		return false
	}
	if hasExpired(eventTime, entry.lastUpdate, p.maxDuration) {
		return false
	}
	return eventTime.Sub(entry.lastUpdate) < entry.backoff
}

// Garbage collect records that have aged past maxDuration. Backoff users are expected
// to invoke this periodically.
func (p *Backoff) GC() {
	p.Lock()
	defer p.Unlock()
	now := p.Clock.Now()
	for id, entry := range p.perItemBackoff {
		if now.Sub(entry.lastUpdate) > p.maxDuration*2 {
			// GC when entry has not been updated for 2*maxDuration
			delete(p.perItemBackoff, id)
		}
	}
}

func (p *Backoff) DeleteEntry(id string) {
	p.Lock()
	defer p.Unlock()
	delete(p.perItemBackoff, id)
}

// Take a lock on *Backoff, before calling initEntryUnsafe
func (p *Backoff) initEntryUnsafe(id string) *backoffEntry {
	entry := &backoffEntry{
		backoff:    p.defaultDuration,
		randomness: rand.New(rand.NewSource(p.randomSeedSource())), // Create a per-item seed.
	}
	p.perItemBackoff[id] = entry
	return entry
}

// After 2*maxDuration we restart the backoff factor to the beginning
func hasExpired(eventTime time.Time, lastUpdate time.Time, maxDuration time.Duration) bool {
	return eventTime.Sub(lastUpdate) > maxDuration*2 // consider stable if it's ok for twice the maxDuration
}
