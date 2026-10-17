package page

import (
	"context"
	"strconv"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/aria"
	"github.com/protolambda/chord/html/group/list"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/bs"
)

// DefaultSidebarLabel is the accessible name of a [Sidebar] without a
// Label.
const DefaultSidebarLabel = "Secondary"

// Sidebar is a secondary navigation in sections, for the sidebar column of
// a [Layout]: e.g. the projects of an account, or the parts of a settings
// area.
//
// It renders:
//
//	<nav id="sidebar" aria-label="Secondary">
//	  <h2 class="h6 small text-uppercase text-body-secondary px-3 mb-2">Section</h2>
//	  <ul class="nav nav-pills flex-column mb-3">
//	    <li class="nav-item"><a class="nav-link d-flex align-items-center gap-2" href="/a">[icon]<span class="text-break">Item</span> <span class="badge text-bg-secondary ms-auto">2</span></a></li>
//	  </ul>
//	</nav>
//
// A section without a Title renders its list only. Labels wrap rather than
// being truncated, so that the distinguishing end of a long name stays
// visible. The current item is marked with aria-current="page" as well as
// the active fill.
//
// In a two-level navigation, mark the sidebar item of the page itself
// [NavItem.Current], and the [Navbar] item of the area the sidebar belongs
// to [NavItem.CurrentSection] (aria-current="true"): a link to another page
// must not claim to be the current page. A sidebar item may also be a
// CurrentSection, such as a project on one of the project's sub-pages.
type Sidebar struct {
	// ID is the id of the nav element. Empty means [SidebarID].
	ID string
	// Label is the accessible name of the navigation landmark. Empty means
	// [DefaultSidebarLabel]; prefer a name that says what the links are,
	// such as "Projects".
	Label string
	// Sections are the groups of links, in order.
	Sections []SideSection
	// Attrs are extra attributes of the nav element, such as hx-swap-oob for
	// an out-of-band update. They must not set id or aria-label.
	Attrs attr.Node
}

var _ elem.Node = Sidebar{}

// SideSection is a group of links of a [Sidebar].
type SideSection struct {
	// Title is the optional heading of the section (an h2).
	Title string
	// Items are the links of the section.
	Items []NavItem
}

// sidebarLink lays out a link of the sidebar: icon, wrapping label, and the
// badge at the end of the line.
var sidebarLink = linkStyle{
	class:      "d-flex align-items-center gap-2",
	labelClass: "text-break",
	badgeClass: "ms-auto",
}

// Eval builds the sidebar. It fails like [Navbar.Eval] for its items.
func (s Sidebar) Eval(ctx context.Context) (elem.Obj, error) {
	const component = "page.Sidebar"
	id, err := checkID(component, s.ID, SidebarID)
	if err != nil {
		return elem.Obj{}, err
	}
	label := s.Label
	if blank(label) {
		label = DefaultSidebarLabel
	}
	parts := make([]elem.Node, 0, 2*len(s.Sections))
	for i, sec := range s.Sections {
		if !blank(sec.Title) {
			parts = append(parts, section.H2(attr.Class("h6 small text-uppercase text-body-secondary px-3 mb-2"))(text.Text(sec.Title)))
		}
		items := make([]elem.Node, 0, len(sec.Items))
		for j, it := range sec.Items {
			link, err := it.link(component+" section "+strconv.Itoa(i)+" item "+strconv.Itoa(j), sidebarLink)
			if err != nil {
				return elem.Obj{}, err
			}
			items = append(items, bs.NavItem()(link))
		}
		parts = append(parts, list.UL(attr.Class("nav nav-pills flex-column mb-3"))(items...))
	}
	attrs := append([]attr.Node{attr.ID(id), aria.Label(label)}, optionalAttrs(s.Attrs)...)
	return section.Nav(attrs...)(parts...).Eval(ctx)
}
