package bs

import (
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/group/list"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"
)

// Breadcrumb creates a Bootstrap breadcrumb container.
func Breadcrumb(attrs ...attr.Node) elem.Scope {
	return func(children ...elem.Node) elem.Node {
		return section.Nav()(
			list.OL(attr.Cons(rawClass("breadcrumb"), attrs...))(children...),
		)
	}
}

// BreadcrumbItem creates a Bootstrap breadcrumb item.
func BreadcrumbItem(attrs ...attr.Node) elem.Scope {
	return list.LI(attr.Cons(rawClass("breadcrumb-item"), attrs...))
}

// Carousel creates a Bootstrap carousel.
func Carousel(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("carousel slide"), attrs...))
}

// Pagination creates a Bootstrap pagination container: nav > ul.pagination.
// Its items are [PageItem] elements, each holding a [PageLink]:
//
//	bs.Pagination()(bs.PageItem()(bs.PageLink(text.Href("?page=2"))(text.Text("2"))))
func Pagination(attrs ...attr.Node) elem.Scope {
	return func(children ...elem.Node) elem.Node {
		return section.Nav()(
			list.UL(attr.Cons(rawClass("pagination"), attrs...))(children...),
		)
	}
}

// PageItem creates a Bootstrap pagination item (li.page-item).
func PageItem(attrs ...attr.Node) elem.Scope {
	return list.LI(attr.Cons(rawClass("page-item"), attrs...))
}

// PageLink creates a Bootstrap pagination link (a.page-link), the content of
// a [PageItem]. It is the anchor itself: give it the href (and hx-*
// attributes) instead of nesting an a element, since HTML does not allow
// nested anchors.
func PageLink(attrs ...attr.Node) elem.Scope {
	return text.A(attr.Cons(rawClass("page-link"), attrs...))
}

// Progress creates a Bootstrap progress container.
func Progress(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("progress"), attrs...))
}

// ProgressBar creates a Bootstrap progress bar.
func ProgressBar(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("progress-bar"), attrs...))
}

// Spinner creates a Bootstrap spinner.
func Spinner(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("spinner-border"), attrs...))
}

// Collapse creates a Bootstrap collapse container.
func Collapse(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("collapse"), attrs...))
}
