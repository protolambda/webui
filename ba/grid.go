package ba

import (
	"github.com/protolambda/chord/core/attr"
)

// The column and offset ranges of the 12-column Bootstrap grid.
const (
	minCols   = 1
	maxCols   = 12
	maxOffset = 11
)

// Container returns a "container" class attribute.
func Container() attr.Node { return rawClass("container") }

// ContainerFluid returns a "container-fluid" class attribute.
func ContainerFluid() attr.Node { return rawClass("container-fluid") }

// ContainerSM returns a "container-sm" class attribute.
func ContainerSM() attr.Node { return rawClass("container-sm") }

// ContainerMD returns a "container-md" class attribute.
func ContainerMD() attr.Node { return rawClass("container-md") }

// ContainerLG returns a "container-lg" class attribute.
func ContainerLG() attr.Node { return rawClass("container-lg") }

// ContainerXL returns a "container-xl" class attribute.
func ContainerXL() attr.Node { return rawClass("container-xl") }

// ContainerXXL returns a "container-xxl" class attribute.
func ContainerXXL() attr.Node { return rawClass("container-xxl") }

// Row returns a "row" class attribute.
func Row() attr.Node { return rawClass("row") }

// Col returns a "col" class attribute.
func Col() attr.Node { return rawClass("col") }

// Col1 returns a "col-1" class attribute.
func Col1() attr.Node { return rawClass("col-1") }

// Col2 returns a "col-2" class attribute.
func Col2() attr.Node { return rawClass("col-2") }

// Col3 returns a "col-3" class attribute.
func Col3() attr.Node { return rawClass("col-3") }

// Col4 returns a "col-4" class attribute.
func Col4() attr.Node { return rawClass("col-4") }

// Col5 returns a "col-5" class attribute.
func Col5() attr.Node { return rawClass("col-5") }

// Col6 returns a "col-6" class attribute.
func Col6() attr.Node { return rawClass("col-6") }

// Col7 returns a "col-7" class attribute.
func Col7() attr.Node { return rawClass("col-7") }

// Col8 returns a "col-8" class attribute.
func Col8() attr.Node { return rawClass("col-8") }

// Col9 returns a "col-9" class attribute.
func Col9() attr.Node { return rawClass("col-9") }

// Col10 returns a "col-10" class attribute.
func Col10() attr.Node { return rawClass("col-10") }

// Col11 returns a "col-11" class attribute.
func Col11() attr.Node { return rawClass("col-11") }

// Col12 returns a "col-12" class attribute.
func Col12() attr.Node { return rawClass("col-12") }

// ColAt returns a "col-{bp}-{n}" class attribute: n of 12 columns from
// breakpoint bp upwards, for n in 1..12. An invalid bp or n fails the
// render with [ErrInvalidValue].
func ColAt(bp Breakpoint, n int) attr.Node { return atIntClass("col-", bp, n, minCols, maxCols) }

// ColSM returns a "col-sm-{n}" class attribute, for n in 1..12. Any other n
// fails the render with [ErrInvalidValue].
func ColSM(n int) attr.Node { return ColAt(SM, n) }

// ColMD returns a "col-md-{n}" class attribute, for n in 1..12. Any other n
// fails the render with [ErrInvalidValue].
func ColMD(n int) attr.Node { return ColAt(MD, n) }

// ColLG returns a "col-lg-{n}" class attribute, for n in 1..12. Any other n
// fails the render with [ErrInvalidValue].
func ColLG(n int) attr.Node { return ColAt(LG, n) }

// ColXL returns a "col-xl-{n}" class attribute, for n in 1..12. Any other n
// fails the render with [ErrInvalidValue].
func ColXL(n int) attr.Node { return ColAt(XL, n) }

// ColXXL returns a "col-xxl-{n}" class attribute, for n in 1..12. Any other n
// fails the render with [ErrInvalidValue].
func ColXXL(n int) attr.Node { return ColAt(XXL, n) }

// ColAuto returns a "col-auto" class attribute: a column as wide as its
// content.
func ColAuto() attr.Node { return rawClass("col-auto") }

// ColAutoAt returns a "col-{bp}-auto" class attribute. An invalid bp fails
// the render with [ErrInvalidValue].
func ColAutoAt(bp Breakpoint) attr.Node { return atClass("col-", bp, "-auto") }

// Offset returns an "offset-{n}" class attribute, for n in 1..11. Bootstrap
// has no offset-0 for the smallest screens (no offset is the default), so 0
// fails the render with [ErrInvalidValue], as does any other n out of range.
func Offset(n int) attr.Node { return intClass("offset-", n, 1, maxOffset) }

// OffsetAt returns an "offset-{bp}-{n}" class attribute, for n in 0..11;
// offset-{bp}-0 resets an offset set for a smaller breakpoint. An invalid bp
// or n fails the render with [ErrInvalidValue].
func OffsetAt(bp Breakpoint, n int) attr.Node { return atIntClass("offset-", bp, n, 0, maxOffset) }

// OffsetSM returns an "offset-sm-{n}" class attribute, for n in 0..11. Any
// other n fails the render with [ErrInvalidValue].
func OffsetSM(n int) attr.Node { return OffsetAt(SM, n) }

// OffsetMD returns an "offset-md-{n}" class attribute, for n in 0..11. Any
// other n fails the render with [ErrInvalidValue].
func OffsetMD(n int) attr.Node { return OffsetAt(MD, n) }

// OffsetLG returns an "offset-lg-{n}" class attribute, for n in 0..11. Any
// other n fails the render with [ErrInvalidValue].
func OffsetLG(n int) attr.Node { return OffsetAt(LG, n) }

// OffsetXL returns an "offset-xl-{n}" class attribute, for n in 0..11. Any
// other n fails the render with [ErrInvalidValue].
func OffsetXL(n int) attr.Node { return OffsetAt(XL, n) }

// OffsetXXL returns an "offset-xxl-{n}" class attribute, for n in 0..11. Any
// other n fails the render with [ErrInvalidValue].
func OffsetXXL(n int) attr.Node { return OffsetAt(XXL, n) }
