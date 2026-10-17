// Package bs provides Bootstrap 5.3 components and utilities.
// Reference: https://getbootstrap.com/docs/5.3/
package bs

import (
	"strings"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/group/div"
)

// btn creates a button element with the given classes. It has type="button"
// unless attrs set the type directly: a button without a type submits its
// form when clicked.
func btn(class string, attrs []attr.Node) elem.Scope {
	all := make([]attr.Node, 0, 2+len(attrs))
	all = append(all, rawClass(class))
	if !setsAttr(attrs, "type") {
		all = append(all, button.Type(button.TypeButton))
	}
	return button.Button(append(all, attrs...)...)
}

// setsAttr reports whether attrs contain an evaluated attribute with the
// given lowercase name, directly or in an [attr.Bundle]. Lazy attributes
// (such as [attr.Fn] and [attr.Seq]) are not inspected, since evaluating
// them here would run application code outside the render.
func setsAttr(attrs []attr.Node, name string) bool {
	for _, a := range attrs {
		switch a := a.(type) {
		case attr.Obj:
			if strings.EqualFold(a.Key, name) {
				return true
			}
		case attr.Bundle:
			if setsAttr(a, name) {
				return true
			}
		}
	}
	return false
}

// Btn creates a basic Bootstrap button.
//
// Like every Btn* constructor, it sets type="button", so that the button does
// not submit an enclosing form. To make a submit (or reset) button, pass the
// type directly among attrs, e.g. bs.BtnPrimary(button.Type(button.TypeSubmit)),
// possibly inside an [attr.Bundle]. A type attribute that is only known when
// evaluated (an [attr.Fn] or [attr.Seq]) is not detected, and the render fails
// with a duplicate type attribute.
func Btn(attrs ...attr.Node) elem.Scope {
	return btn("btn", attrs)
}

// BtnPrimary creates a primary Bootstrap button.
func BtnPrimary(attrs ...attr.Node) elem.Scope {
	return btn("btn btn-primary", attrs)
}

// BtnSecondary creates a secondary Bootstrap button.
func BtnSecondary(attrs ...attr.Node) elem.Scope {
	return btn("btn btn-secondary", attrs)
}

// BtnSuccess creates a success Bootstrap button.
func BtnSuccess(attrs ...attr.Node) elem.Scope {
	return btn("btn btn-success", attrs)
}

// BtnDanger creates a danger Bootstrap button.
func BtnDanger(attrs ...attr.Node) elem.Scope {
	return btn("btn btn-danger", attrs)
}

// BtnWarning creates a warning Bootstrap button.
func BtnWarning(attrs ...attr.Node) elem.Scope {
	return btn("btn btn-warning", attrs)
}

// BtnInfo creates an info Bootstrap button.
func BtnInfo(attrs ...attr.Node) elem.Scope {
	return btn("btn btn-info", attrs)
}

// BtnLight creates a light Bootstrap button.
func BtnLight(attrs ...attr.Node) elem.Scope {
	return btn("btn btn-light", attrs)
}

// BtnDark creates a dark Bootstrap button.
func BtnDark(attrs ...attr.Node) elem.Scope {
	return btn("btn btn-dark", attrs)
}

// BtnLink creates a link-styled Bootstrap button.
func BtnLink(attrs ...attr.Node) elem.Scope {
	return btn("btn btn-link", attrs)
}

// BtnOutlinePrimary creates an outline primary Bootstrap button.
func BtnOutlinePrimary(attrs ...attr.Node) elem.Scope {
	return btn("btn btn-outline-primary", attrs)
}

// BtnOutlineSecondary creates an outline secondary Bootstrap button.
func BtnOutlineSecondary(attrs ...attr.Node) elem.Scope {
	return btn("btn btn-outline-secondary", attrs)
}

// BtnGroup creates a Bootstrap button group wrapper.
func BtnGroup(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("btn-group"), attrs...))
}
