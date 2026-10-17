package sse

import (
	"sync"
	"sync/atomic"
)

// Limiter bounds concurrent streams per key and in total. A key groups the
// streams of one client or one resource: an account or session ID (not a
// secret token), or the ID of a public item that anyone can watch. The
// limiter holds at most total keys, so its memory stays bounded.
//
// Acquire a slot before the stream starts, and refuse when none is free:
// with an end event that says why (the page stops retrying), or with status
// 429 (the htmx SSE extension retries with backoff).
//
// A Limiter is safe for concurrent use. Create one with [NewLimiter].
type Limiter struct {
	perKey int
	total  int

	mu     sync.Mutex
	active int
	keys   map[string]int
}

// NewLimiter returns a limiter that allows perKey concurrent streams per
// key and total streams in all. It panics when either bound is less than
// one: that is a programming error, so validate configured bounds before.
func NewLimiter(perKey, total int) *Limiter {
	if perKey < 1 || total < 1 {
		panic("sse: NewLimiter needs bounds of at least one")
	}
	return &Limiter{perKey: perKey, total: total, keys: make(map[string]int)}
}

// Acquire takes a slot for key. It returns the function that releases the
// slot and true, or nil and false when key or the total is at its bound.
// The release function is idempotent and safe to call from any goroutine;
// call it when the stream ends, typically with defer:
//
//	release, ok := streams.Acquire(accountID)
//	if !ok {
//		return s.End(ctx, views.LiveEnd("Too many live views are open.", "", ""))
//	}
//	defer release()
func (l *Limiter) Acquire(key string) (release func(), ok bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.active >= l.total || l.keys[key] >= l.perKey {
		return nil, false
	}
	l.active++
	l.keys[key]++
	var released atomic.Bool
	return func() {
		if !released.CompareAndSwap(false, true) {
			return
		}
		l.mu.Lock()
		defer l.mu.Unlock()
		l.active--
		if n := l.keys[key] - 1; n > 0 {
			l.keys[key] = n
		} else {
			delete(l.keys, key)
		}
	}, true
}

// Active returns the number of slots taken in all.
func (l *Limiter) Active() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.active
}

// ActiveKey returns the number of slots taken for key.
func (l *Limiter) ActiveKey(key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.keys[key]
}
