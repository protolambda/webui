package bs

import (
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/group/list"
)

// ListGroup creates a Bootstrap list group (ul.list-group).
func ListGroup(attrs ...attr.Node) elem.Scope {
	return list.UL(attr.Cons(rawClass("list-group"), attrs...))
}

// ListGroupFlush creates a Bootstrap list group without outer borders and
// rounded corners (ul.list-group.list-group-flush), e.g. inside a card.
func ListGroupFlush(attrs ...attr.Node) elem.Scope {
	return list.UL(attr.Cons(rawClass("list-group list-group-flush"), attrs...))
}

// ListGroupItem creates a Bootstrap list group item (li.list-group-item).
func ListGroupItem(attrs ...attr.Node) elem.Scope {
	return list.LI(attr.Cons(rawClass("list-group-item"), attrs...))
}

// ListGroupItemActive creates the selected list group item:
// li.list-group-item.active with aria-current="true" (unless attrs set
// aria-current), so that the selection is not conveyed by color alone.
func ListGroupItemActive(attrs ...attr.Node) elem.Scope {
	return list.LI(compose(attrs, rawClass("list-group-item active"), fallbackRaw("aria-current", "true")))
}
