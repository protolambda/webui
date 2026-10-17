package app_test

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/protolambda/chord/ct"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/internal/gallery/app"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/webuitest"
)

// fakeClock is a clock that moves only when the test advances it.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// TestHTMXErrors checks that error answers to htmx requests are alerts
// retargeted into the alerts region of the page, with the real status,
// and full error pages in the gallery's layout otherwise.
func TestHTMXErrors(gt *testing.T) {
	t := devtest.ParallelT(gt)
	g := newGallery(t, app.Options{})
	c := g.signedIn(newAccount("Ada"))
	const failed = "This request failed on purpose: the server answered htmx with this alert, retargeted into the alerts region."

	t.Run("htmx", func(t devtest.T) {
		res := c.Post(t, "/htmx/fail", webuitest.Form(nil), webuitest.HTMX(), webuitest.HXTarget("errors"))
		t.Must(webuitest.HXRetarget(res, ct.Exact("#alerts")))
		t.Must(res.Header("HX-Reswap", ct.Exact("innerHTML")))
		t.Must(res.Header("Cache-Control", ct.Exact("no-store")))
		t.Must(res.Header("Vary", ct.Contains("HX-Request")))
		frag := c.Fragment(t, res, http.StatusConflict, "div")
		t.Must(frag.Find(ct.Role("alert")).Texts(failed))
		t.Must(frag.Find(ct.Tag("nav")).None())
	})

	t.Run("plain", func(t devtest.T) {
		page := c.Page(t, c.Post(t, "/htmx/fail", webuitest.Form(nil)), http.StatusConflict)
		t.Must(page.Find(ct.ID("alerts")).Find(ct.Role("alert")).Texts(failed))
		t.Must(page.Find(ct.ID("navbar")).Count(1))
		t.Must(page.Find(ct.Role("heading", ct.Level(1))).Texts("409 Conflict"))
		t.Must(page.Find(ct.Tag("title")).Texts("409 Conflict · webui gallery"))
	})

	t.Run("not found", func(t devtest.T) {
		page := c.Page(t, c.Get(t, "/no/such/page"), http.StatusNotFound)
		t.Must(page.Find(ct.Role("heading", ct.Level(1))).Texts("404 Not Found"))
		res := c.Get(t, "/no/such/page", webuitest.HTMX())
		t.Must(webuitest.HXRetarget(res, ct.Exact("#alerts")))
		c.Fragment(t, res, http.StatusNotFound, "div")

		res = c.Post(t, "/htmx/tasks/9999/delete", webuitest.Form(nil), webuitest.HTMX())
		t.Must(webuitest.HXRetarget(res, ct.Exact("#alerts")))
		t.Must(c.Fragment(t, res, http.StatusNotFound, "div").Find(ct.Role("alert")).Texts(
			"There is no such task. It may have been deleted already."))
		res = c.Post(t, "/htmx/tasks/x/delete", webuitest.Form(nil), webuitest.HTMX())
		t.Must(res.Status(http.StatusNotFound))
	})

	t.Run("invalid task", func(t devtest.T) {
		res := c.Post(t, "/htmx/tasks", webuitest.Form(url.Values{"title": {"\x00"}}), webuitest.HTMX())
		t.Must(webuitest.HXRetarget(res, ct.Exact("#alerts")))
		t.Must(c.Fragment(t, res, http.StatusUnprocessableEntity, "div").Find(ct.Role("alert")).Texts(
			"Give the task a title of 1 to 80 characters, on one line."))
	})

	t.Run("search", func(t devtest.T) {
		for _, q := range []string{"tab\tbed", strings.Repeat("q", 101), strings.Repeat("q", 401)} {
			res := c.Get(t, "/htmx/search?q="+url.QueryEscape(q), webuitest.HTMX())
			t.Must(webuitest.HXRetarget(res, ct.Exact("#alerts")))
			t.Must(res.Status(http.StatusBadRequest))
		}
	})
}

