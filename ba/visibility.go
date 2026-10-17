package ba

import (
	"github.com/protolambda/chord/core/attr"
)

// Visible creates a Bootstrap visible class.
func Visible() attr.Node { return rawClass("visible") }

// Invisible creates a Bootstrap invisible class: the element keeps its space
// but is not shown, and is hidden from assistive technology too.
func Invisible() attr.Node { return rawClass("invisible") }

// VisuallyHidden creates a Bootstrap visually-hidden class: the content is
// not shown on screen but is still read by assistive technology. Use it for
// the text of an icon-only control or for extra context such as
// "(current)", rather than [DNone], which hides content from everyone.
func VisuallyHidden() attr.Node { return rawClass("visually-hidden") }

// VisuallyHiddenFocusable creates a Bootstrap visually-hidden-focusable
// class: hidden like [VisuallyHidden] until the element (or an element
// inside it) has keyboard focus, as for a "skip to content" link.
func VisuallyHiddenFocusable() attr.Node { return rawClass("visually-hidden-focusable") }
