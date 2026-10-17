package web_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthttp"
	"github.com/protolambda/chord/html/text"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/asset"
	"github.com/protolambda/webui/form"
	"github.com/protolambda/webui/mw"
	"github.com/protolambda/webui/page"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/sse"
	"github.com/protolambda/webui/web"
)

// failWith returns a handler that fails with err.
func failWith(err error) web.HandlerFunc {
	return func(http.ResponseWriter, *http.Request) error { return err }
}

func TestHandlePassesThrough(gt *testing.T) {
	t := devtest.ParallelT(gt)
	h := web.Errors{}.Handle(func(w http.ResponseWriter, r *http.Request) error {
		return web.Render(w, r, http.StatusOK, text.Text("ok"))
	})
	res := cthttp.Serve(h, request(t, nil, http.MethodGet, "/", nil))
	t.Must(res.Status(http.StatusOK))
	t.Must(res.Body(ct.Exact("ok")))
}

func TestErrorPage(gt *testing.T) {
	t := devtest.ParallelT(gt)
	l := new(logs)
	h := web.Errors{}.Handle(failWith(web.NotFound("There is no such task.", errors.New("task 7 not in the store"))))
	res := cthttp.Serve(h, request(t, l, http.MethodGet, "/tasks/7", nil))
	t.Must(res.Status(http.StatusNotFound))
	t.Must(res.Header("Content-Type", ct.Exact(web.ContentType)))
	t.Must(res.Header("Vary", ct.Exact("HX-Request")))
	t.Must(res.Header("Cache-Control", ct.Exact("no-store")))
	t.Must(res.HeaderAbsent("HX-Retarget"))
	doc := res.HTML()
	t.Must(doc.Valid(ct.UniqueIDs(), ct.ARIAReferences(), ct.LocalTargets(), ct.NoInlineHandlers()))
	t.Must(doc.Find(ct.Tag("title")).Texts("404 Not Found"))
	main := doc.Find(ct.Role("main"))
	t.Must(main.Find(ct.Role("heading", ct.Level(1), ct.Named("404 Not Found"))))
	t.Must(main.Find(ct.ID(page.AlertsID)).Find(ct.Role("alert")).Texts("There is no such task."))
	t.Must(main.Find(ct.Role("link", ct.Named(web.DefaultHomeLabel))).Matches(ct.Attr("href", "/")))
	t.Must(be.False(strings.Contains(res.BodyString(), "task 7")))

	recs := l.all()
	t.Must(be.Equal(1, len(recs)))
	t.Must(be.Equal(slog.LevelDebug, recs[0].level))
	t.Must(be.Equal("404", recs[0].attrs["status"]))
	t.Must(be.Substring(recs[0].attrs["err"], "task 7 not in the store"))
}

func TestErrorFragment(gt *testing.T) {
	t := devtest.ParallelT(gt)
	h := web.Errors{}.Handle(func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("HX-Trigger", "taskSaved")
		return web.Conflict("", nil)
	})
	res := cthttp.Serve(h, asHTMX(request(t, nil, http.MethodPost, "/tasks/7", nil), "https://app.example/tasks"))
	t.Must(res.Status(http.StatusConflict))
	t.Must(res.Header("HX-Retarget", ct.Exact("#"+page.AlertsID)))
	t.Must(res.Header("HX-Reswap", ct.Exact("innerHTML")))
	t.Must(res.Header("HX-Push-Url", ct.Exact("false")))
	t.Must(res.HeaderAbsent("HX-Trigger"))
	t.Must(res.Header("Vary", ct.Exact("HX-Request")))
	t.Must(res.Body(ct.Exact(`<div class="alert alert-danger" role="alert">` + web.DefaultMessage(http.StatusConflict) + `</div>`)))
	frag := res.HTMLFragment("div")
	t.Must(frag.Find(ct.Role("alert")))
	t.Must(frag.Find(ct.Tag("html")).None())
}

