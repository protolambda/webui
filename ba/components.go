package ba

import (
	"github.com/protolambda/chord/core/attr"
)

// Tooltip creates tooltip attributes for an element.
func Tooltip(opts ...attr.Node) attr.Node {
	return attr.Cons(attr.Name("data-bs-toggle").Raw("tooltip"), opts...)
}

// Popover creates popover attributes for an element.
func Popover(opts ...attr.Node) attr.Node {
	return attr.Cons(attr.Name("data-bs-toggle").Raw("popover"), opts...)
}
