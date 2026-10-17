package ba

import (
	"github.com/protolambda/chord/core/attr"
)

// DNone creates a Bootstrap d-none class.
func DNone() attr.Node { return rawClass("d-none") }

// DInline creates a Bootstrap d-inline class.
func DInline() attr.Node { return rawClass("d-inline") }

// DInlineBlock creates a Bootstrap d-inline-block class.
func DInlineBlock() attr.Node { return rawClass("d-inline-block") }

// DBlock creates a Bootstrap d-block class.
func DBlock() attr.Node { return rawClass("d-block") }

// DGrid creates a Bootstrap d-grid class.
func DGrid() attr.Node { return rawClass("d-grid") }

// DInlineGrid creates a Bootstrap d-inline-grid class.
func DInlineGrid() attr.Node { return rawClass("d-inline-grid") }

// DFlex creates a Bootstrap d-flex class.
func DFlex() attr.Node { return rawClass("d-flex") }

// DInlineFlex creates a Bootstrap d-inline-flex class.
func DInlineFlex() attr.Node { return rawClass("d-inline-flex") }

// DTable creates a Bootstrap d-table class.
func DTable() attr.Node { return rawClass("d-table") }

// DTableRow creates a Bootstrap d-table-row class.
func DTableRow() attr.Node { return rawClass("d-table-row") }

// DTableCell creates a Bootstrap d-table-cell class.
func DTableCell() attr.Node { return rawClass("d-table-cell") }

// DNoneAt creates a Bootstrap d-{bp}-none class: hidden from breakpoint bp
// upwards. Combine a plain display utility with an At variant to show an
// element only on some screens: DNone() with DBlockAt(MD) shows it from md
// upwards. An invalid bp fails the render with [ErrInvalidValue].
func DNoneAt(bp Breakpoint) attr.Node { return atClass("d-", bp, "-none") }

// DInlineAt creates a Bootstrap d-{bp}-inline class. An invalid bp fails the
// render with [ErrInvalidValue].
func DInlineAt(bp Breakpoint) attr.Node { return atClass("d-", bp, "-inline") }

// DInlineBlockAt creates a Bootstrap d-{bp}-inline-block class. An invalid
// bp fails the render with [ErrInvalidValue].
func DInlineBlockAt(bp Breakpoint) attr.Node { return atClass("d-", bp, "-inline-block") }

// DBlockAt creates a Bootstrap d-{bp}-block class. An invalid bp fails the
// render with [ErrInvalidValue].
func DBlockAt(bp Breakpoint) attr.Node { return atClass("d-", bp, "-block") }

// DGridAt creates a Bootstrap d-{bp}-grid class. An invalid bp fails the
// render with [ErrInvalidValue].
func DGridAt(bp Breakpoint) attr.Node { return atClass("d-", bp, "-grid") }

// DInlineGridAt creates a Bootstrap d-{bp}-inline-grid class. An invalid bp
// fails the render with [ErrInvalidValue].
func DInlineGridAt(bp Breakpoint) attr.Node { return atClass("d-", bp, "-inline-grid") }

// DFlexAt creates a Bootstrap d-{bp}-flex class. An invalid bp fails the
// render with [ErrInvalidValue].
func DFlexAt(bp Breakpoint) attr.Node { return atClass("d-", bp, "-flex") }

// DInlineFlexAt creates a Bootstrap d-{bp}-inline-flex class. An invalid bp
// fails the render with [ErrInvalidValue].
func DInlineFlexAt(bp Breakpoint) attr.Node { return atClass("d-", bp, "-inline-flex") }

// DTableAt creates a Bootstrap d-{bp}-table class. An invalid bp fails the
// render with [ErrInvalidValue].
func DTableAt(bp Breakpoint) attr.Node { return atClass("d-", bp, "-table") }

// DTableRowAt creates a Bootstrap d-{bp}-table-row class. An invalid bp
// fails the render with [ErrInvalidValue].
func DTableRowAt(bp Breakpoint) attr.Node { return atClass("d-", bp, "-table-row") }

// DTableCellAt creates a Bootstrap d-{bp}-table-cell class. An invalid bp
// fails the render with [ErrInvalidValue].
func DTableCellAt(bp Breakpoint) attr.Node { return atClass("d-", bp, "-table-cell") }

// DPrintNone creates a Bootstrap d-print-none class: hidden when printed,
// e.g. for navigation and buttons.
func DPrintNone() attr.Node { return rawClass("d-print-none") }

// DPrintBlock creates a Bootstrap d-print-block class: combined with
// [DNone], the element shows only when printed.
func DPrintBlock() attr.Node { return rawClass("d-print-block") }