// TestClassification checks the default mapping of errors to statuses and
// messages, and that no message reveals the error.
func TestClassification(gt *testing.T) {
	t := devtest.ParallelT(gt)
	const secret = "SECRET-DETAIL"
	cases := []struct {
		name    string
		err     error
		status  int
		message string
		level   slog.Level
	}{
		{"error", web.BadRequest("The title is missing.", errors.New(secret)), 400, "The title is missing.", slog.LevelDebug},
		{"wrapped error", fmt.Errorf("load: %w", web.NotFound("", errors.New(secret))), 404, web.DefaultMessage(404), slog.LevelDebug},
		{"error 5xx", web.StatusError(http.StatusBadGateway, "", errors.New(secret)), 502, web.DefaultMessage(502), slog.LevelError},
		{"error without status", &web.Error{Message: "Odd."}, 500, "Odd.", slog.LevelError},
		{"error with redirect status", &web.Error{Status: http.StatusFound}, 500, web.DefaultMessage(500), slog.LevelError},
		{"error teapot", &web.Error{Status: http.StatusTeapot}, 418, "I'm a teapot.", slog.LevelDebug},
		{"max bytes", fmt.Errorf("read: %w", &http.MaxBytesError{Limit: 10}), 413, web.DefaultMessage(413), slog.LevelDebug},
		{"form too large", fmt.Errorf("%w: %s", form.ErrTooLarge, secret), 413, web.DefaultMessage(413), slog.LevelDebug},
		{"form type", form.ErrUnsupportedType, 415, web.DefaultMessage(415), slog.LevelDebug},
		{"form malformed", fmt.Errorf("%w: %s", form.ErrMalformed, secret), 400, web.MessageForm, slog.LevelDebug},
		{"form fields", form.ErrTooManyFields, 400, web.MessageForm, slog.LevelDebug},
		{"form file", form.ErrFile, 400, web.MessageForm, slog.LevelDebug},
		{"csrf", fmt.Errorf("%w: %s", session.ErrCSRF, secret), 403, web.MessageForged, slog.LevelDebug},
		{"cross origin", mw.ErrCrossOrigin, 403, web.MessageForged, slog.LevelDebug},
		{"canceled", fmt.Errorf("query: %w", context.Canceled), 503, web.MessageInterrupted, slog.LevelWarn},
		{"deadline", context.DeadlineExceeded, 503, web.MessageInterrupted, slog.LevelWarn},
		{"other", errors.New(secret), 500, web.DefaultMessage(500), slog.LevelError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			l := new(logs)
			res := cthttp.Serve(web.Errors{}.Handle(failWith(c.err)), request(t, l, http.MethodPost, "/tasks", nil))
			t.Must(res.Status(c.status))
			t.Must(res.HTML().Find(ct.Role("alert")).Texts(c.message))
			t.Must(be.False(strings.Contains(res.BodyString(), secret)))

			htmx := cthttp.Serve(web.Errors{}.Handle(failWith(c.err)), asHTMX(request(t, nil, http.MethodPost, "/tasks", nil), ""))
			t.Must(htmx.Status(c.status))
			t.Must(htmx.HTMLFragment("div").Find(ct.Role("alert")).Texts(c.message))

			recs := l.all()
			t.Must(be.Equal(1, len(recs)))
			t.Must(be.Equal(c.level, recs[0].level))
			t.Must(be.Equal("request failed", recs[0].msg))
		})
	}
}

func TestClassifyHook(gt *testing.T) {
	t := devtest.ParallelT(gt)
	errStale := errors.New("stale revision")
	errs := web.Errors{Classify: func(err error) *web.Error {
		switch {
		case errors.Is(err, errStale):
			return web.Conflict("The task changed meanwhile.", err)
		case errors.Is(err, session.ErrCSRF):
			return web.BadRequest("Reload the form.", err)
		}
		return nil
	}}
	cases := []struct {
		err     error
		status  int
		message string
	}{
		{fmt.Errorf("save: %w", errStale), 409, "The task changed meanwhile."},
		{session.ErrCSRF, 400, "Reload the form."},
		{form.ErrTooLarge, 413, web.DefaultMessage(413)},
		{web.NotFound("Gone.", nil), 404, "Gone."},
	}
	for _, c := range cases {
		res := cthttp.Serve(errs.Handle(failWith(c.err)), request(t, nil, http.MethodPost, "/", nil))
		t.Must(res.Status(c.status))
		t.Must(res.HTML().Find(ct.Role("alert")).Texts(c.message))
	}
}

