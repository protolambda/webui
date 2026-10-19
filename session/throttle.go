package session

import (
	"errors"
	"sync"
	"time"
)

// Defaults of [ThrottleOptions].
const (
	// DefaultThrottleLimit is the number of attempts a key may make within
	// one window.
	DefaultThrottleLimit = 10
	// DefaultThrottleWindow is the length of a window.
	DefaultThrottleWindow = time.Minute
	// DefaultThrottleKeys bounds the keys a throttle remembers.
	DefaultThrottleKeys = 4096
)

// ThrottleOptions configures a [Throttle]. The zero value gives the
// defaults.
type ThrottleOptions struct {
	// Limit is the number of attempts a key may make within one Window;
	// zero means [DefaultThrottleLimit].
	Limit int
	// Window is how long a key's attempts count: a window starts with the
	// first attempt of a key, and the next attempt after it passed starts
	// another. Zero means [DefaultThrottleWindow]. Like the lifetimes of
	// sessions, the window counts time the computer spends asleep (see
	// [Options.IdleTTL]).
	Window time.Duration
	// MaxKeys bounds the keys the throttle remembers, and so its memory;
	// zero means [DefaultThrottleKeys]. A new key beyond the bound drops
	// the key seen least recently, with its attempts: a flood of keys can
	// make a quiet key forget its count, so size the bound above the
	// number of keys that attempt within a window.
	MaxKeys int
	// Now returns the current time; nil means [time.Now].
	Now func() time.Time
}

// Throttle bounds the attempts per key within a window, with memory
// bounded by the number of keys it remembers: a throttle for logins,
// keyed by the account name and by the client address, which counts
// successful logins as well as failed ones (a client that can log in, even
// to one account, can otherwise push the sessions of others out of a
// [Store]; see [Options.MaxSessions]). Create it with [NewThrottle]. It is
// safe for concurrent use.
//
// The window is fixed: it starts with a key's first attempt, every attempt
// within it counts, and once the count reaches the limit the key is
// refused until the window has passed; the next attempt then starts a new
// window. Refused attempts do not extend the window.
//
// Key logins twice, with the name of the account (whatever its case, so
// that "Ada" and "ada" share a count) and with the client's address group
// (server.AddrPrefix of the request's RemoteAddr, or the address a trusted
// reverse proxy reports), and refuse when either key is at its limit: the
// name bounds guessing one password from many addresses, the address
// bounds guessing many passwords from one. Answer a refusal with 429 Too
// Many Requests and a Retry-After header (see the example).
// Resolve a proxy's client address with an explicit trust policy, such as
// github.com/protolambda/webui/mw.ClientIP: using the proxy's RemoteAddr
// would give all of its clients one shared allowance.
type Throttle struct {
	limit  int
	window time.Duration
	max    int
	now    func() time.Time

	mu   sync.Mutex
	keys map[string]*attempts
	// head is the key seen most recently, tail the least.
	head, tail *attempts
}

// attempts counts the attempts of one key in its current window.
type attempts struct {
	key   string
	count int
	// start is the start of the window; the zero time before the first.
	start      time.Time
	prev, next *attempts
}

var errThrottleOptions = errors.New("session: negative limit, window or key bound in throttle options")

// NewThrottle returns a throttle configured by opts. It fails on a
// negative limit, window or key bound.
func NewThrottle(opts ThrottleOptions) (*Throttle, error) {
	if opts.Limit < 0 || opts.Window < 0 || opts.MaxKeys < 0 {
		return nil, errThrottleOptions
	}
	t := &Throttle{
		limit:  opts.Limit,
		window: opts.Window,
		max:    opts.MaxKeys,
		now:    opts.Now,
		keys:   make(map[string]*attempts),
	}
	if t.limit == 0 {
		t.limit = DefaultThrottleLimit
	}
	if t.window == 0 {
		t.window = DefaultThrottleWindow
	}
	if t.max == 0 {
		t.max = DefaultThrottleKeys
	}
	if t.now == nil {
		t.now = time.Now
	}
	return t, nil
}

// Allow counts an attempt of key and reports whether it is within the
// limit. When it is not, nothing is counted and retryAfter says how long
// until the window of key has passed, for a Retry-After header (round it
// up to whole seconds).
func (t *Throttle) Allow(key string) (retryAfter time.Duration, ok bool) {
	now := t.now()
	t.mu.Lock()
	defer t.mu.Unlock()
	a := t.keys[key]
	if a == nil {
		if len(t.keys) >= t.max {
			t.dropLocked(t.tail)
		}
		a = &attempts{key: key}
		t.keys[key] = a
		t.pushFrontLocked(a)
	} else {
		t.moveToFrontLocked(a)
	}
	if a.count == 0 || elapsed(a.start, now) >= t.window {
		a.start, a.count = now, 0
	}
	if a.count >= t.limit {
		return max(t.window-elapsed(a.start, now), 0), false
	}
	a.count++
	return 0, true
}

// Len returns the number of keys the throttle remembers.
func (t *Throttle) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.keys)
}

// dropLocked forgets a, which is listed.
func (t *Throttle) dropLocked(a *attempts) {
	delete(t.keys, a.key)
	t.unlinkLocked(a)
}

func (t *Throttle) pushFrontLocked(a *attempts) {
	a.prev, a.next = nil, t.head
	if t.head != nil {
		t.head.prev = a
	} else {
		t.tail = a
	}
	t.head = a
}

func (t *Throttle) unlinkLocked(a *attempts) {
	if a.prev != nil {
		a.prev.next = a.next
	} else {
		t.head = a.next
	}
	if a.next != nil {
		a.next.prev = a.prev
	} else {
		t.tail = a.prev
	}
	a.prev, a.next = nil, nil
}

func (t *Throttle) moveToFrontLocked(a *attempts) {
	if t.head == a {
		return
	}
	t.unlinkLocked(a)
	t.pushFrontLocked(a)
}
