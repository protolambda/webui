package ba

import (
	"fmt"
	"strconv"

	"github.com/protolambda/chord/core/attr"
)

// Breakpoint is a Bootstrap responsive breakpoint. A responsive utility
// such as [DNoneAt] applies from its breakpoint upwards: DNoneAt(MD) is
// d-md-none, which hides the element on screens 768px wide and wider.
//
// The zero value is not a breakpoint: the smallest screens (xs) have no
// infix in Bootstrap, so use the plain utility (e.g. [DNone]) for them. A
// helper given the zero value or another value outside SM..XXL returns an
// attribute that fails the render with an error wrapping [ErrInvalidValue].
type Breakpoint uint8

// The Bootstrap 5.3 breakpoints, by minimum viewport width.
const (
	SM  Breakpoint = iota + 1 // sm: 576px and wider
	MD                        // md: 768px and wider
	LG                        // lg: 992px and wider
	XL                        // xl: 1200px and wider
	XXL                       // xxl: 1400px and wider
)

// String returns the class infix of the breakpoint ("sm" to "xxl"), or
// "Breakpoint(N)" for a value that is not a breakpoint.
func (b Breakpoint) String() string {
	if s, ok := b.infix(); ok {
		return s
	}
	return "Breakpoint(" + strconv.Itoa(int(b)) + ")"
}

// infix returns the class infix of a valid breakpoint.
func (b Breakpoint) infix() (string, bool) {
	switch b {
	case SM:
		return "sm", true
	case MD:
		return "md", true
	case LG:
		return "lg", true
	case XL:
		return "xl", true
	case XXL:
		return "xxl", true
	default:
		return "", false
	}
}

// invalidBreakpoint is the failing attribute of a responsive helper for the
// class pattern, e.g. "d-{bp}-none".
func invalidBreakpoint(pattern string, b Breakpoint) attr.Node {
	return invalid{fmt.Errorf("ba: %s: %w: %s (want SM to XXL)", pattern, ErrInvalidValue, b)}
}

// atClass returns the class prefix+infix+suffix, e.g. "d-md-none" for
// ("d-", MD, "-none").
func atClass(prefix string, b Breakpoint, suffix string) attr.Node {
	infix, ok := b.infix()
	if !ok {
		return invalidBreakpoint(prefix+"{bp}"+suffix, b)
	}
	return rawClass(prefix + infix + suffix)
}

// atIntClass returns the class prefix+infix+"-"+n, e.g. "col-md-4", when
// lo <= n <= hi.
func atIntClass(prefix string, b Breakpoint, n, lo, hi int) attr.Node {
	infix, ok := b.infix()
	if !ok {
		return invalidBreakpoint(prefix+"{bp}-"+strconv.Itoa(n), b)
	}
	return intClass(prefix+infix+"-", n, lo, hi)
}