func TestStreamWriteErrorIsSilent(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, wrote := range []bool{false, true} {
		l := new(logs)
		w := newFakeWriter()
		h := web.Errors{}.Handle(func(w http.ResponseWriter, r *http.Request) error {
			if wrote {
				_, _ = w.Write([]byte("data: x\n\n"))
			}
			return fmt.Errorf("send: %w", sse.ErrWrite)
		})
		h.ServeHTTP(w, request(t, l, http.MethodGet, "/live", nil))
		if wrote {
			t.Must(be.Equal("data: x\n\n", w.body.String()))
		} else {
			t.Must(be.Equal(0, w.status))
			t.Must(be.Equal(0, len(w.header)))
		}
		for _, rec := range l.all() {
			t.Must(be.Equal(slog.LevelDebug, rec.level))
		}
		t.Must(be.Equal(1, len(l.all())))
	}
}

func TestClientGone(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		err   error
		level slog.Level
	}{
		{context.Canceled, slog.LevelDebug},
		{web.NotFound("", nil), slog.LevelDebug},
		{errors.New("db failed"), slog.LevelError},
	}
	for _, c := range cases {
		l := new(logs)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		r := request(t, l, http.MethodGet, "/", nil).WithContext(mw.WithLogger(ctx, slog.New(l.handler())))
		w := newFakeWriter()
		web.Errors{}.Handle(failWith(c.err)).ServeHTTP(w, r)
		t.Must(be.Equal(0, w.status))
		t.Must(be.Equal("", w.body.String()))
		recs := l.all()
		t.Must(be.Equal(1, len(recs)))
		t.Must(be.Equal(c.level, recs[0].level))
		t.Must(be.Equal("request failed after the client went away", recs[0].msg))
	}
}

func TestErrorAfterResponseStarted(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := map[string]func(w http.ResponseWriter){
		"write":        func(w http.ResponseWriter) { _, _ = w.Write([]byte("partial")) },
		"write header": func(w http.ResponseWriter) { w.WriteHeader(http.StatusAccepted) },
		"flush":        func(w http.ResponseWriter) { _ = http.NewResponseController(w).Flush() },
		"flusher":      func(w http.ResponseWriter) { w.(http.Flusher).Flush() },
		"read from":    func(w http.ResponseWriter) { _, _ = io.Copy(w, strings.NewReader("copied")) },
	}
	for name, start := range cases {
		t.Run(name, func(t devtest.T) {
			l := new(logs)
			rec := httptest.NewRecorder()
			web.Errors{}.Handle(func(w http.ResponseWriter, r *http.Request) error {
				start(w)
				return errors.New("failed late")
			}).ServeHTTP(rec, request(t, l, http.MethodGet, "/", nil))
			t.Must(be.False(strings.Contains(rec.Body.String(), "alert")))
			recs := l.all()
			t.Must(be.Equal(1, len(recs)))
			t.Must(be.Equal("request failed after the response started", recs[0].msg))
			t.Must(be.Equal(slog.LevelError, recs[0].level))
		})
	}
}

func TestErrorAfterInformationalStatus(gt *testing.T) {
	t := devtest.ParallelT(gt)
	w := newFakeWriter()
	web.Errors{}.Handle(func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusEarlyHints)
		return web.NotFound("", nil)
	}).ServeHTTP(w, request(t, nil, http.MethodGet, "/", nil))
	// The fake keeps the first status; the error page still followed.
	t.Must(be.Substring(w.body.String(), `role="alert"`))
}

func TestWriterKeepsResponseController(gt *testing.T) {
	t := devtest.ParallelT(gt)
	w := newFakeWriter()
	deadline := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	web.Errors{}.Handle(func(w http.ResponseWriter, r *http.Request) error {
		rc := http.NewResponseController(w)
		if err := rc.SetWriteDeadline(deadline); err != nil {
			return err
		}
		if err := rc.EnableFullDuplex(); err != nil {
			return err
		}
		return web.Render(w, r, http.StatusOK, text.Text("ok"))
	}).ServeHTTP(w, request(t, nil, http.MethodGet, "/", nil))
	t.Must(be.Equal(deadline, w.deadline))
	t.Must(be.True(w.duplex))
	t.Must(be.Equal("ok", w.body.String()))
}

