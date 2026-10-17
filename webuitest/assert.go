package webuitest

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/protolambda/mustbe"
	"github.com/protolambda/mustbe/assertion"
)

// TB is the part of [testing.TB] that the drivers of this package use: the
// request builders, [Client], [Browser], [Stream] and [Crawl]. *testing.T,
// *testing.B, devtest.T and [PrintT] satisfy it, and every TB is a
// [mustbe.T].
//
// Drivers report infrastructure failures (a request that cannot be built, a
// stream that does not answer) with Error followed by FailNow. When FailNow
// returns, as it does for [PrintT], the driver returns a zero value.
//
// The [mustbe.MT] of mustbe.WrapT is not a TB: it has no Cleanup, which
// [ServeStream], [NewBrowser] and [Browser.Stream] need to close what they
// start and to report a handler that panics or does not return. Pass the
// *testing.T to drivers and the MT to Must. That is the pattern inside a
// testing/synctest bubble, where devtest.T cannot be used because it calls
// T.Deadline:
//
//	synctest.Test(gt, func(gt *testing.T) {
//		t := mustbe.WrapT(gt)
//		st := webuitest.ServeStream(gt, h, webuitest.Get(gt, "/events", webuitest.EventSource()))
//		t.Must(be.Equal("tick", st.Next(gt).Name))
//	})
type TB interface {
	Helper()
	Context() context.Context
	Error(args ...any)
	FailNow()
	Cleanup(func())
}

var _ mustbe.T = TB(nil)

// fatalf reports a driver failure and stops the test where FailNow does.
func fatalf(t TB, format string, args ...any) {
	t.Helper()
	t.Error(fmt.Sprintf(format, args...))
	t.FailNow()
}

// PrintT is a [TB] and a [mustbe.MT] for Example tests, which have no
// testing.TB: Error prints its arguments to standard output, so that an
// Example's // Output: block shows what a failing assertion reports.
//
// FailNow does nothing, so execution continues after a failure. Context
// returns context.Background(). Cleanup drops the function, because an
// Example has no end of test: close streams and servers explicitly. Drivers
// that fail return zero values, so Examples should only show driver calls
// that succeed, and assertion failures.
type PrintT struct{}

var (
	_ TB        = PrintT{}
	_ mustbe.MT = PrintT{}
)

// Error prints args like fmt.Println.
func (PrintT) Error(args ...any) { fmt.Println(args...) }

// FailNow does nothing.
func (PrintT) FailNow() {}

// Context returns context.Background().
func (PrintT) Context() context.Context { return context.Background() }

// Helper does nothing.
func (PrintT) Helper() {}

// Cleanup drops fn: Examples have no end of test.
func (PrintT) Cleanup(func()) {}

// Must checks a and prints its failure, like [mustbe.Must].
func (p PrintT) Must(a assertion.Assertion) { mustbe.Must(p, a) }

// Mustf checks a and prints its failure annotated with the message.
func (p PrintT) Mustf(a assertion.Assertion, msg string, args ...any) {
	mustbe.WrapT(p).Mustf(a, msg, args...)
}

// All returns an assertion that checks every assertion of as, in order, and
// reports every failure, not only the first: its error joins the errors of
// all failed assertions with [errors.Join], below a line naming desc, so
// errors.Is finds the sentinel of any of them. It passes when all pass, also
// when as is empty. A nil assertion fails.
//
// Use it for contracts made of independent parts, such as the security
// headers of a response, where every broken part is worth seeing at once.
func All(desc string, as ...assertion.Assertion) assertion.Assertion {
	return allAssertion{desc: desc, as: as}
}

type allAssertion struct {
	desc string
	as   []assertion.Assertion
}

var _ assertion.Assertion = allAssertion{}

func (a allAssertion) String() string {
	if a.desc != "" {
		return a.desc
	}
	parts := make([]string, len(a.as))
	for i, x := range a.as {
		if x == nil {
			parts[i] = "<nil>"
			continue
		}
		parts[i] = x.String()
	}
	return "all(" + strings.Join(parts, ", ") + ")"
}

func (a allAssertion) Check(ctx context.Context) error {
	var errs []error
	for i, x := range a.as {
		if x == nil {
			errs = append(errs, fmt.Errorf("%w: assertion %d is nil", ErrNilAssertion, i))
			continue
		}
		if err := x.Check(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	desc := a.String()
	if len(errs) == 1 {
		return fmt.Errorf("%s: %w", desc, errs[0])
	}
	return fmt.Errorf("%s: %d failures:\n%w", desc, len(errs), errors.Join(errs...))
}

// ErrNilAssertion reports a nil assertion passed to [All].
var ErrNilAssertion = errors.New("webuitest: nil assertion")

// ErrLeak reports a secret found where it must not be, such as in the
// logs ([Logs.NotLogged]).
var ErrLeak = errors.New("webuitest: secret leaked")
