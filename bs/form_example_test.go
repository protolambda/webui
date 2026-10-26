package bs_test

import (
	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/form"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/form/input"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/bs"
	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/session"
)

func ExampleField() {
	core.Dump(bs.Field{
		ID:           "email",
		Name:         "email",
		Label:        text.Text("Email"),
		Type:         input.Email,
		Autocomplete: "email",
		Required:     true,
		Help:         text.Text("We never share it."),
	})
	// Output: <div class="mb-3"><label class="form-label" for="email">Email</label><input class="form-control" type="email" id="email" name="email" autocomplete="email" required aria-describedby="email-help"/><div class="form-text" id="email-help">We never share it.</div></div>
}

// A field that failed server-side validation keeps the submitted value and
// shows the error, which screen readers read first.
func ExampleField_error() {
	core.Dump(bs.Field{
		ID:    "title",
		Name:  "title",
		Label: text.Text("Title"),
		Value: `Tasks <for> "today"`,
		Help:  text.Text("At most 80 characters."),
		Error: text.Text("The title is too long."),
	})
	// Output: <div class="mb-3"><label class="form-label" for="title">Title</label><input class="form-control is-invalid" type="text" id="title" name="title" value="Tasks &lt;for&gt; &#34;today&#34;" aria-invalid="true" aria-describedby="title-error title-help"/><div class="invalid-feedback" id="title-error">The title is too long.</div><div class="form-text" id="title-help">At most 80 characters.</div></div>
}

// Before and After put the input in an input group.
func ExampleField_inputGroup() {
	core.Dump(bs.Field{
		ID:          "search",
		Name:        "q",
		Label:       text.Text("Search notes"),
		LabelHidden: true,
		Type:        input.Search,
		InputMode:   bs.InputModeSearch,
		After:       []elem.Node{bs.BtnOutlineSecondary()(text.Text("Search"))},
	})
	// Output: <div class="mb-3"><label class="form-label visually-hidden" for="search">Search notes</label><div class="input-group"><input class="form-control" type="search" id="search" name="q" inputmode="search"/><button class="btn btn-outline-secondary" type="button">Search</button></div></div>
}

// A password field never carries a value, neither in Value nor in Attrs.
func ExampleField_password() {
	core.Dump(bs.Field{ID: "password", Name: "password", Label: text.Text("Password"), Type: input.Password, Autocomplete: "current-password"})
	core.Dump(bs.Field{ID: "password", Name: "password", Label: text.Text("Password"), Type: input.Password, Value: "hunter2"})
	core.Dump(bs.Field{ID: "password", Name: "password", Label: text.Text("Password"), Type: input.Password, Attrs: input.Value("hunter2")})
	// Output:
	// <div class="mb-3"><label class="form-label" for="password">Password</label><input class="form-control" type="password" id="password" name="password" autocomplete="current-password"/></div>
	// ERROR: render: at [0]: bs.Field: invalid option: a password field must not carry a value
	// ERROR: render: at [0]: bs.Field: invalid option: a password field must not carry a value
}

// In a row of controls, such as a filter bar, WrapperClass replaces the
// bottom margin (mb-3) of the field wrappers; the labels and their wiring
// stay. Here the wrappers are grid columns, aligned at their bottom edge,
// so that the button lines up with the controls.
func ExampleField_row() {
	core.Dump(form.Form(form.Method(form.MethodGet), form.Action("/tasks"), attr.Class("row g-2 align-items-end mb-3"))(
		bs.Field{ID: "filter-q", Name: "q", Type: input.Search, Label: text.Text("Search"), WrapperClass: "col"},
		bs.Select{ID: "filter-state", Name: "state", Label: text.Text("State"), WrapperClass: "col-auto", Options: []bs.SelectOption{
			{Value: "", Label: "Any"}, {Value: "open", Label: "Open"}, {Value: "done", Label: "Done"},
		}},
		bs.ColAuto()(bs.BtnSecondary(button.Type(button.TypeSubmit))(text.Text("Filter"))),
	))
	// Output: <form method="get" action="/tasks" class="row g-2 align-items-end mb-3"><div class="col"><label class="form-label" for="filter-q">Search</label><input class="form-control" type="search" id="filter-q" name="q"/></div><div class="col-auto"><label class="form-label" for="filter-state">State</label><select class="form-select" id="filter-state" name="state"><option value="">Any</option><option value="open">Open</option><option value="done">Done</option></select></div><div class="col-auto"><button class="btn btn-secondary" type="submit">Filter</button></div></form>
}