func TestWriterReadFromAndHijack(gt *testing.T) {
	t := devtest.ParallelT(gt)
	l := new(logs)
	rec := httptest.NewRecorder()
	web.Errors{}.Handle(func(w http.ResponseWriter, r *http.Request) error {
		if _, err := w.(io.ReaderFrom).ReadFrom(strings.NewReader("copied")); err != nil {
			return err
		}
		return errors.New("late")
	}).ServeHTTP(rec, request(t, l, http.MethodGet, "/", nil))
	t.Must(be.Equal("copied", rec.Body.String()))
	t.Must(be.Equal("request failed after the response started", l.all()[0].msg))

	// A recorder cannot be hijacked: the error is the handler's to answer,
	// and the response has not started.
	res := cthttp.Serve(web.Errors{}.Handle(func(w http.ResponseWriter, r *http.Request) error {
		_, _, err := w.(http.Hijacker).Hijack()
		return web.StatusError(http.StatusNotImplemented, "", err)
	}), request(t, nil, http.MethodGet, "/", nil))
	t.Must(res.Status(http.StatusNotImplemented))
}

func TestErrorResetsHeaders(gt *testing.T) {
	t := devtest.ParallelT(gt)
	h := web.Errors{}.Handle(func(w http.ResponseWriter, r *http.Request) error {
		hd := w.Header()
		hd.Set("Content-Type", "application/zip")
		hd.Set("Content-Disposition", `attachment; filename="tasks.zip"`)
		hd.Set("Content-Length", "123456")
		hd.Set("Content-Encoding", "gzip")
		hd.Set("ETag", `"v1"`)
		hd.Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
		hd.Set("Cache-Control", "public, max-age=3600")
		hd.Set("Location", "/elsewhere")
		hd["HX-Redirect"] = []string{"/elsewhere"} // not canonical
		hd.Set("HX-Location", "/elsewhere")
		hd.Set("Set-Cookie", "session=abc; HttpOnly")
		hd.Set("Content-Security-Policy", "default-src 'self'")
		return errors.New("export failed")
	})
	res := cthttp.Serve(h, request(t, nil, http.MethodGet, "/export", nil))
	t.Must(res.Status(http.StatusInternalServerError))
	t.Must(res.Header("Content-Type", ct.Exact(web.ContentType)))
	t.Must(res.Header("Cache-Control", ct.Exact("no-store")))
	for _, name := range []string{"Content-Disposition", "Content-Encoding", "ETag", "Last-Modified", "Location", "HX-Redirect", "HX-Location"} {
		t.Mustf(res.HeaderAbsent(name), "header %s", name)
	}
	t.Must(be.Equal("", res.Headers().Get("Content-Length")))
	t.Must(res.Header("Set-Cookie", ct.Exact("session=abc; HttpOnly")))
	t.Must(res.Header("Content-Security-Policy", ct.Exact("default-src 'self'")))
	t.Must(res.HTML().Find(ct.Role("alert")))
}

func TestErrorHooks(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var seen []web.ErrorView
	errs := web.Errors{
		Home:   "/app/",
		Target: "#errors",
		Document: func(r *http.Request, v web.ErrorView) elem.Node {
			seen = append(seen, v)
			l := v.Layout()
			l.Navbar = page.Navbar{Brand: page.Brand{Label: "Tasks", Href: "/app/"}}
			return page.Document{Title: v.Title + " · Tasks", Body: l}
		},
		Fragment: func(r *http.Request, v web.ErrorView) elem.Node {
			seen = append(seen, v)
			return page.ErrorAlert(text.Text(v.Title), text.Text(": "), text.Text(v.Message))
		},
	}
	h := errs.Handle(failWith(web.Forbidden("Only the owner can do this.", nil)))

	doc := cthttp.Serve(h, request(t, nil, http.MethodGet, "/tasks/7/edit", nil)).HTML()
	t.Must(doc.Find(ct.Tag("title")).Texts("403 Forbidden · Tasks"))
	t.Must(doc.Find(ct.Role("navigation", ct.Named("Main"))))
	t.Must(doc.Find(ct.Role("link", ct.Named(web.DefaultHomeLabel))).Matches(ct.Attr("href", "/app/")))

	res := cthttp.Serve(h, asHTMX(request(t, nil, http.MethodPost, "/tasks/7/edit", nil), ""))
	t.Must(res.Header("HX-Retarget", ct.Exact("#errors")))
	t.Must(res.Body(ct.Exact(`<div class="alert alert-danger" role="alert">403 Forbidden: Only the owner can do this.</div>`)))

	want := web.ErrorView{Status: 403, Title: "403 Forbidden", Message: "Only the owner can do this.", Home: "/app/"}
	t.Must(be.DeepEqual([]web.ErrorView{want, want}, seen))
}

