package bs_test

import (
	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/form"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/bs"
)

// A dialog that works without script, and the button that opens it. Render
// the dialog outside every form, e.g. in the layout's dialogs region.
func ExampleDialog() {
	core.Dump(bs.BtnDanger(bs.DialogOpen("delete-note"))(text.Text("Delete")))
	core.Dump(bs.Dialog{
		ID:    "delete-note",
		Title: text.Text("Delete the note?"),
		Body:  text.Text("This cannot be undone."),
		Footer: elem.Bundle{
			bs.BtnSecondary(bs.DialogClose("delete-note"))(text.Text("Cancel")),
			form.Form(attr.Name("method").Raw("post"), form.Action("/notes/7/delete"))(
				bs.BtnDanger(button.Type(button.TypeSubmit))(text.Text("Delete")),
			),
		},
	})
	// Output:
	// <button class="btn btn-danger" type="button" commandfor="delete-note" command="show-modal" popovertarget="delete-note">Delete</button>
	// <dialog id="delete-note" class="modal" popover aria-labelledby="delete-note-title"><div class="modal-dialog"><div class="modal-content"><div class="modal-header"><h2 class="modal-title h5" id="delete-note-title">Delete the note?</h2><button class="btn-close" type="button" aria-label="Close" commandfor="delete-note" command="close" popovertarget="delete-note" popovertargetaction="hide" autofocus></button></div><div class="modal-body">This cannot be undone.</div><div class="modal-footer"><button class="btn btn-secondary" type="button" commandfor="delete-note" command="close" popovertarget="delete-note" popovertargetaction="hide">Cancel</button><form method="post" action="/notes/7/delete"><button class="btn btn-danger" type="submit">Delete</button></form></div></div></div></dialog>
}

// Size, centering and the title's heading level are options; an unknown
// size fails the render.
func ExampleDialog_options() {
	core.Dump(bs.Dialog{
		ID:         "help",
		Title:      text.Text("Keyboard shortcuts"),
		TitleLevel: 3,
		Size:       bs.ModalLG,
		Centered:   true,
		Scrollable: true,
	})
	core.Dump(bs.Dialog{ID: "help", Title: text.Text("Help"), Size: "huge"})
	core.Dump(bs.Dialog{ID: "two words", Title: text.Text("Help")})
	// Output:
	// <dialog id="help" class="modal" popover aria-labelledby="help-title"><div class="modal-dialog modal-lg modal-dialog-centered modal-dialog-scrollable"><div class="modal-content"><div class="modal-header"><h3 class="modal-title h5" id="help-title">Keyboard shortcuts</h3><button class="btn-close" type="button" aria-label="Close" commandfor="help" command="close" popovertarget="help" popovertargetaction="hide" autofocus></button></div></div></div></dialog>
	// ERROR: render: at [0]: bs.Dialog: invalid option: unknown size
	// ERROR: render: at [0]: bs.Dialog: invalid id: want an ASCII letter followed by ASCII letters, digits, '-' or '_'
}

func ExampleDialogButton() {
	core.Dump(bs.DialogButton("help", attr.Class("btn btn-link"))(text.Text("Help")))
	// Output: <button type="button" commandfor="help" command="show-modal" popovertarget="help" class="btn btn-link">Help</button>
}

// An accordion that works without script: opening one item closes the
// others, because the items share a details name.
func ExampleDetailsAccordion() {
	core.Dump(bs.DetailsAccordion{
		ID: "faq",
		Items: []bs.AccordionItem{
			{Header: text.Text("What is a project?"), Body: text.Text("A group of tasks."), Show: true},
			{Header: text.Text("Can I share a project?"), Body: text.Text("Yes, from its settings.")},
		},
	})
	// Output: <div class="accordion" id="faq"><details class="accordion-item" name="faq" open><summary class="accordion-button">What is a project?</summary><div class="accordion-body">A group of tasks.</div></details><details class="accordion-item" name="faq"><summary class="accordion-button">Can I share a project?</summary><div class="accordion-body">Yes, from its settings.</div></details></div>
}

// A dropdown menu that works without script, with a link and a submit
// button of a form.
func ExampleDetailsDropdown() {
	core.Dump(form.Form(attr.Name("method").Raw("post"), form.Action("/tasks/3"))(
		bs.DetailsDropdown{
			Toggle:  text.Text("Actions"),
			Variant: bs.VariantPrimary,
			Items: []elem.Node{
				bs.DropdownItem(text.Href("/tasks/3/edit"))(text.Text("Edit")),
				bs.DropdownDivider(),
				bs.DropdownButton(button.Type(button.TypeSubmit), button.Name("action"), button.Value("archive"))(text.Text("Archive")),
			},
			MenuAttrs: attr.Class("dropdown-menu-end"),
		},
	))
	// Output: <form method="post" action="/tasks/3"><details class="dropdown"><summary class="btn btn-primary dropdown-toggle">Actions</summary><ul class="dropdown-menu dropdown-menu-end"><li><a class="dropdown-item" href="/tasks/3/edit">Edit</a></li><li><hr class="dropdown-divider"/></li><li><button class="dropdown-item" type="submit" name="action" value="archive">Archive</button></li></ul></details></form>
}

// A typed table: header cells have scope="col", the first column holds row
// headers (and is sorted: aria-sort on its header cell only), and the
// actions column has a visually hidden header.
func ExampleDataTable() {
	core.Dump(bs.DataTable{
		Caption:       text.Text("Projects"),
		CaptionHidden: true,
		Small:         true,
		Hover:         true,
		Columns: []bs.Column{
			{Header: text.Text("Name"), RowHeader: true, HeaderAttrs: attr.KV("aria-sort", "ascending")},
			{Header: text.Text("Tasks"), Attrs: attr.Class("text-end")},
			{Header: text.Text("Actions"), HeaderHidden: true},
		},
		Rows: []bs.TableRow{
			{Cells: []elem.Node{text.Text("Website"), text.Text("12"), text.A(text.Href("/projects/1"))(text.Text("Open"))}},
		},
	})
	// Output: <div class="table-responsive position-relative"><table class="table table-sm table-hover"><caption class="visually-hidden">Projects</caption><thead><tr><th scope="col" aria-sort="ascending">Name</th><th scope="col" class="text-end">Tasks</th><th scope="col"><span class="visually-hidden">Actions</span></th></tr></thead><tbody><tr><th scope="row">Website</th><td class="text-end">12</td><td><a href="/projects/1">Open</a></td></tr></tbody></table></div>
}

// Without rows, the Empty node fills one row across all columns.
func ExampleDataTable_empty() {
	core.Dump(bs.DataTable{
		Caption: text.Text("Tasks"),
		Columns: []bs.Column{{Header: text.Text("Title")}, {Header: text.Text("Due")}},
		Empty:   text.Text("No tasks yet."),
	})
	// Output: <div class="table-responsive position-relative"><table class="table"><caption>Tasks</caption><thead><tr><th scope="col">Title</th><th scope="col">Due</th></tr></thead><tbody><tr><td colspan="2">No tasks yet.</td></tr></tbody></table></div>
}
