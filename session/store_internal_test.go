package session

import (
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"
	"unsafe"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"
)

// checkList verifies that the recency list and the map hold the same
// sessions, linked consistently, in order of last use.
func checkList[T any](t devtest.T, st *Store[T]) {
	st.mu.Lock()
	defer st.mu.Unlock()
	n := 0
	var prev *Session[T]
	for s := st.head; s != nil; s = s.next {
		t.Must(be.Same(prev, s.prev))
		t.Must(be.True(s.listed))
		t.Must(be.Same(s, st.sessions[s.key]))
		if prev != nil {
			t.Must(be.False(prev.lastUsed.Before(s.lastUsed)))
		}
		prev = s
		n++
	}
	t.Must(be.Same(prev, st.tail))
	t.Must(be.Equal(len(st.sessions), n))
}

func TestStoreListInvariants(gt *testing.T) {
	t := devtest.ParallelT(gt)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	st, err := NewStore(Options[int]{
		MaxSessions: 8,
		IdleTTL:     time.Hour,
		AbsoluteTTL: 3 * time.Hour,
		Now:         func() time.Time { return now },
		Rand:        rand.NewChaCha8([32]byte{7}),
		Passive:     func(r *http.Request) bool { return r.URL.Path == "/live" },
	})
	t.Must(be.NoError(err))
	ops := rand.New(rand.NewPCG(1, 2))
	var cookies []*http.Cookie
	var sessions []*Session[int]
	request := func(path string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		if len(cookies) > 0 {
			r.AddCookie(cookies[ops.IntN(len(cookies))])
		}
		return r
	}
	for i := range 2000 {
		switch op := ops.IntN(10); {
		case op < 3:
			rec := httptest.NewRecorder()
			var r *http.Request
			if ops.IntN(2) == 0 {
				r = request("/login")
			}
			s, err := st.New(rec, r, i)
			t.Must(be.NoError(err))
			cookies = append(cookies, rec.Result().Cookies()[0])
			sessions = append(sessions, s)
		case op < 6:
			st.Lookup(request("/"))
		case op == 6:
			st.Lookup(request("/live"))
		case op == 7 && len(sessions) > 0:
			sessions[ops.IntN(len(sessions))].End()
		case op == 8:
			st.Sweep()
		default:
			now = now.Add(time.Duration(ops.IntN(20)) * time.Minute)
		}
		checkList(t, st)
		t.Must(be.LessOrEq(st.Len(), 8))
	}
	for _, s := range sessions {
		st.mu.Lock()
		listed := s.listed
		st.mu.Unlock()
		t.Must(be.Equal(!listed, s.Ended()))
	}
	st.EndAll()
	checkList(t, st)
	t.Must(be.NilPtr(st.head))
}

type item struct{ ID int }

// The internal test can observe the store lock: it lets match return only
// once Rotate owns that lock and is waiting for the session lock that
// match holds. This forces the check/end interleaving without a sleep or
// an assumption about how quickly the rotation goroutine is scheduled.
func TestEndFuncFollowsRotationAfterMatch(gt *testing.T) {
	t := devtest.ParallelT(gt)
	st, err := NewStore(Options[int]{Rand: rand.NewChaCha8([32]byte{10})})
	t.Must(be.NoError(err))
	oldRec := httptest.NewRecorder()
	old, err := st.New(oldRec, nil, 1)
	t.Must(be.NoError(err))
	other, err := st.New(httptest.NewRecorder(), nil, 2)
	t.Must(be.NoError(err))
	r := httptest.NewRequest(http.MethodPost, "/reauth", nil)
	r.AddCookie(oldRec.Result().Cookies()[0])

	var rotated *Session[int]
	var rotateErr error
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	started := false
	n := st.EndFunc(func(id int) bool {
		if id == 1 && !started {
			started = true
			go func() {
				defer close(done)
				rotated, rotateErr = st.Rotate(rec, r)
			}()
			for st.mu.TryLock() {
				st.mu.Unlock()
				select {
				case <-done:
					t.Fatal("rotation returned before acquiring the store lock")
				default:
					runtime.Gosched()
				}
			}
			// Only Rotate can hold st.mu here. It cannot release it until
			// match returns and releases old.mu, so it moves the value
			// before EndFunc can acquire st.mu to end the old session.
		}
		return id == 1
	})
	<-done
	t.Must(be.NoError(rotateErr))
	t.Must(be.Equal(1, n))
	t.Must(be.Equal(EndRotated, old.EndReason()))
	t.Must(be.True(rotated.Ended()))
	t.Must(be.Equal(EndRevoked, rotated.EndReason()))
	r = httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(rec.Result().Cookies()[0])
	_, ok := st.Lookup(r)
	t.Mustf(be.False(ok), "the rotated cookie names no session")
	t.Must(be.False(other.Ended()))
	t.Must(be.Equal(1, st.Len()))
}

