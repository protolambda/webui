package app

import (
	"net/http"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/aria"
	"github.com/protolambda/chord/html/form"
	"github.com/protolambda/chord/html/form/input"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/bi"
	"github.com/protolambda/webui/bs"
	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/page"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/visible"
	"github.com/protolambda/webui/web"
)

const (
	renameDialog = "rename-project"
	resetDialog  = "reset-project"
	aboutDialog  = "about-dialogs"
	renameForm   = "rename-form"
	// renameAlerts is the alerts region at the top of the rename dialog.
	renameAlerts = "rename-alerts"
	// projectID is the section of the dialogs page that shows the project,
	// which a successful htmx rename replaces.
	projectID = "project"
)

const renameProblem = "Give the project a name of 1 to 60 characters, on one line."

// dialogAlerts maps the ids of the htmx forms in dialogs to the selectors
// of the alerts regions at the top of their dialogs.
var dialogAlerts = map[string]string{renameForm: "#" + renameAlerts}

// dialogAlertsTarget is the TargetFor hook of the gallery's error policy:
// the error answers to the htmx forms in dialogs go to the alerts region
// of their dialog, since the page behind an open modal dialog is inert,
// and its alerts region neither seen nor announced. HX-Trigger, the id of
// the form, only picks one of the gallery's own selectors: it decides
// where a message shows, nothing else. Other requests get "", the page's
// alerts region.
func dialogAlertsTarget(r *http.Request) string {
	return dialogAlerts[hx2.ParseRequestHeaders(r.Header).Trigger]
}

func (a *App) dialogs(w http.ResponseWriter, r *http.Request) error {
	token, err := a.forms.PageToken(w, r)
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
			projectSection(a.project.get(), false),
		},
		Dialogs: dialogList(token, name, problem),
	})
}

// projectSection renders the project name and the openers of the dialogs
// that change it. A successful htmx rename replaces it; focus marks its
// heading as the focus target of that answer (page.FocusTarget), as the
// control that had the focus left with the closed dialog.
func projectSection(name string, focus bool) elem.Node {
	title := projectID + "-title"
	return section.Section(attr.ID(projectID), aria.Labelledby(title), attr.Class("mb-3"))(
		section.H2(attr.ID(title), attr.Class("h5"), attr.If(focus, page.FocusTarget()))(text.Text("Project")),
		text.P()(text.Text("Name: "), text.Strong()(visible.Text(name))),
		row(
			bs.DialogButton(renameDialog, attr.Class("btn btn-primary"))(bi.Pencil, text.Text(" Rename the project")),
			bs.DialogButton(resetDialog, attr.Class("btn btn-outline-danger"))(bi.ArrowCounterclockwise, text.Text(" Reset the name")),
		),
	)
}

// dialogList renders the dialogs of the dialogs page, for the page and for
// the htmx answer that closes them (page.DialogsOOB), which renders every
// dialog that the page's openers name.
func dialogList(token, name, problem string) []elem.Node {
	return []elem.Node{
		bs.Dialog{
			ID:    renameDialog,
			Title: text.Text("Rename the project"),
			Body: elem.Bundle{
				// The dialog's own alerts region, rendered with it, so
				// that it is in the page when an error answer arrives.
				div.Div(attr.ID(renameAlerts), aria.Live("polite"))(),
				renameProjectForm(token, name, problem),
			},
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
				session.PostForm("/dialogs/reset", token)(submit(bs.VariantDanger, "Reset")),
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
				para("A form inside a dialog posts like any other form. The reset form is a plain form: its answer is a redirect, and the new page has no open dialog. The rename form posts with htmx. A name with a problem comes back as the form alone, and other errors, such as an expired form, as an alert in the region at the top of the dialog: the page behind an open modal dialog is inert, so its own alerts region would show nothing."),
				para("A successful rename replaces the project section and, out of band, the dialogs region, which closes the dialog, and the alerts region with a notice. The focus moves to the project heading, as the button that had it is gone. Without htmx, the rename form answers with a redirect too, or with the page and an alert."),
			},
			Footer: bs.BtnPrimary(bs.DialogClose(aboutDialog))(text.Text("Close")),
		},
	}
}

// renameProjectForm renders the form of the rename dialog. Its submit
// button sits in the dialog's footer and names the form by its id.
func renameProjectForm(token, name, problem string) elem.Node {
	return session.PostForm("/dialogs/rename", token,
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

// renameProject handles the rename form. A name with a problem gets the
// form again with status 422: alone for htmx, which swaps it in place in
// the open dialog, or in the page. A valid name redirects without htmx;
// with htmx, the answer swaps the project section instead of the form
// (HX-Retarget, HX-Reswap with page.ShowAlerts, so that the notice is
// seen), closes the dialog by rendering the dialogs region again out of
// band (page.DialogsOOB), and shows the notice out of band.
func (a *App) renameProject(w http.ResponseWriter, r *http.Request) error {
	f, err := a.forms.Read(w, r)
	if err != nil {
		return err
	}
	defer f.Zero()
	token := f.String(session.CSRFField)
	name, ok := cleanLine(f.String("name"), maxProjectName)
	if !ok {
		return web.Choose(w, r, http.StatusUnprocessableEntity,
			renameProjectForm(token, name, renameProblem), a.dialogsPage(r, token, name, renameProblem))
	}
	a.project.set(name)
	if !web.IsHTMX(r) {
		notices.Redirect(w, r, "/dialogs", "renamed")
		return nil
	}
	hx2.SetRetarget(w.Header(), "#"+projectID)
	hx2.SetReswap(w.Header(), hx2.SwapOuterHTML+" "+page.ShowAlerts)
	return web.Render(w, r, http.StatusOK, elem.Bundle{
		projectSection(name, true),
		page.DialogsOOB(dialogList(token, name, "")...),
		page.AlertsOOB(page.Notice("", text.Text("Project renamed."))),
	})
}

func (a *App) resetProject(w http.ResponseWriter, r *http.Request) error {
	f, err := a.forms.Read(w, r)
	if err != nil {
		return err
	}
	f.Zero()
	a.project.set(defaultProject)
	notices.Redirect(w, r, "/dialogs", "reset")
	return nil
}
