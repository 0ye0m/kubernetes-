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

package clock

import (
	"runtime"
	"testing"
	"time"
)

var (
	_ = Clock(RealClock{})
	_ = Clock(&FakeClock{})
	_ = Clock(&IntervalClock{})

	_ = Timer(&realTimer{})
	_ = Timer(&fakeTimer{})

	_ = Ticker(&realTicker{})
	_ = Ticker(&fakeTicker{})
)

func TestFakeClock(t *testing.T) {
	startTime := time.Now()
	tc := NewFakeClock(startTime)
	tc.Step(time.Second)
	now := tc.Now()
	if now.Sub(startTime) != time.Second {
		t.Errorf("input: %s now=%s gap=%s expected=%s", startTime, now, now.Sub(startTime), time.Second)
	}

	tt := tc.Now()
	tc.SetTime(tt.Add(time.Hour))
	if tc.Now().Sub(tt) != time.Hour {
		t.Errorf("input: %s now=%s gap=%s expected=%s", tt, tc.Now(), tc.Now().Sub(tt), time.Hour)
	}
}

func TestFakeClockSleep(t *testing.T) {
	startTime := time.Now()
	tc := NewFakeClock(startTime)
	tc.Sleep(time.Duration(1) * time.Hour)
	now := tc.Now()
	if now.Sub(startTime) != time.Hour {
		t.Errorf("Fake sleep failed, expected time to advance by one hour, instead, its %v", now.Sub(startTime))
	}
}

func TestFakeAfter(t *testing.T) {
	tc := NewFakeClock(time.Now())
	if tc.HasWaiters() {
		t.Errorf("unexpected waiter?")
	}
	oneSec := tc.After(time.Second)
	if !tc.HasWaiters() {
		t.Errorf("unexpected lack of waiter?")
	}

	oneOhOneSec := tc.After(time.Second + time.Millisecond)
	twoSec := tc.After(2 * time.Second)
	select {
	case <-oneSec:
		t.Errorf("unexpected channel read")
	case <-oneOhOneSec:
		t.Errorf("unexpected channel read")
	case <-twoSec:
		t.Errorf("unexpected channel read")
	default:
	}

	tc.Step(999 * time.Millisecond)
	select {
	case <-oneSec:
		t.Errorf("unexpected channel read")
	case <-oneOhOneSec:
		t.Errorf("unexpected channel read")
	case <-twoSec:
		t.Errorf("unexpected channel read")
	default:
	}

	tc.Step(time.Millisecond)
	select {
	case <-oneSec:
		// Expected!
	case <-oneOhOneSec:
		t.Errorf("unexpected channel read")
	case <-twoSec:
		t.Errorf("unexpected channel read")
	default:
		t.Errorf("unexpected non-channel read")
	}
	tc.Step(time.Millisecond)
	select {
	case <-oneSec:
		// should not double-trigger!
		t.Errorf("unexpected channel read")
	case <-oneOhOneSec:
		// Expected!
	case <-twoSec:
		t.Errorf("unexpected channel read")
	default:
		t.Errorf("unexpected non-channel read")
	}
}

func TestFakeTick(t *testing.T) {
	tc := NewFakeClock(time.Now())
	if tc.HasWaiters() {
		t.Errorf("unexpected waiter?")
	}
	oneSec := tc.NewTicker(time.Second).C()
	if !tc.HasWaiters() {
		t.Errorf("unexpected lack of waiter?")
	}

	oneOhOneSec := tc.NewTicker(time.Second + time.Millisecond).C()
	twoSec := tc.NewTicker(2 * time.Second).C()
	select {
	case <-oneSec:
		t.Errorf("unexpected channel read")
	case <-oneOhOneSec:
		t.Errorf("unexpected channel read")
	case <-twoSec:
		t.Errorf("unexpected channel read")
	default:
	}

	tc.Step(999 * time.Millisecond) // t=.999
	select {
	case <-oneSec:
		t.Errorf("unexpected channel read")
	case <-oneOhOneSec:
		t.Errorf("unexpected channel read")
	case <-twoSec:
		t.Errorf("unexpected channel read")
	default:
	}

	tc.Step(time.Millisecond) // t=1.000
	select {
	case <-oneSec:
		// Expected!
	case <-oneOhOneSec:
		t.Errorf("unexpected channel read")
	case <-twoSec:
		t.Errorf("unexpected channel read")
	default:
		t.Errorf("unexpected non-channel read")
	}
	tc.Step(time.Millisecond) // t=1.001
	select {
	case <-oneSec:
		// should not double-trigger!
		t.Errorf("unexpected channel read")
	case <-oneOhOneSec:
		// Expected!
	case <-twoSec:
		t.Errorf("unexpected channel read")
	default:
		t.Errorf("unexpected non-channel read")
	}

	tc.Step(time.Second) // t=2.001
	tc.Step(time.Second) // t=3.001
	tc.Step(time.Second) // t=4.001
	tc.Step(time.Second) // t=5.001

	// The one second ticker should not accumulate ticks
	accumulatedTicks := 0
	drained := false
	for !drained {
		select {
		case <-oneSec:
			accumulatedTicks++
		default:
			drained = true
		}
	}
	if accumulatedTicks != 1 {
		t.Errorf("unexpected number of accumulated ticks: %d", accumulatedTicks)
	}
}

// This test can fail under high CPU contention. The simple way to demonstrate this is:
// ```
// go get -u golang.org/x/tools/cmd/stress
// stress go test -run TestMissedTicks
// ```
func TestMissedTicks(t *testing.T) {
	tc := NewFakeClock(time.Now())
	oneSec := tc.NewTicker(time.Second).C()

	done := make(chan struct{})
	expectedTickCount := 5
	go func() {
		var tickCount int
		for {
			select {
			case <-oneSec:
				tickCount++
			case <-done:
				if tickCount != expectedTickCount {
					t.Errorf("unexpected number of ticks: want %d got %d", expectedTickCount, tickCount)
				}
				done <- struct{}{}
			}
		}
	}()

	// Tick 5 times and then "wait" until all ticks have been counted before finishing the test.
	for i := 0; i < expectedTickCount; i++ {
		tc.Step(time.Second)
		// "ensure" that the other goroutine has read the tick before we continue.
		// THIS IS FLAKY!
		runtime.Gosched()
		// Adding a second call to Gosched reduces the failure %, but not to zero.
		runtime.Gosched()
	}

	done <- struct{}{}
	<-done
}
