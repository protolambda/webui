package ba

import (
	"github.com/protolambda/chord/core/attr"
)

// Border creates a Bootstrap border class.
func Border() attr.Node { return rawClass("border") }

// BorderTop creates a Bootstrap border-top class.
func BorderTop() attr.Node { return rawClass("border-top") }

// BorderEnd creates a Bootstrap border-end class.
func BorderEnd() attr.Node { return rawClass("border-end") }

// BorderBottom creates a Bootstrap border-bottom class.
func BorderBottom() attr.Node { return rawClass("border-bottom") }

// BorderStart creates a Bootstrap border-start class.
func BorderStart() attr.Node { return rawClass("border-start") }

// Border0 creates a Bootstrap border-0 class.
func Border0() attr.Node { return rawClass("border-0") }

// BorderWidth creates a Bootstrap border-{n} class, the border width, for n
// in 0..5 (border-0 removes the border). Combine it with [Border]. Any other
// n fails the render with [ErrInvalidValue].
func BorderWidth(n int) attr.Node { return intClass("border-", n, 0, 5) }

// BorderPrimary creates a Bootstrap border-primary class.
func BorderPrimary() attr.Node { return rawClass("border-primary") }

// BorderSecondary creates a Bootstrap border-secondary class.
func BorderSecondary() attr.Node { return rawClass("border-secondary") }

// BorderSuccess creates a Bootstrap border-success class.
func BorderSuccess() attr.Node { return rawClass("border-success") }

// BorderDanger creates a Bootstrap border-danger class.
func BorderDanger() attr.Node { return rawClass("border-danger") }

// BorderWarning creates a Bootstrap border-warning class.
func BorderWarning() attr.Node { return rawClass("border-warning") }

// BorderInfo creates a Bootstrap border-info class.
func BorderInfo() attr.Node { return rawClass("border-info") }

// BorderLight creates a Bootstrap border-light class.
func BorderLight() attr.Node { return rawClass("border-light") }

// BorderDark creates a Bootstrap border-dark class.
func BorderDark() attr.Node { return rawClass("border-dark") }

// BorderBlack creates a Bootstrap border-black class.
func BorderBlack() attr.Node { return rawClass("border-black") }

// BorderWhite creates a Bootstrap border-white class.
func BorderWhite() attr.Node { return rawClass("border-white") }

// BorderPrimarySubtle creates a Bootstrap border-primary-subtle class.
func BorderPrimarySubtle() attr.Node { return rawClass("border-primary-subtle") }

// BorderSecondarySubtle creates a Bootstrap border-secondary-subtle class.
func BorderSecondarySubtle() attr.Node { return rawClass("border-secondary-subtle") }

// BorderSuccessSubtle creates a Bootstrap border-success-subtle class.
func BorderSuccessSubtle() attr.Node { return rawClass("border-success-subtle") }

// BorderDangerSubtle creates a Bootstrap border-danger-subtle class.
func BorderDangerSubtle() attr.Node { return rawClass("border-danger-subtle") }

// BorderWarningSubtle creates a Bootstrap border-warning-subtle class.
func BorderWarningSubtle() attr.Node { return rawClass("border-warning-subtle") }

// BorderInfoSubtle creates a Bootstrap border-info-subtle class.
func BorderInfoSubtle() attr.Node { return rawClass("border-info-subtle") }

// BorderLightSubtle creates a Bootstrap border-light-subtle class.
func BorderLightSubtle() attr.Node { return rawClass("border-light-subtle") }

// BorderDarkSubtle creates a Bootstrap border-dark-subtle class.
func BorderDarkSubtle() attr.Node { return rawClass("border-dark-subtle") }

// Rounded creates a Bootstrap rounded class.
func Rounded() attr.Node { return rawClass("rounded") }

// RoundedSize creates a Bootstrap rounded-{n} class, the corner radius from
// none (0) to the largest (5). Any other n fails the render with
// [ErrInvalidValue].
func RoundedSize(n int) attr.Node { return intClass("rounded-", n, 0, 5) }

// RoundedTop creates a Bootstrap rounded-top class.
func RoundedTop() attr.Node { return rawClass("rounded-top") }

// RoundedEnd creates a Bootstrap rounded-end class.
func RoundedEnd() attr.Node { return rawClass("rounded-end") }

// RoundedBottom creates a Bootstrap rounded-bottom class.
func RoundedBottom() attr.Node { return rawClass("rounded-bottom") }

// RoundedStart creates a Bootstrap rounded-start class.
func RoundedStart() attr.Node { return rawClass("rounded-start") }

// RoundedCircle creates a Bootstrap rounded-circle class.
func RoundedCircle() attr.Node { return rawClass("rounded-circle") }

// RoundedPill creates a Bootstrap rounded-pill class.
func RoundedPill() attr.Node { return rawClass("rounded-pill") }

// Rounded0 creates a Bootstrap rounded-0 class.
func Rounded0() attr.Node { return rawClass("rounded-0") }
