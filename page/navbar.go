package page

import (
	"context"
	"strconv"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/aria"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/group/list"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/bi"
	"github.com/protolambda/webui/bs"
)

// DefaultNavbarLabel is the accessible name of a [Navbar] without a Label.
const DefaultNavbarLabel = "Main"

// Navbar is the top navigation of a page: a brand link, the navigation
// items, and an end slot (e.g. a logout form). It has no toggler and needs
// no script: on narrow screens its parts wrap onto further lines.
//
// It renders:
//
//	<nav id="navbar" class="navbar navbar-expand bg-body-tertiary border-bottom" aria-label="Main">
//	  <div class="container-fluid flex-wrap gap-2">
//	    <a class="navbar-brand" href="/">[icon] Brand</a>
//	    <ul class="navbar-nav nav-underline flex-wrap me-auto">
//	      <li class="nav-item"><a class="nav-link" href="/notes">[icon] Notes</a></li>
//	      <li class="nav-item"><a class="nav-link active" aria-current="page" href="/tasks">Tasks <span class="badge text-bg-warning">3</span></a></li>
//	    </ul>
//	    <div class="d-flex flex-wrap align-items-center gap-2">End</div>
//	  </div>
//	</nav>
//
// The current item is marked with aria-current="page" as well as the active
// style (an underline, not only a color). An item for the section that
// holds the current page, such as "Settings" on /settings/profile when a
// [Sidebar] lists the parts of the settings, is marked
// [NavItem.CurrentSection]: the same active style with aria-current="true",
// so that only links to the current page claim to be it. Without Items the
// list is left out, and without End the end slot.
type Navbar struct {
	// ID is the id of the nav element. Empty means [NavbarID].
	ID string
	// Label is the accessible name of the navigation landmark. Empty means
	// [DefaultNavbarLabel].
	Label string
	// Brand is the brand link at the start. The zero Brand leaves it out.
	Brand Brand
	// Items are the navigation links, in order.
	Items []NavItem
	// End holds the controls at the end of the navbar, such as a logout
	// form or a settings link.
	End []elem.Node
	// Attrs are extra attributes of the nav element, such as hx-swap-oob for
	// an out-of-band update. They must not set id or aria-label.
	Attrs attr.Node
}

var _ elem.Node = Navbar{}

// Brand is the brand link of a [Navbar]: the application's name, linking to
// its start page.
type Brand struct {
	// Label is the text of the brand link, its accessible name. A Brand
	// with an Href or Icon but without a Label fails the render.
	Label string
	// Href is the target of the brand link. Empty means "/".
	Href string
	// Icon is an optional decorative icon before the label.
	Icon bi.Icon
	// Current marks the brand link as the link of the current page
	// (aria-current="page"), on the start page.
	Current bool
}

// NavItem is a link of a [Navbar] or a [Sidebar].
type NavItem struct {
	// Label is the text of the link, its accessible name. It is required.
	Label string
	// Href is the target of the link: a relative reference or an http(s)
	// URL, in printable ASCII (escape path segments with url.PathEscape).
	// It is required.
	Href string
	// Icon is an optional decorative icon before the label.
	Icon bi.Icon
	// Current marks the link of the current page: the active style and
	// aria-current="page".
	Current bool
	// CurrentSection marks the link of the section that holds the current
	// page, when the link's target is not that page itself: the active
	// style and aria-current="true" (the current item of a set). Use it for
	// a navbar link such as "Settings" on /settings/profile, whose own link
	// in the [Sidebar] is Current. Current takes precedence when both are
	// set.
	CurrentSection bool
	// Badge is an optional short text in a badge after the label, such as
	// a count. It is part of the link's accessible name, after a space.
	Badge string
	// BadgeVariant is the color of the badge. Empty means
	// bs.VariantSecondary.
	BadgeVariant bs.Variant
}

