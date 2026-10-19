package web_test

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/aria"
	"github.com/protolambda/chord/html/form"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/bs"
	"github.com/protolambda/webui/hx2"
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
	// Vary: HX-Request, HX-History-Restore-Request
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
// everyone else, boosted links included: htmx swaps their answer into the
// whole body.
func ExampleChoose() {
	list := text.P()(text.Text("3 tasks"))
	h := web.Errors{}.Handle(func(w http.ResponseWriter, r *http.Request) error {
		full := page.Document{Title: "Tasks", Body: page.Layout{NoSkipLink: true, Main: list}}
		return web.Choose(w, r, http.StatusOK, list, full)
	})
	serve(h, htmxRequest(http.MethodGet, "/tasks", "https://app.example/tasks"), "Vary")
	serve(h, httptest.NewRequest(http.MethodGet, "/tasks", nil), "Vary")
	boosted := htmxRequest(http.MethodGet, "/tasks", "https://app.example/notes")
	boosted.Header.Set("HX-Boosted", "true")
	serve(h, boosted)
	// Output:
	// 200
	// Vary: HX-Request, HX-History-Restore-Request, HX-Boosted
	// <p>3 tasks</p>
	// 200
	// Vary: HX-Request, HX-History-Restore-Request, HX-Boosted
	// <main id="main" class="flex-grow-1"><div class="container py-3"><div id="alerts" aria-live="polite"></div><p>3 tasks</p></div></main>
	// 200
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
// with the real status; unknown errors get a fixed message. htmx scrolls
// the region into view (show:top), HX-Reselect unset keeps an hx-select of
// the page from filtering the alert out, and HX-Push-Url false keeps the
// URL.
func ExampleErrors_Handle_htmx() {
	save := web.Errors{}.Handle(func(w http.ResponseWriter, r *http.Request) error {
		return errors.New("store: disk full at /var/lib/tasks")
	})
	serve(save, htmxRequest(http.MethodPost, "/tasks", "https://app.example/tasks"), "HX-Retarget", "HX-Reswap", "HX-Reselect", "HX-Push-Url")
	// Output:
	// 500
	// HX-Retarget: #alerts
	// HX-Reswap: innerHTML show:top
	// HX-Reselect: unset
	// HX-Push-Url: false
	// <div class="alert alert-danger">The request failed. The server log has details.</div>
}

// Recover answers a panic with the 500 error page; the panic value goes to
// the log only, as panic_value next to the panic flag that mw.LogRequests
// uses too. Placed inside mw.LogRequests, the request record has the 500,
// and the panic record carries the request's method and pattern.
func ExampleErrors_Recover() {
	// Print the records without their varying attributes.
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.TimeKey, "stack", "duration", "bytes":
				return slog.Attr{}
			}
			return a
		},
	}))
	errs := web.Errors{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		panic("task store not opened")
	})
	h := mw.Chain(mux, mw.LogRequests(mw.RequestLog{Logger: logger, Routes: mux}), errs.Recover)
	serve(h, httptest.NewRequest(http.MethodGet, "/tasks/7", nil))
	// Output:
	// level=ERROR msg="handler panic" method=GET pattern="GET /tasks/{id}" panic=true panic_value="task store not opened"
	// level=INFO msg="http request" method=GET pattern="GET /tasks/{id}" status=500
	// 500
	// <main id="main" class="flex-grow-1"><div class="container py-3"><header id="page-header" class="mb-3"><h1 class="h4 mb-0">500 Internal Server Error</h1></header><div id="alerts" aria-live="polite"><div class="alert alert-danger" role="alert">The request failed. The server log has details.</div></div><p><a href="/">Go to the start page</a></p></div></main>
}

// NotFound answers the requests that no other route matches, without
// naming the path; a boosted link to a missing page shows the alert on the
// page it was clicked on.
func ExampleErrors_NotFound() {
	errs := web.Errors{}
	mux := http.NewServeMux()
	mux.Handle("/", errs.NotFound())
	serve(mux, httptest.NewRequest(http.MethodGet, "/projects/7", nil), "Cache-Control")
	boosted := htmxRequest(http.MethodGet, "/projects/7", "https://app.example/tasks")
	boosted.Header.Set("HX-Boosted", "true")
	serve(mux, boosted, "HX-Retarget", "HX-Push-Url")
	// Output:
	// 404
	// Cache-Control: no-store
	// <main id="main" class="flex-grow-1"><div class="container py-3"><header id="page-header" class="mb-3"><h1 class="h4 mb-0">404 Not Found</h1></header><div id="alerts" aria-live="polite"><div class="alert alert-danger" role="alert">Nothing was found here.</div></div><p><a href="/">Go to the start page</a></p></div></main>
	// 404
	// HX-Retarget: #alerts
	// HX-Push-Url: false
	// <div class="alert alert-danger">Nothing was found here.</div>
}

