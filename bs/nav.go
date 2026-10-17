package bs

import (
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/group/list"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"
)

// Nav creates a Bootstrap nav container.
func Nav(attrs ...attr.Node) elem.Scope {
	return list.UL(attr.Cons(rawClass("nav"), attrs...))
}

// NavItem creates a Bootstrap nav item.
func NavItem(attrs ...attr.Node) elem.Scope {
	return list.LI(attr.Cons(rawClass("nav-item"), attrs...))
}

// NavLink creates a Bootstrap nav link.
func NavLink(attrs ...attr.Node) elem.Scope {
	return text.A(attr.Cons(rawClass("nav-link"), attrs...))
}

// Navbar creates a Bootstrap navbar.
func Navbar(attrs ...attr.Node) elem.Scope {
	return section.Nav(attr.Cons(rawClass("navbar"), attrs...))
}

// NavbarBrand creates a Bootstrap navbar brand.
func NavbarBrand(attrs ...attr.Node) elem.Scope {
	return text.A(attr.Cons(rawClass("navbar-brand"), attrs...))
}

// NavbarToggler creates a Bootstrap navbar toggler button.
// The toggler icon span is automatically included as a child.
func NavbarToggler(attrs ...attr.Node) elem.Node {
	return button.Button(attr.Cons(
		rawClass("navbar-toggler"),
		button.Type(button.TypeButton),
		attr.Name("data-bs-toggle").Raw("collapse"),
		attr.Bundle(attrs),
	))(text.Span(rawClass("navbar-toggler-icon"))())
}

// NavbarCollapse creates a Bootstrap navbar collapse container.
func NavbarCollapse(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("collapse navbar-collapse"), attrs...))
}

// NavbarNav creates a Bootstrap navbar nav container.
func NavbarNav(attrs ...attr.Node) elem.Scope {
	return list.UL(attr.Cons(rawClass("navbar-nav"), attrs...))
}
