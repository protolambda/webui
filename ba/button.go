package ba

import (
	"github.com/protolambda/chord/core/attr"
)

// BtnSM creates a Bootstrap btn-sm class: a small button. Combine it with a
// button class, e.g. bs.BtnPrimary(ba.BtnSM()).
func BtnSM() attr.Node { return rawClass("btn-sm") }

// BtnLG creates a Bootstrap btn-lg class: a large button. Combine it with a
// button class, e.g. bs.BtnPrimary(ba.BtnLG()).
func BtnLG() attr.Node { return rawClass("btn-lg") }
