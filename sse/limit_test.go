package sse_test

import (
	"sync"
	"testing"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/sse"
)

func TestLimiterPerKey(gt *testing.T) {
	t := devtest.ParallelT(gt)
	l := sse.NewLimiter(2, 10)
	r1, ok := l.Acquire("a")
	t.Must(be.True(ok))
	r2, ok := l.Acquire("a")
	t.Must(be.True(ok))
	r3, ok := l.Acquire("a")
	t.Must(be.False(ok))
	t.Must(be.Nil(r3))
	_, ok = l.Acquire("b") // other keys are not affected
	t.Must(be.True(ok))
	t.Must(be.Equal(2, l.ActiveKey("a")))
	t.Must(be.Equal(3, l.Active()))

	r1()
	t.Must(be.Equal(1, l.ActiveKey("a")))
	_, ok = l.Acquire("a")
	t.Must(be.True(ok))
	r2()
	t.Must(be.Equal(1, l.ActiveKey("a")))
	t.Must(be.Equal(0, l.ActiveKey("unknown")))
}

func TestLimiterTotal(gt *testing.T) {
	t := devtest.ParallelT(gt)
	l := sse.NewLimiter(5, 2)
	ra, ok := l.Acquire("a")
	t.Must(be.True(ok))
	_, ok = l.Acquire("b")
	t.Must(be.True(ok))
	_, ok = l.Acquire("c")
	t.Must(be.False(ok))
	_, ok = l.Acquire("a")
	t.Must(be.False(ok))
	ra()
	_, ok = l.Acquire("c")
	t.Must(be.True(ok))
	t.Must(be.Equal(2, l.Active()))
}

// Releasing twice frees one slot, not two.
func TestLimiterReleaseIsIdempotent(gt *testing.T) {
	t := devtest.ParallelT(gt)
	l := sse.NewLimiter(1, 1)
	release, ok := l.Acquire("")
	t.Must(be.True(ok))
	release()
	release()
	t.Must(be.Equal(0, l.Active()))
	again, ok := l.Acquire("")
	t.Must(be.True(ok))
	release() // a stale release must not free the new slot
	t.Must(be.Equal(1, l.Active()))
	_, ok = l.Acquire("")
	t.Must(be.False(ok))
	again()
	t.Must(be.Equal(0, l.Active()))
}

func TestLimiterConcurrent(gt *testing.T) {
	t := devtest.ParallelT(gt)
	const perKey, total = 3, 8
	l := sse.NewLimiter(perKey, total)
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		maxSeen int
	)
	for i := range 64 {
		key := []string{"a", "b", "c", "d"}[i%4]
		wg.Go(func() {
			for range 50 {
				release, ok := l.Acquire(key)
				if !ok {
					continue
				}
				n, k := l.Active(), l.ActiveKey(key)
				mu.Lock()
				maxSeen = max(maxSeen, n)
				mu.Unlock()
				if n > total || k > perKey {
					gt.Errorf("bounds exceeded: %d total, %d for key", n, k)
				}
				release()
				release()
			}
		})
	}
	wg.Wait()
	t.Must(be.Equal(0, l.Active()))
	t.Must(be.LessOrEq(maxSeen, total))
}

func TestNewLimiterPanicsOnInvalidBounds(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Panic(func() { sse.NewLimiter(0, 1) }))
	t.Must(be.Panic(func() { sse.NewLimiter(1, 0) }))
	t.Must(be.Panic(func() { sse.NewLimiter(-1, -1) }))
	t.Must(be.NoPanic(func() { sse.NewLimiter(1, 1) }))
}