// Intercept keeps the plain-text errors of other code out of htmx swaps:
// htmx would insert the text, and the markup that it quotes from the
// request, into the page. An htmx request gets the policy's alert of the
// status instead; a plain request gets the text, which a browser shows as
// text.
func ExampleErrors_Intercept() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tasks", func(w http.ResponseWriter, r *http.Request) {
		if _, err := strconv.Atoi(r.FormValue("page")); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest) // quotes the request
			return
		}
	})
	h := mw.Chain(mux, web.Errors{}.Intercept)
	target := "/tasks?page=" + url.QueryEscape(`<i hx-get="/x" hx-trigger="load">`)
	serve(h, htmxRequest(http.MethodGet, target, "https://app.example/tasks"), "Content-Type", "HX-Retarget")
	serve(h, httptest.NewRequest(http.MethodGet, target, nil), "Content-Type", "Vary")
	// The 404 and 405 answers of http.ServeMux are plain text too.
	serve(h, htmxRequest(http.MethodPost, "/tasks", "https://app.example/tasks"), "HX-Retarget", "Allow")
	// Output:
	// 400
	// Content-Type: text/html; charset=utf-8
	// HX-Retarget: #alerts
	// <div class="alert alert-danger">The request is not valid.</div>
	// 400
	// Content-Type: text/plain; charset=utf-8
	// Vary: HX-Request, HX-History-Restore-Request
	// strconv.Atoi: parsing "<i hx-get=\"/x\" hx-trigger=\"load\">": invalid syntax
	// 405
	// HX-Retarget: #alerts
	// Allow: GET, HEAD
	// <div class="alert alert-danger">This action is not available here.</div>
}

// Answer turns an error into a handler for the refusal hooks of
// middleware: a refusal of mw.SameOrigin gets the policy's answer, here the
// alert of mw.ErrCrossOrigin for an htmx request.
func ExampleErrors_Answer() {
	errs := web.Errors{}
	tasks := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	h := mw.Chain(tasks,
		mw.HostCheck(mw.MustParseSite("https://app.example")),
		mw.SameOrigin(mw.OriginPolicy{Deny: errs.Answer(mw.ErrCrossOrigin)}),
	)
	r := htmxRequest(http.MethodPost, "https://app.example/tasks", "https://app.example/tasks")
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	serve(h, r, "HX-Retarget")
	// Output:
	// 403
	// HX-Retarget: #alerts
	// <div class="alert alert-danger">The form expired or came from another site. Open the form again and retry.</div>
}

// A form in a modal dialog shows its errors inside the dialog: the page
// behind an open modal is inert, so an alert in the page's alerts region
// would be neither seen nor announced. The dialog's body starts with an
// alerts region of its own, and TargetFor sends the error answers to the
// dialog's form there. Other requests keep the page's alerts region.
func ExampleErrors_dialog() {
	rename := bs.Dialog{
		ID:    "rename",
		Title: text.Text("Rename the project"),
		Body: elem.Bundle{
			div.Div(attr.ID("rename-alerts"), aria.Live("polite"))(),
			form.Form(attr.ID("rename-form"), form.Method(form.MethodPost), form.Action("/projects/7/name"), hx2.Post("/projects/7/name"))(
				bs.BtnPrimary(button.Type(button.TypeSubmit))(text.Text("Rename")),
			),
		},
	}
	core.Dump(rename) // in page.Layout's Dialogs

	dialogAlerts := map[string]string{"rename-form": "#rename-alerts"}
	errs := web.Errors{TargetFor: func(r *http.Request) string {
		// HX-Trigger, the id of the form, picks one of the application's
		// own regions: a presentation, never an authorization.
		return dialogAlerts[hx2.ParseRequestHeaders(r.Header).Trigger]
	}}
	save := errs.Handle(func(w http.ResponseWriter, r *http.Request) error {
		return web.Conflict("", errors.New("project 7 renamed meanwhile"))
	})
	r := htmxRequest(http.MethodPost, "/projects/7/name", "https://app.example/projects/7")
	r.Header.Set("HX-Trigger", "rename-form")
	serve(save, r, "HX-Retarget")
	serve(save, htmxRequest(http.MethodPost, "/projects/7/name", "https://app.example/projects/7"), "HX-Retarget")
	// Output:
	// <dialog id="rename" class="modal" popover aria-labelledby="rename-title"><div class="modal-dialog"><div class="modal-content"><div class="modal-header"><h2 class="modal-title h5" id="rename-title">Rename the project</h2><button class="btn-close" type="button" aria-label="Close" commandfor="rename" command="close" popovertarget="rename" popovertargetaction="hide" autofocus></button></div><div class="modal-body"><div id="rename-alerts" aria-live="polite"></div><form id="rename-form" method="post" action="/projects/7/name" hx-post="/projects/7/name"><button class="btn btn-primary" type="submit">Rename</button></form></div></div></div></dialog>
	// 409
	// HX-Retarget: #rename-alerts
	// <div class="alert alert-danger">The item changed meanwhile. Reload the page and try again.</div>
	// 409
	// HX-Retarget: #alerts
	// <div class="alert alert-danger">The item changed meanwhile. Reload the page and try again.</div>
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
	// <div class="alert alert-danger">The task changed meanwhile. Reload it and try again.</div>
}

// Pages behind the guard send visitors without a session to the login
// page, with the page to return to.
func ExampleErrors_RequireAuth() {
	loggedIn := func(r *http.Request) bool { return false }
	guard := web.Errors{}.RequireAuth("/login", loggedIn)
	tasks := guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	serve(tasks, httptest.NewRequest(http.MethodGet, "/tasks?page=2", nil), "Location")
	// A fragment request returns to the page it came from.
	serve(tasks, htmxRequest(http.MethodPost, "/tasks/7/done", "https://app.example/tasks?page=2"), "HX-Redirect")
	// A boosted link returns to the page it leads to.
	boosted := htmxRequest(http.MethodGet, "/tasks?page=3", "https://app.example/")
	boosted.Header.Set("HX-Boosted", "true")
	serve(tasks, boosted, "HX-Redirect")
	// Output:
	// 303
	// Location: /login?next=%2Ftasks%3Fpage%3D2
	// <a href="/login?next=%2Ftasks%3Fpage%3D2">See Other</a>.
	// 401
	// HX-Redirect: /login?next=%2Ftasks%3Fpage%3D2
	// 401
	// HX-Redirect: /login?next=%2Ftasks%3Fpage%3D3
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
