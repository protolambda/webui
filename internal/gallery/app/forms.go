package app

import (
	"net/http"
	"slices"
	"strings"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/aria"
	"github.com/protolambda/chord/html/form"
	"github.com/protolambda/chord/html/form/input"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/bi"
	"github.com/protolambda/webui/bs"
	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/page"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/web"
)

var visibilities = []string{"private", "team", "public"}

// taskInput is what the task form submits.
type taskInput struct {
	Title, Email, Description, Priority, Visibility string
	Labels                                          []string
	Notify                                          bool
}

// taskProblems are the validation messages of a task form, per field; empty
// means valid. The messages are fixed texts: they never repeat the input.
type taskProblems struct {
	Title, Email, Description, Priority, Visibility, Labels string
}

func (p taskProblems) count() int {
	n := 0
	for _, m := range []string{p.Title, p.Email, p.Description, p.Priority, p.Visibility, p.Labels} {
		if m != "" {
			n++
		}
	}
	return n
}

// validate checks the input and returns it cleaned, with its problems.
func (in taskInput) validate() (taskInput, taskProblems) {
	var p taskProblems
	var ok bool
	if in.Title, ok = cleanLine(in.Title, maxTitle); !ok {
		p.Title = "Give the task a title of 1 to 80 characters, on one line."
	}
	if in.Email = strings.TrimSpace(in.Email); in.Email != "" && !plausibleEmail(in.Email) {
		p.Email = "Enter an email address such as name@example.com, or leave it empty."
	}
	if in.Description = strings.TrimSpace(in.Description); in.Description != "" {
		if in.Description, ok = cleanText(in.Description, maxDescription); !ok {
			p.Description = "Keep the description under 1000 characters of text."
		}
	}
	if !slices.Contains(priorities, priority(in.Priority)) {
		p.Priority = "Choose a priority."
	}
	if !slices.Contains(visibilities, in.Visibility) {
		p.Visibility = "Choose who can see the task."
	}
	for i, l := range in.Labels {
		if !slices.Contains(labels, l) || slices.Contains(in.Labels[:i], l) {
			p.Labels = "Choose labels from the list."
			break
		}
	}
	return in, p
}

// plausibleEmail reports whether v looks like an email address: one @
// between non-empty parts, a dot in the domain, no spaces, at most 254
// bytes. The server cannot know more without sending mail.
func plausibleEmail(v string) bool {
	local, domain, ok := strings.Cut(v, "@")
	return ok && len(v) <= 254 && local != "" && domain != "" &&
		!strings.ContainsAny(v, " \t\r\n<>") && !strings.Contains(domain, "@") &&
		strings.Contains(strings.Trim(domain, "."), ".")
}

func (a *App) formsDemo(w http.ResponseWriter, r *http.Request) error {
	token, err := a.forms.PageToken(w, r)
	if err != nil {
		return err
	}
	return web.Render(w, r, http.StatusOK, a.formsPage(r, token, taskInput{Visibility: "team"}, taskProblems{}))
}

func (a *App) formsPage(r *http.Request, token string, in taskInput, p taskProblems) elem.Node {
	return a.page(r, view{
		Title: "Forms",
		Icon:  bi.UiChecks,
		Main: elem.Bundle{
			para("The server validates every field. A plain post answers a form with problems with the page again (status 422); with htmx the form swaps in place. A valid task redirects to this page with a notice (Post/Redirect/Get). Browser validation is off (novalidate), so that the server's messages show."),
			bs.Row()(bs.ColLG(8)(taskForm(token, in, p))),
		},
	})
}

