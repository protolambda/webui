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
