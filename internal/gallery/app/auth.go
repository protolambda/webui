package app

import (
	"crypto/subtle"
	"net/http"
	"slices"
	"strings"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/form"
	"github.com/protolambda/chord/html/form/input"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/bi"
	"github.com/protolambda/webui/bs"
	webform "github.com/protolambda/webui/form"
	"github.com/protolambda/webui/page"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/visible"
	"github.com/protolambda/webui/web"
)

const (
	loginProblem = "The name or password is wrong."
	loginExpired = "The login form expired. Enter your password again."
	noteProblem  = "Write a note of 1 to 280 characters."
)

// signedIn reports whether r carries a live session, for the guard of the
// private pages.
func (a *App) signedIn(r *http.Request) bool {
	_, ok := session.FromContext[Account](r.Context())
	return ok
}

// notesOf returns a copy of the notes of a session; nil once it ended.
func notesOf(s *session.Session[Account]) []string {
	var notes []string
	_ = s.Update(func(acc *Account) { notes = slices.Clone(acc.Notes) })
	return notes
}

func (a *App) loginForm(w http.ResponseWriter, r *http.Request) error {
	next := web.SafeNext(r.URL.Query().Get(web.NextParam), "")
	if a.signedIn(r) {
		web.Redirect(w, r, web.SafeNext(next, "/notes"))
		return nil
	}
	token, err := a.preauth.Issue(w, r)
	if err != nil {
		return err
	}
	return web.Render(w, r, http.StatusOK, a.loginPage(r, token, next, "", ""))
}

// loginPage renders the login form with the name of the last attempt, and
// problem, when not empty, as the alert that says why it failed.
func (a *App) loginPage(r *http.Request, token, next, name, problem string) elem.Node {
	var alerts []elem.Node
	if problem != "" {
		alerts = append(alerts, page.ErrorAlert(text.Text(problem)))
	}
	return a.page(r, view{
		Title:  "Log in",
		Icon:   bi.BoxArrowInRight,
		Alerts: alerts,
		Main: bs.Row()(bs.ColMD(6)(
			para("The login form is protected by a pre-authentication token: a signed cookie and a form token, without any state on the server. A successful login starts a session with a new ID."),
			session.PostForm("/login", token, form.Novalidate())(
				elem.If(next != "", input.Input(input.Type(input.Hidden), input.Name(web.NextParam), input.Value(next))),
				bs.Field{ID: "login-name", Name: "name", Label: text.Text("Name"), Value: name, Autocomplete: "username", Required: true, Attrs: input.Maxlength("40")},
				bs.Field{
					ID: "login-password", Name: "password", Type: input.Password, Label: text.Text("Password"),
					Autocomplete: "current-password", Required: true,
					Help: text.Text("This demo accepts any name with the password \"" + DemoPassword + "\"."),
				},
				submit(bs.VariantPrimary, "Log in"),
			),
		)),
	})
}

// login checks the login form and starts a session. The form token is the
// pre-authentication token: the visitor has no session yet. It reads the
// form with form.Read and checks the token with PreAuth.Verify itself,
// rather than with session.Forms.Read, which would check it against a
// session that the visitor started meanwhile in another tab.
//
// A form whose token fails (it expired, or its cookie is gone) gets the
// login page again with a fresh token, the name and the page to return
// to, never the password, and status 403. That is safe: mw.SameOrigin has
// refused cross-site posts already, and the answer signs no one in.
func (a *App) login(w http.ResponseWriter, r *http.Request) error {
	f, err := webform.Read(w, r, formLimits)
	if err != nil {
		return err
	}
	defer f.Zero()
	next := web.SafeNext(f.String(web.NextParam), "")
	name, nameOK := cleanLine(f.String("name"), maxName)
	if !nameOK {
		name = ""
	}
	again := func(status int, problem string) error {
		token, err := a.preauth.Issue(w, r)
		if err != nil {
			return err
		}
		return web.Render(w, r, status, a.loginPage(r, token, next, name, problem))
	}
	if err := a.preauth.Verify(r, f.String(session.CSRFField)); err != nil {
		return again(http.StatusForbidden, loginExpired)
	}
	password := f.Take("password")
	passwordOK := subtle.ConstantTimeCompare(password, []byte(DemoPassword)) == 1
	clear(password)
	if !nameOK || !passwordOK {
		return again(http.StatusUnprocessableEntity, loginProblem)
	}
	if _, err := a.sessions.New(w, r, Account{Name: name}); err != nil {
		return err
	}
	a.preauth.ClearCookie(w)
	notices.Redirect(w, r, web.SafeNext(next, "/notes"), "signed-in")
	return nil
}

