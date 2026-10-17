package ba

import (
	"github.com/protolambda/chord/core/attr"
)

// PositionStatic creates a Bootstrap position-static class.
func PositionStatic() attr.Node { return rawClass("position-static") }

// PositionRelative creates a Bootstrap position-relative class.
func PositionRelative() attr.Node { return rawClass("position-relative") }

// PositionAbsolute creates a Bootstrap position-absolute class.
func PositionAbsolute() attr.Node { return rawClass("position-absolute") }

// PositionFixed creates a Bootstrap position-fixed class.
func PositionFixed() attr.Node { return rawClass("position-fixed") }

// PositionSticky creates a Bootstrap position-sticky class.
func PositionSticky() attr.Node { return rawClass("position-sticky") }

// Top0 creates a Bootstrap top-0 class.
func Top0() attr.Node { return rawClass("top-0") }

// Top50 creates a Bootstrap top-50 class.
func Top50() attr.Node { return rawClass("top-50") }

// Top100 creates a Bootstrap top-100 class.
func Top100() attr.Node { return rawClass("top-100") }

// Bottom0 creates a Bootstrap bottom-0 class.
func Bottom0() attr.Node { return rawClass("bottom-0") }

// Bottom50 creates a Bootstrap bottom-50 class.
func Bottom50() attr.Node { return rawClass("bottom-50") }

// Bottom100 creates a Bootstrap bottom-100 class.
func Bottom100() attr.Node { return rawClass("bottom-100") }

// Start0 creates a Bootstrap start-0 class.
func Start0() attr.Node { return rawClass("start-0") }

// Start50 creates a Bootstrap start-50 class.
func Start50() attr.Node { return rawClass("start-50") }

// Start100 creates a Bootstrap start-100 class.
func Start100() attr.Node { return rawClass("start-100") }

// End0 creates a Bootstrap end-0 class.
func End0() attr.Node { return rawClass("end-0") }

// End50 creates a Bootstrap end-50 class.
func End50() attr.Node { return rawClass("end-50") }

// End100 creates a Bootstrap end-100 class.
func End100() attr.Node { return rawClass("end-100") }

// TranslateMiddle creates a Bootstrap translate-middle class.
func TranslateMiddle() attr.Node { return rawClass("translate-middle") }

// TranslateMiddleX creates a Bootstrap translate-middle-x class.
func TranslateMiddleX() attr.Node { return rawClass("translate-middle-x") }

// TranslateMiddleY creates a Bootstrap translate-middle-y class.
func TranslateMiddleY() attr.Node { return rawClass("translate-middle-y") }
