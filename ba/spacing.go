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

// MAt creates a Bootstrap m-{bp}-{n} (margin) class that applies from
// breakpoint bp upwards, for n in 0..5. An invalid bp or n fails the render
// with [ErrInvalidValue].
func MAt(bp Breakpoint, n int) attr.Node { return atIntClass("m-", bp, n, minSpace, maxSpace) }

// MTAt creates a Bootstrap mt-{bp}-{n} (margin-top) class that applies from
// breakpoint bp upwards, for n in 0..5. An invalid bp or n fails the render
// with [ErrInvalidValue].
func MTAt(bp Breakpoint, n int) attr.Node { return atIntClass("mt-", bp, n, minSpace, maxSpace) }

// MBAt creates a Bootstrap mb-{bp}-{n} (margin-bottom) class that applies from
// breakpoint bp upwards, for n in 0..5. An invalid bp or n fails the render
// with [ErrInvalidValue]. It overrides a plain margin from bp up: MB(3) with
// MBAt(MD, 0) is a bottom margin on small screens only.
func MBAt(bp Breakpoint, n int) attr.Node { return atIntClass("mb-", bp, n, minSpace, maxSpace) }

// MSAt creates a Bootstrap ms-{bp}-{n} (margin-start) class that applies from
// breakpoint bp upwards, for n in 0..5. An invalid bp or n fails the render
// with [ErrInvalidValue].
func MSAt(bp Breakpoint, n int) attr.Node { return atIntClass("ms-", bp, n, minSpace, maxSpace) }

// MEAt creates a Bootstrap me-{bp}-{n} (margin-end) class that applies from
// breakpoint bp upwards, for n in 0..5. An invalid bp or n fails the render
// with [ErrInvalidValue].
func MEAt(bp Breakpoint, n int) attr.Node { return atIntClass("me-", bp, n, minSpace, maxSpace) }

// MXAt creates a Bootstrap mx-{bp}-{n} (horizontal margin) class that applies
// from breakpoint bp upwards, for n in 0..5. An invalid bp or n fails the
// render with [ErrInvalidValue].
func MXAt(bp Breakpoint, n int) attr.Node { return atIntClass("mx-", bp, n, minSpace, maxSpace) }

// MYAt creates a Bootstrap my-{bp}-{n} (vertical margin) class that applies
// from breakpoint bp upwards, for n in 0..5. An invalid bp or n fails the
// render with [ErrInvalidValue].
func MYAt(bp Breakpoint, n int) attr.Node { return atIntClass("my-", bp, n, minSpace, maxSpace) }

// PAt creates a Bootstrap p-{bp}-{n} (padding) class that applies from
// breakpoint bp upwards, for n in 0..5. An invalid bp or n fails the render
// with [ErrInvalidValue].
func PAt(bp Breakpoint, n int) attr.Node { return atIntClass("p-", bp, n, minSpace, maxSpace) }

// PTAt creates a Bootstrap pt-{bp}-{n} (padding-top) class that applies from
// breakpoint bp upwards, for n in 0..5. An invalid bp or n fails the render
// with [ErrInvalidValue].
func PTAt(bp Breakpoint, n int) attr.Node { return atIntClass("pt-", bp, n, minSpace, maxSpace) }

// PBAt creates a Bootstrap pb-{bp}-{n} (padding-bottom) class that applies
// from breakpoint bp upwards, for n in 0..5. An invalid bp or n fails the
// render with [ErrInvalidValue].
func PBAt(bp Breakpoint, n int) attr.Node { return atIntClass("pb-", bp, n, minSpace, maxSpace) }

// PSAt creates a Bootstrap ps-{bp}-{n} (padding-start) class that applies from
// breakpoint bp upwards, for n in 0..5. An invalid bp or n fails the render
// with [ErrInvalidValue].
func PSAt(bp Breakpoint, n int) attr.Node { return atIntClass("ps-", bp, n, minSpace, maxSpace) }

// PEAt creates a Bootstrap pe-{bp}-{n} (padding-end) class that applies from
// breakpoint bp upwards, for n in 0..5. An invalid bp or n fails the render
// with [ErrInvalidValue].
func PEAt(bp Breakpoint, n int) attr.Node { return atIntClass("pe-", bp, n, minSpace, maxSpace) }

// PXAt creates a Bootstrap px-{bp}-{n} (horizontal padding) class that applies
// from breakpoint bp upwards, for n in 0..5. An invalid bp or n fails the
// render with [ErrInvalidValue].
func PXAt(bp Breakpoint, n int) attr.Node { return atIntClass("px-", bp, n, minSpace, maxSpace) }

// PYAt creates a Bootstrap py-{bp}-{n} (vertical padding) class that applies
// from breakpoint bp upwards, for n in 0..5. An invalid bp or n fails the
// render with [ErrInvalidValue].
func PYAt(bp Breakpoint, n int) attr.Node { return atIntClass("py-", bp, n, minSpace, maxSpace) }

// MAutoAt creates a Bootstrap m-{bp}-auto class that applies from breakpoint
// bp upwards. An invalid bp fails the render with [ErrInvalidValue].
func MAutoAt(bp Breakpoint) attr.Node { return atClass("m-", bp, "-auto") }

// MTAutoAt creates a Bootstrap mt-{bp}-auto class that applies from breakpoint
// bp upwards. An invalid bp fails the render with [ErrInvalidValue].
func MTAutoAt(bp Breakpoint) attr.Node { return atClass("mt-", bp, "-auto") }

// MBAutoAt creates a Bootstrap mb-{bp}-auto class that applies from breakpoint
// bp upwards. An invalid bp fails the render with [ErrInvalidValue].
func MBAutoAt(bp Breakpoint) attr.Node { return atClass("mb-", bp, "-auto") }

// MSAutoAt creates a Bootstrap ms-{bp}-auto class that applies from breakpoint
// bp upwards. An invalid bp fails the render with [ErrInvalidValue].
func MSAutoAt(bp Breakpoint) attr.Node { return atClass("ms-", bp, "-auto") }

// MEAutoAt creates a Bootstrap me-{bp}-auto class that applies from breakpoint
// bp upwards. An invalid bp fails the render with [ErrInvalidValue].
func MEAutoAt(bp Breakpoint) attr.Node { return atClass("me-", bp, "-auto") }

// MXAutoAt creates a Bootstrap mx-{bp}-auto class that applies from breakpoint
// bp upwards. An invalid bp fails the render with [ErrInvalidValue].
func MXAutoAt(bp Breakpoint) attr.Node { return atClass("mx-", bp, "-auto") }

// MYAutoAt creates a Bootstrap my-{bp}-auto class that applies from breakpoint
// bp upwards. An invalid bp fails the render with [ErrInvalidValue].
func MYAutoAt(bp Breakpoint) attr.Node { return atClass("my-", bp, "-auto") }
