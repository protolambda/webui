package webuitest_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/html/text"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/webuitest"
)

// recordT is a TB that records failures instead of failing, to test how
// drivers fail. FailNow returns, like PrintT's.
type recordT struct {
	ctx      context.Context
	mu       sync.Mutex
	msgs     []string
	failed   bool
	cleanups []func()
}

var _ webuitest.TB = (*recordT)(nil)

func newRecordT(ctx context.Context) *recordT { return &recordT{ctx: ctx} }

func (r *recordT) Helper()                  {}
func (r *recordT) Context() context.Context { return r.ctx }

func (r *recordT) Error(args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = append(r.msgs, fmt.Sprint(args...))
}

func (r *recordT) FailNow() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failed = true
}

func (r *recordT) Cleanup(f func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cleanups = append(r.cleanups, f)
}

// cleanup runs the cleanups, last first.
func (r *recordT) cleanup() {
	r.mu.Lock()
	fs := r.cleanups
	r.cleanups = nil
	r.mu.Unlock()
	for i := len(fs) - 1; i >= 0; i-- {
		fs[i]()
	}
}

func (r *recordT) output() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Join(r.msgs, "\n")
}

func TestAll(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(webuitest.All(""))
	t.Must(webuitest.All("x", be.True(true)))
	err := webuitest.All("", be.True(true), nil).Check(t.Context())
	t.Must(be.ErrorIs(err, webuitest.ErrNilAssertion))
	t.Must(be.ErrorContains(err, "all(isTrue(true), <nil>): webuitest: nil assertion: assertion 1 is nil"))
	inner := webuitest.All("inner", be.ErrorIs(webuitest.ErrLeak, webuitest.ErrLeak), webuitest.All("ct", ct.View(text.P()()).Find(ct.Tag("a"))))
	err = inner.Check(t.Context())
	t.Must(be.ErrorIs(err, ct.ErrCount))
	t.Must(be.ErrorContains(err, "inner: ct: unexpected number of matches"))
	t.Must(be.Equal("inner", inner.String()))
}

func TestPrintT(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var p webuitest.PrintT
	p.Helper()
	p.FailNow()
	p.Cleanup(func() { t.Fatal("PrintT runs no cleanups") })
	t.Must(be.NoError(p.Context().Err()))
}
