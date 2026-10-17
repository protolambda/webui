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

// TextStartAt creates a Bootstrap text-{bp}-start class that applies from
// breakpoint bp upwards. An invalid bp fails the render with
// [ErrInvalidValue].
func TextStartAt(bp Breakpoint) attr.Node { return atClass("text-", bp, "-start") }

// TextCenterAt creates a Bootstrap text-{bp}-center class that applies from
// breakpoint bp upwards. An invalid bp fails the render with
// [ErrInvalidValue].
func TextCenterAt(bp Breakpoint) attr.Node { return atClass("text-", bp, "-center") }

// TextEndAt creates a Bootstrap text-{bp}-end class that applies from
// breakpoint bp upwards. For example, TextStart() with TextEndAt(MD) aligns a
// footer cell to the end from md up. An invalid bp fails the render with
// [ErrInvalidValue].
func TextEndAt(bp Breakpoint) attr.Node { return atClass("text-", bp, "-end") }

// TextWrap creates a Bootstrap text-wrap class.
func TextWrap() attr.Node { return rawClass("text-wrap") }

// TextNowrap creates a Bootstrap text-nowrap class.
func TextNowrap() attr.Node { return rawClass("text-nowrap") }

// TextBreak creates a Bootstrap text-break class: long words (such as
// identifiers or URLs) break instead of overflowing.
func TextBreak() attr.Node { return rawClass("text-break") }

// TextTruncate creates a Bootstrap text-truncate class: the text stays on one
// line and ends with an ellipsis when it overflows. The element must be
// block or inline-block. Keep the full text reachable, e.g. in a title
// attribute or on a detail page.
func TextTruncate() attr.Node { return rawClass("text-truncate") }

// TextLowercase creates a Bootstrap text-lowercase class.
func TextLowercase() attr.Node { return rawClass("text-lowercase") }

// TextUppercase creates a Bootstrap text-uppercase class.
func TextUppercase() attr.Node { return rawClass("text-uppercase") }

// TextCapitalize creates a Bootstrap text-capitalize class.
func TextCapitalize() attr.Node { return rawClass("text-capitalize") }

// TextDecorationNone creates a Bootstrap text-decoration-none class, e.g. for
// a link without underline.
func TextDecorationNone() attr.Node { return rawClass("text-decoration-none") }

// TextDecorationUnderline creates a Bootstrap text-decoration-underline class.
func TextDecorationUnderline() attr.Node { return rawClass("text-decoration-underline") }

// TextDecorationLineThrough creates a Bootstrap text-decoration-line-through
// class.
func TextDecorationLineThrough() attr.Node { return rawClass("text-decoration-line-through") }

// TextMuted creates a Bootstrap text-body-secondary class. Bootstrap 5.3
// renamed text-muted to text-body-secondary; TextMuted is the same as
// [TextBodySecondary].
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

// TextBlack creates a Bootstrap text-black class.
func TextBlack() attr.Node { return rawClass("text-black") }

// TextWhite creates a Bootstrap text-white class.
func TextWhite() attr.Node { return rawClass("text-white") }

// TextBody creates a Bootstrap text-body class: the body text color of the
// active color mode.
func TextBody() attr.Node { return rawClass("text-body") }

// TextBodySecondary creates a Bootstrap text-body-secondary class: muted
// body text.
func TextBodySecondary() attr.Node { return rawClass("text-body-secondary") }

// TextBodyTertiary creates a Bootstrap text-body-tertiary class: body text
// muted more than text-body-secondary.
func TextBodyTertiary() attr.Node { return rawClass("text-body-tertiary") }

// TextBodyEmphasis creates a Bootstrap text-body-emphasis class: body text
// with more contrast.
func TextBodyEmphasis() attr.Node { return rawClass("text-body-emphasis") }

// TextReset creates a Bootstrap text-reset class: the color is inherited
// from the parent, e.g. for a link inside muted text.
func TextReset() attr.Node { return rawClass("text-reset") }

// TextPrimaryEmphasis creates a Bootstrap text-primary-emphasis class, a
// text color that reads well on bg-primary-subtle in either color mode.
func TextPrimaryEmphasis() attr.Node { return rawClass("text-primary-emphasis") }

// TextSecondaryEmphasis creates a Bootstrap text-secondary-emphasis class.
func TextSecondaryEmphasis() attr.Node { return rawClass("text-secondary-emphasis") }

