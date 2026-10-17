package page

import (
	"context"
	"strconv"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/bi"
	"github.com/protolambda/webui/bs"
)

// Header is the header of a page: the page's one h1, optional actions on
// the same line, an optional breadcrumb above it, and optional content
// below it (such as tabs).
//
// It renders:
//
//	<header id="page-header" class="mb-3">
//	  <nav aria-label="Breadcrumb"><ol class="breadcrumb mb-2">...</ol></nav>
//	  <div class="d-flex flex-wrap align-items-center gap-2">
//	    <h1 class="h4 mb-0 me-auto">[icon] Title</h1>
//	    <div class="d-flex flex-wrap align-items-center gap-2">Actions</div>
//	  </div>
//	  Below
//	</header>
//
// Without Actions the h1 stands alone in the header. The header has the id
// [HeaderID]; render one per page, first in the main content ([Layout]
// does so with its Header field).
type Header struct {
	// Title is the content of the h1, the page's main heading. It is
	// required.
	Title elem.Node
	// Icon is an optional decorative icon before the title.
	Icon bi.Icon
	// Crumbs are the optional breadcrumb above the title, from the start
	// page down. See [Crumb].
	Crumbs []Crumb
	// Actions are the controls on the line of the title, such as a "New"
	// button. They wrap below the title on narrow screens.
	Actions []elem.Node
	// Below is optional content after the title line, such as tabs.
	Below elem.Node
}

var _ elem.Node = Header{}

// Crumb is a step of the breadcrumb of a [Header].
type Crumb struct {
	// Label is the text of the step. It is required.
	Label string
	// Href is the link of the step, with the rules of [NavItem.Href]. A
	// last step without Href is the current page: it renders as text with
	// aria-current="page". Another step without Href renders as plain text.
	Href string
}

// Eval builds the header. It fails with [ErrNoName] for a nil Title or a
// crumb without a label, and with [ErrInvalid] for a crumb link that is not
// allowed.
func (h Header) Eval(ctx context.Context) (elem.Obj, error) {
	const component = "page.Header"
	if h.Title == nil {
		return elem.Obj{}, fail(component, ErrNoName, "title")
	}
	parts := make([]elem.Node, 0, 3)
	if len(h.Crumbs) > 0 {
		crumbs, err := breadcrumb(component, h.Crumbs)
		if err != nil {
			return elem.Obj{}, err
		}
		parts = append(parts, crumbs)
	}
	var title elem.Node = h.Title
	if h.Icon != "" {
		title = elem.Bundle{h.Icon.With(attr.Class("me-2")), h.Title}
	}
	if len(h.Actions) == 0 {
		parts = append(parts, section.H1(attr.Class("h4 mb-0"))(title))
	} else {
		parts = append(parts, div.Div(attr.Class("d-flex flex-wrap align-items-center gap-2"))(
			section.H1(attr.Class("h4 mb-0 me-auto"))(title),
			div.Div(attr.Class("d-flex flex-wrap align-items-center gap-2"))(h.Actions...),
		))
	}
	if h.Below != nil {
		parts = append(parts, h.Below)
	}
	return section.Header(attr.ID(HeaderID), attr.Class("mb-3"))(parts...).Eval(ctx)
}

// breadcrumb renders the crumbs as a Bootstrap breadcrumb.
func breadcrumb(component string, crumbs []Crumb) (elem.Node, error) {
	items := make([]elem.Node, 0, len(crumbs))
	for i, c := range crumbs {
		if blank(c.Label) {
			return nil, fail(component, ErrNoName, "crumb "+strconv.Itoa(i)+" label")
		}
		switch {
		case c.Href != "":
			if !linkURL(c.Href) {
				return nil, fail(component, ErrInvalid, "crumb "+strconv.Itoa(i)+" href")
			}
			items = append(items, bs.BreadcrumbItem()(text.A(text.Href(c.Href))(text.Text(c.Label))))
		case i == len(crumbs)-1:
			items = append(items, bs.BreadcrumbActive()(text.Text(c.Label)))
		default:
			items = append(items, bs.BreadcrumbItem()(text.Text(c.Label)))
		}
	}
	return bs.Breadcrumb(attr.Class("mb-2"))(items...), nil
}
