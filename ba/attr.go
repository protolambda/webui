package ba

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/protolambda/chord/core/attr"
)

// ErrInvalidValue indicates a utility argument that has no Bootstrap class,
// such as ba.M(9), ba.ColMD(13), an unknown [Breakpoint], or an unknown
// font weight. The helper returns an attribute that fails the render with an
// error wrapping ErrInvalidValue, so that a bad value is never rendered as a
// class that does not exist.
var ErrInvalidValue = errors.New("invalid utility value")

// rawClass returns a class attribute that is written verbatim.
//
// Invariant: value is a compile-time literal of this package, or one built
// only from such literals, validated digits (see intClass) and validated
// breakpoint infixes (see Breakpoint.infix). It never contains caller text,
// so it can never break out of the quoted attribute. Caller-provided strings
// are validated against a fixed set before they reach rawClass (see FW).
func rawClass(value string) attr.Node {
	return attr.Name("class").Raw(value)
}

// invalid is an attribute that fails evaluation: the lazily failing result
// of a helper that received a value without a Bootstrap class.
type invalid struct{ err error }

var _ attr.Node = invalid{}

func (v invalid) Eval(context.Context) (attr.Obj, error) {
	return attr.Obj{}, v.err
}

// intClass returns the class prefix+n, e.g. "mt-3", when lo <= n <= hi, and
// otherwise an attribute that fails the render with ErrInvalidValue. The
// error shows the class that would have been produced; n is an integer, so
// it cannot carry markup or secrets.
func intClass(prefix string, n, lo, hi int) attr.Node {
	if n < lo || n > hi {
		return invalid{fmt.Errorf("ba: %s%d: %w (want %d to %d)", prefix, n, ErrInvalidValue, lo, hi)}
	}
	return rawClass(prefix + strconv.Itoa(n))
}

// oneOf returns the class prefix+v when v is one of allowed, and otherwise
// an attribute that fails the render with ErrInvalidValue. The error does
// not repeat v, which may come from anywhere.
func oneOf(prefix, v string, allowed ...string) attr.Node {
	for _, a := range allowed {
		if v == a {
			return rawClass(prefix + a)
		}
	}
	return invalid{fmt.Errorf("ba: %s*: %w (want one of %q)", prefix, ErrInvalidValue, allowed)}
}