// TestHTMXSwaps checks the fragments of the htmx page: a filter swap with
// an out-of-band filter, an added and a deleted task with out-of-band
// updates, a search and a poll.
func TestHTMXSwaps(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clock := newFakeClock()
	g := newGallery(t, app.Options{Now: clock.Now})
	c := g.signedIn(newAccount("Ada"))

	t.Run("filter", func(t devtest.T) {
		res := c.Get(t, "/htmx/tasks?filter=done", webuitest.HTMX(), webuitest.HXTarget("task-list"))
		t.Must(res.Header("Vary", ct.Contains("HX-Request")))
		frag := c.Fragment(t, res, http.StatusOK, "div")
		t.Must(frag.Find(ct.ID("task-list")).Find(ct.Role("rowheader")).Texts("Fix the login redirect", "Update the screenshots"))
		t.Must(frag.Find(ct.ID("task-filter")).Matches(ct.Attr("hx-swap-oob", "true")))
		t.Must(frag.Find(ct.ID("task-filter")).Find(ct.Class("active")).Texts("Done"))

		// The same URL without htmx is the whole page, filtered alike.
		page := c.Page(t, c.Get(t, "/htmx/tasks?filter=done"), http.StatusOK)
		t.Must(page.Find(ct.ID("task-list")).Find(ct.Role("rowheader")).Texts("Fix the login redirect", "Update the screenshots"))
		t.Must(page.Find(ct.ID("task-filter"), ct.HasAttr("hx-swap-oob")).None())
	})

	t.Run("add and delete", func(t devtest.T) {
		page := c.Page(t, c.Get(t, "/htmx"), http.StatusOK)
		t.Must(page.Find(ct.ID("task-count")).Texts("5 open of 7"))
		res := c.Submit(t, page, page.Find(ct.ID("add-task")), nil, url.Values{"title": {"Water the plants"}}, webuitest.HTMX(), webuitest.HXTarget("task-list"))
		frag := c.Fragment(t, res, http.StatusOK, "div")
		t.Must(frag.Find(ct.ID("task-list")).Find(ct.Role("rowheader", ct.Named("Water the plants"))).Count(1))
		for _, id := range []string{"task-filter", "task-count", "add-task", "alerts"} {
			t.Mustf(frag.Find(ct.ID(id), ct.HasAttr("hx-swap-oob")).Count(1), "%s", id)
		}
		t.Must(frag.Find(ct.ID("task-count")).Texts("6 open of 8"))
		t.Must(frag.Find(ct.ID("alerts")).Find(ct.Role("status")).Texts("Task added."))
		t.Must(frag.Find(ct.ID("add-task")).Find(ct.ID("new-task"), ct.HasAttr("value")).None())

		// Delete it with its own form, from the swapped list.
		del := frag.Find(ct.Tag("form"), ct.HasDescendant(ct.Role("button", ct.Named("Delete Water the plants"))))
		t.Must(del.Matches(ct.HasAttr("hx-confirm")))
		res = c.Submit(t, frag, del, nil, nil, webuitest.HTMX(), webuitest.HXTarget("task-list"))
		frag = c.Fragment(t, res, http.StatusOK, "div")
		t.Must(frag.Find(ct.Role("rowheader", ct.Named("Water the plants"))).None())
		t.Must(frag.Find(ct.ID("task-count")).Texts("5 open of 7"))

		// Without htmx, the forms redirect with a notice.
		res = c.Submit(t, page, page.Find(ct.ID("add-task")), nil, url.Values{"title": {"Plain"}})
		t.Must(webuitest.Location(res, http.StatusSeeOther, ct.Exact("/htmx?notice=task-added")))
	})

	t.Run("search", func(t devtest.T) {
		res := c.Get(t, "/htmx/search?q=RELEASE", webuitest.HTMX(), webuitest.HXTarget("search-results"))
		frag := c.Fragment(t, res, http.StatusOK, "div")
		t.Must(frag.Find(ct.Class("list-group-item")).Texts("Write the release notes"))
		res = c.Get(t, "/htmx/search?q=%20", webuitest.HTMX())
		t.Must(c.Fragment(t, res, http.StatusOK, "div").Find(ct.Tag("p")).Texts("Type to search the tasks."))
		res = c.Get(t, "/htmx/search?q=nothing+like+this", webuitest.HTMX())
		t.Must(c.Fragment(t, res, http.StatusOK, "div").Find(ct.Tag("p")).Texts("No task matches."))

		page := c.Page(t, c.Get(t, "/htmx/search?q=release"), http.StatusOK)
		t.Must(page.Find(ct.ID("search-q")).Matches(ct.Attr("value", "release")))
		t.Must(page.Find(ct.ID("search-results")).Find(ct.Class("list-group-item")).Texts("Write the release notes"))
	})

	t.Run("poll and clock", func(t devtest.T) {
		clock.Advance(90 * time.Second)
		res := c.Get(t, "/htmx/poll", webuitest.HTMX())
		c.Fragment(t, res, http.StatusOK, "p")
		t.Must(res.Body(ct.Contains("running for 1m30s.")))
		res = c.Get(t, "/disclosure/time", webuitest.HTMX())
		t.Must(res.Body(ct.Exact("12:01:30 UTC")))
	})

	t.Run("toast", func(t devtest.T) {
		page := c.Page(t, c.Get(t, "/components/feedback"), http.StatusOK)
		t.Must(page.Find(ct.ID("toasts")).Matches(ct.Class("toast-container"), ct.Attr("aria-live", "polite")))
		trigger := page.Find(ct.Role("link", ct.Named("Show a toast")))
		t.Must(trigger.Matches(ct.Attr("hx-get", "/components/toast"), ct.Attr("hx-swap", "none")))

		res := c.Get(t, "/components/toast", webuitest.HTMX())
		frag := c.Fragment(t, res, http.StatusOK, "div")
		t.Must(frag.Find(ct.ID("toasts")).Matches(ct.Attr("hx-swap-oob", "beforeend")))
		t.Must(frag.Find(ct.ID("toasts")).Find(ct.Class("toast")).Matches(ct.Class("show"), ct.Attr("role", "status")))

		// Without htmx, the toast shows in the container of the page.
		page = c.Page(t, c.Get(t, "/components/toast"), http.StatusOK)
		t.Must(page.Find(ct.ID("toasts")).Find(ct.Class("toast")).Count(1))
		t.Must(webuitest.Location(c.Get(t, "/components/toast/dismiss"), http.StatusSeeOther, ct.Exact("/components/feedback#toast-demo")))
		t.Must(c.Get(t, "/components/toast/dismiss", webuitest.HTMX()).Body(ct.Exact("")))
	})
}

