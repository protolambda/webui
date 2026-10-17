package session

import (
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