// taskForm renders the task form with the values and problems of a
// submission.
func taskForm(token string, in taskInput, p taskProblems) elem.Node {
	var summary elem.Node
	if n := p.count(); n > 0 {
		msg := "The form has a problem. Check the highlighted field."
		if n > 1 {
			msg = "The form has problems. Check the highlighted fields."
		}
		summary = page.ErrorAlert(text.Text(msg))
	}

	priorityOptions := []bs.SelectOption{{Value: "", Label: "Choose a priority", Selected: in.Priority == ""}}
	for _, pr := range priorities {
		priorityOptions = append(priorityOptions, bs.SelectOption{Value: string(pr), Label: capitalize(string(pr)), Selected: in.Priority == string(pr)})
	}

	labelBoxes := make([]elem.Node, 0, len(labels))
	for _, l := range labels {
		labelBoxes = append(labelBoxes, bs.Checkbox{
			ID: "task-label-" + l, Name: "labels", Value: l, Label: text.Text(capitalize(l)),
			Checked: slices.Contains(in.Labels, l), Inline: true, Attrs: invalidIf(p.Labels != ""),
		})
	}
	radios := make([]elem.Node, 0, len(visibilities))
	for _, v := range visibilities {
		radios = append(radios, bs.Radio{
			ID: "task-visibility-" + v, Name: "visibility", Value: v, Label: text.Text(capitalize(v)),
			Checked: in.Visibility == v, Required: true, Inline: true, Attrs: invalidIf(p.Visibility != ""),
		})
	}

	return session.PostForm("/forms", token,
		attr.ID("task-form"), form.Novalidate(),
		hx2.Post("/forms"), hx2.Target("this"), hx2.Swap(hx2.SwapOuterHTML),
	)(
		elem.If(summary != nil, summary),
		bs.Field{
			ID: "task-title", Name: "title", Label: text.Text("Title"), Value: in.Title, Required: true,
			Help: text.Text("One line, at most 80 characters."), Error: errText(p.Title),
			Attrs: input.Maxlength("80"),
		},
		bs.Field{
			ID: "task-email", Name: "email", Type: input.Email, Label: text.Text("Notify email"), Value: in.Email,
			Autocomplete: "email", Placeholder: "name@example.com", Help: text.Text("Optional."), Error: errText(p.Email),
			Before: []elem.Node{bs.InputGroupText()(bi.Envelope)},
		},
		bs.Textarea{
			ID: "task-description", Name: "description", Label: text.Text("Description"), Value: in.Description, Rows: 3,
			Error: errText(p.Description),
		},
		bs.Select{
			ID: "task-priority", Name: "priority", Label: text.Text("Priority"), Options: priorityOptions, Required: true,
			Error: errText(p.Priority),
		},
		group("task-labels", "Labels", p.Labels, labelBoxes...),
		group("task-visibility", "Visible to", p.Visibility, radios...),
		bs.Checkbox{ID: "task-notify", Name: "notify", Value: "yes", Label: text.Text("Notify me when it is done"), Checked: in.Notify, Switch: true},
		div.Div(attr.Class("d-flex gap-2"))(
			// The id lets htmx give the focus back to the button after
			// it swapped the form.
			submit(bs.VariantPrimary, "Save task", attr.ID("task-save")),
			link("/forms", "Start over", attr.Class("btn btn-outline-secondary")),
		),
	)
}

// group renders a fieldset of checks with a legend and a group-level
// validation message, which the fieldset's description names.
func group(id, legend, problem string, checks ...elem.Node) elem.Node {
	attrs := []attr.Node{attr.ID(id), attr.Class("mb-3")}
	var msg elem.Node
	if problem != "" {
		attrs = append(attrs, aria.Describedby(id+"-error"))
		msg = div.Div(attr.ID(id+"-error"), attr.Class("invalid-feedback d-block"))(text.Text(problem))
	}
	return form.Fieldset(attrs...)(
		form.Legend(attr.Class("form-label fs-6"))(text.Text(legend)),
		div.Div()(checks...),
		elem.If(msg != nil, msg),
	)
}

// invalidIf marks a check of a group with a problem as invalid.
func invalidIf(invalid bool) attr.Node {
	if !invalid {
		return nil
	}
	return attr.Bundle{attr.Class("is-invalid"), aria.Invalid("true")}
}

// submitTask handles the task form: a valid task is saved and the browser
// redirected (Post/Redirect/Get); a form with problems is rendered again
// with status 422, as a page or, for htmx, as the form alone.
func (a *App) submitTask(w http.ResponseWriter, r *http.Request) error {
	f, err := a.forms.Read(w, r)
	if err != nil {
		return err
	}
	defer f.Zero()
	in, problems := taskInput{
		Title:       f.String("title"),
		Email:       f.String("email"),
		Description: f.String("description"),
		Priority:    f.String("priority"),
		Visibility:  f.String("visibility"),
		Labels:      f.Strings("labels"),
		Notify:      f.String("notify") == "yes",
	}.validate()
	if problems.count() > 0 {
		token := f.String(session.CSRFField)
		return web.Choose(w, r, http.StatusUnprocessableEntity, taskForm(token, in, problems), a.formsPage(r, token, in, problems))
	}
	if _, err := a.tasks.add(task{Title: in.Title, Priority: priority(in.Priority), Labels: in.Labels}); err != nil {
		return web.Conflict("The task list is full. Delete a task on the htmx page first.", err)
	}
	notices.Redirect(w, r, "/forms", "task-saved")
	return nil
}