// TestPreAuthFragments checks that the htmx fragments keep the anonymous
// forms of the htmx page valid once a new token would renew their
// pre-authentication cookie, past half of its lifetime: the poll and the
// search, whose answers hold no form, issue no token, and the filter, whose
// delete forms need one, renews the other forms of the page out of band.
func TestPreAuthFragments(gt *testing.T) {
	t := devtest.ParallelT(gt)
	clock := newFakeClock()
	g := newGallery(t, app.Options{Now: clock.Now})
	b := webuitest.NewBrowser(t, g, webuitest.BrowserOptions{})

	res := b.Get(t, "/htmx")
	t.Must(res.Status(http.StatusOK))
	page := res.HTML()
	t.Must(contract("")(page))
	issued := webuitest.CSRFToken(t, page)

	// A fresh cookie serves the filter as it is: the other forms stay.
	res = b.Get(t, "/htmx/tasks?filter=open", webuitest.HTMX())
	t.Must(res.Status(http.StatusOK))
	t.Must(webuitest.NoCookies(res))
	t.Must(res.HTMLFragment("div").Find(ct.ID("add-task")).None())

	clock.Advance(session.DefaultPreAuthTTL/2 + time.Second)
	for _, target := range []string{"/htmx/poll", "/htmx/search?q=release"} {
		res = b.Get(t, target, webuitest.HTMX())
		t.Mustf(res.Status(http.StatusOK), "%s", target)
		t.Mustf(webuitest.NoCookies(res), "%s", target)
	}
	res = b.Submit(t, page, page.Find(ct.ID("add-task")), nil, url.Values{"title": {"After a poll"}}, webuitest.HTMX())
	t.Must(res.Status(http.StatusOK))

	res = b.Get(t, "/htmx/tasks?filter=all", webuitest.HTMX())
	t.Must(webuitest.Cookie(res, "__Host-preauth", webuitest.CookieWant{
		HostPrefix: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, Path: "/",
	}))
	frag := res.HTMLFragment("div")
	t.Must(fragmentContract("")(frag))
	for _, id := range []string{"add-task", "fail-form"} {
		t.Mustf(frag.Find(ct.ID(id), ct.Attr("hx-swap-oob", "true")).Count(1), "%s", id)
	}
	// Every form of the fragment carries the renewed token, which the
	// browser's new cookie accepts in place of the page's old one.
	t.Must(be.NotEqual(issued, webuitest.CSRFToken(t, frag)))
	res = b.Submit(t, page, page.Find(ct.ID("fail-form")), nil, nil, webuitest.HTMX())
	t.Must(res.Status(http.StatusForbidden))
	res = b.Submit(t, frag, frag.Find(ct.ID("fail-form")), nil, nil, webuitest.HTMX())
	t.Must(res.Status(http.StatusConflict))
}

