package app_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/protolambda/chord/ct"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/internal/gallery/app"
	"github.com/protolambda/webui/page"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/web"
	"github.com/protolambda/webui/webuitest"
)

// taskCount returns the text of the task count badge of the htmx page.
func taskCount(t devtest.T, c *webuitest.Client) string {
	t.Helper()
	p := c.Page(t, c.Get(t, "/htmx"), http.StatusOK)
	text, err := p.Find(ct.ID("task-count")).Text(t.Context())
	t.Must(be.NoError(err))
	return text
}

// validTask are the edits of a valid task form.
func validTask(title string) url.Values {
	return url.Values{"title": {title}, "priority": {"high"}, "labels": {"docs", "chore"}, "email": {"ada@example.com"}}
}

func TestTaskForm(gt *testing.T) {
	t := devtest.ParallelT(gt)
	g := newGallery(t, app.Options{})
	c := g.signedIn(newAccount("Ada"))
	formsPage := func(t devtest.T) *ct.Subject {
		t.Helper()
		return c.Page(t, c.Get(t, "/forms"), http.StatusOK)
	}
	taskForm := ct.ID("task-form")

	t.Run("valid", func(t devtest.T) {
		p := formsPage(t)
		res := c.Submit(t, p, p.Find(taskForm), nil, validTask("Write the changelog"))
		t.Must(webuitest.Location(res, http.StatusSeeOther, ct.Exact("/forms?notice=task-saved")))
		t.Must(webuitest.NoCookies(res))

		done := c.Page(t, c.Get(t, "/forms?notice=task-saved"), http.StatusOK)
		t.Must(done.Find(ct.ID("alerts")).Find(ct.Role("status")).Texts("Task saved."))
		// The new form is empty again.
		t.Must(done.Find(ct.ID("task-title"), ct.HasAttr("value")).None())

		list := c.Page(t, c.Get(t, "/htmx/tasks?filter=open"), http.StatusOK)
		t.Must(list.Find(ct.ID("task-list")).Find(ct.Role("rowheader", ct.Named("Write the changelog"))).Count(1))
	})

	t.Run("invalid plain", func(t devtest.T) {
		before := taskCount(t, c)
		p := formsPage(t)
		res := c.Submit(t, p, p.Find(taskForm), nil, url.Values{
			"title": {"  "}, "email": {"not-an-address"}, "priority": {"urgent"}, "labels": {"docs", "docs"},
		})
		page := c.Page(t, res, http.StatusUnprocessableEntity)
		form := page.Find(taskForm)
		t.Must(form.Find(ct.Role("alert")).Texts("The form has problems. Check the highlighted fields."))
		for _, id := range []string{"task-title", "task-email", "task-priority"} {
			t.Mustf(page.Find(ct.ID(id)).Matches(ct.Attr("aria-invalid", "true"), ct.Class("is-invalid")), "%s", id)
		}
		t.Must(page.Find(ct.ID("task-title-error")).Texts("Give the task a title of 1 to 80 characters, on one line."))
		t.Must(page.Find(ct.ID("task-labels-error")).Texts("Choose labels from the list."))
		t.Must(page.Find(ct.ID("task-description"), ct.HasAttr("aria-invalid")).None())
		// What the user typed is kept, as text.
		t.Must(page.Find(ct.ID("task-email")).Matches(ct.Attr("value", "not-an-address")))
		t.Must(be.Equal(before, taskCount(t, c)))
	})

	t.Run("invalid htmx", func(t devtest.T) {
		p := formsPage(t)
		res := c.Submit(t, p, p.Find(taskForm), nil, url.Values{"title": {"x\ty"}, "priority": {"low"}}, webuitest.HTMX(), webuitest.HXTarget("task-form"))
		t.Must(res.Header("Vary", ct.Contains("HX-Request")))
		frag := c.Fragment(t, res, http.StatusUnprocessableEntity, "div")
		t.Must(frag.Find(ct.Tag("form")).Count(1))
		t.Must(frag.Find(taskForm).Find(ct.Role("alert")).Texts("The form has a problem. Check the highlighted field."))
		t.Must(frag.Find(ct.ID("task-title")).Matches(ct.Attr("aria-invalid", "true")))
		t.Must(frag.Find(ct.ID("main")).None())
		t.Must(frag.Find(ct.ID("alerts")).None())
		// The fragment carries the token again, for the next attempt.
		t.Must(frag.Find(ct.Attr("name", session.CSRFField)).Matches(ct.Attr("value", testCSRF)))
	})

	t.Run("valid htmx", func(t devtest.T) {
		p := formsPage(t)
		res := c.Submit(t, p, p.Find(taskForm), nil, validTask("Review the notes"), webuitest.HTMX(), webuitest.HXTarget("task-form"))
		t.Must(res.Status(http.StatusOK))
		t.Must(webuitest.HXRedirect(res, ct.Exact("/forms?notice=task-saved")))
	})

	t.Run("missing or forged token", func(t devtest.T) {
		before := taskCount(t, c)
		for name, edit := range map[string][]string{"missing": nil, "forged": {"forged-token"}, "empty": {""}} {
			p := formsPage(t)
			edits := validTask("Forged " + name)
			edits[session.CSRFField] = edit
			res := c.Submit(t, p, p.Find(taskForm), nil, edits)
			page := c.Page(t, res, http.StatusForbidden)
			t.Mustf(page.Find(ct.ID("alerts")).Find(ct.Role("alert")).Texts(web.MessageForged), "%s", name)

			res = c.Submit(t, p, p.Find(taskForm), nil, edits, webuitest.HTMX())
			t.Must(res.Status(http.StatusForbidden))
			t.Must(webuitest.HXRetarget(res, ct.Exact("#alerts")))
		}
		t.Must(be.Equal(before, taskCount(t, c)))
	})

	t.Run("limits", func(t devtest.T) {
		// A field over the form's field limit, and a body over the
		// middleware's body limit.
		res := c.Post(t, "/forms", webuitest.Form(url.Values{"title": {strings.Repeat("x", 5<<10)}}))
		t.Must(res.Status(http.StatusRequestEntityTooLarge))
		res = c.Post(t, "/forms", webuitest.Form(url.Values{"description": {strings.Repeat("x", 300<<10)}}))
		t.Must(res.Status(http.StatusRequestEntityTooLarge))
		res = c.Post(t, "/forms", webuitest.Raw("application/json", []byte(`{"title":"x"}`)))
		t.Must(res.Status(http.StatusUnsupportedMediaType))
	})
}

