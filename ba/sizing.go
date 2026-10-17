package ba

import (
	"github.com/protolambda/chord/core/attr"
)

// W25 creates a Bootstrap w-25 class.
func W25() attr.Node { return rawClass("w-25") }

// W50 creates a Bootstrap w-50 class.
func W50() attr.Node { return rawClass("w-50") }

// W75 creates a Bootstrap w-75 class.
func W75() attr.Node { return rawClass("w-75") }

// W100 creates a Bootstrap w-100 class.
func W100() attr.Node { return rawClass("w-100") }

// WAuto creates a Bootstrap w-auto class.
func WAuto() attr.Node { return rawClass("w-auto") }

// H25 creates a Bootstrap h-25 class.
func H25() attr.Node { return rawClass("h-25") }

// H50 creates a Bootstrap h-50 class.
func H50() attr.Node { return rawClass("h-50") }

// H75 creates a Bootstrap h-75 class.
func H75() attr.Node { return rawClass("h-75") }

// H100 creates a Bootstrap h-100 class.
func H100() attr.Node { return rawClass("h-100") }

// HAuto creates a Bootstrap h-auto class.
func HAuto() attr.Node { return rawClass("h-auto") }

// MW100 creates a Bootstrap mw-100 class.
func MW100() attr.Node { return rawClass("mw-100") }

// MH100 creates a Bootstrap mh-100 class.
func MH100() attr.Node { return rawClass("mh-100") }

// VWMin100 creates a Bootstrap min-vw-100 class.
func VWMin100() attr.Node { return rawClass("min-vw-100") }

// VHMin100 creates a Bootstrap min-vh-100 class.
func VHMin100() attr.Node { return rawClass("min-vh-100") }