func TestErrorHookRenderFails(gt *testing.T) {
	t := devtest.ParallelT(gt)
	l := new(logs)
	errs := web.Errors{
		Document: func(*http.Request, web.ErrorView) elem.Node { return failing() },
		Fragment: func(*http.Request, web.ErrorView) elem.Node { return failing() },
	}
	h := errs.Handle(failWith(web.BadRequest("Use <b> & co.", nil)))

	res := cthttp.Serve(h, request(t, l, http.MethodGet, "/", nil))
	t.Must(res.Status(http.StatusBadRequest))
	t.Must(res.Header("Content-Type", ct.Exact("text/plain; charset=utf-8")))
	t.Must(res.Header("X-Content-Type-Options", ct.Exact("nosniff")))
	t.Must(res.Body(ct.Exact("Use <b> & co.")))

	htmx := cthttp.Serve(h, asHTMX(request(t, nil, http.MethodGet, "/", nil), ""))
	t.Must(htmx.Status(http.StatusBadRequest))
	t.Must(htmx.Body(ct.Exact("Use &lt;b&gt; &amp; co.")))

	var failed int
	for _, rec := range l.all() {
		if rec.msg == "error page failed to render" {
			failed++
			t.Must(be.Equal(slog.LevelError, rec.level))
		}
	}
	t.Must(be.Equal(1, failed))
}

func TestDefaultDocumentAssets(gt *testing.T) {
	t := devtest.ParallelT(gt)
	set := asset.Must(asset.New(asset.Options{
		Prefix: "/assets/app/",
		FS:     fstest.MapFS{"app.css": {Data: []byte("p{}")}},
		Files:  []asset.File{{Name: "app.css"}},
	}))
	res := cthttp.Serve(web.Errors{Assets: []*asset.Set{set}}.NotFound(), request(t, nil, http.MethodGet, "/x", nil))
	t.Must(res.HTML().Find(ct.Attr("rel", "stylesheet")).Matches(ct.Attr("href", set.URL("app.css"))))
}

// TestLogsRouteNotPath checks that the error log names the route pattern,
// never the path, which may hold a capability.
func TestLogsRouteNotPath(gt *testing.T) {
	t := devtest.ParallelT(gt)
	l := new(logs)
	errs := web.Errors{}
	mux := http.NewServeMux()
	mux.Handle("GET /share/{token}", errs.Handle(failWith(errors.New("lookup failed"))))
	mux.Handle("/", errs.NotFound())

	res := cthttp.Serve(mux, request(t, l, http.MethodGet, "/share/CAPABILITY-TOKEN?x=QUERY-VALUE", nil))
	t.Must(res.Status(http.StatusInternalServerError))
	res = cthttp.Serve(mux, request(t, l, http.MethodGet, "/missing/PATH-VALUE", nil))
	t.Must(res.Status(http.StatusNotFound))
	t.Must(be.False(strings.Contains(res.BodyString(), "PATH-VALUE")))

	recs := l.all()
	t.Must(be.Equal(2, len(recs)))
	t.Must(be.Equal("GET /share/{token}", recs[0].attrs["route"]))
	t.Must(be.Equal("/", recs[1].attrs["route"]))
	logText := l.text()
	for _, leaked := range []string{"CAPABILITY-TOKEN", "QUERY-VALUE", "PATH-VALUE"} {
		t.Mustf(be.False(strings.Contains(logText, leaked)), "log contains %s", leaked)
	}
}