// TestTaskFormHostile submits inputs that try to become markup: the
// gallery saves or refuses them, and every page that shows them still
// satisfies the contract (no script, no inline style, no htmx attribute).
func TestTaskFormHostile(gt *testing.T) {
	t := devtest.ParallelT(gt)
	g := newGallery(t, app.Options{})
	c := g.signedIn(newAccount("Ada"))
	for _, v := range webuitest.Hostile() {
		p := c.Page(t, c.Get(t, "/forms"), http.StatusOK)
		edits := validTask(v)
		edits["description"] = []string{v}
		res := c.Submit(t, p, p.Find(ct.ID("task-form")), nil, edits)
		switch res.StatusCode() {
		case http.StatusSeeOther:
		case http.StatusUnprocessableEntity:
			c.Page(t, res, http.StatusUnprocessableEntity)
		default:
			t.Fatalf("hostile input answered with status %d", res.StatusCode())
		}
	}
	// The saved ones show as text in the task list.
	c.Page(t, c.Get(t, "/htmx"), http.StatusOK)
	c.Page(t, c.Get(t, "/components/content?page=3"), http.StatusOK)
	t.Must(g.logs.NotLogged(testCSRF))
}

// TestPreAuthForms submits the task form without a session: the form
// token is the pre-authentication token, bound to the browser's cookie.
func TestPreAuthForms(gt *testing.T) {
	t := devtest.ParallelT(gt)
	g := newGallery(t, app.Options{})
	b := webuitest.NewBrowser(t, g, webuitest.BrowserOptions{})

	res := b.Get(t, "/forms")
	t.Must(webuitest.Cookie(res, "__Host-preauth", webuitest.CookieWant{
		HostPrefix: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, Path: "/",
	}))
	p := res.HTML()
	t.Must(contract("")(p))
	res = b.Submit(t, p, p.Find(ct.ID("task-form")), nil, validTask("From a browser"))
	t.Must(webuitest.Location(res, http.StatusSeeOther, ct.Exact("/forms?notice=task-saved")))

	// The token is worthless without its cookie: another browser, or the
	// same token after the cookie is gone, is refused.
	token := webuitest.CSRFToken(t, p)
	other := b.Fork()
	res = other.Post(t, "/forms", webuitest.Form(url.Values{session.CSRFField: {token}, "title": {"Stolen"}, "priority": {"low"}, "visibility": {"team"}}))
	t.Must(res.Status(http.StatusForbidden))
	t.Must(webuitest.NoCookies(res))

	t.Run("client", func(t devtest.T) {
		// A Client keeps no cookies: it sends the cookie of the page back
		// with the form, and the form's token goes with that cookie only.
		c := g.anonymous()
		res := c.Get(t, "/forms")
		p := c.Page(t, res, http.StatusOK)
		form := p.Find(ct.ID("task-form"))
		t.Must(webuitest.Location(c.Submit(t, p, form, nil, validTask("From a client"), webuitest.SendCookies(res)),
			http.StatusSeeOther, ct.Exact("/forms?notice=task-saved")))
		t.Must(c.Submit(t, p, form, nil, validTask("Without the cookie")).Status(http.StatusForbidden))
		next := c.Get(t, "/forms")
		t.Must(be.NotEqual(webuitest.CookieValue(t, res, "__Host-preauth"), webuitest.CookieValue(t, next, "__Host-preauth")))
		t.Must(c.Submit(t, p, form, nil, validTask("With another cookie"), webuitest.SendCookies(next)).Status(http.StatusForbidden))
		t.Must(g.logs.NotLogged(webuitest.CookieValue(t, res, "__Host-preauth")))
	})
}

