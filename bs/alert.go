package bs

import (
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/group/div"
)

// roleAlert is the role="alert" default of alerts.
var roleAlert = fallbackRaw("role", "alert")

// alert creates a div.alert with the given classes and role="alert",
// unless attrs set a role.
func alert(class attr.Node, attrs []attr.Node) elem.Scope {
	return div.Div(compose(attrs, class, roleAlert))
}

// Alert creates a basic Bootstrap alert: div.alert with role="alert".
//
// Like every Alert* constructor, it sets role="alert", which makes screen
// readers announce the alert as soon as it appears (also when an htmx swap
// inserts it). For a message that should not interrupt, such as a
// confirmation, pass another role among attrs, e.g. aria.Role("status"):
// a role among attrs replaces the default, wherever it is (bundles,
// sequences and lazy attributes included).
func Alert(attrs ...attr.Node) elem.Scope {
	return alert(rawClass("alert"), attrs)
}

// AlertOf creates a Bootstrap alert of the variant (div.alert.alert-{v}).
// An unknown variant fails the render with [ErrInvalidOption].
func AlertOf(v Variant, attrs ...attr.Node) elem.Scope {
	return alert(attr.Bundle{rawClass("alert"), variantClass("bs.AlertOf", "alert-", v)}, attrs)
}

// AlertPrimary creates a primary Bootstrap alert.
func AlertPrimary(attrs ...attr.Node) elem.Scope {
	return alert(rawClass("alert alert-primary"), attrs)
}

// AlertSecondary creates a secondary Bootstrap alert.
func AlertSecondary(attrs ...attr.Node) elem.Scope {
	return alert(rawClass("alert alert-secondary"), attrs)
}

// AlertSuccess creates a success Bootstrap alert.
func AlertSuccess(attrs ...attr.Node) elem.Scope {
	return alert(rawClass("alert alert-success"), attrs)
}

// AlertDanger creates a danger Bootstrap alert.
func AlertDanger(attrs ...attr.Node) elem.Scope {
	return alert(rawClass("alert alert-danger"), attrs)
}

// AlertWarning creates a warning Bootstrap alert.
func AlertWarning(attrs ...attr.Node) elem.Scope {
	return alert(rawClass("alert alert-warning"), attrs)
}

// AlertInfo creates an info Bootstrap alert.
func AlertInfo(attrs ...attr.Node) elem.Scope {
	return alert(rawClass("alert alert-info"), attrs)
}

// AlertLight creates a light Bootstrap alert.
func AlertLight(attrs ...attr.Node) elem.Scope {
	return alert(rawClass("alert alert-light"), attrs)
}

// AlertDark creates a dark Bootstrap alert.
func AlertDark(attrs ...attr.Node) elem.Scope {
	return alert(rawClass("alert alert-dark"), attrs)
}

// AlertDismissible creates a dismissible Bootstrap alert:
// div.alert.alert-dismissible.fade.show with role="alert" (unless attrs set
// a role), and a close button after the children, as in Bootstrap's
// documentation (it is positioned at the top right). Give the color among
// attrs, e.g. attr.Class("alert-success").
//
// The close button uses data-bs-dismiss="alert" and needs Bootstrap's
// script (the theme package's opt-in Bootstrap JS asset); without it the
// button does nothing. Without that script, render an [Alert] with a
// [CloseButton] that removes the alert through htmx instead, or no close
// button at all.
func AlertDismissible(attrs ...attr.Node) elem.Scope {
	return func(children ...elem.Node) elem.Node {
		allChildren := make([]elem.Node, 0, len(children)+1)
		allChildren = append(allChildren, children...)
		allChildren = append(allChildren, dismissButton("alert", ""))
		return div.Div(compose(attrs, rawClass("alert alert-dismissible fade show"), roleAlert))(allChildren...)
	}
}
