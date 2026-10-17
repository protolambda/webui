package ba

import (
	"github.com/protolambda/chord/core/attr"
)

// The spacing scale of Bootstrap: 0 (none) to 5 (3rem). Bootstrap's
// negative margins are disabled in its default build, so negative values are
// invalid too.
const (
	minSpace = 0
	maxSpace = 5
)

// M creates a Bootstrap m-{n} (margin) class, for n in 0..5. Any other n
// fails the render with [ErrInvalidValue].
func M(n int) attr.Node { return intClass("m-", n, minSpace, maxSpace) }

// MT creates a Bootstrap mt-{n} (margin-top) class, for n in 0..5. Any other
// n fails the render with [ErrInvalidValue].
func MT(n int) attr.Node { return intClass("mt-", n, minSpace, maxSpace) }

// MB creates a Bootstrap mb-{n} (margin-bottom) class, for n in 0..5. Any
// other n fails the render with [ErrInvalidValue].
func MB(n int) attr.Node { return intClass("mb-", n, minSpace, maxSpace) }

// MS creates a Bootstrap ms-{n} (margin-start: left in left-to-right text)
// class, for n in 0..5. Any other n fails the render with [ErrInvalidValue].
func MS(n int) attr.Node { return intClass("ms-", n, minSpace, maxSpace) }

// ME creates a Bootstrap me-{n} (margin-end: right in left-to-right text)
// class, for n in 0..5. Any other n fails the render with [ErrInvalidValue].
func ME(n int) attr.Node { return intClass("me-", n, minSpace, maxSpace) }

// MX creates a Bootstrap mx-{n} (horizontal margin) class, for n in 0..5. Any
// other n fails the render with [ErrInvalidValue].
func MX(n int) attr.Node { return intClass("mx-", n, minSpace, maxSpace) }

// MY creates a Bootstrap my-{n} (vertical margin) class, for n in 0..5. Any
// other n fails the render with [ErrInvalidValue].
func MY(n int) attr.Node { return intClass("my-", n, minSpace, maxSpace) }

// MAuto creates a Bootstrap m-auto class.
func MAuto() attr.Node { return rawClass("m-auto") }

// MTAuto creates a Bootstrap mt-auto class: in a flex column, it pushes the
// element to the bottom.
func MTAuto() attr.Node { return rawClass("mt-auto") }

// MBAuto creates a Bootstrap mb-auto class.
func MBAuto() attr.Node { return rawClass("mb-auto") }

// MSAuto creates a Bootstrap ms-auto class: in a flex row, it pushes the
// element (and those after it) to the end, e.g. navbar items on the right.
func MSAuto() attr.Node { return rawClass("ms-auto") }

// MEAuto creates a Bootstrap me-auto class.
func MEAuto() attr.Node { return rawClass("me-auto") }

// MXAuto creates a Bootstrap mx-auto class: it centers a block element that
// has a width.
func MXAuto() attr.Node { return rawClass("mx-auto") }

// MYAuto creates a Bootstrap my-auto class.
func MYAuto() attr.Node { return rawClass("my-auto") }

// P creates a Bootstrap p-{n} (padding) class, for n in 0..5. Any other n
// fails the render with [ErrInvalidValue].
func P(n int) attr.Node { return intClass("p-", n, minSpace, maxSpace) }

// PT creates a Bootstrap pt-{n} (padding-top) class, for n in 0..5. Any
// other n fails the render with [ErrInvalidValue].
func PT(n int) attr.Node { return intClass("pt-", n, minSpace, maxSpace) }

// PB creates a Bootstrap pb-{n} (padding-bottom) class, for n in 0..5. Any
// other n fails the render with [ErrInvalidValue].
func PB(n int) attr.Node { return intClass("pb-", n, minSpace, maxSpace) }

// PS creates a Bootstrap ps-{n} (padding-start) class, for n in 0..5. Any
// other n fails the render with [ErrInvalidValue].
func PS(n int) attr.Node { return intClass("ps-", n, minSpace, maxSpace) }

// PE creates a Bootstrap pe-{n} (padding-end) class, for n in 0..5. Any
// other n fails the render with [ErrInvalidValue].
func PE(n int) attr.Node { return intClass("pe-", n, minSpace, maxSpace) }

// PX creates a Bootstrap px-{n} (horizontal padding) class, for n in 0..5.
// Any other n fails the render with [ErrInvalidValue].
func PX(n int) attr.Node { return intClass("px-", n, minSpace, maxSpace) }

// PY creates a Bootstrap py-{n} (vertical padding) class, for n in 0..5. Any
// other n fails the render with [ErrInvalidValue].
func PY(n int) attr.Node { return intClass("py-", n, minSpace, maxSpace) }