// logout ends the session of the request, the one whose CSRF token the
// form carries, and clears its cookie (session.Store.EndSession, which
// also ends a detached session of a test). Without a session there is
// nothing to end, and nothing to check.
func (a *App) logout(w http.ResponseWriter, r *http.Request) error {
	s, ok := session.FromContext[Account](r.Context())
	if !ok {
		a.sessions.ClearCookie(w)
		web.Redirect(w, r, "/")
		return nil
	}
	f, err := a.forms.Read(w, r)
	if err != nil {
		return err
	}
	f.Zero()
	a.sessions.EndSession(w, s)
	notices.Redirect(w, r, "/", "signed-out")
	return nil
}

func (a *App) notes(w http.ResponseWriter, r *http.Request) error {
	s, ok := session.FromContext[Account](r.Context())
	if !ok {
		return web.Forbidden(web.MessageLogin, nil)
	}
	return web.Render(w, r, http.StatusOK, a.notesPage(r, s, "", ""))
}

func (a *App) notesPage(r *http.Request, s *session.Session[Account], draft, problem string) elem.Node {
	notes := notesOf(s)
	items := make([]elem.Node, 0, len(notes))
	for _, n := range notes {
		items = append(items, bs.ListGroupItem(attr.Class("text-break"))(noteLines(n)...))
	}
	list := para("No notes yet.", attr.Class("text-body-secondary"))
	if len(items) > 0 {
		list = bs.ListGroup(attr.Class("mb-3"))(items...)
	}
	return a.page(r, view{
		Title: "Notes",
		Icon:  bi.Lock,
		Main: elem.Bundle{
			para("A private page behind web.Errors.RequireAuth. The notes live in your session and end with it."),
			a.notesRegion(r),
			list,
			session.PostForm("/notes", s.CSRF(), form.Novalidate())(
				bs.Textarea{ID: "note", Name: "note", Label: text.Text("New note"), Value: draft, Rows: 2, Required: true, Error: errText(problem)},
				submit(bs.VariantPrimary, "Add note"),
			),
		},
	})
}

// noteLines renders a note line by line, each line through visible.Text,
// which would show a line break as an escape.
func noteLines(note string) []elem.Node {
	lines := strings.Split(strings.ReplaceAll(note, "\r\n", "\n"), "\n")
	nodes := make([]elem.Node, 0, len(lines))
	for _, l := range lines {
		nodes = append(nodes, div.Div()(visible.Text(l)))
	}
	return nodes
}

func (a *App) addNote(w http.ResponseWriter, r *http.Request) error {
	s, ok := session.FromContext[Account](r.Context())
	if !ok {
		return web.Forbidden(web.MessageLogin, nil)
	}
	f, err := a.forms.Read(w, r)
	if err != nil {
		return err
	}
	defer f.Zero()
	note, ok := cleanText(f.String("note"), maxNote)
	if !ok {
		return web.Render(w, r, http.StatusUnprocessableEntity, a.notesPage(r, s, note, noteProblem))
	}
	full := false
	if err := s.Update(func(acc *Account) {
		if full = len(acc.Notes) >= maxNotes; !full {
			acc.Notes = append(acc.Notes, note)
		}
	}); err != nil {
		return web.Forbidden(web.MessageLogin, err)
	}
	if full {
		return web.Conflict("This demo keeps 20 notes per session at most.", nil)
	}
	a.noteChanges.notify()
	notices.Redirect(w, r, "/notes", "note-added")
	return nil
}