// anyElement matches every element.
var anyElement = ct.Custom("any element", func(ct.Node) (bool, error) { return true, nil })

func TestDialogForms(gt *testing.T) {
	t := devtest.ParallelT(gt)
	g := newGallery(t, app.Options{})
	c := g.signedIn(newAccount("Ada"))
	dialogs := func(t devtest.T) *ct.Subject {
		t.Helper()
		return c.Page(t, c.Get(t, "/dialogs"), http.StatusOK)
	}
	rename := func(p *ct.Subject) (*ct.Selection, *ct.Selection) {
		// The submit button sits in the dialog's footer, outside the
		// form, and names the form with its form attribute.
		dialog := p.Find(ct.ID("rename-project"))
		return dialog.Find(ct.Tag("form")), dialog.Find(ct.Role("button", ct.Named("Rename")))
	}

	t.Run("dialogs", func(t devtest.T) {
		p := dialogs(t)
		t.Must(p.Find(ct.ID("dialogs")).Find(ct.Tag("dialog")).Count(3))
		t.Must(p.Find(ct.Tag("dialog"), ct.HasAttr("open")).None())
		t.Must(p.Find(ct.Role("button", ct.Named("Rename the project"))).Matches(
			ct.Attr("commandfor", "rename-project"), ct.Attr("command", "show-modal"), ct.Attr("popovertarget", "rename-project"),
		))
		t.Must(p.Find(ct.ID("page-header")).Find(ct.Role("button", ct.Named("How dialogs work"))).Count(1))
	})

	t.Run("invalid plain", func(t devtest.T) {
		p := dialogs(t)
		form, button := rename(p)
		page := c.Page(t, c.Submit(t, p, form, button, url.Values{"name": {""}}), http.StatusUnprocessableEntity)
		t.Must(page.Find(ct.ID("alerts")).Find(ct.Role("alert")).Texts(
			"Give the project a name of 1 to 60 characters, on one line. Open the rename dialog to try again."))
		t.Must(page.Find(ct.ID("project-name")).Matches(ct.Attr("aria-invalid", "true")))
	})

	t.Run("invalid htmx", func(t devtest.T) {
		p := dialogs(t)
		form, button := rename(p)
		res := c.Submit(t, p, form, button, url.Values{"name": {strings.Repeat("n", 61)}}, webuitest.HTMX(), webuitest.HXTarget("rename-form"))
		frag := c.Fragment(t, res, http.StatusUnprocessableEntity, "div")
		t.Must(frag.Find(ct.ID("rename-form")).Count(1))
		t.Must(frag.Find(ct.ID("project-name-error")).Texts("Give the project a name of 1 to 60 characters, on one line."))
	})

	t.Run("dialog alerts", func(t devtest.T) {
		// The rename dialog has an alerts region of its own, first in its
		// body, rendered empty with the page.
		p := dialogs(t)
		body := p.Find(ct.ID("rename-project")).Find(ct.Class("modal-body"))
		t.Must(body.Find(ct.Tag("div")).First().Matches(ct.ID("rename-alerts"), ct.Attr("aria-live", "polite")))
		t.Must(p.Find(ct.ID("rename-alerts")).Texts(""))
		t.Must(p.Find(ct.ID("rename-alerts")).Find(anyElement).None())

		// A forged or missing token on the htmx rename gets the policy's
		// alert in that region: the page behind the open dialog is inert.
		for name, token := range map[string][]string{"forged": {"forged-token"}, "missing": nil} {
			p := dialogs(t)
			form, button := rename(p)
			res := c.SubmitHTMX(t, p, form, button, url.Values{"name": {"Forged"}, session.CSRFField: token})
			t.Mustf(res.Status(http.StatusForbidden), "%s", name)
			t.Mustf(webuitest.HXRetarget(res, ct.Exact("#rename-alerts")), "%s", name)
			t.Must(res.Header("HX-Reswap", ct.Exact(web.ErrorReswap)))
			t.Must(c.Fragment(t, res, http.StatusForbidden, "div").Find(ct.Class("alert-danger")).Texts(web.MessageForged))
		}
		t.Must(dialogs(t).Find(ct.ID("project")).Find(ct.Tag("strong")).Texts("Website relaunch"))

		// The same error on a request of the page goes to the page's
		// alerts region, also with the dialog form's target but another
		// trigger.
		res := c.Post(t, "/dialogs/rename", webuitest.Form(url.Values{session.CSRFField: {"forged-token"}}),
			webuitest.HTMX(), webuitest.HXTarget("rename-form"), webuitest.Header("HX-Trigger", "elsewhere"))
		t.Must(webuitest.HXRetarget(res, ct.Exact("#alerts")))
	})

	t.Run("rename with htmx", func(t devtest.T) {
		p := dialogs(t)
		form, button := rename(p)
		res := c.SubmitHTMX(t, p, form, button, url.Values{"name": {"Autumn plans"}})
		t.Must(res.Status(http.StatusOK))
		// The answer replaces the project section, not the form, and
		// scrolls to the notice.
		t.Must(webuitest.HXRetarget(res, ct.Exact("#project")))
		t.Must(res.Header("HX-Reswap", ct.Exact("outerHTML "+page.ShowAlerts)))
		frag := c.Fragment(t, res, http.StatusOK, "div")
		section := frag.Find(ct.ID("project"), ct.HasAttr("hx-swap-oob").Not())
		t.Must(section.Find(ct.Tag("strong")).Texts("Autumn plans"))
		// The focus goes to the section's heading: the button that had it
		// left with the dialog.
		t.Must(frag.Find(ct.HasAttr("autofocus"), ct.HasAttr("tabindex")).Matches(ct.ID("project-title"), ct.Attr("tabindex", "-1")))
		// The dialogs region, out of band, closes the open dialog: it
		// holds every dialog of the page again, closed, with the new name
		// and an empty alerts region.
		region := frag.Find(ct.ID("dialogs"))
		t.Must(region.Matches(ct.Attr("hx-swap-oob", "innerHTML")))
		t.Must(region.Find(ct.Tag("dialog")).Count(3))
		t.Must(region.Find(ct.Tag("dialog"), ct.HasAttr("open")).None())
		t.Must(region.Find(ct.ID("project-name")).Matches(ct.Attr("value", "Autumn plans")))
		t.Must(region.Find(ct.ID("rename-alerts")).Find(anyElement).None())
		t.Must(frag.Find(ct.ID("alerts")).Matches(ct.Attr("hx-swap-oob", "innerHTML")))
		t.Must(frag.Find(ct.ID("alerts")).Find(ct.Class("alert")).Texts("Project renamed."))
		// Every opener on the page still names a dialog of the answer.
		for _, id := range []string{"rename-project", "reset-project", "about-dialogs"} {
			t.Mustf(p.Find(ct.Attr("commandfor", id), ct.Attr("command", "show-modal")).Count(1), "%s", id)
			t.Mustf(region.Find(ct.ID(id)).Count(1), "%s", id)
		}
		t.Must(dialogs(t).Find(ct.ID("project")).Find(ct.Tag("strong")).Texts("Autumn plans"))
	})

	t.Run("rename and reset", func(t devtest.T) {
		p := dialogs(t)
		form, button := rename(p)
		res := c.Submit(t, p, form, button, url.Values{"name": {"<b>Spring</b> cleaning"}})
		t.Must(webuitest.Location(res, http.StatusSeeOther, ct.Exact("/dialogs?notice=renamed")))
		p = c.Page(t, c.Get(t, "/dialogs?notice=renamed"), http.StatusOK)
		t.Must(p.Find(ct.ID("alerts")).Find(ct.Role("status")).Texts("Project renamed."))
		t.Must(p.Find(ct.ID("project")).Find(ct.Tag("strong")).Texts("<b>Spring</b> cleaning"))
		t.Must(p.Find(ct.Tag("b")).None())

		reset := p.Find(ct.ID("reset-project"))
		res = c.Submit(t, p, reset.Find(ct.Tag("form")), reset.Find(ct.Role("button", ct.Named("Reset"))), nil)
		t.Must(webuitest.Location(res, http.StatusSeeOther, ct.Exact("/dialogs?notice=reset")))
		p = dialogs(t)
		t.Must(p.Find(ct.ID("project")).Find(ct.Tag("strong")).Texts("Website relaunch"))
	})
}
