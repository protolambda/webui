package bs

import (
	"context"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/section"
)

// Modal represents a Bootstrap modal component.
type Modal struct {
	ID         string    // required for targeting
	Title      elem.Node // modal-title content
	Body       elem.Node // modal-body content
	Footer     elem.Node // modal-footer content (typically buttons)
	Size       string    // "", "sm", "lg", "xl"
	Centered   bool      // vertically centered
	Scrollable bool      // scrollable body
	Static     bool      // static backdrop
	Attrs      attr.Node // additional attributes
}

// Eval builds the modal structure with proper Bootstrap markup.
func (m Modal) Eval(ctx context.Context) (elem.Obj, error) {
	dialogClass := "modal-dialog"
	if m.Size != "" {
		dialogClass += " modal-" + m.Size
	}
	if m.Centered {
		dialogClass += " modal-dialog-centered"
	}
	if m.Scrollable {
		dialogClass += " modal-dialog-scrollable"
	}

	var modalAttrs []attr.Node
	if m.Attrs != nil {
		modalAttrs = append(modalAttrs, m.Attrs)
	}
	modalAttrs = append(modalAttrs,
		rawClass("modal fade"),
		attr.ID(m.ID),
		attr.Name("tabindex").Raw("-1"),
	)
	if m.Static {
		modalAttrs = append(modalAttrs, attr.Name("data-bs-backdrop").Raw("static"))
	}

	// Header
	var headerChildren []elem.Node
	if m.Title != nil {
		headerChildren = append(headerChildren, section.H1(rawClass("modal-title fs-5"))(m.Title))
	}
	headerChildren = append(headerChildren, closeButton("modal"))

	// Content
	var contentChildren []elem.Node
	contentChildren = append(contentChildren,
		div.Div(rawClass("modal-header"))(headerChildren...),
	)
	if m.Body != nil {
		contentChildren = append(contentChildren, div.Div(rawClass("modal-body"))(m.Body))
	}
	if m.Footer != nil {
		contentChildren = append(contentChildren, div.Div(rawClass("modal-footer"))(m.Footer))
	}

	modalContent := div.Div(rawClass("modal-content"))(contentChildren...)
	modalDialog := div.Div(attr.Class(dialogClass))(modalContent)

	return div.Div(modalAttrs...)(modalDialog).Eval(ctx)
}

// ModalTrigger creates a button that triggers a modal.
func ModalTrigger(targetID string, attrs ...attr.Node) elem.Scope {
	return button.Button(attr.Cons(
		button.Type(button.TypeButton),
		attr.Name("data-bs-toggle").Raw("modal"),
		attr.Data("bs-target", "#"+targetID),
		attr.Bundle(attrs),
	))
}