func TestWriteNilError(gt *testing.T) {
	t := devtest.ParallelT(gt)
	w := newFakeWriter()
	web.Errors{}.Write(w, request(t, nil, http.MethodGet, "/", nil), nil)
	t.Must(be.Equal(0, w.status))
}

func TestRecover(gt *testing.T) {
	t := devtest.ParallelT(gt)
	l := new(logs)
	h := web.Errors{}.Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", "attachment")
		panic("broken invariant")
	}))
	res := cthttp.Serve(h, request(t, l, http.MethodGet, "/tasks", nil))
	t.Must(res.Status(http.StatusInternalServerError))
	t.Must(res.HeaderAbsent("Content-Disposition"))
	t.Must(res.HTML().Find(ct.Role("alert")).Texts(web.DefaultMessage(500)))
	t.Must(be.False(strings.Contains(res.BodyString(), "broken invariant")))

	recs := l.all()
	t.Must(be.Equal(1, len(recs)))
	t.Must(be.Equal("handler panic", recs[0].msg))
	t.Must(be.Equal(slog.LevelError, recs[0].level))
	t.Must(be.Equal("broken invariant", recs[0].attrs["panic"]))
	t.Must(be.Substring(recs[0].attrs["stack"], "goroutine"))

	htmx := cthttp.Serve(h, asHTMX(request(t, nil, http.MethodPost, "/tasks", nil), ""))
	t.Must(htmx.Status(http.StatusInternalServerError))
	t.Must(htmx.Header("HX-Retarget", ct.Exact("#alerts")))
}

// recovered runs fn and returns the value it panics with.
func recovered(fn func()) (v any) {
	defer func() { v = recover() }()
	fn()
	return nil
}

func TestRecoverAbort(gt *testing.T) {
	t := devtest.ParallelT(gt)
	l := new(logs)
	abort := web.Errors{}.Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	rec := httptest.NewRecorder()
	v := recovered(func() { abort.ServeHTTP(rec, request(t, l, http.MethodGet, "/", nil)) })
	t.Must(be.True(v == http.ErrAbortHandler))
	t.Must(be.Equal(0, len(l.all())))
	t.Must(be.Equal("", rec.Body.String()))

	late := web.Errors{}.Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<p>half"))
		panic("late")
	}))
	rec = httptest.NewRecorder()
	v = recovered(func() { late.ServeHTTP(rec, request(t, l, http.MethodGet, "/", nil)) })
	t.Must(be.True(v == http.ErrAbortHandler))
	t.Must(be.Equal("<p>half", rec.Body.String()))
	t.Must(be.Equal("handler panic", l.all()[0].msg))
}

func TestRecoverWithHandle(gt *testing.T) {
	t := devtest.ParallelT(gt)
	errs := web.Errors{}
	h := errs.Recover(errs.Handle(func(w http.ResponseWriter, r *http.Request) error {
		var m map[string]int
		m["x"] = 1 // a runtime error
		return nil
	}))
	res := cthttp.Serve(h, request(t, nil, http.MethodGet, "/", nil))
	t.Must(res.Status(http.StatusInternalServerError))
}

func TestPanicsOnMisuse(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Panic(func() { web.Errors{}.Handle(nil) }))
	t.Must(be.Panic(func() { web.Errors{}.Recover(nil) }))
	t.Must(be.Panic(func() { web.Errors{}.RequireAuth("/login", nil) }))
	t.Must(be.Panic(func() {
		web.Errors{}.RequireAuth("https://idp.example/login", func(*http.Request) bool { return true })
	}))
	t.Must(be.Panic(func() { web.Errors{}.RequireAuth("", func(*http.Request) bool { return true }) }))
}

