package bs

import (
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/group/div"
)

// typeButton is the type="button" default of the buttons in this package.
var typeButton = fallbackRaw("type", "button")

// btn creates a button element with the given classes. It has type="button"
// unless attrs set a type: a button without a type submits its form when
// clicked.
func btn(class attr.Node, attrs []attr.Node) elem.Scope {
	return button.Button(compose(attrs, class, typeButton))
}

// Btn creates a basic Bootstrap button (button.btn).
//
// Like every Btn* constructor, it sets type="button", so that the button
// does not submit an enclosing form. To make a submit (or reset) button,
// pass the type among attrs, e.g. bs.BtnPrimary(button.Type(button.TypeSubmit)).
// The type is detected wherever it is among attrs: in a bundle, a sequence,
// or a lazy [attr.Fn] (which is evaluated once, during the render). Give a
// submit button an id when htmx replaces its form in place, so that the
// focus stays on it (see Forms in the package documentation).
func Btn(attrs ...attr.Node) elem.Scope {
	return btn(rawClass("btn"), attrs)
}

// BtnOf creates a Bootstrap button of the variant (button.btn.btn-{v}). An
// unknown variant fails the render with [ErrInvalidOption].
func BtnOf(v Variant, attrs ...attr.Node) elem.Scope {
	return btn(attr.Bundle{rawClass("btn"), variantClass("bs.BtnOf", "btn-", v)}, attrs)
}

// BtnOutlineOf creates an outline Bootstrap button of the variant
// (button.btn.btn-outline-{v}). An unknown variant fails the render with
// [ErrInvalidOption].
func BtnOutlineOf(v Variant, attrs ...attr.Node) elem.Scope {
	return btn(attr.Bundle{rawClass("btn"), variantClass("bs.BtnOutlineOf", "btn-outline-", v)}, attrs)
}

// BtnPrimary creates a primary Bootstrap button.
func BtnPrimary(attrs ...attr.Node) elem.Scope {
	return btn(rawClass("btn btn-primary"), attrs)
}

// BtnSecondary creates a secondary Bootstrap button.
func BtnSecondary(attrs ...attr.Node) elem.Scope {
	return btn(rawClass("btn btn-secondary"), attrs)
}

// BtnSuccess creates a success Bootstrap button.
func BtnSuccess(attrs ...attr.Node) elem.Scope {
	return btn(rawClass("btn btn-success"), attrs)
}

// BtnDanger creates a danger Bootstrap button.
func BtnDanger(attrs ...attr.Node) elem.Scope {
	return btn(rawClass("btn btn-danger"), attrs)
}

// BtnWarning creates a warning Bootstrap button.
func BtnWarning(attrs ...attr.Node) elem.Scope {
	return btn(rawClass("btn btn-warning"), attrs)
}

// BtnInfo creates an info Bootstrap button.
func BtnInfo(attrs ...attr.Node) elem.Scope {
	return btn(rawClass("btn btn-info"), attrs)
}

// BtnLight creates a light Bootstrap button.
func BtnLight(attrs ...attr.Node) elem.Scope {
	return btn(rawClass("btn btn-light"), attrs)
}

// BtnDark creates a dark Bootstrap button.
func BtnDark(attrs ...attr.Node) elem.Scope {
	return btn(rawClass("btn btn-dark"), attrs)
}

// BtnLink creates a link-styled Bootstrap button. It is still a button:
// use a link (text.A) for navigation.
func BtnLink(attrs ...attr.Node) elem.Scope {
	return btn(rawClass("btn btn-link"), attrs)
}

// BtnOutlinePrimary creates an outline primary Bootstrap button.
func BtnOutlinePrimary(attrs ...attr.Node) elem.Scope {
	return btn(rawClass("btn btn-outline-primary"), attrs)
}

// BtnOutlineSecondary creates an outline secondary Bootstrap button.
func BtnOutlineSecondary(attrs ...attr.Node) elem.Scope {
	return btn(rawClass("btn btn-outline-secondary"), attrs)
}

// BtnOutlineSuccess creates an outline success Bootstrap button.
func BtnOutlineSuccess(attrs ...attr.Node) elem.Scope {
	return btn(rawClass("btn btn-outline-success"), attrs)
}

// BtnOutlineDanger creates an outline danger Bootstrap button.
func BtnOutlineDanger(attrs ...attr.Node) elem.Scope {
	return btn(rawClass("btn btn-outline-danger"), attrs)
}

// BtnOutlineWarning creates an outline warning Bootstrap button.
func BtnOutlineWarning(attrs ...attr.Node) elem.Scope {
	return btn(rawClass("btn btn-outline-warning"), attrs)
}

// BtnOutlineInfo creates an outline info Bootstrap button.
func BtnOutlineInfo(attrs ...attr.Node) elem.Scope {
	return btn(rawClass("btn btn-outline-info"), attrs)
}

// BtnOutlineLight creates an outline light Bootstrap button.
func BtnOutlineLight(attrs ...attr.Node) elem.Scope {
	return btn(rawClass("btn btn-outline-light"), attrs)
}

// BtnOutlineDark creates an outline dark Bootstrap button.
func BtnOutlineDark(attrs ...attr.Node) elem.Scope {
	return btn(rawClass("btn btn-outline-dark"), attrs)
}

// BtnGroup creates a Bootstrap button group wrapper:
// div.btn-group with role="group", unless attrs set a role. Give the group
// a name with aria-label (or aria-labelledby) among attrs.
func BtnGroup(attrs ...attr.Node) elem.Scope {
	return div.Div(compose(attrs, rawClass("btn-group"), fallbackRaw("role", "group")))
}

// BtnToolbar creates a Bootstrap button toolbar, a row of button groups:
// div.btn-toolbar with role="toolbar", unless attrs set a role. Give the
// toolbar a name with aria-label among attrs.
func BtnToolbar(attrs ...attr.Node) elem.Scope {
	return div.Div(compose(attrs, rawClass("btn-toolbar"), fallbackRaw("role", "toolbar")))
}

// CloseButton creates Bootstrap's close button:
// <button class="btn-close" type="button" aria-label="Close">. The button
// shows a cross and has no text, so aria-label names it; pass another
// aria-label (or a type) among attrs to replace the default.
//
// The button does nothing by itself: give it the attributes of what it
// closes, such as [DialogClose] for a [Dialog], an htmx request, or
// data-bs-dismiss for the Bootstrap JS components.
func CloseButton(attrs ...attr.Node) elem.Node {
	return button.Button(compose(attrs,
		rawClass("btn-close"),
		typeButton,
		fallbackRaw("aria-label", "Close"),
	))()
}

// dismissButton is the close button of a Bootstrap JS component: its
// data-bs-dismiss attribute needs Bootstrap's script. An empty label keeps
// the default "Close".
func dismissButton(dismiss, label string) elem.Node {
	attrs := []attr.Node{attr.Name("data-bs-dismiss").Raw(dismiss)}
	if label != "" {
		attrs = append(attrs, attr.Name("aria-label").Value(label))
	}
	return CloseButton(attrs...)
}