// TestNavigation checks the two levels of navigation: the navbar marks the
// current page, or the current section on the pages inside it, whose own
// link the sidebar marks.
func TestNavigation(gt *testing.T) {
	t := devtest.ParallelT(gt)
	g := newGallery(t, app.Options{})
	c := g.anonymous()
	navbarLink := func(p *ct.Subject, name string, more ...ct.Query) *ct.Selection {
		return p.Find(ct.ID("navbar")).Find(ct.Role("link", ct.Named(name)), more...)
	}
	sidebarLink := func(p *ct.Subject, name string, more ...ct.Query) *ct.Selection {
		return p.Find(ct.ID("sidebar")).Find(ct.Role("link", ct.Named(name)), more...)
	}

	p := c.Page(t, c.Get(t, "/components"), http.StatusOK)
	t.Must(navbarLink(p, "Components").Matches(ct.Attr("aria-current", "page"), ct.Class("active")))
	t.Must(sidebarLink(p, "Overview").Matches(ct.Attr("aria-current", "page")))
	t.Must(p.Find(ct.ID("page-header")).Find(ct.Tag("nav")).Find(ct.Role("link")).Texts("Gallery"))

	p = c.Page(t, c.Get(t, "/components/content?page=2"), http.StatusOK)
	t.Must(navbarLink(p, "Components").Matches(ct.Attr("aria-current", "true"), ct.Class("active")))
	t.Must(sidebarLink(p, "Content").Matches(ct.Attr("aria-current", "page")))
	t.Must(sidebarLink(p, "Controls", ct.HasAttr("aria-current")).None())
	t.Must(sidebarLink(p, "Table and pagination").Matches(ct.Attr("href", "#table")))
	crumbs := p.Find(ct.ID("page-header")).Find(ct.Tag("li"))
	t.Must(crumbs.Texts("Gallery", "Components", "Content"))
	t.Must(crumbs.Last().Matches(ct.Attr("aria-current", "page")))
	t.Must(p.Find(ct.Tag("caption")).Texts("Tasks, page 2 of 3"))
	t.Must(p.Find(ct.Role("navigation", ct.Named("Task pages"))).Find(ct.Attr("aria-current", "page")).Texts("2"))

	// Pages outside sections have no sidebar, and mark their own link.
	p = c.Page(t, c.Get(t, "/forms"), http.StatusOK)
	t.Must(navbarLink(p, "Forms").Matches(ct.Attr("aria-current", "page")))
	t.Must(navbarLink(p, "Components", ct.HasAttr("aria-current")).None())
	t.Must(p.Find(ct.ID("sidebar")).None())
	t.Must(p.Find(ct.Tag("footer")).Count(1))
}