// TextSuccessEmphasis creates a Bootstrap text-success-emphasis class.
func TextSuccessEmphasis() attr.Node { return rawClass("text-success-emphasis") }

// TextDangerEmphasis creates a Bootstrap text-danger-emphasis class.
func TextDangerEmphasis() attr.Node { return rawClass("text-danger-emphasis") }

// TextWarningEmphasis creates a Bootstrap text-warning-emphasis class.
func TextWarningEmphasis() attr.Node { return rawClass("text-warning-emphasis") }

// TextInfoEmphasis creates a Bootstrap text-info-emphasis class.
func TextInfoEmphasis() attr.Node { return rawClass("text-info-emphasis") }

// TextLightEmphasis creates a Bootstrap text-light-emphasis class.
func TextLightEmphasis() attr.Node { return rawClass("text-light-emphasis") }

// TextDarkEmphasis creates a Bootstrap text-dark-emphasis class.
func TextDarkEmphasis() attr.Node { return rawClass("text-dark-emphasis") }

// LinkPrimary creates a Bootstrap link-primary class, a link color with
// matching hover and focus states.
func LinkPrimary() attr.Node { return rawClass("link-primary") }

// LinkSecondary creates a Bootstrap link-secondary class.
func LinkSecondary() attr.Node { return rawClass("link-secondary") }

// LinkSuccess creates a Bootstrap link-success class.
func LinkSuccess() attr.Node { return rawClass("link-success") }

// LinkDanger creates a Bootstrap link-danger class.
func LinkDanger() attr.Node { return rawClass("link-danger") }

// LinkWarning creates a Bootstrap link-warning class.
func LinkWarning() attr.Node { return rawClass("link-warning") }

// LinkInfo creates a Bootstrap link-info class.
func LinkInfo() attr.Node { return rawClass("link-info") }

// LinkLight creates a Bootstrap link-light class.
func LinkLight() attr.Node { return rawClass("link-light") }

// LinkDark creates a Bootstrap link-dark class.
func LinkDark() attr.Node { return rawClass("link-dark") }

// LinkBodyEmphasis creates a Bootstrap link-body-emphasis class: a link in
// the high-contrast body color, e.g. for titles that link to a detail page.
func LinkBodyEmphasis() attr.Node { return rawClass("link-body-emphasis") }

// FS creates a Bootstrap fs-{n} (font size) class, from the largest (1, the
// size of h1) to the smallest (6, the size of h6), for n in 1..6. Any other
// n fails the render with [ErrInvalidValue]. Not to be confused with
// [FSItalic] and [FSNormal], which set the font style.
func FS(n int) attr.Node { return intClass("fs-", n, 1, 6) }

// FW creates a Bootstrap fw-{weight} class for one of the Bootstrap font
// weights: "bold", "bolder", "semibold", "medium", "normal", "light" or
// "lighter". Any other weight fails the render with [ErrInvalidValue]; the
// error does not repeat the weight.
func FW(weight string) attr.Node {
	return oneOf("fw-", weight, "bold", "bolder", "semibold", "medium", "normal", "light", "lighter")
}

// FWBold creates a Bootstrap fw-bold class.
func FWBold() attr.Node { return rawClass("fw-bold") }

// FWSemibold creates a Bootstrap fw-semibold class.
func FWSemibold() attr.Node { return rawClass("fw-semibold") }

// FWMedium creates a Bootstrap fw-medium class.
func FWMedium() attr.Node { return rawClass("fw-medium") }

// FWNormal creates a Bootstrap fw-normal class.
func FWNormal() attr.Node { return rawClass("fw-normal") }

// FWLight creates a Bootstrap fw-light class.
func FWLight() attr.Node { return rawClass("fw-light") }

// FSItalic creates a Bootstrap fst-italic (font style) class.
func FSItalic() attr.Node { return rawClass("fst-italic") }

// FSNormal creates a Bootstrap fst-normal (font style) class.
func FSNormal() attr.Node { return rawClass("fst-normal") }

// LH creates a Bootstrap lh-{v} (line height) class for one of the
// Bootstrap line heights: "1", "sm", "base" or "lg". Any other value fails
// the render with [ErrInvalidValue]; the error does not repeat the value.
func LH(v string) attr.Node { return oneOf("lh-", v, "1", "sm", "base", "lg") }

// FontMonospace creates a Bootstrap font-monospace class.
func FontMonospace() attr.Node { return rawClass("font-monospace") }
