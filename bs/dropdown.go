package bs

import (
	"context"
	"fmt"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/group/list"
	"github.com/protolambda/chord/html/interactive"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"
)

// Dropdown represents a Bootstrap dropdown component, opened by Bootstrap's
// script.
//
// A Dropdown needs Bootstrap's script with Popper (the theme package's
// opt-in Bootstrap JS asset, a bundle that includes Popper): without it the
// menu never opens. Prefer [DetailsDropdown], which takes the same items
// and works without script.
//
// It renders div.dropdown > button.btn.btn-{variant}.dropdown-toggle
// (type="button" data-bs-toggle="dropdown" aria-expanded="false") +
// ul.dropdown-menu with the items. Every field is optional: a nil Toggle
// renders the toggle button without content, which then needs an
// aria-label in ToggleAttrs for an accessible name (unlike
// [DetailsDropdown], Dropdown does not check this).
type Dropdown struct {
	Toggle      elem.Node   // toggle button content, its accessible name
	Items       []elem.Node // menu items: DropdownItem, DropdownButton, DropdownDivider, DropdownHeader
	Variant     Variant     // toggle button color; zero means VariantSecondary
	ToggleAttrs attr.Node   // additional attributes of the toggle button
	MenuAttrs   attr.Node   // additional attributes of the ul.dropdown-menu, e.g. attr.Class("dropdown-menu-end")
	Attrs       attr.Node   // additional attributes of the div.dropdown
}

// Eval builds the dropdown structure with Bootstrap markup. An unknown
// Variant fails with [ErrInvalidOption].
func (d Dropdown) Eval(ctx context.Context) (elem.Obj, error) {
	toggleClass, err := dropdownToggleClass("bs.Dropdown", d.Variant)
	if err != nil {
		return elem.Obj{}, err
	}
	toggleBtn := button.Button(compose(userAttrs(d.ToggleAttrs),
		rawClass(toggleClass),
		typeButton,
		attr.Name("data-bs-toggle").Raw("dropdown"),
		fallbackRaw("aria-expanded", "false"),
	))(optional(d.Toggle)...)

	menu := list.UL(append([]attr.Node{rawClass("dropdown-menu")}, userAttrs(d.MenuAttrs)...)...)(d.Items...)

	dropdownAttrs := append([]attr.Node{rawClass("dropdown")}, userAttrs(d.Attrs)...)
	return div.Div(dropdownAttrs...)(toggleBtn, menu).Eval(ctx)
}

// dropdownToggleClass returns the class list of a dropdown toggle.
func dropdownToggleClass(component string, v Variant) (string, error) {
	if v == "" {
		v = VariantSecondary
	}
	if !v.Valid() {
		return "", fmt.Errorf("%s: %w: unknown variant", component, ErrInvalidOption)
	}
	return "btn btn-" + string(v) + " dropdown-toggle", nil
}

// DetailsDropdown is a dropdown menu that works without script: a details
// element whose summary is the toggle button, which the browser opens and
// closes.
//
// It renders:
//
//	<details class="dropdown">
//	  <summary class="btn btn-secondary dropdown-toggle">Toggle</summary>
//	  <ul class="dropdown-menu">items</ul>
//	</details>
//
// The items are the same as those of a [Dropdown]: links ([DropdownItem]),
// buttons ([DropdownButton], e.g. submit buttons of a form or buttons with
// htmx requests), dividers and headers. The menu closes when the toggle is
// clicked again; unlike Bootstrap's script, a click elsewhere or Escape
// does not close it, and arrow keys do not move between the items (Tab
// does). The dropdown needs the webui theme CSS, which shows the menu of an
// open details.dropdown.
//
// The menu opens below the toggle, aligned to its start, or to its end with
// the dropdown-menu-end class in MenuAttrs. Unlike Bootstrap's script (with
// Popper), nothing shifts or flips it to stay in the window: a menu aligned
// to the end of a toggle near the left edge of the window, or to the start
// of one near the right edge, reaches past that edge when it is wider than
// the room there, as it is likely to be on a phone. Choose the alignment by
// where the toggle sits (end-aligned at the end of a toolbar or navbar,
// start-aligned elsewhere), and keep the items short.
type DetailsDropdown struct {
	// Toggle is the content of the summary, its accessible name. Without
	// it, ToggleAttrs must give an aria-label.
	Toggle      elem.Node
	Items       []elem.Node // menu items: DropdownItem, DropdownButton, DropdownDivider, DropdownHeader
	Variant     Variant     // toggle button color; zero means VariantSecondary
	Open        bool        // render the menu open
	ToggleAttrs attr.Node   // additional attributes of the summary
	MenuAttrs   attr.Node   // additional attributes of the ul.dropdown-menu, e.g. attr.Class("dropdown-menu-end")
	Attrs       attr.Node   // additional attributes of the details element
}

