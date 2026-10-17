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

// BgBlack creates a Bootstrap bg-black class.
func BgBlack() attr.Node { return rawClass("bg-black") }

// BgWhite creates a Bootstrap bg-white class.
func BgWhite() attr.Node { return rawClass("bg-white") }

// BgBody creates a Bootstrap bg-body class: the page background of the
// active color mode.
func BgBody() attr.Node { return rawClass("bg-body") }

// BgBodySecondary creates a Bootstrap bg-body-secondary class: a background
// one step away from the page background, in the active color mode.
func BgBodySecondary() attr.Node { return rawClass("bg-body-secondary") }

// BgBodyTertiary creates a Bootstrap bg-body-tertiary class: a background
// between the page background and bg-body-secondary.
func BgBodyTertiary() attr.Node { return rawClass("bg-body-tertiary") }

// BgTransparent creates a Bootstrap bg-transparent class.
func BgTransparent() attr.Node { return rawClass("bg-transparent") }

// BgPrimarySubtle creates a Bootstrap bg-primary-subtle class, a muted
// background that adapts to the color mode. Pair it with
// [TextPrimaryEmphasis] and [BorderPrimarySubtle].
func BgPrimarySubtle() attr.Node { return rawClass("bg-primary-subtle") }

// BgSecondarySubtle creates a Bootstrap bg-secondary-subtle class.
func BgSecondarySubtle() attr.Node { return rawClass("bg-secondary-subtle") }

// BgSuccessSubtle creates a Bootstrap bg-success-subtle class.
func BgSuccessSubtle() attr.Node { return rawClass("bg-success-subtle") }

// BgDangerSubtle creates a Bootstrap bg-danger-subtle class.
func BgDangerSubtle() attr.Node { return rawClass("bg-danger-subtle") }

// BgWarningSubtle creates a Bootstrap bg-warning-subtle class.
func BgWarningSubtle() attr.Node { return rawClass("bg-warning-subtle") }

// BgInfoSubtle creates a Bootstrap bg-info-subtle class.
func BgInfoSubtle() attr.Node { return rawClass("bg-info-subtle") }

// BgLightSubtle creates a Bootstrap bg-light-subtle class.
func BgLightSubtle() attr.Node { return rawClass("bg-light-subtle") }

// BgDarkSubtle creates a Bootstrap bg-dark-subtle class.
func BgDarkSubtle() attr.Node { return rawClass("bg-dark-subtle") }

// TextBgPrimary creates a Bootstrap text-bg-primary class: the primary
// background with a text color that contrasts with it.
func TextBgPrimary() attr.Node { return rawClass("text-bg-primary") }

// TextBgSecondary creates a Bootstrap text-bg-secondary class.
func TextBgSecondary() attr.Node { return rawClass("text-bg-secondary") }

// TextBgSuccess creates a Bootstrap text-bg-success class.
func TextBgSuccess() attr.Node { return rawClass("text-bg-success") }

// TextBgDanger creates a Bootstrap text-bg-danger class.
func TextBgDanger() attr.Node { return rawClass("text-bg-danger") }

// TextBgWarning creates a Bootstrap text-bg-warning class.
func TextBgWarning() attr.Node { return rawClass("text-bg-warning") }

// TextBgInfo creates a Bootstrap text-bg-info class.
func TextBgInfo() attr.Node { return rawClass("text-bg-info") }

// TextBgLight creates a Bootstrap text-bg-light class.
func TextBgLight() attr.Node { return rawClass("text-bg-light") }

// TextBgDark creates a Bootstrap text-bg-dark class.
func TextBgDark() attr.Node { return rawClass("text-bg-dark") }