// A value that the form shows but does not let the user change renders as
// plain text, read-only, with its label. Without a Name it is not
// submitted.
func ExampleField_plaintext() {
	core.Dump(bs.Field{ID: "task-ref", Label: text.Text("Reference"), Value: "T-1024", Plaintext: true})
	// Output: <div class="mb-3"><label class="form-label" for="task-ref">Reference</label><input class="form-control-plaintext" type="text" id="task-ref" value="T-1024" readonly/></div>
}

func ExampleTextarea() {
	core.Dump(bs.Textarea{
		ID:    "body",
		Name:  "body",
		Label: text.Text("Note"),
		Rows:  4,
		Value: "First line\nsecond line",
	})
	// Output: <div class="mb-3"><label class="form-label" for="body">Note</label><textarea class="form-control" id="body" name="body" rows="4">First line
	// second line</textarea></div>
}

func ExampleSelect() {
	core.Dump(bs.Select{
		ID:    "priority",
		Name:  "priority",
		Label: text.Text("Priority"),
		Options: []bs.SelectOption{
			{Value: "", Label: "Choose..."},
			{Value: "low", Label: "Low"},
			{Value: "high", Label: "High", Selected: true},
		},
		Error: text.Text("Choose a priority."),
	})
	// Output: <div class="mb-3"><label class="form-label" for="priority">Priority</label><select class="form-select is-invalid" id="priority" name="priority" aria-invalid="true" aria-describedby="priority-error"><option value="">Choose...</option><option value="low">Low</option><option value="high" selected>High</option></select><div class="invalid-feedback" id="priority-error">Choose a priority.</div></div>
}

func ExampleCheckbox() {
	core.Dump(bs.Checkbox{ID: "done", Name: "done", Value: "1", Label: text.Text("Done"), Checked: true})
	core.Dump(bs.Checkbox{ID: "notify", Name: "notify", Label: text.Text("Email me"), Switch: true})
	// Output:
	// <div class="form-check"><input class="form-check-input" type="checkbox" id="done" name="done" value="1" checked/><label class="form-check-label" for="done">Done</label></div>
	// <div class="form-check form-switch"><input class="form-check-input" type="checkbox" role="switch" id="notify" name="notify"/><label class="form-check-label" for="notify">Email me</label></div>
}

func ExampleRadio() {
	core.Dump(elem.Bundle{
		bs.Radio{ID: "view-list", Name: "view", Value: "list", Label: text.Text("List"), Checked: true, Inline: true},
		bs.Radio{ID: "view-board", Name: "view", Value: "board", Label: text.Text("Board"), Inline: true},
	})
	// Output: <div class="form-check form-check-inline"><input class="form-check-input" type="radio" id="view-list" name="view" value="list" checked/><label class="form-check-label" for="view-list">List</label></div><div class="form-check form-check-inline"><input class="form-check-input" type="radio" id="view-board" name="view" value="board"/><label class="form-check-label" for="view-board">Board</label></div>
}

// A form that htmx replaces in place with the answer to its submission,
// rendered again with the error. Without htmx it posts like any form (method
// and action), and the server answers with the whole page; the CSRF field
// goes along either way. The field and the submit button have ids that stay
// the same in every rendering, so that htmx gives the focus back to the
// control that had it (see Forms in the package documentation).
func ExampleBtnPrimary_htmxForm() {
	core.Dump(form.Form(form.Method(form.MethodPost), form.Action("/notes"), hx2.Post("/notes"), hx2.Swap(hx2.SwapOuterHTML))(
		session.CSRFInput("dG9rZW4"),
		bs.Field{ID: "note-title", Name: "title", Label: text.Text("Title"), Error: text.Text("Enter a title.")},
		bs.BtnPrimary(button.Type(button.TypeSubmit), attr.ID("note-save"))(text.Text("Save")),
	))
	// Output: <form method="post" action="/notes" hx-post="/notes" hx-swap="outerHTML"><input type="hidden" name="csrf" value="dG9rZW4"/><div class="mb-3"><label class="form-label" for="note-title">Title</label><input class="form-control is-invalid" type="text" id="note-title" name="title" aria-invalid="true" aria-describedby="note-title-error"/><div class="invalid-feedback" id="note-title-error">Enter a title.</div></div><button class="btn btn-primary" type="submit" id="note-save">Save</button></form>
}