// Eval builds the dropdown. It fails with [ErrInvalidOption] for an
// unknown Variant, and with [ErrNoName] when there is neither a Toggle nor
// an aria-label or aria-labelledby in ToggleAttrs.
func (d DetailsDropdown) Eval(ctx context.Context) (elem.Obj, error) {
	const component = "bs.DetailsDropdown"
	toggleClass, err := dropdownToggleClass(component, d.Variant)
	if err != nil {
		return elem.Obj{}, err
	}
	toggleAttrs := userAttrs(d.ToggleAttrs)
	if d.Toggle == nil {
		objs, err := flatten(ctx, toggleAttrs, nil)
		if err != nil {
			return elem.Obj{}, fmt.Errorf("%s: %w", component, err)
		}
		if !sets(objs, "aria-label") && !sets(objs, "aria-labelledby") {
			return elem.Obj{}, fmt.Errorf("%s: %w: set a Toggle, or an aria-label in ToggleAttrs", component, ErrNoName)
		}
		toggleAttrs = make([]attr.Node, len(objs))
		for i, o := range objs {
			toggleAttrs[i] = o
		}
	}

	summary := interactive.Summary(append([]attr.Node{rawClass(toggleClass)}, toggleAttrs...)...)(optional(d.Toggle)...)
	menu := list.UL(append([]attr.Node{rawClass("dropdown-menu")}, userAttrs(d.MenuAttrs)...)...)(d.Items...)

	detailsAttrs := []attr.Node{rawClass("dropdown")}
	if d.Open {
		detailsAttrs = append(detailsAttrs, interactive.Open())
	}
	detailsAttrs = append(detailsAttrs, userAttrs(d.Attrs)...)
	return interactive.Details(detailsAttrs...)(summary, menu).Eval(ctx)
}

// DropdownItem creates a Bootstrap dropdown link: li > a.dropdown-item. The
// attrs go on the a; give it the href.
func DropdownItem(attrs ...attr.Node) elem.Scope {
	return func(children ...elem.Node) elem.Node {
		return list.LI()(
			text.A(attr.Cons(rawClass("dropdown-item"), attrs...))(children...),
		)
	}
}

// DropdownItemActive creates the dropdown link of the current page or
// selection: li > a.dropdown-item.active with aria-current="true" (unless
// attrs set aria-current).
func DropdownItemActive(attrs ...attr.Node) elem.Scope {
	return func(children ...elem.Node) elem.Node {
		return list.LI()(
			text.A(compose(attrs, rawClass("dropdown-item active"), fallbackRaw("aria-current", "true")))(children...),
		)
	}
}

// DropdownButton creates a Bootstrap dropdown button: li >
// button.dropdown-item with type="button" (unless attrs set a type), for an
// action rather than navigation: a submit button of a form (with
// button.Type(button.TypeSubmit), and formaction for another action), or a
// button with htmx request attributes.
func DropdownButton(attrs ...attr.Node) elem.Scope {
	return func(children ...elem.Node) elem.Node {
		return list.LI()(btn(rawClass("dropdown-item"), attrs)(children...))
	}
}

// DropdownDivider creates a Bootstrap dropdown divider: li > hr.dropdown-divider.
func DropdownDivider() elem.Node {
	return list.LI()(text.HR(rawClass("dropdown-divider")))
}

// DropdownHeader creates a Bootstrap dropdown header: li > h6.dropdown-header.
func DropdownHeader(attrs ...attr.Node) elem.Scope {
	return func(children ...elem.Node) elem.Node {
		return list.LI()(
			section.H6(attr.Cons(rawClass("dropdown-header"), attrs...))(children...),
		)
	}
}
