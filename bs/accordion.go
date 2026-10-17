package bs

import (
	"context"
	"fmt"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/section"
)

// Accordion represents a Bootstrap accordion component.
type Accordion struct {
	ID         string          // required for collapse targeting
	Items      []AccordionItem // accordion items
	Flush      bool            // flush style (no borders)
	AlwaysOpen bool            // allow multiple open
	Attrs      attr.Node
}

// AccordionItem represents a single accordion item.
// A nil Header or Body renders the item's structure without that content.
type AccordionItem struct {
	Header elem.Node // accordion-header content
	Body   elem.Node // accordion-body content
	Show   bool      // initially expanded
}

// Eval builds the accordion structure with proper Bootstrap markup.
func (a Accordion) Eval(ctx context.Context) (elem.Obj, error) {
	accordionClass := "accordion"
	if a.Flush {
		accordionClass += " accordion-flush"
	}

	var accordionAttrs []attr.Node
	if a.Attrs != nil {
		accordionAttrs = append(accordionAttrs, a.Attrs)
	}
	accordionAttrs = append(accordionAttrs, rawClass(accordionClass), attr.ID(a.ID))

	var items []elem.Node
	for i, item := range a.Items {
		itemID := fmt.Sprintf("%s-item-%d", a.ID, i)
		collapseID := fmt.Sprintf("%s-collapse-%d", a.ID, i)

		buttonClass := "accordion-button"
		collapseClass := "accordion-collapse collapse"
		if item.Show {
			collapseClass += " show"
		} else {
			buttonClass += " collapsed"
		}

		header := section.H2(rawClass("accordion-header"))(
			button.Button(
				rawClass(buttonClass),
				button.Type(button.TypeButton),
				attr.Name("data-bs-toggle").Raw("collapse"),
				attr.Data("bs-target", "#"+collapseID),
			)(optional(item.Header)...),
		)

		collapseAttrs := []attr.Node{
			attr.ID(collapseID),
			rawClass(collapseClass),
		}
		if !a.AlwaysOpen {
			collapseAttrs = append(collapseAttrs, attr.Data("bs-parent", "#"+a.ID))
		}

		collapse := div.Div(collapseAttrs...)(
			div.Div(rawClass("accordion-body"))(optional(item.Body)...),
		)

		items = append(items, div.Div(
			rawClass("accordion-item"),
			attr.ID(itemID),
		)(header, collapse))
	}

	return div.Div(accordionAttrs...)(items...).Eval(ctx)
}
