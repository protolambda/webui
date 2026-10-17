package ba

import (
	"github.com/protolambda/chord/core/attr"
)

// BgPrimary creates a Bootstrap bg-primary class.
func BgPrimary() attr.Node { return rawClass("bg-primary") }

// BgSecondary creates a Bootstrap bg-secondary class.
func BgSecondary() attr.Node { return rawClass("bg-secondary") }

// BgSuccess creates a Bootstrap bg-success class.
func BgSuccess() attr.Node { return rawClass("bg-success") }

// BgDanger creates a Bootstrap bg-danger class.
func BgDanger() attr.Node { return rawClass("bg-danger") }

// BgWarning creates a Bootstrap bg-warning class.
func BgWarning() attr.Node { return rawClass("bg-warning") }

// BgInfo creates a Bootstrap bg-info class.
func BgInfo() attr.Node { return rawClass("bg-info") }

// BgLight creates a Bootstrap bg-light class.
func BgLight() attr.Node { return rawClass("bg-light") }

// BgDark creates a Bootstrap bg-dark class.
func BgDark() attr.Node { return rawClass("bg-dark") }

// BgBody creates a Bootstrap bg-body class.
func BgBody() attr.Node { return rawClass("bg-body") }

// BgWhite creates a Bootstrap bg-white class.
func BgWhite() attr.Node { return rawClass("bg-white") }

// BgTransparent creates a Bootstrap bg-transparent class.
func BgTransparent() attr.Node { return rawClass("bg-transparent") }
