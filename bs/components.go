package bs

import (
	"fmt"
	"strconv"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/group/list"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"
)

// Breadcrumb creates a Bootstrap breadcrumb: nav[aria-label="Breadcrumb"] >
// ol.breadcrumb. The attrs go on the ol; the children are
// [BreadcrumbItem] and [BreadcrumbActive] items. Use [BreadcrumbLabeled]
// for another navigation label (such as a translation).
func Breadcrumb(attrs ...attr.Node) elem.Scope {
	return BreadcrumbLabeled("Breadcrumb", attrs...)
}

// BreadcrumbLabeled creates a Bootstrap breadcrumb like [Breadcrumb], with
// label as the accessible name of its nav landmark.
func BreadcrumbLabeled(label string, attrs ...attr.Node) elem.Scope {
	return func(children ...elem.Node) elem.Node {
		return section.Nav(attr.Name("aria-label").Value(label))(
			list.OL(attr.Cons(rawClass("breadcrumb"), attrs...))(children...),
		)
	}
}

// BreadcrumbItem creates a Bootstrap breadcrumb item (li.breadcrumb-item),
// usually holding a link.
func BreadcrumbItem(attrs ...attr.Node) elem.Scope {
	return list.LI(attr.Cons(rawClass("breadcrumb-item"), attrs...))
}

// BreadcrumbActive creates the breadcrumb item of the current page:
// li.breadcrumb-item.active with aria-current="page" (unless attrs set
// aria-current). It holds text, not a link.
func BreadcrumbActive(attrs ...attr.Node) elem.Scope {
	return list.LI(compose(attrs, rawClass("breadcrumb-item active"), fallbackRaw("aria-current", "page")))
}

// Carousel creates a Bootstrap carousel (div.carousel.slide).
//
// A carousel needs Bootstrap's script (the theme package's opt-in
// Bootstrap JS asset) to move between slides; without it only the active
// slide shows.
func Carousel(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("carousel slide"), attrs...))
}

// Pagination creates a Bootstrap pagination container:
// nav[aria-label="Pagination"] > ul.pagination. The attrs go on the ul. Its
// items are [PageItem] elements, each holding a [PageLink]:
//
//	bs.Pagination()(bs.PageItem()(bs.PageLink(text.Href("?page=2"))(text.Text("2"))))
//
// Use [PaginationLabeled] for another navigation label, e.g. when a page
// has two paginated lists.
func Pagination(attrs ...attr.Node) elem.Scope {
	return PaginationLabeled("Pagination", attrs...)
}

// PaginationLabeled creates a Bootstrap pagination container like
// [Pagination], with label as the accessible name of its nav landmark.
func PaginationLabeled(label string, attrs ...attr.Node) elem.Scope {
	return func(children ...elem.Node) elem.Node {
		return section.Nav(attr.Name("aria-label").Value(label))(
			list.UL(attr.Cons(rawClass("pagination"), attrs...))(children...),
		)
	}
}

// PageItem creates a Bootstrap pagination item (li.page-item).
func PageItem(attrs ...attr.Node) elem.Scope {
	return list.LI(attr.Cons(rawClass("page-item"), attrs...))
}

// PageItemActive creates the pagination item of the current page
// (li.page-item.active). Put a [PageLinkCurrent] in it.
func PageItemActive(attrs ...attr.Node) elem.Scope {
	return list.LI(attr.Cons(rawClass("page-item active"), attrs...))
}

// PageItemDisabled creates a disabled pagination item
// (li.page-item.disabled). Put a [PageLink] without href in it, so that
// the link cannot be followed or focused.
func PageItemDisabled(attrs ...attr.Node) elem.Scope {
	return list.LI(attr.Cons(rawClass("page-item disabled"), attrs...))
}

// PageLink creates a Bootstrap pagination link (a.page-link), the content of
// a [PageItem]. It is the anchor itself: give it the href (and hx-*
// attributes) instead of nesting an a element, since HTML does not allow
// nested anchors.
func PageLink(attrs ...attr.Node) elem.Scope {
	return text.A(attr.Cons(rawClass("page-link"), attrs...))
}

// PageLinkCurrent creates the pagination link of the current page:
// a.page-link with aria-current="page" (unless attrs set aria-current).
func PageLinkCurrent(attrs ...attr.Node) elem.Scope {
	return text.A(compose(attrs, rawClass("page-link"), fallbackRaw("aria-current", "page")))
}

// Progress creates a Bootstrap progress container: div.progress with
// role="progressbar" (unless attrs set a role). As in Bootstrap 5.3, the
// container carries the role and the values: pass an aria-label and
// [ProgressValue] among attrs, and a [ProgressBar] as the child:
//
//	bs.Progress(aria.Label("Upload"), bs.ProgressValue(50, 0, 100))(
//		bs.ProgressBar(attr.Class("w-50"))(),
//	)
func Progress(attrs ...attr.Node) elem.Scope {
	return div.Div(compose(attrs, rawClass("progress"), fallbackRaw("role", "progressbar")))
}

// ProgressBar creates the bar of a [Progress] (div.progress-bar). Its
// width shows the value. Bootstrap's documentation sets the width with a
// style attribute, which a strict Content-Security-Policy (style-src
// without 'unsafe-inline') blocks: use a width class instead, such as
// w-25, w-50, w-75 or w-100, and keep the exact value in [ProgressValue].
func ProgressBar(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("progress-bar"), attrs...))
}

// ProgressValue returns the aria-valuenow, aria-valuemin and aria-valuemax
// attributes of a [Progress]. It fails the render with [ErrInvalidOption]
// unless minimum < maximum and minimum <= now <= maximum.
func ProgressValue(now, minimum, maximum int) attr.Node {
	if minimum >= maximum || now < minimum || now > maximum {
		return errAttr{fmt.Errorf("bs.ProgressValue: %w: want minimum < maximum and minimum <= now <= maximum", ErrInvalidOption)}
	}
	return attr.Bundle{
		attr.Name("aria-valuenow").Raw(strconv.Itoa(now)),
		attr.Name("aria-valuemin").Raw(strconv.Itoa(minimum)),
		attr.Name("aria-valuemax").Raw(strconv.Itoa(maximum)),
	}
}

// Spinner creates a Bootstrap border spinner: div.spinner-border with
// role="status" (unless attrs set a role). The children are its accessible
// label, rendered visually hidden (the spinner shows no text); without
// children the label is "Loading...":
//
//	bs.Spinner()                          // label "Loading..."
//	bs.Spinner()(text.Text("Saving..."))  // label "Saving..."
//
// For a small spinner, add attr.Class("spinner-border-sm") among attrs.
func Spinner(attrs ...attr.Node) elem.Scope {
	return spinner("spinner-border", attrs)
}

// SpinnerGrow creates a Bootstrap growing spinner (div.spinner-grow), with
// the role and label of [Spinner].
func SpinnerGrow(attrs ...attr.Node) elem.Scope {
	return spinner("spinner-grow", attrs)
}

func spinner(class string, attrs []attr.Node) elem.Scope {
	return func(children ...elem.Node) elem.Node {
		if len(children) == 0 {
			children = []elem.Node{text.Text("Loading...")}
		}
		return div.Div(compose(attrs, rawClass(class), fallbackRaw("role", "status")))(visuallyHidden(children...))
	}
}

// Collapse creates a Bootstrap collapse container (div.collapse).
//
// It is hidden until Bootstrap's script (the theme package's opt-in
// Bootstrap JS asset) shows it; without that script, use a details element
// (see [DetailsAccordion]) or always-visible content.
func Collapse(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("collapse"), attrs...))
}
