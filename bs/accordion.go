package bs

import (
	"context"
	"fmt"
	"strconv"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/interactive"
)

// Accordion represents a Bootstrap accordion component, expanded and
// collapsed by Bootstrap's script.
//
// An Accordion needs Bootstrap's script (the theme package's opt-in
// Bootstrap JS asset): without it the panels stay in their initial state.
// Prefer [DetailsAccordion], which takes the same items and works without
// script.
//
// The markup follows Bootstrap 5.3: div.accordion#ID holding, per item,
// div.accordion-item#ID-item-{i} > h2.accordion-header >
// button.accordion-button (aria-expanded, aria-controls) and
// div.accordion-collapse#ID-collapse-{i} > div.accordion-body. Attrs are
// added to the div.accordion and must not set an id.
type Accordion struct {
	ID          string          // required id: a token, see ValidID
	Items       []AccordionItem // accordion items
	Flush       bool            // flush style (no outer borders)
	AlwaysOpen  bool            // allow several open items
	HeaderLevel HeadingLevel    // heading level of the item headers; zero means h2
	Attrs       attr.Node       // additional attributes of the div.accordion
}

// AccordionItem represents a single item of an [Accordion] or a
// [DetailsAccordion]. A nil Header or Body renders the item's structure
// without that content.
type AccordionItem struct {
	Header elem.Node // header (button or summary) content
	Body   elem.Node // accordion-body content
	Show   bool      // initially expanded
}

// Eval builds the accordion structure with Bootstrap markup. It fails with
// [ErrInvalidID] for an invalid ID and with [ErrInvalidOption] for a
// HeaderLevel out of range.
func (a Accordion) Eval(ctx context.Context) (elem.Obj, error) {
	const component = "bs.Accordion"
	if err := checkID(component, a.ID); err != nil {
		return elem.Obj{}, err
	}
	accordionClass := "accordion"
	if a.Flush {
		accordionClass += " accordion-flush"
	}

	h, err := heading(component, a.HeaderLevel, 2, rawClass("accordion-header"))
	if err != nil {
		return elem.Obj{}, err
	}
	items := make([]elem.Node, 0, len(a.Items))
	for i, item := range a.Items {
		n := strconv.Itoa(i)
		itemID := a.ID + "-item-" + n
		collapseID := a.ID + "-collapse-" + n

		buttonClass := "accordion-button"
		collapseClass := "accordion-collapse collapse"
		expanded := "false"
		if item.Show {
			collapseClass += " show"
			expanded = "true"
		} else {
			buttonClass += " collapsed"
		}

		header := h(
			button.Button(
				rawClass(buttonClass),
				button.Type(button.TypeButton),
				attr.Name("data-bs-toggle").Raw("collapse"),
				attr.Name("data-bs-target").Value("#"+collapseID),
				attr.Name("aria-expanded").Raw(expanded),
				attr.Name("aria-controls").Value(collapseID),
			)(optional(item.Header)...),
		)

		collapseAttrs := []attr.Node{
			attr.ID(collapseID),
			rawClass(collapseClass),
		}
		if !a.AlwaysOpen {
			collapseAttrs = append(collapseAttrs, attr.Name("data-bs-parent").Value("#"+a.ID))
		}

		collapse := div.Div(collapseAttrs...)(
			div.Div(rawClass("accordion-body"))(optional(item.Body)...),
		)

		items = append(items, div.Div(
			rawClass("accordion-item"),
			attr.ID(itemID),
		)(header, collapse))
	}

	accordionAttrs := append([]attr.Node{rawClass(accordionClass), attr.ID(a.ID)}, userAttrs(a.Attrs)...)
	return div.Div(accordionAttrs...)(items...).Eval(ctx)
}

// DetailsAccordion is an accordion that works without script: each item is
// a details element styled as a Bootstrap accordion item, which the browser
// opens and closes.
//
// It renders:
//
//	<div class="accordion [accordion-flush]" id="ID">
//	  <details class="accordion-item" name="ID" [open]>
//	    <summary class="accordion-button">Header</summary>
//	    <div class="accordion-body">Body</div>
//	  </details>
//	</div>
//
// The items share the details name ID, so that opening one closes the
// others (an exclusive group); AlwaysOpen leaves out the name, so that
// items open independently. The open state is the open attribute, which
// the browser keeps current; the summary has no collapsed class. The
// accordion needs the webui theme CSS for the look of the open and closed
// states (the chevron and the open header).
type DetailsAccordion struct {
	ID         string          // required id: a token, see ValidID; also the group name of the items
	Items      []AccordionItem // accordion items; Show renders an item open
	Flush      bool            // flush style (no outer borders)
	AlwaysOpen bool            // let several items be open: no shared name
	Attrs      attr.Node       // additional attributes of the div.accordion; must not set an id
}

// Eval builds the accordion. It fails with [ErrInvalidID] for an invalid
// ID, and with [ErrInvalidOption] when more than one item is shown in an
// exclusive (not AlwaysOpen) accordion, of which browsers would open only
// the first.
func (a DetailsAccordion) Eval(ctx context.Context) (elem.Obj, error) {
	const component = "bs.DetailsAccordion"
	if err := checkID(component, a.ID); err != nil {
		return elem.Obj{}, err
	}
	accordionClass := "accordion"
	if a.Flush {
		accordionClass += " accordion-flush"
	}

	shown := 0
	items := make([]elem.Node, 0, len(a.Items))
	for _, item := range a.Items {
		detailsAttrs := []attr.Node{rawClass("accordion-item")}
		if !a.AlwaysOpen {
			detailsAttrs = append(detailsAttrs, attr.Name("name").Value(a.ID))
		}
		if item.Show {
			shown++
			detailsAttrs = append(detailsAttrs, interactive.Open())
		}
		items = append(items, interactive.Details(detailsAttrs...)(
			interactive.Summary(rawClass("accordion-button"))(optional(item.Header)...),
			div.Div(rawClass("accordion-body"))(optional(item.Body)...),
		))
	}
	if !a.AlwaysOpen && shown > 1 {
		return elem.Obj{}, fmt.Errorf("%s: %w: %d items shown in an exclusive accordion, want at most 1", component, ErrInvalidOption, shown)
	}

	accordionAttrs := append([]attr.Node{rawClass(accordionClass), attr.ID(a.ID)}, userAttrs(a.Attrs)...)
	return div.Div(accordionAttrs...)(items...).Eval(ctx)
}
