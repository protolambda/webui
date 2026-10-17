package ba

import (
	"github.com/protolambda/chord/core/attr"
)

// TextStart creates a Bootstrap text-start class.
func TextStart() attr.Node { return rawClass("text-start") }

// TextCenter creates a Bootstrap text-center class.
func TextCenter() attr.Node { return rawClass("text-center") }

// TextEnd creates a Bootstrap text-end class.
func TextEnd() attr.Node { return rawClass("text-end") }

// TextWrap creates a Bootstrap text-wrap class.
func TextWrap() attr.Node { return rawClass("text-wrap") }

// TextNowrap creates a Bootstrap text-nowrap class.
func TextNowrap() attr.Node { return rawClass("text-nowrap") }

// TextBreak creates a Bootstrap text-break class.
func TextBreak() attr.Node { return rawClass("text-break") }

// TextLowercase creates a Bootstrap text-lowercase class.
func TextLowercase() attr.Node { return rawClass("text-lowercase") }

// TextUppercase creates a Bootstrap text-uppercase class.
func TextUppercase() attr.Node { return rawClass("text-uppercase") }

// TextCapitalize creates a Bootstrap text-capitalize class.
func TextCapitalize() attr.Node { return rawClass("text-capitalize") }

// TextMuted creates a Bootstrap text-body-secondary class (formerly text-muted).
func TextMuted() attr.Node { return rawClass("text-body-secondary") }

// TextPrimary creates a Bootstrap text-primary class.
func TextPrimary() attr.Node { return rawClass("text-primary") }

// TextSecondary creates a Bootstrap text-secondary class.
func TextSecondary() attr.Node { return rawClass("text-secondary") }

// TextSuccess creates a Bootstrap text-success class.
func TextSuccess() attr.Node { return rawClass("text-success") }

// TextDanger creates a Bootstrap text-danger class.
func TextDanger() attr.Node { return rawClass("text-danger") }

// TextWarning creates a Bootstrap text-warning class.
func TextWarning() attr.Node { return rawClass("text-warning") }

// TextInfo creates a Bootstrap text-info class.
func TextInfo() attr.Node { return rawClass("text-info") }

// TextLight creates a Bootstrap text-light class.
func TextLight() attr.Node { return rawClass("text-light") }

// TextDark creates a Bootstrap text-dark class.
func TextDark() attr.Node { return rawClass("text-dark") }

// TextBody creates a Bootstrap text-body class.
func TextBody() attr.Node { return rawClass("text-body") }

// TextWhite creates a Bootstrap text-white class.
func TextWhite() attr.Node { return rawClass("text-white") }

// FW creates a Bootstrap fw-{weight} class.
func FW(weight string) attr.Node { return attr.Class("fw-" + weight) }

// FWBold creates a Bootstrap fw-bold class.
func FWBold() attr.Node { return rawClass("fw-bold") }

// FWNormal creates a Bootstrap fw-normal class.
func FWNormal() attr.Node { return rawClass("fw-normal") }

// FWLight creates a Bootstrap fw-light class.
func FWLight() attr.Node { return rawClass("fw-light") }

// FSItalic creates a Bootstrap fst-italic class.
func FSItalic() attr.Node { return rawClass("fst-italic") }

// FSNormal creates a Bootstrap fst-normal class.
func FSNormal() attr.Node { return rawClass("fst-normal") }

// LH creates a Bootstrap lh-{v} (line-height) class.
func LH(v string) attr.Node { return attr.Class("lh-" + v) }

// FontMonospace creates a Bootstrap font-monospace class.
func FontMonospace() attr.Node { return rawClass("font-monospace") }
