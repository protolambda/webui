package bs

import (
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/group/div"
)

// Alert creates a basic Bootstrap alert.
func Alert(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("alert"), attrs...))
}

// AlertPrimary creates a primary Bootstrap alert.
func AlertPrimary(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("alert alert-primary"), attrs...))
}

// AlertSecondary creates a secondary Bootstrap alert.
func AlertSecondary(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("alert alert-secondary"), attrs...))
}

// AlertSuccess creates a success Bootstrap alert.
func AlertSuccess(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("alert alert-success"), attrs...))
}

// AlertDanger creates a danger Bootstrap alert.
func AlertDanger(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("alert alert-danger"), attrs...))
}

// AlertWarning creates a warning Bootstrap alert.
func AlertWarning(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("alert alert-warning"), attrs...))
}

// AlertInfo creates an info Bootstrap alert.
func AlertInfo(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("alert alert-info"), attrs...))
}

// AlertDismissible creates a dismissible Bootstrap alert.
func AlertDismissible(attrs ...attr.Node) elem.Scope {
	return func(children ...elem.Node) elem.Node {
		allChildren := make([]elem.Node, 0, 1+len(children))
		allChildren = append(allChildren, closeButton("alert"))
		allChildren = append(allChildren, children...)
		return div.Div(attr.Cons(
			rawClass("alert alert-dismissible fade show"),
			attr.Bundle(attrs),
		))(allChildren...)
	}
}
