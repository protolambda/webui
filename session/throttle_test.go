package session_test

import (
	"sync"
	"testing"
	"time"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/session"
)

func newThrottle(t devtest.T, opts session.ThrottleOptions) *session.Throttle {
	th, err := session.NewThrottle(opts)
	t.Must(be.NoError(err))
	return th
}

func TestThrottleFixedWindow(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clock := newClock()
	th := newThrottle(t, session.ThrottleOptions{Limit: 3, Window: time.Minute, Now: clock.Now})
	for i := range 3 {
		clock.Advance(10 * time.Second)
		retry, ok := th.Allow("ada")
		t.Mustf(be.True(ok), "attempt %d", i+1)
		t.Must(be.Equal(time.Duration(0), retry))
	}
	// The window started with the first attempt, 20 seconds ago.
	retry, ok := th.Allow("ada")
	t.Must(be.False(ok))
	t.Must(be.Equal(40*time.Second, retry))
	// Refused attempts neither count nor extend the window.
	clock.Advance(20 * time.Second)
	retry, ok = th.Allow("ada")
	t.Must(be.False(ok))
	t.Must(be.Equal(20*time.Second, retry))
	clock.Advance(10 * time.Second)
	retry, ok = th.Allow("ada")
	t.Must(be.False(ok))
	t.Must(be.Equal(10*time.Second, retry))
	clock.Advance(10 * time.Second)
	_, ok = th.Allow("ada")
	t.Mustf(be.True(ok), "a new window once the old one passed")
	// The new window counts from this attempt.
	clock.Advance(59 * time.Second)
	_, ok = th.Allow("ada")
	t.Must(be.True(ok))
	_, ok = th.Allow("ada")
	t.Must(be.True(ok))
	retry, ok = th.Allow("ada")
	t.Must(be.False(ok))
	t.Must(be.Equal(time.Second, retry))
}

func TestThrottleKeysAreIndependent(gt *testing.T) {
	t := devtest.ParallelT(gt)
	th := newThrottle(t, session.ThrottleOptions{Limit: 1})
	_, ok := th.Allow("ada")
	t.Must(be.True(ok))
	_, ok = th.Allow("ada")
	t.Must(be.False(ok))
	_, ok = th.Allow("bo")
	t.Must(be.True(ok))
	_, ok = th.Allow("Ada") // the caller folds the case when it should share a count
	t.Must(be.True(ok))
	t.Must(be.Equal(3, th.Len()))
}

// The throttle remembers MaxKeys keys at most: a new key beyond the bound
// drops the key seen least recently, with its count.
func TestThrottleBoundsKeys(gt *testing.T) {
	t := devtest.ParallelT(gt)
	th := newThrottle(t, session.ThrottleOptions{Limit: 1, MaxKeys: 2})
	for _, key := range []string{"a", "b"} {
		_, ok := th.Allow(key)
		t.Must(be.True(ok))
	}
	_, ok := th.Allow("a") // refused, and a is now the most recently seen
	t.Must(be.False(ok))
	_, ok = th.Allow("c") // drops b
	t.Must(be.True(ok))
	t.Must(be.Equal(2, th.Len()))
	_, ok = th.Allow("a")
	t.Mustf(be.False(ok), "a is remembered")
	_, ok = th.Allow("b")
	t.Mustf(be.True(ok), "b was forgotten, c stays")
	t.Must(be.Equal(2, th.Len()))
	_, ok = th.Allow("c")
	t.Mustf(be.True(ok), "c was the least recently seen when b arrived, and was dropped")
	t.Must(be.Equal(2, th.Len()))
}

func TestThrottleDefaults(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clock := newClock()
	th := newThrottle(t, session.ThrottleOptions{Now: clock.Now})
	for i := range session.DefaultThrottleLimit {
		_, ok := th.Allow("ada")
		t.Mustf(be.True(ok), "attempt %d", i+1)
	}
	retry, ok := th.Allow("ada")
	t.Must(be.False(ok))
	t.Must(be.Equal(session.DefaultThrottleWindow, retry))
	clock.Advance(session.DefaultThrottleWindow)
	_, ok = th.Allow("ada")
	t.Must(be.True(ok))
	t.Must(be.Equal(10, session.DefaultThrottleLimit))
	t.Must(be.Equal(time.Minute, session.DefaultThrottleWindow))
	t.Must(be.Equal(4096, session.DefaultThrottleKeys))
}

func TestNewThrottleRejectsBadOptions(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for name, opts := range map[string]session.ThrottleOptions{
		"negative limit":  {Limit: -1},
		"negative window": {Window: -time.Second},
		"negative keys":   {MaxKeys: -1},
	} {
		_, err := session.NewThrottle(opts)
		t.Mustf(be.Error(err), name)
	}
}

// TestThrottleCountsTimeAsleep sets the wall clock forward without a
// monotonic reading, as a computer that slept reports time: the window
// passes.
func TestThrottleCountsTimeAsleep(gt *testing.T) {
	t := devtest.ParallelT(gt)
	now := time.Now()
	th := newThrottle(t, session.ThrottleOptions{Limit: 1, Window: time.Minute, Now: func() time.Time { return now }})
	_, ok := th.Allow("ada")
	t.Must(be.True(ok))
	_, ok = th.Allow("ada")
	t.Must(be.False(ok))
	now = now.Round(0).Add(time.Minute) // the wall clock alone advanced
	_, ok = th.Allow("ada")
	t.Must(be.True(ok))
}

func TestThrottleConcurrentUse(gt *testing.T) {
	t := devtest.ParallelT(gt)
	const limit, goroutines, attempts = 50, 8, 100
	th := newThrottle(t, session.ThrottleOptions{Limit: limit, MaxKeys: 16})
	var wg sync.WaitGroup
	allowed := make(chan int, goroutines)
	for g := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n := 0
			for i := range attempts {
				if _, ok := th.Allow("shared"); ok {
					n++
				}
				// Other keys churn the bound meanwhile.
				_, _ = th.Allow(string(rune('a' + (g*attempts+i)%26)))
			}
			allowed <- n
		}()
	}
	wg.Wait()
	close(allowed)
	total := 0
	for n := range allowed {
		total += n
	}
	t.Must(be.Equal(limit, total))
	t.Must(be.True(th.Len() <= 16))
}
