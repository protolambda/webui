package bs

import (
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/table"
)

// Table creates a Bootstrap table.
func Table(attrs ...attr.Node) elem.Scope {
	return table.Table(attr.Cons(rawClass("table"), attrs...))
}

// TableStriped creates a striped Bootstrap table.
func TableStriped(attrs ...attr.Node) elem.Scope {
	return table.Table(attr.Cons(rawClass("table table-striped"), attrs...))
}

// TableBordered creates a bordered Bootstrap table.
func TableBordered(attrs ...attr.Node) elem.Scope {
	return table.Table(attr.Cons(rawClass("table table-bordered"), attrs...))
}

// TableHover creates a hover Bootstrap table.
func TableHover(attrs ...attr.Node) elem.Scope {
	return table.Table(attr.Cons(rawClass("table table-hover"), attrs...))
}

// TableResponsive creates a responsive table wrapper.
func TableResponsive(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("table-responsive"), attrs...))
}
