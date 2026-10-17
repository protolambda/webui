package bs

import (
	"context"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/group/div"
)

// Modal represents a Bootstrap modal component, opened and closed by
// Bootstrap's script.
//
// A Modal needs Bootstrap's script (the theme package's opt-in Bootstrap
// JS asset): without it the modal never shows. Prefer [Dialog], which
// works without script.
//
// The markup follows Bootstrap 5.3: div.modal.fade#ID tabindex="-1"
// aria-hidden="true" aria-labelledby="ID-title", holding .modal-dialog >
// .modal-content > .modal-header (the title heading with id ID-title and a
// close button), .modal-body and .modal-footer. The aria-labelledby is
// rendered only when Title is set: without a Title, name the modal with an
// aria-label in Attrs. Attrs are added to the div.modal; they may replace
// the tabindex, aria-hidden, aria-labelledby and (with Static)
// data-bs-keyboard defaults, but must not set an id or (with Static)
// data-bs-backdrop.
type Modal struct {
	ID string // required id: a token, see ValidID
	// Title is the .modal-title content, the modal's accessible name. Nil
	// leaves out the title and aria-labelledby; then set an aria-label in
	// Attrs, since Bootstrap's script gives the open modal role="dialog",
	// which needs a name.
	Title      elem.Node
	TitleLevel HeadingLevel // heading level of the title; zero means h2
	CloseLabel string       // accessible name of the close button; empty means "Close"
	Body       elem.Node    // .modal-body content; nil leaves out the body
	Footer     elem.Node    // .modal-footer content (typically buttons); nil leaves out the footer
	Size       ModalSize    // dialog size; zero means Bootstrap's default width
	Centered   bool         // vertically centered
	Scrollable bool         // scroll the body instead of the page
	// Static gives the modal a static backdrop (data-bs-backdrop="static"
	// data-bs-keyboard="false"): clicking beside the modal does not close
	// it, nor does Escape. Set data-bs-keyboard="true" in Attrs to keep
	// Escape working.
	Static bool
	Attrs  attr.Node // additional attributes of the div.modal
}

// Eval builds the modal structure with Bootstrap markup. It fails with
// [ErrInvalidID] for an invalid ID and with [ErrInvalidOption] for an
// unknown Size or TitleLevel.
func (m Modal) Eval(ctx context.Context) (elem.Obj, error) {
	const component = "bs.Modal"
	if err := checkID(component, m.ID); err != nil {
		return elem.Obj{}, err
	}
	dialogClass, err := modalDialogClass(component, m.Size, m.Centered, m.Scrollable)
	if err != nil {
		return elem.Obj{}, err
	}
	titleID := m.ID + "-title"

	own := []attr.Node{
		rawClass("modal fade"),
		attr.ID(m.ID),
		fallbackRaw("tabindex", "-1"),
	}
	if m.Title != nil {
		own = append(own, fallbackValue("aria-labelledby", titleID))
	}
	own = append(own, fallbackRaw("aria-hidden", "true"))
	if m.Static {
		own = append(own,
			attr.Name("data-bs-backdrop").Raw("static"),
			fallbackRaw("data-bs-keyboard", "false"),
		)
	}

	var headerChildren []elem.Node
	if m.Title != nil {
		h, err := heading(component, m.TitleLevel, 2, rawClass("modal-title h5"), attr.ID(titleID))
		if err != nil {
			return elem.Obj{}, err
		}
		headerChildren = append(headerChildren, h(m.Title))
	}
	headerChildren = append(headerChildren, dismissButton("modal", m.CloseLabel))

	contentChildren := []elem.Node{div.Div(rawClass("modal-header"))(headerChildren...)}
	if m.Body != nil {
		contentChildren = append(contentChildren, div.Div(rawClass("modal-body"))(m.Body))
	}
	if m.Footer != nil {
		contentChildren = append(contentChildren, div.Div(rawClass("modal-footer"))(m.Footer))
	}

	modalDialog := div.Div(rawClass(dialogClass))(div.Div(rawClass("modal-content"))(contentChildren...))
	return div.Div(compose(userAttrs(m.Attrs), own...))(modalDialog).Eval(ctx)
}

// ModalTrigger creates a button that opens the [Modal] with the given id
// through Bootstrap's script: <button type="button" data-bs-toggle="modal"
// data-bs-target="#id">. It has no classes; add them among attrs (e.g.
// attr.Class("btn btn-primary")). A type among attrs replaces the default.
// An invalid id fails the render with [ErrInvalidID].
//
// It needs Bootstrap's script; [DialogButton] opens a [Dialog] without it.
func ModalTrigger(targetID string, attrs ...attr.Node) elem.Scope {
	return button.Button(compose(attrs,
		typeButton,
		attr.Name("data-bs-toggle").Raw("modal"),
		idTarget("bs.ModalTrigger", "data-bs-target", targetID),
	))
}
