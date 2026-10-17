package ba

import (
	"github.com/protolambda/chord/core/attr"
)

// Shadow creates a Bootstrap shadow class.
func Shadow() attr.Node { return rawClass("shadow") }

// ShadowSM creates a Bootstrap shadow-sm class.
func ShadowSM() attr.Node { return rawClass("shadow-sm") }

// ShadowLG creates a Bootstrap shadow-lg class.
func ShadowLG() attr.Node { return rawClass("shadow-lg") }

// ShadowNone creates a Bootstrap shadow-none class.
func ShadowNone() attr.Node { return rawClass("shadow-none") }
