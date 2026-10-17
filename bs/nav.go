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

// Nav creates a Bootstrap nav container (ul.nav). Wrap it in a nav
// element with an aria-label when it is a navigation landmark.
func Nav(attrs ...attr.Node) elem.Scope {
	return list.UL(attr.Cons(rawClass("nav"), attrs...))
}

// NavTabs creates a Bootstrap tabbed nav (ul.nav.nav-tabs). The tabs are
// links to pages (or page states): mark the current one with
// [NavLinkActive]. It is not an ARIA tab list, which needs script.
func NavTabs(attrs ...attr.Node) elem.Scope {
	return list.UL(attr.Cons(rawClass("nav nav-tabs"), attrs...))
}

// NavPills creates a Bootstrap pill nav (ul.nav.nav-pills).
func NavPills(attrs ...attr.Node) elem.Scope {
	return list.UL(attr.Cons(rawClass("nav nav-pills"), attrs...))
}

// NavUnderline creates a Bootstrap underlined nav (ul.nav.nav-underline).
func NavUnderline(attrs ...attr.Node) elem.Scope {
	return list.UL(attr.Cons(rawClass("nav nav-underline"), attrs...))
}

// NavItem creates a Bootstrap nav item (li.nav-item).
func NavItem(attrs ...attr.Node) elem.Scope {
	return list.LI(attr.Cons(rawClass("nav-item"), attrs...))
}

// NavLink creates a Bootstrap nav link (a.nav-link).
func NavLink(attrs ...attr.Node) elem.Scope {
	return text.A(attr.Cons(rawClass("nav-link"), attrs...))
}

// NavLinkActive creates the nav link of the current page:
// a.nav-link.active with aria-current="page" (unless attrs set
// aria-current), so that the current page is not conveyed by color alone.
func NavLinkActive(attrs ...attr.Node) elem.Scope {
	return text.A(compose(attrs, rawClass("nav-link active"), fallbackRaw("aria-current", "page")))
}

// NavLinkDisabled creates a disabled nav link: a.nav-link.disabled with
// aria-disabled="true" (unless attrs set aria-disabled). Give it no href,
// so that it cannot be followed or focused.
func NavLinkDisabled(attrs ...attr.Node) elem.Scope {
	return text.A(compose(attrs, rawClass("nav-link disabled"), fallbackRaw("aria-disabled", "true")))
}

// Navbar creates a Bootstrap navbar (nav.navbar). Give it an aria-label
// among attrs when the page has more than one navigation landmark.
//
// Without Bootstrap's script a collapsed navbar cannot be opened: add a
// navbar-expand class (e.g. attr.Class("navbar-expand")) so that the links
// always show, and let them wrap (flex-wrap) on narrow screens.
func Navbar(attrs ...attr.Node) elem.Scope {
	return section.Nav(attr.Cons(rawClass("navbar"), attrs...))
}

// NavbarBrand creates a Bootstrap navbar brand (a.navbar-brand).
func NavbarBrand(attrs ...attr.Node) elem.Scope {
	return text.A(attr.Cons(rawClass("navbar-brand"), attrs...))
}

// NavbarToggler creates a Bootstrap navbar toggler button, with the toggler
// icon span as its child. It has type="button", data-bs-toggle="collapse",
// aria-expanded="false" and aria-label="Toggle navigation"; attrs replace
// the type, aria-expanded and aria-label defaults, and must give the
// target (data-bs-target and aria-controls). Prefer [NavbarTogglerFor],
// which sets both from the id of the collapse.
//
// The toggler needs Bootstrap's script (the theme package's opt-in
// Bootstrap JS asset); see [Navbar] for a navbar without it.
func NavbarToggler(attrs ...attr.Node) elem.Node {
	return navbarToggler(nil, attrs)
}

// NavbarTogglerFor creates a navbar toggler like [NavbarToggler] for the
// [NavbarCollapse] with the given id: it adds data-bs-target="#id" and
// aria-controls="id". An invalid id fails the render with [ErrInvalidID].
func NavbarTogglerFor(targetID string, attrs ...attr.Node) elem.Node {
	return navbarToggler([]attr.Node{
		idTarget("bs.NavbarTogglerFor", "data-bs-target", targetID),
		idValue("bs.NavbarTogglerFor", "aria-controls", targetID),
	}, attrs)
}

func navbarToggler(target []attr.Node, attrs []attr.Node) elem.Node {
	own := []attr.Node{
		rawClass("navbar-toggler"),
		typeButton,
		attr.Name("data-bs-toggle").Raw("collapse"),
	}
	own = append(own, target...)
	own = append(own,
		fallbackRaw("aria-expanded", "false"),
		fallbackRaw("aria-label", "Toggle navigation"),
	)
	return button.Button(compose(attrs, own...))(text.Span(rawClass("navbar-toggler-icon"))())
}

// NavbarCollapse creates a Bootstrap navbar collapse container
// (div.collapse.navbar-collapse). Below the navbar's expand breakpoint it
// is hidden until Bootstrap's script shows it; give it an id for
// [NavbarTogglerFor].
func NavbarCollapse(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("collapse navbar-collapse"), attrs...))
}

// NavbarNav creates a Bootstrap navbar nav container (ul.navbar-nav).
func NavbarNav(attrs ...attr.Node) elem.Scope {
	return list.UL(attr.Cons(rawClass("navbar-nav"), attrs...))
}