// Eval builds the navbar. It fails with [ErrNoName] for an item or a brand
// without a label, and with [ErrInvalid] for an invalid ID, a link target
// that is not allowed (see [NavItem]), or an unknown badge variant.
func (n Navbar) Eval(ctx context.Context) (elem.Obj, error) {
	const component = "page.Navbar"
	id, err := checkID(component, n.ID, NavbarID)
	if err != nil {
		return elem.Obj{}, err
	}
	label := n.Label
	if blank(label) {
		label = DefaultNavbarLabel
	}
	parts := make([]elem.Node, 0, 3)
	if n.Brand != (Brand{}) {
		brand, err := n.Brand.node(component)
		if err != nil {
			return elem.Obj{}, err
		}
		parts = append(parts, brand)
	}
	if len(n.Items) > 0 {
		items := make([]elem.Node, 0, len(n.Items))
		for i, it := range n.Items {
			link, err := it.link(component+" item "+strconv.Itoa(i), navbarLink)
			if err != nil {
				return elem.Obj{}, err
			}
			items = append(items, bs.NavItem()(link))
		}
		parts = append(parts, list.UL(attr.Class("navbar-nav nav-underline flex-wrap me-auto"))(items...))
	}
	if len(n.End) > 0 {
		parts = append(parts, div.Div(attr.Class("d-flex flex-wrap align-items-center gap-2"))(n.End...))
	}
	attrs := append([]attr.Node{
		attr.ID(id),
		attr.Class("navbar navbar-expand bg-body-tertiary border-bottom"),
		aria.Label(label),
	}, optionalAttrs(n.Attrs)...)
	return section.Nav(attrs...)(
		div.Div(attr.Class("container-fluid flex-wrap gap-2"))(parts...),
	).Eval(ctx)
}

// node renders the brand link.
func (b Brand) node(component string) (elem.Node, error) {
	if blank(b.Label) {
		return nil, fail(component, ErrNoName, "brand label")
	}
	href := b.Href
	if href == "" {
		href = "/"
	}
	if !linkURL(href) {
		return nil, fail(component, ErrInvalid, "brand href")
	}
	attrs := []attr.Node{attr.Class("navbar-brand"), text.Href(href)}
	if b.Current {
		attrs = append(attrs, aria.Current("page"))
	}
	return text.A(attrs...)(iconText(b.Icon, "me-2", b.Label)), nil
}

// linkStyle is how a [NavItem] link is laid out in its container.
type linkStyle struct {
	class      string // extra classes of the link
	iconMargin string // margin class of the icon
	labelClass string // when set, the label is wrapped in a span of this class
	badgeClass string // extra classes of the badge
}

// navbarLink lays out a link of the navbar: inline icon, label and badge.
var navbarLink = linkStyle{iconMargin: "me-1"}

// link renders the nav link of the item.
func (it NavItem) link(component string, style linkStyle) (elem.Node, error) {
	if blank(it.Label) {
		return nil, fail(component, ErrNoName, "label")
	}
	if !linkURL(it.Href) {
		return nil, fail(component, ErrInvalid, "href")
	}
	var badgeAttrs []attr.Node
	if style.badgeClass != "" {
		badgeAttrs = append(badgeAttrs, attr.Class(style.badgeClass))
	}
	b, err := badge(component, it.Badge, it.BadgeVariant, badgeAttrs...)
	if err != nil {
		return nil, err
	}
	attrs := []attr.Node{text.Href(it.Href)}
	if style.class != "" {
		attrs = append(attrs, attr.Class(style.class))
	}
	var label elem.Node = text.Text(it.Label)
	if style.labelClass != "" {
		label = text.Span(attr.Class(style.labelClass))(label)
	}
	var content elem.Bundle
	if it.Icon != "" {
		if style.iconMargin != "" {
			content = append(content, it.Icon.With(attr.Class(style.iconMargin)))
		} else {
			content = append(content, it.Icon)
		}
	}
	content = append(content, label, b)
	switch {
	case it.Current:
		return bs.NavLinkActive(attrs...)(content), nil
	case it.CurrentSection:
		// The explicit aria-current replaces bs.NavLinkActive's "page".
		return bs.NavLinkActive(append(attrs, aria.Current("true"))...)(content), nil
	}
	return bs.NavLink(attrs...)(content), nil
}
