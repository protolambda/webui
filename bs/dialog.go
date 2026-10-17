package bs

import (
	"context"
	"fmt"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/interactive"
)

// Dialog is a modal dialog that works without script: a native dialog
// element with the look of a Bootstrap modal, opened and closed by buttons
// with invoker commands.
//
// It renders:
//
//	<dialog id="ID" class="modal" popover aria-labelledby="ID-title">
//	  <div class="modal-dialog [modal-lg] [modal-dialog-centered] [modal-dialog-scrollable]">
//	    <div class="modal-content">
//	      <div class="modal-header">
//	        <h2 class="modal-title h5" id="ID-title">Title</h2>
//	        <button class="btn-close" type="button" aria-label="Close" commandfor="ID" command="close"
//	          popovertarget="ID" popovertargetaction="hide" autofocus></button>
//	      </div>
//	      <div class="modal-body">Body</div>
//	      <div class="modal-footer">Footer</div>
//	    </div>
//	  </div>
//	</dialog>
//
// Open it with a button carrying [DialogOpen] (or a [DialogButton]), and
// close it with the header's close button, Escape, or a button carrying
// [DialogClose]. In browsers with invoker commands (commandfor and
// command), the opener shows a real modal dialog: the browser keeps the
// focus inside, makes the page behind it inert, closes it on Escape and
// returns the focus to the opener. Older browsers ignore commandfor and
// follow popovertarget instead, which shows the dialog as a popover: it
// opens and closes the same way, but is not modal for the keyboard. The
// close button has autofocus because a popover moves the focus only to an
// autofocus descendant.
//
// Browsers with invoker commands follow commandfor and ignore
// popovertarget. Chromium says so in the console, "popovertarget is ignored
// on elements with commandfor.", whenever such an opener or closer is used.
// The warning is expected: it is the price of the fallback, and browser
// checks that fail on console messages should allow it.
//
// The dialog needs the webui theme CSS: Bootstrap hides .modal elements
// (display: none), and the theme shows an open dialog.modal and resets the
// browser's own dialog box. The dialog is never rendered open, which would
// show it without modality (and make the opener fail).
//
// Place dialogs outside every form, e.g. in the dialogs region of the page
// layout: forms do not nest, and a dialog often holds a form of its own.
//
// While the dialog is open, the page behind it is inert, and an htmx answer
// does not close it. An htmx form in a dialog therefore needs an alerts
// region (a live region) at the top of the dialog's body for its error
// answers (see the TargetFor field of web.Errors), and a successful answer
// closes the dialog by replacing or removing it out of band (see
// page.DialogsOOB and page.FocusTarget), or by redirecting.
type Dialog struct {
	ID         string       // required id: a token, see ValidID
	Title      elem.Node    // .modal-title content, the dialog's accessible name
	TitleLevel HeadingLevel // heading level of the title; zero means h2
	CloseLabel string       // accessible name of the header's close button; empty means "Close"
	Body       elem.Node    // .modal-body content; nil leaves out the body
	Footer     elem.Node    // .modal-footer content (typically buttons); nil leaves out the footer
	Size       ModalSize    // dialog size; zero means Bootstrap's default width
	Centered   bool         // vertically centered
	Scrollable bool         // scroll the body instead of the dialog layer
	// Attrs are added to the dialog element. They may replace the popover
	// and aria-labelledby defaults (e.g. an aria-label instead of a Title),
	// but must not set an id or the open attribute.
	Attrs attr.Node
}