func TestErrorType(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cause := errors.New("store down")
	cases := []struct {
		err    *web.Error
		status int
		text   string
	}{
		{web.BadRequest("Bad.", nil), 400, "web: status 400: Bad."},
		{web.Forbidden("", cause), 403, "web: status 403: store down"},
		{web.NotFound("", nil), 404, "web: status 404"},
		{web.Conflict("Changed.", cause), 409, "web: status 409: store down"},
		{web.Internal(cause), 500, "web: status 500: store down"},
		{web.StatusError(429, "Slow down.", nil), 429, "web: status 429: Slow down."},
	}
	for _, c := range cases {
		t.Must(be.Equal(c.status, c.err.Status))
		t.Must(be.Equal(c.text, c.err.Error()))
	}
	t.Must(be.ErrorIs(web.Internal(cause), cause))
	t.Must(be.ErrorIs(fmt.Errorf("x: %w", web.NotFound("", cause)), cause))
	var nilErr *web.Error
	t.Must(be.Equal("web: <nil>", nilErr.Error()))
	t.Must(be.Nil(nilErr.Unwrap()))
}

func TestDefaultMessages(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for status := 400; status <= 599; status++ {
		msg := web.DefaultMessage(status)
		t.Mustf(be.True(strings.HasSuffix(msg, ".")), "message of %d", status)
		t.Mustf(be.True(len(msg) > 0 && msg[0] >= 'A' && msg[0] <= 'Z'), "message of %d", status)
	}
	t.Must(be.Equal("The request failed.", web.DefaultMessage(200)))
	t.Must(be.Equal("The request failed.", web.DefaultMessage(599)))
	t.Must(be.Equal("404 Not Found", web.StatusTitle(404)))
	t.Must(be.Equal("Error 599", web.StatusTitle(599)))
}

func TestRecoverPassesThrough(gt *testing.T) {
	t := devtest.ParallelT(gt)
	h := web.Errors{}.Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fine"))
	}))
	res := cthttp.Serve(h, request(t, nil, http.MethodGet, "/", nil))
	t.Must(res.Status(http.StatusOK))
	t.Must(res.Body(ct.Exact("fine")))
}

// unwrapper is a middleware response writer that unwraps to the writer it
// wraps, like the writers of the mw package.
type unwrapper struct{ http.ResponseWriter }

func (u unwrapper) Unwrap() http.ResponseWriter { return u.ResponseWriter }

// cycle is a broken writer that unwraps to itself.
type cycle struct{ *fakeWriter }

func (c cycle) Unwrap() http.ResponseWriter { return c }

// TestWriteFindsTrackerThroughWrappers checks that Errors.Write, called
// directly with a writer that wraps the one of Recover, knows that the
// response started.
func TestWriteFindsTrackerThroughWrappers(gt *testing.T) {
	t := devtest.ParallelT(gt)
	l := new(logs)
	errs := web.Errors{}
	h := errs.Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w = unwrapper{w}
		_, _ = w.Write([]byte("partial"))
		errs.Write(w, r, errors.New("late"))
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, request(t, l, http.MethodGet, "/", nil))
	t.Must(be.Equal("partial", rec.Body.String()))
	t.Must(be.Equal("request failed after the response started", l.all()[0].msg))

	// Without a tracker, Write assumes the response has not started, also
	// for a writer whose Unwrap never ends.
	w := cycle{newFakeWriter()}
	errs.Write(w, request(t, nil, http.MethodGet, "/", nil), web.NotFound("", nil))
	t.Must(be.Equal(http.StatusNotFound, w.status))
}

// hijacker is a writer whose connection can be taken over.
type hijacker struct {
	*fakeWriter
	conn net.Conn
}

func (h hijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return h.conn, bufio.NewReadWriter(bufio.NewReader(h.conn), bufio.NewWriter(h.conn)), nil
}

func TestErrorAfterHijack(gt *testing.T) {
	t := devtest.ParallelT(gt)
	l := new(logs)
	server, client := net.Pipe()
	defer func() { _ = client.Close() }()
	w := hijacker{fakeWriter: newFakeWriter(), conn: server}
	web.Errors{}.Handle(func(w http.ResponseWriter, r *http.Request) error {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return err
		}
		_ = conn.Close()
		return errors.New("protocol failed")
	}).ServeHTTP(w, request(t, l, http.MethodGet, "/ws", nil))
	t.Must(be.Equal(0, w.status))
	t.Must(be.Equal("request failed after the response started", l.all()[0].msg))
}