func TestEndFuncPanicLeavesSessionsUnlocked(gt *testing.T) {
	t := devtest.ParallelT(gt)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var ended []string
	st, err := NewStore(Options[*item]{
		IdleTTL: time.Hour,
		Now:     func() time.Time { return now },
		Rand:    rand.NewChaCha8([32]byte{8}),
		OnEnd:   func(id string, _ **item) { ended = append(ended, id) },
	})
	t.Must(be.NoError(err))
	other, err := st.New(httptest.NewRecorder(), nil, &item{ID: 2})
	t.Must(be.NoError(err))
	empty, err := st.New(httptest.NewRecorder(), nil, nil) // the most recent: checked first
	t.Must(be.NoError(err))

	// A match that dereferences a nil value panics on the first session.
	t.Must(be.Panic(func() { st.EndFunc(func(v *item) bool { return v.ID == 1 }) }))
	for _, s := range []*Session[*item]{empty, other} {
		t.Must(be.True(s.mu.TryLock()))
		s.mu.Unlock()
		t.Must(be.False(s.Ended()))
	}
	data, ok := other.Data()
	t.Must(be.True(ok))
	t.Must(be.Equal(2, data.ID))

	// The sweeper is not stuck on the session either.
	now = now.Add(time.Hour)
	t.Must(be.Equal(2, st.Sweep()))
	t.Must(be.True(empty.Ended()))
	t.Must(be.True(other.Ended()))
	t.Must(be.Len(ended, 2))
}

func TestEachPanicLeavesSessionsUnlocked(gt *testing.T) {
	t := devtest.ParallelT(gt)
	st, err := NewStore(Options[*item]{Rand: rand.NewChaCha8([32]byte{9})})
	t.Must(be.NoError(err))
	other, err := st.New(httptest.NewRecorder(), nil, &item{ID: 2})
	t.Must(be.NoError(err))
	empty, err := st.New(httptest.NewRecorder(), nil, nil) // the most recent: visited first
	t.Must(be.NoError(err))

	// A visitor that dereferences a nil value panics on the first session.
	t.Must(be.Panic(func() { st.Each(func(v *item) { _ = v.ID }) }))
	for _, s := range []*Session[*item]{empty, other} {
		t.Must(be.True(s.mu.TryLock()))
		s.mu.Unlock()
		t.Must(be.False(s.Ended()))
	}
	t.Must(be.True(st.mu.TryLock()))
	st.mu.Unlock()
	checkList(t, st)
}

// timeLayout is the layout of [time.Time] in the Go releases this module
// supports: when the hasMonotonic bit of wall is set, ext holds the
// monotonic clock reading in nanoseconds.
type timeLayout struct {
	wall uint64
	ext  int64
	loc  *time.Location
}

// skewed returns t with its wall clock moved by wall and its monotonic
// clock by mono, as when the clocks drift apart: the monotonic clock stops
// on some systems while the computer sleeps, and the wall clock can be set
// back. The time package offers no way to build such a time, so skewed
// edits the fields; ok reports whether the result behaves as intended.
func skewed(t time.Time, wall, mono time.Duration) (time.Time, bool) {
	if unsafe.Sizeof(t) != unsafe.Sizeof(timeLayout{}) {
		return t, false
	}
	u := t.Add(wall)
	(*timeLayout)(unsafe.Pointer(&u)).ext = (*timeLayout)(unsafe.Pointer(&t)).ext + int64(mono)
	return u, u.Sub(t) == mono && u.Round(0).Sub(t.Round(0)) == wall
}

func TestElapsed(gt *testing.T) {
	t := devtest.ParallelT(gt)
	wall := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t.Must(be.Equal(time.Minute, elapsed(wall, wall.Add(time.Minute))))
	t.Must(be.Equal(-time.Minute, elapsed(wall, wall.Add(-time.Minute))))

	t0 := time.Now()
	t.Must(be.Equal(time.Hour, elapsed(t0, t0.Add(time.Hour))))
	t.Must(be.Equal(time.Hour, elapsed(t0.Round(0), t0.Add(time.Hour))))
	t.Must(be.Equal(time.Hour, elapsed(t0, t0.Round(0).Add(time.Hour))))

	asleep, ok := skewed(t0, 10*time.Hour, time.Minute)
	if !ok {
		t.Skip("cannot build a time whose clocks disagree")
	}
	t.Must(be.Equal(10*time.Hour, elapsed(t0, asleep)))
	setBack, ok := skewed(t0, -time.Hour, 20*time.Minute)
	t.Must(be.True(ok))
	t.Must(be.Equal(20*time.Minute, elapsed(t0, setBack)))
}

func TestExpiryCountsTimeAsleep(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t0 := time.Now()
	idle, ok := skewed(t0, 10*time.Hour, time.Minute) // the lid was closed for 10 hours
	if !ok {
		t.Skip("cannot build a time whose clocks disagree")
	}
	absolute, ok := skewed(t0, 13*time.Hour, time.Minute)
	t.Must(be.True(ok))
	setBack, ok := skewed(t0, -time.Hour, 20*time.Minute)
	t.Must(be.True(ok))

	now := t0
	st, err := NewStore(Options[int]{
		Now:  func() time.Time { return now },
		Rand: rand.NewChaCha8([32]byte{9}),
	})
	t.Must(be.NoError(err))
	login := func() (*Session[int], *http.Request) {
		now = t0
		rec := httptest.NewRecorder()
		s, err := st.New(rec, nil, 1)
		t.Must(be.NoError(err))
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.AddCookie(rec.Result().Cookies()[0])
		return s, r
	}

	s, r := login()
	now = idle
	_, ok = st.Lookup(r)
	t.Must(be.False(ok))
	t.Must(be.Equal(EndIdle, s.EndReason()))

	s, _ = login()
	now = absolute
	t.Must(be.Equal(1, st.Sweep()))
	t.Must(be.Equal(EndAbsolute, s.EndReason()))

	// Setting the wall clock back does not lengthen a session.
	s, r = login()
	now = setBack
	_, ok = st.Lookup(r)
	t.Must(be.False(ok))
	t.Must(be.Equal(EndIdle, s.EndReason()))
}
