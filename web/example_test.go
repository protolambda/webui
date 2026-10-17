package web_test

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/bs"
	"github.com/protolambda/webui/mw"
	"github.com/protolambda/webui/page"
	"github.com/protolambda/webui/web"
)

// serve runs h for a request and prints the status, the named headers and
// the body (or the part of it from the first "<main" to "</main>").
func serve(h http.Handler, r *http.Request, headers ...string) {
	// Keep the examples' error logs quiet.
	r = r.WithContext(mw.WithLogger(r.Context(), slog.New(slog.DiscardHandler)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	fmt.Println(rec.Code)
	for _, name := range headers {
		if v := rec.Header().Values(name); len(v) > 0 {
			fmt.Printf("%s: %s\n", name, strings.Join(v, ", "))
		}
	}
	body := strings.TrimSpace(rec.Body.String())
	if start, end := strings.Index(body, "<main"), strings.Index(body, "</main>"); start >= 0 && end > start {
		body = body[start : end+len("</main>")]
	}
	if body != "" {
		fmt.Println(body)
	}
}

// htmxRequest returns a request that htmx made from the page at current.
func htmxRequest(method, target, current string) *http.Request {
	r := httptest.NewRequest(method, target, nil)
	r.Header.Set("HX-Request", "true")
	r.Header.Set("HX-Current-URL", current)
	return r
}

// A form post ends with a redirect: 303 for the browser, HX-Redirect for
// htmx, and HX-Refresh when the target is a fragment of the current page.
func ExampleRedirect() {
	saved := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		web.Redirect(w, r, "/tasks#new")
	})
	serve(saved, httptest.NewRequest(http.MethodPost, "/tasks", nil), "Location", "Vary")
	serve(saved, htmxRequest(http.MethodPost, "/tasks", "https://app.example/notes"), "HX-Redirect", "HX-Refresh")
	serve(saved, htmxRequest(http.MethodPost, "/tasks", "https://app.example/tasks"), "HX-Redirect", "HX-Refresh")
	// Output:
	// 303
	// Location: /tasks#new
	// Vary: HX-Request
	// 200
	// HX-Redirect: /tasks#new
	// 200
	// HX-Redirect: /tasks#new
	// HX-Refresh: true
}

// Render writes a view only once it rendered completely.
func ExampleRender() {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := web.Render(w, r, http.StatusOK, text.P()(text.Text("3 tasks"))); err != nil {
			http.Error(w, "render failed", http.StatusInternalServerError)
		}
	})
	serve(h, httptest.NewRequest(http.MethodGet, "/tasks", nil), "Content-Type")
	// Output:
	// 200
	// Content-Type: text/html; charset=utf-8
	// <p>3 tasks</p>
}

// One handler serves the fragment to htmx and the page around it to
// everyone else.
func ExampleChoose() {
	list := text.P()(text.Text("3 tasks"))
	h := web.Errors{}.Handle(func(w http.ResponseWriter, r *http.Request) error {
		full := page.Document{Title: "Tasks", Body: page.Layout{NoSkipLink: true, Main: list}}
		return web.Choose(w, r, http.StatusOK, list, full)
	})
	serve(h, htmxRequest(http.MethodGet, "/tasks", "https://app.example/tasks"), "Vary")
	serve(h, httptest.NewRequest(http.MethodGet, "/tasks", nil), "Vary")
	// Output:
	// 200
	// Vary: HX-Request
	// <p>3 tasks</p>
	// 200
	// Vary: HX-Request
	// <main id="main" class="flex-grow-1"><div class="container py-3"><div id="alerts" aria-live="polite"></div><p>3 tasks</p></div></main>
}

// A handler returns its error; a plain request gets an error page in the
// application's layout, with a public message only.
func ExampleErrors_Handle() {
	errs := web.Errors{
		Document: func(r *http.Request, v web.ErrorView) elem.Node {
			l := v.Layout()
			l.NoSkipLink = true
			return page.Document{Title: v.Title + " · Tasks", Body: l}
		},
	}
	show := errs.Handle(func(w http.ResponseWriter, r *http.Request) error {
		return web.NotFound("There is no such task.", errors.New("task 7: not in the store"))
	})
	serve(show, httptest.NewRequest(http.MethodGet, "/tasks/7", nil), "Content-Type", "Cache-Control")
	// Output:
	// 404
	// Content-Type: text/html; charset=utf-8
	// Cache-Control: no-store
	// <main id="main" class="flex-grow-1"><div class="container py-3"><header id="page-header" class="mb-3"><h1 class="h4 mb-0">404 Not Found</h1></header><div id="alerts" aria-live="polite"><div class="alert alert-danger" role="alert">There is no such task.</div></div><p><a href="/">Go to the start page</a></p></div></main>
}

