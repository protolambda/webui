package app

import (
	"net/http"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/form"
	"github.com/protolambda/chord/html/form/input"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/bi"
	"github.com/protolambda/webui/bs"
	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/page"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/web"
)

const (
	renameDialog = "rename-project"
	resetDialog  = "reset-project"
	aboutDialog  = "about-dialogs"
	renameForm   = "rename-form"
)

const renameProblem = "Give the project a name of 1 to 60 characters, on one line."

func (a *App) dialogs(w http.ResponseWriter, r *http.Request) error {
	token, err := a.formToken(w, r)
	if err != nil {
		return err
	}
	return web.Render(w, r, http.StatusOK, a.dialogsPage(r, token, a.project.get(), ""))
}

// dialogsPage renders the dialogs page, with the value and problem of a
// rename submission.
func (a *App) dialogsPage(r *http.Request, token, name, problem string) elem.Node {
	var alerts []elem.Node
	if problem != "" {
		alerts = append(alerts, page.ErrorAlert(text.Text(problem+" Open the rename dialog to try again.")))
	}
	return a.page(r, view{
		Title:   "Dialogs",
		Icon:    bi.WindowStack,
		Actions: []elem.Node{bs.DialogButton(aboutDialog, attr.Class("btn btn-outline-secondary btn-sm"))(bi.QuestionCircle, text.Text(" How dialogs work"))},
		Alerts:  alerts,
		Main: elem.Bundle{
			para("Dialogs are native dialog elements opened by buttons with invoker commands; browsers without them fall back to popovers. No script runs. Dialogs sit in the dialogs region of the layout, outside every form."),
			text.P()(text.Text("Project: "), text.Strong()(text.Text(a.project.get()))),
			row(
				bs.DialogButton(renameDialog, attr.Class("btn btn-primary"))(bi.Pencil, text.Text(" Rename the project")),
				bs.DialogButton(resetDialog, attr.Class("btn btn-outline-danger"))(bi.ArrowCounterclockwise, text.Text(" Reset the name")),
			),
		},
		Dialogs: []elem.Node{
			bs.Dialog{
				ID:    renameDialog,
				Title: text.Text("Rename the project"),
				Body:  renameProjectForm(token, name, problem),
				Footer: elem.Bundle{
					bs.BtnSecondary(bs.DialogClose(renameDialog))(text.Text("Cancel")),
					submit(bs.VariantPrimary, "Rename", attr.KV("form", renameForm)),
				},
			},
			bs.Dialog{
				ID:       resetDialog,
				Title:    text.Text("Reset the project name?"),
				Centered: true,
				Size:     bs.ModalSM,
				Body:     para("The name goes back to \"" + defaultProject + "\"."),
				Footer: elem.Bundle{
					bs.BtnSecondary(bs.DialogClose(resetDialog))(text.Text("Cancel")),
					postForm("/dialogs/reset", token)(submit(bs.VariantDanger, "Reset")),
				},
			},
			bs.Dialog{
				ID:         aboutDialog,
				Title:      text.Text("How dialogs work"),
				Size:       bs.ModalLG,
				Scrollable: true,
				Body: elem.Bundle{
					para("An opener is a button with commandfor naming the dialog and command=\"show-modal\". The browser shows the dialog as a modal: it keeps the focus inside, makes the page behind it inert, closes it on Escape and returns the focus to the opener."),
					para("Browsers without invoker commands follow the popovertarget attribute of the same buttons instead and show the dialog as a popover. The webui theme then makes the page behind it inert where the browser supports it."),
					para("The close button in the header has autofocus, since a popover moves the focus only to such a control. A dialog is never rendered open."),
					para("A form inside a dialog posts like any other form. The rename form answers htmx with the form alone, so that a problem shows inside the open dialog; without htmx it answers with the page and an alert."),
				},
				Footer: bs.BtnPrimary(bs.DialogClose(aboutDialog))(text.Text("Close")),
			},
		},
	})
}

// renameProjectForm renders the form of the rename dialog. Its submit
// button sits in the dialog's footer and names the form by its id.
func renameProjectForm(token, name, problem string) elem.Node {
	return postForm("/dialogs/rename", token,
		attr.ID(renameForm), form.Novalidate(),
		hx2.Post("/dialogs/rename"), hx2.Target("this"), hx2.Swap(hx2.SwapOuterHTML),
	)(
		bs.Field{
			ID: "project-name", Name: "name", Label: text.Text("Project name"), Value: name, Required: true,
			Help: text.Text("At most 60 characters."), Error: errText(problemIf(problem)), Attrs: input.Maxlength("60"),
		},
	)
}

// problemIf returns the field message for a rename problem.
func problemIf(problem string) string {
	if problem == "" {
		return ""
	}
	return renameProblem
}

func (a *App) renameProject(w http.ResponseWriter, r *http.Request) error {
	f, err := a.readForm(w, r)
	if err != nil {
		return err
	}
	defer f.Zero()
	name, ok := cleanLine(f.String("name"), maxProjectName)
	if !ok {
		token := f.String(session.CSRFField)
		return web.Choose(w, r, http.StatusUnprocessableEntity,
			renameProjectForm(token, name, renameProblem), a.dialogsPage(r, token, name, renameProblem))
	}
	a.project.set(name)
	notices.Redirect(w, r, "/dialogs", "renamed")
	return nil
}

func (a *App) resetProject(w http.ResponseWriter, r *http.Request) error {
	f, err := a.readForm(w, r)
	if err != nil {
		return err
	}
	f.Zero()
	a.project.set(defaultProject)
	notices.Redirect(w, r, "/dialogs", "reset")
	return nil
}
