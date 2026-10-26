package bs

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/table"
)

// Table creates a Bootstrap table (table.table).
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

// TableResponsive creates a responsive table wrapper
// (div.table-responsive.position-relative), which scrolls a wide table
// horizontally. It is positioned because Bootstrap positions its
// visually-hidden text absolutely (every element but a caption): in a
// scroller that is not positioned, the containing block of such text, a
// hidden column header or hidden text in a cell, is outside the scroller,
// so the overflow does not clip it, and on a narrow screen text at the far
// end of a wide table widens the whole page. Positioned, the scroller is
// the containing block of such content, which then scrolls and is clipped
// with the table.
func TableResponsive(attrs ...attr.Node) elem.Scope {
	return div.Div(attr.Cons(rawClass("table-responsive position-relative"), attrs...))
}

// DataTable is a typed Bootstrap table: a caption, column headers with
// scope="col", and rows of cells, in a horizontally scrolling wrapper
// ([TableResponsive]):
//
//	div.table-responsive.position-relative > table.table > caption, thead, tbody
//
// The wrapper is position-relative because Bootstrap's visually-hidden
// text (a hidden header, or hidden text in a cell; a hidden caption stays
// in the flow) is absolutely positioned. Without a positioned wrapper its
// containing block is outside the scrolling box, so the overflow does not
// clip it, and on a narrow screen text at the far end of a wide table
// widens the whole page.
//
// Every row must have one cell per column; Eval fails with
// [ErrInvalidOption] otherwise, when there are no columns, and when a
// column's Attrs set an id. The table renders the nodes as given: cell text
// is escaped by chord as usual.
type DataTable struct {
	// Caption names the table (its accessible name). Without a caption,
	// give the table a name with aria-labelledby or aria-label in Attrs.
	Caption elem.Node
	// CaptionHidden makes the caption visually hidden, e.g. when a visible
	// heading already introduces the table.
	CaptionHidden bool

	Columns []Column
	Rows    []TableRow
	// Empty, when not nil, is rendered in a single row spanning all
	// columns when there are no rows (e.g. "No items yet").
	Empty elem.Node

	Small    bool // table-sm: half the cell padding
	Hover    bool // table-hover: highlight the row under the pointer
	Striped  bool // table-striped: zebra rows
	Bordered bool // table-bordered: borders on all cells

	Attrs   attr.Node // additional attributes of the table
	Wrapper attr.Node // additional attributes of the wrapping div
}

// Column is a column of a [DataTable].
type Column struct {
	// Header is the content of the column's header cell
	// (th scope="col").
	Header elem.Node
	// HeaderHidden makes the header visually hidden, e.g. for a column of
	// action buttons: screen readers still announce it with each cell.
	HeaderHidden bool
	// RowHeader makes the cells of this column row headers
	// (th scope="row"), such as the name column of a list of items.
	RowHeader bool
	// Attrs are added to the header cell and to every body cell of the
	// column, typically a class such as text-end. They are evaluated once
	// per render, and the result is shared by all those cells, so lazy and
	// single-use attribute nodes work. They must not set an id (Eval fails).
	// A scope among them replaces the default scope of every th of the
	// column (the header cell's, and the row headers' on a RowHeader
	// column). Attributes for the header cell alone, such as aria-sort, go
	// in HeaderAttrs.
	Attrs attr.Node
	// HeaderAttrs are added only to the column's header cell (th
	// scope="col" in thead), after Attrs: for example aria-sort, abbr, an
	// id, or a scope that replaces scope="col". They must not repeat an
	// attribute of Attrs other than class (classes merge).
	HeaderAttrs attr.Node
}

// TableRow is a row of a [DataTable].
type TableRow struct {
	// Cells holds one node per column; a nil cell is empty.
	Cells []elem.Node
	// Attrs are the additional attributes of the tr, e.g. an id that an
	// htmx swap targets.
	Attrs attr.Node
}

// Eval builds the table with Bootstrap markup.
func (t DataTable) Eval(ctx context.Context) (elem.Obj, error) {
	if len(t.Columns) == 0 {
		return elem.Obj{}, fmt.Errorf("bs.DataTable: %w: no columns", ErrInvalidOption)
	}
	for i, r := range t.Rows {
		if len(r.Cells) != len(t.Columns) {
			return elem.Obj{}, fmt.Errorf("bs.DataTable: %w: row %d has %d cells, want %d", ErrInvalidOption, i, len(r.Cells), len(t.Columns))
		}
	}

	class := "table"
	if t.Small {
		class += " table-sm"
	}
	if t.Hover {
		class += " table-hover"
	}
	if t.Striped {
		class += " table-striped"
	}
	if t.Bordered {
		class += " table-bordered"
	}

	var parts []elem.Node
	if t.Caption != nil {
		var captionAttrs []attr.Node
		if t.CaptionHidden {
			captionAttrs = append(captionAttrs, rawClass("visually-hidden"))
		}
		parts = append(parts, table.Caption(captionAttrs...)(t.Caption))
	}

	// Evaluate each column's attributes once, here: the evaluated objects
	// are shared by the header cell and every body cell of the column.
	colAttrs := make([][]attr.Node, len(t.Columns))
	for i, c := range t.Columns {
		objs, err := flatten(ctx, userAttrs(c.Attrs), nil)
		if err != nil {
			return elem.Obj{}, fmt.Errorf("bs.DataTable: column %d: %w", i, err)
		}
		if sets(objs, "id") {
			return elem.Obj{}, fmt.Errorf("bs.DataTable: %w: column %d attributes set an id, which every cell would repeat", ErrInvalidOption, i)
		}
		user := make([]attr.Node, len(objs))
		for k, o := range objs {
			user[k] = o
		}
		colAttrs[i] = user
	}

	headers := make([]elem.Node, 0, len(t.Columns))
	for i, c := range t.Columns {
		content := optional(c.Header)
		if c.HeaderHidden && c.Header != nil {
			content = []elem.Node{visuallyHidden(c.Header)}
		}
		user := slices.Concat(colAttrs[i], userAttrs(c.HeaderAttrs))
		headers = append(headers, table.TH(compose(user, fallbackRaw("scope", "col")))(content...))
	}
	parts = append(parts, table.Thead()(table.TR()(headers...)))

	rows := make([]elem.Node, 0, len(t.Rows)+1)
	for _, r := range t.Rows {
		cells := make([]elem.Node, 0, len(r.Cells))
		for j, cell := range r.Cells {
			if t.Columns[j].RowHeader {
				cells = append(cells, table.TH(compose(colAttrs[j], fallbackRaw("scope", "row")))(optional(cell)...))
			} else {
				cells = append(cells, table.TD(colAttrs[j]...)(optional(cell)...))
			}
		}
		rows = append(rows, table.TR(userAttrs(r.Attrs)...)(cells...))
	}
	if len(t.Rows) == 0 && t.Empty != nil {
		rows = append(rows, table.TR()(table.TD(attr.Name("colspan").Raw(strconv.Itoa(len(t.Columns))))(t.Empty)))
	}
	parts = append(parts, table.Tbody()(rows...))

	tableAttrs := append([]attr.Node{rawClass(class)}, userAttrs(t.Attrs)...)
	return TableResponsive(userAttrs(t.Wrapper)...)(table.Table(tableAttrs...)(parts...)).Eval(ctx)
}