// Eval builds the dialog. It fails with [ErrInvalidID] for an invalid ID,
// with [ErrNoName] when there is neither a Title nor an aria-label or
// aria-labelledby in Attrs, and with [ErrInvalidOption] for an unknown Size
// or TitleLevel, or an open attribute in Attrs.
func (d Dialog) Eval(ctx context.Context) (elem.Obj, error) {
	const component = "bs.Dialog"
	if err := checkID(component, d.ID); err != nil {
		return elem.Obj{}, err
	}
	dialogClass, err := modalDialogClass(component, d.Size, d.Centered, d.Scrollable)
	if err != nil {
		return elem.Obj{}, err
	}
	// Evaluate the caller's attributes once, here, to check them; the
	// evaluated objects are rendered below.
	user, err := flatten(ctx, userAttrs(d.Attrs), nil)
	if err != nil {
		return elem.Obj{}, fmt.Errorf("%s: %w", component, err)
	}
	if sets(user, "open") {
		return elem.Obj{}, fmt.Errorf("%s: %w: a dialog must not be rendered open", component, ErrInvalidOption)
	}
	if d.Title == nil && !sets(user, "aria-label") && !sets(user, "aria-labelledby") {
		return elem.Obj{}, fmt.Errorf("%s: %w: set a Title, or an aria-label in Attrs", component, ErrNoName)
	}
	titleID := d.ID + "-title"

	own := []attr.Node{
		attr.ID(d.ID),
		rawClass("modal"),
		fallbackBool("popover"),
	}
	if d.Title != nil {
		own = append(own, fallbackValue("aria-labelledby", titleID))
	}

	var headerChildren []elem.Node
	if d.Title != nil {
		h, err := heading(component, d.TitleLevel, 2, rawClass("modal-title h5"), attr.ID(titleID))
		if err != nil {
			return elem.Obj{}, err
		}
		headerChildren = append(headerChildren, h(d.Title))
	}
	closeAttrs := []attr.Node{DialogClose(d.ID), attr.Autofocus()}
	if d.CloseLabel != "" {
		closeAttrs = append(closeAttrs, attr.Name("aria-label").Value(d.CloseLabel))
	}
	headerChildren = append(headerChildren, CloseButton(closeAttrs...))

	contentChildren := []elem.Node{div.Div(rawClass("modal-header"))(headerChildren...)}
	if d.Body != nil {
		contentChildren = append(contentChildren, div.Div(rawClass("modal-body"))(d.Body))
	}
	if d.Footer != nil {
		contentChildren = append(contentChildren, div.Div(rawClass("modal-footer"))(d.Footer))
	}

	userNodes := make([]attr.Node, len(user))
	for i, o := range user {
		userNodes[i] = o
	}
	modalDialog := div.Div(rawClass(dialogClass))(div.Div(rawClass("modal-content"))(contentChildren...))
	return interactive.Dialog(compose(userNodes, own...))(modalDialog).Eval(ctx)
}

// DialogOpen returns the attributes of a button that opens the [Dialog]
// with the given id: commandfor="id" command="show-modal"
// popovertarget="id". Put them on a button with type="button", such as
// any Btn* button:
//
//	bs.BtnPrimary(bs.DialogOpen("edit-note"))(text.Text("Edit"))
//
// popovertarget is the fallback for browsers without invoker commands; on
// each use, Chromium logs that it ignores it, as expected (see [Dialog]).
// An invalid id fails the render with [ErrInvalidID].
func DialogOpen(id string) attr.Node {
	if !ValidID(id) {
		return errAttr{fmt.Errorf("bs.DialogOpen: %w", ErrInvalidID)}
	}
	return attr.Bundle{
		attr.Name("commandfor").Value(id),
		attr.Name("command").Raw("show-modal"),
		attr.Name("popovertarget").Value(id),
	}
}

// DialogClose returns the attributes of a button that closes the [Dialog]
// with the given id: commandfor="id" command="close" popovertarget="id"
// popovertargetaction="hide". Put them on a button with type="button"
// inside the dialog, e.g. a footer's cancel button:
//
//	bs.BtnSecondary(bs.DialogClose("edit-note"))(text.Text("Cancel"))
//
// As with [DialogOpen], Chromium logs on each use that it ignores the
// popovertarget fallback, as expected. An invalid id fails the render with
// [ErrInvalidID].
func DialogClose(id string) attr.Node {
	if !ValidID(id) {
		return errAttr{fmt.Errorf("bs.DialogClose: %w", ErrInvalidID)}
	}
	return attr.Bundle{
		attr.Name("commandfor").Value(id),
		attr.Name("command").Raw("close"),
		attr.Name("popovertarget").Value(id),
		attr.Name("popovertargetaction").Raw("hide"),
	}
}

// DialogButton creates a button that opens the [Dialog] with the given
// id: <button type="button" commandfor="id" command="show-modal"
// popovertarget="id">. It has no classes; add them among attrs (e.g.
// attr.Class("btn btn-primary")), or use [DialogOpen] on a Btn* button. A
// type among attrs replaces the default. Chromium logs on each use that it
// ignores the popovertarget fallback, as expected (see [Dialog]). An
// invalid id fails the render with [ErrInvalidID].
func DialogButton(id string, attrs ...attr.Node) elem.Scope {
	return button.Button(compose(attrs, typeButton, DialogOpen(id)))
}
