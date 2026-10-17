package ba

import (
	"github.com/protolambda/chord/core/attr"
)

// Visible creates a Bootstrap visible class.
func Visible() attr.Node { return rawClass("visible") }

// Invisible creates a Bootstrap invisible class.
func Invisible() attr.Node { return rawClass("invisible") }