// An htmx request gets the alert, retargeted to the page's alerts region,
// with the real status; unknown errors get a fixed message.
func ExampleErrors_Handle_htmx() {
	save := web.Errors{}.Handle(func(w http.ResponseWriter, r *http.Request) error {
		return errors.New("store: disk full at /var/lib/tasks")
	})
	serve(save, htmxRequest(http.MethodPost, "/tasks", "https://app.example/tasks"), "HX-Retarget", "HX-Reswap", "HX-Push-Url")
	// Output:
	// 500
	// HX-Retarget: #alerts
	// HX-Reswap: innerHTML
	// HX-Push-Url: false
	// <div class="alert alert-danger" role="alert">The request failed. The server log has details.</div>
}

// Applications map their own errors before the defaults.
func ExampleErrors_classify() {
	errStale := errors.New("stale revision")
	errs := web.Errors{Classify: func(err error) *web.Error {
		if errors.Is(err, errStale) {
			return web.Conflict("The task changed meanwhile. Reload it and try again.", err)
		}
		return nil
	}}
	save := errs.Handle(func(w http.ResponseWriter, r *http.Request) error {
		return fmt.Errorf("save task 7: %w", errStale)
	})
	serve(save, htmxRequest(http.MethodPost, "/tasks/7", "https://app.example/tasks/7"))
	// Output:
	// 409
	// <div class="alert alert-danger" role="alert">The task changed meanwhile. Reload it and try again.</div>
}

// Pages behind the guard send visitors without a session to the login
// page, with the page to return to.
func ExampleErrors_RequireAuth() {
	loggedIn := func(r *http.Request) bool { return false }
	guard := web.Errors{}.RequireAuth("/login", loggedIn)
	tasks := guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	serve(tasks, httptest.NewRequest(http.MethodGet, "/tasks?page=2", nil), "Location")
	serve(tasks, htmxRequest(http.MethodPost, "/tasks/7/done", "https://app.example/tasks?page=2"), "HX-Redirect")
	// Output:
	// 303
	// Location: /login?next=%2Ftasks%3Fpage%3D2
	// <a href="/login?next=%2Ftasks%3Fpage%3D2">See Other</a>.
	// 401
	// HX-Redirect: /login?next=%2Ftasks%3Fpage%3D2
}

// SafeNext accepts local paths only, so a crafted "next" parameter cannot
// lead to another site.
func ExampleSafeNext() {
	for _, next := range []string{"/tasks?page=2", "https://evil.example/", "//evil.example/", "/\\evil.example", "/\t/evil.example", ""} {
		fmt.Printf("%q -> %q\n", next, web.SafeNext(next, "/"))
	}
	// Output:
	// "/tasks?page=2" -> "/tasks?page=2"
	// "https://evil.example/" -> "/"
	// "//evil.example/" -> "/"
	// "/\\evil.example" -> "/"
	// "/\t/evil.example" -> "/"
	// "" -> "/"
}

// Notices report the outcome of an action after its redirect, by code: the
// page shows only registered messages.
func ExampleNotices() {
	notices := web.Notices{
		"saved":   {Message: "Task saved."},
		"deleted": {Message: "Task deleted.", Variant: bs.VariantInfo},
	}
	save := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		notices.Redirect(w, r, "/tasks?page=2", "saved")
	})
	list := web.Errors{}.Handle(func(w http.ResponseWriter, r *http.Request) error {
		return web.Render(w, r, http.StatusOK, page.AlertsRegion(notices.Alert(r)))
	})
	serve(save, httptest.NewRequest(http.MethodPost, "/tasks", nil), "Location")
	serve(list, httptest.NewRequest(http.MethodGet, "/tasks?page=2&notice=saved", nil))
	serve(list, httptest.NewRequest(http.MethodGet, "/tasks?notice=%3Cscript%3E", nil))
	// Output:
	// 303
	// Location: /tasks?page=2&notice=saved
	// 200
	// <div id="alerts" aria-live="polite"><div class="alert alert-success" role="status">Task saved.</div></div>
	// 200
	// <div id="alerts" aria-live="polite"></div>
}

func ExampleWithParam() {
	fmt.Println(web.WithParam("/tasks?notice=saved&page=2#list", "notice", "deleted"))
	fmt.Println(web.WithParam("/login", web.NextParam, "/tasks?page=2"))
	// Output:
	// /tasks?page=2&notice=deleted#list
	// /login?next=%2Ftasks%3Fpage%3D2
}

// A confirmation link acts on a real visit only, not on a prefetch.
func ExampleMayAct() {
	confirm := web.Errors{}.Handle(func(w http.ResponseWriter, r *http.Request) error {
		if !web.MayAct(r) {
			return web.Render(w, r, http.StatusOK, section.Main()(text.Text("Confirm with the button.")))
		}
		return web.Render(w, r, http.StatusOK, section.Main()(text.Text("Email confirmed.")))
	})
	prefetch := httptest.NewRequest(http.MethodGet, "/confirm", nil)
	prefetch.Header.Set("Sec-Purpose", "prefetch")
	serve(confirm, prefetch)
	serve(confirm, httptest.NewRequest(http.MethodGet, "/confirm", nil))
	// Output:
	// 200
	// <main>Confirm with the button.</main>
	// 200
	// <main>Email confirmed.</main>
}
