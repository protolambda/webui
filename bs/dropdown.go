package bs

import (
	"context"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/group/list"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"
)

// Dropdown represents a Bootstrap dropdown component.
// A nil Toggle renders the toggle button without content, and a nil Attrs
// adds no attributes.
type Dropdown struct {
	Toggle elem.Node   // toggle button content
	Items  []elem.Node // dropdown items
	Attrs  attr.Node
}

// Eval builds the dropdown structure with proper Bootstrap markup.
func (d Dropdown) Eval(ctx context.Context) (elem.Obj, error) {
	var dropdownAttrs []attr.Node
	if d.Attrs != nil {
		dropdownAttrs = append(dropdownAttrs, d.Attrs)
	}
	dropdownAttrs = append(dropdownAttrs, rawClass("dropdown"))

	toggleBtn := button.Button(
		rawClass("btn btn-secondary dropdown-toggle"),
		button.Type(button.TypeButton),
		attr.Name("data-bs-toggle").Raw("dropdown"),
	)(optional(d.Toggle)...)

	menu := list.UL(rawClass("dropdown-menu"))(d.Items...)

	return div.Div(dropdownAttrs...)(toggleBtn, menu).Eval(ctx)
}

// DropdownItem creates a Bootstrap dropdown item.
func DropdownItem(attrs ...attr.Node) elem.Scope {
	return func(children ...elem.Node) elem.Node {
		return list.LI()(
			text.A(attr.Cons(rawClass("dropdown-item"), attrs...))(children...),
		)
	}
}

// DropdownDivider creates a Bootstrap dropdown divider.
func DropdownDivider() elem.Node {
	return list.LI()(text.HR(rawClass("dropdown-divider")))
}

// DropdownHeader creates a Bootstrap dropdown header.
func DropdownHeader(attrs ...attr.Node) elem.Scope {
	return func(children ...elem.Node) elem.Node {
		return list.LI()(
			section.H6(attr.Cons(rawClass("dropdown-header"), attrs...))(children...),
		)
	}
}
