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
	"sync/atomic"
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
	"github.com/protolambda/webui/hx2"
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
	t.Must(res.Header("Vary", ct.Exact("HX-History-Restore-Request")))
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
		w.Header().Set("HX-Reselect", "#main")
		return web.Conflict("", nil)
	})
	res := cthttp.Serve(h, asHTMX(request(t, nil, http.MethodPost, "/tasks/7", nil), "https://app.example/tasks"))
	t.Must(res.Status(http.StatusConflict))
	t.Must(res.Header("HX-Retarget", ct.Exact("#"+page.AlertsID)))
	// htmx scrolls the alerts region into view, as the control that failed
	// may be far below it.
	t.Must(res.Header("HX-Reswap", ct.Exact("innerHTML show:top")))
	t.Must(be.Equal(hx2.SwapInnerHTML+" show:top", web.ErrorReswap))
	// An hx-select that the page puts on the trigger would filter the
	// alert out: the answer turns it off.
	t.Must(res.Header("HX-Reselect", ct.Exact("unset")))
	t.Must(res.Header("HX-Push-Url", ct.Exact("false")))
	t.Must(res.HeaderAbsent("HX-Trigger"))
	t.Must(res.Header("Vary", ct.Exact("HX-Request")))
	t.Must(res.Header("Vary", ct.Exact("HX-History-Restore-Request")))
	t.Must(res.Body(ct.Exact(`<div class="alert alert-danger">` + web.DefaultMessage(http.StatusConflict) + `</div>`)))
	// The alerts region is a live region, which announces the alert: the
	// alert has no role of its own, which would nest a second one.
	frag := res.HTMLFragment("div")
	t.Must(frag.Find(ct.Class("alert-danger")).Texts(web.DefaultMessage(http.StatusConflict)))
	t.Must(frag.Find(ct.Role("alert")).None())
	t.Must(frag.Find(ct.Attr("aria-live", "")).None())
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
			t.Must(htmx.HTMLFragment("div").Find(ct.Class("alert-danger")).Texts(c.message))

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

func TestFallbackHook(gt *testing.T) {
	t := devtest.ParallelT(gt)
	errStale := errors.New("stale revision")
	var fallbackSaw []error
	errs := web.Errors{
		Classify: func(err error) *web.Error {
			if errors.Is(err, errStale) {
				return web.Conflict("The task changed meanwhile.", err)
			}
			return nil
		},
		Fallback: func(err error) *web.Error {
			fallbackSaw = append(fallbackSaw, err)
			if errors.Is(err, errKeep) {
				return nil
			}
			return web.BadRequest("The request was not understood.", err)
		},
	}
	unknown := errors.New("parse revision")
	cases := []struct {
		err     error
		status  int
		message string
	}{
		{unknown, 400, "The request was not understood."},
		{errKeep, 500, web.DefaultMessage(500)},
		// Classify, an *Error and webui's own classes come first.
		{errStale, 409, "The task changed meanwhile."},
		{web.Internal(unknown), 500, web.DefaultMessage(500)},
		{form.ErrTooLarge, 413, web.DefaultMessage(413)},
		{fmt.Errorf("read: %w", form.ErrMalformed), 400, web.MessageForm},
		{session.ErrCSRF, 403, web.MessageForged},
		{mw.ErrCrossOrigin, 403, web.MessageForged},
		{context.DeadlineExceeded, 503, web.MessageInterrupted},
	}
	for _, c := range cases {
		res := cthttp.Serve(errs.Handle(failWith(c.err)), request(t, nil, http.MethodPost, "/", nil))
		t.Must(res.Status(c.status))
		t.Must(res.HTML().Find(ct.Role("alert")).Texts(c.message))
	}
	t.Must(be.DeepEqual([]error{unknown, errKeep}, fallbackSaw))

	// sse.ErrWrite stays silent: nothing is written.
	w := newFakeWriter()
	errs.Write(w, request(t, nil, http.MethodGet, "/events", nil), fmt.Errorf("tick: %w", sse.ErrWrite))
	t.Must(be.Equal(0, w.status))
	t.Must(be.Len(fallbackSaw, 2))
}

// errKeep is an error that the Fallback of TestFallbackHook leaves to the
// default 500.
var errKeep = errors.New("store closed")

// archived is an application error that the error page renders an answer
// of its own for.
type archived struct{ project string }

func (e *archived) Error() string { return "project " + e.project + " is archived" }

func TestErrorViewErr(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var seen []error
	errs := web.Errors{
		Classify: func(err error) *web.Error {
			if a := (*archived)(nil); errors.As(err, &a) {
				return web.Conflict("The project is archived.", err)
			}
			return nil
		},
		Document: func(r *http.Request, v web.ErrorView) elem.Node {
			seen = append(seen, v.Err)
			l := v.Layout()
			if a := (*archived)(nil); errors.As(v.Err, &a) {
				l.Main = text.P()(text.A(text.Href("/archive"))(text.Text("Open the archive")))
			}
			return page.Document{Title: v.Title, Body: l}
		},
		Fragment: func(r *http.Request, v web.ErrorView) elem.Node {
			seen = append(seen, v.Err)
			return v.Alert()
		},
	}
	err := fmt.Errorf("rename: %w", &archived{project: "PRIVATE-NAME"})
	h := errs.Handle(failWith(err))

	res := cthttp.Serve(h, request(t, nil, http.MethodGet, "/projects/7", nil))
	t.Must(res.Status(http.StatusConflict))
	doc := res.HTML()
	t.Must(doc.Find(ct.Role("alert")).Texts("The project is archived."))
	t.Must(doc.Find(ct.Role("link", ct.Named("Open the archive"))))
	t.Must(be.False(strings.Contains(res.BodyString(), "PRIVATE-NAME")))

	t.Must(cthttp.Serve(h, asHTMX(request(t, nil, http.MethodPost, "/projects/7", nil), "")).Status(http.StatusConflict))
	t.Must(be.DeepEqual([]error{err, err}, seen))

	// Other errors do not match, and a panic and the replies of Intercept
	// have no error.
	seen = nil
	cthttp.Serve(errs.Handle(failWith(errors.New("disk full"))), request(t, nil, http.MethodGet, "/", nil))
	cthttp.Serve(errs.Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("broken") })), request(t, nil, http.MethodGet, "/", nil))
	cthttp.Serve(errs.Intercept(http.NotFoundHandler()), asHTMX(request(t, nil, http.MethodGet, "/", nil), ""))
	t.Must(be.Len(seen, 3))
	t.Must(be.Equal(errors.New("disk full").Error(), seen[0].Error()))
	t.Must(be.Nil(seen[1]))
	t.Must(be.Nil(seen[2]))
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
	// net/http cancels the request's context when the client goes away;
	// contexts that handlers or middleware derive from it report the same,
	// a deadline or a cause of their own included.
	derive := map[string]func(ctx context.Context) context.Context{
		"request": func(ctx context.Context) context.Context { return ctx },
		"with cause": func(ctx context.Context) context.Context {
			ctx, cancel := context.WithCancelCause(ctx)
			t.Cleanup(func() { cancel(nil) })
			return ctx
		},
		"with timeout": func(ctx context.Context) context.Context {
			ctx, cancel := context.WithTimeout(ctx, time.Hour)
			t.Cleanup(cancel)
			return ctx
		},
	}
	for name, derived := range derive {
		for _, c := range cases {
			l := new(logs)
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			ctx = derived(ctx)
			r := request(t, l, http.MethodGet, "/", nil).WithContext(mw.WithLogger(ctx, slog.New(l.handler())))
			w := newFakeWriter()
			web.Errors{}.Handle(failWith(c.err)).ServeHTTP(w, r)
			t.Mustf(be.Equal(0, w.status), "%s: %v", name, c.err)
			t.Must(be.Equal("", w.body.String()))
			recs := l.all()
			t.Must(be.Equal(1, len(recs)))
			t.Must(be.Equal(c.level, recs[0].level))
			t.Must(be.Equal("request failed after the client went away", recs[0].msg))
		}
	}
}

// TestServerSideDeadline checks that a request whose context ended on the
// server's side, past a deadline that a timeout middleware set or canceled
// with the application's own cause, is answered: its client still waits.
func TestServerSideDeadline(gt *testing.T) {
	t := devtest.ParallelT(gt)
	ended := map[string]func(ctx context.Context) context.Context{
		"deadline": func(ctx context.Context) context.Context {
			ctx, cancel := context.WithDeadline(ctx, time.Unix(0, 0))
			t.Cleanup(cancel)
			return ctx
		},
		"deadline with cause": func(ctx context.Context) context.Context {
			ctx, cancel := context.WithDeadlineCause(ctx, time.Unix(0, 0), errors.New("too slow"))
			t.Cleanup(cancel)
			return ctx
		},
		"cancel cause": func(ctx context.Context) context.Context {
			ctx, cancel := context.WithCancelCause(ctx)
			cancel(errors.New("shutting down"))
			return ctx
		},
	}
	cases := []struct {
		name    string
		err     func(r *http.Request) error
		status  int
		message string
		level   slog.Level
	}{
		{"context error", func(r *http.Request) error { return fmt.Errorf("query: %w", r.Context().Err()) }, 503, web.MessageInterrupted, slog.LevelWarn},
		{"other error", func(*http.Request) error { return errors.New("db failed") }, 500, web.DefaultMessage(500), slog.LevelError},
		{"public error", func(*http.Request) error { return web.NotFound("", nil) }, 404, web.DefaultMessage(404), slog.LevelDebug},
	}
	for name, end := range ended {
		for _, c := range cases {
			t.Run(name+"/"+c.name, func(t devtest.T) {
				timeout := func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						next.ServeHTTP(w, r.WithContext(end(r.Context())))
					})
				}
				l := new(logs)
				h := timeout(web.Errors{}.Handle(func(w http.ResponseWriter, r *http.Request) error {
					<-r.Context().Done()
					return c.err(r)
				}))

				res := cthttp.Serve(h, request(t, l, http.MethodGet, "/tasks", nil))
				t.Must(res.Status(c.status))
				t.Must(res.Header("Content-Type", ct.Exact(web.ContentType)))
				t.Must(res.HTML().Find(ct.ID(page.AlertsID)).Find(ct.Role("alert")).Texts(c.message))

				htmx := cthttp.Serve(h, asHTMX(request(t, l, http.MethodPost, "/tasks", nil), ""))
				t.Must(htmx.Status(c.status))
				t.Must(htmx.Header("HX-Retarget", ct.Exact("#"+page.AlertsID)))
				t.Must(htmx.HTMLFragment("div").Find(ct.Class("alert-danger")).Texts(c.message))

				// One record per request, none about a failed render.
				recs := l.all()
				t.Must(be.Equal(2, len(recs)))
				for _, rec := range recs {
					t.Must(be.Equal("request failed", rec.msg))
					t.Must(be.Equal(c.level, rec.level))
				}
			})
		}
	}
}

// TestErrorDocumentKeepsContextValues checks that the error page of a
// request past its deadline renders with the request context's values.
func TestErrorDocumentKeepsContextValues(gt *testing.T) {
	t := devtest.ParallelT(gt)
	type key struct{}
	errs := web.Errors{Document: func(r *http.Request, v web.ErrorView) elem.Node {
		return elem.Fn(func(ctx context.Context) (elem.Node, error) {
			return text.P()(text.Text(ctx.Value(key{}).(string))), nil
		})
	}}
	ctx, cancel := context.WithDeadline(context.WithValue(t.Context(), key{}, "from the request"), time.Unix(0, 0))
	defer cancel()
	r := request(t, nil, http.MethodGet, "/", nil)
	r = r.WithContext(mw.WithLogger(ctx, mw.Logger(r.Context())))
	res := cthttp.Serve(errs.Handle(failWith(ctx.Err())), r)
	t.Must(res.Status(http.StatusServiceUnavailable))
	t.Must(res.Body(ct.Exact("<p>from the request</p>")))
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
		Reswap: hx2.SwapAfterBegin,
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
	forbidden := web.Forbidden("Only the owner can do this.", nil)
	h := errs.Handle(failWith(forbidden))

	doc := cthttp.Serve(h, request(t, nil, http.MethodGet, "/tasks/7/edit", nil)).HTML()
	t.Must(doc.Find(ct.Tag("title")).Texts("403 Forbidden · Tasks"))
	t.Must(doc.Find(ct.Role("navigation", ct.Named("Main"))))
	t.Must(doc.Find(ct.Role("link", ct.Named(web.DefaultHomeLabel))).Matches(ct.Attr("href", "/app/")))

	res := cthttp.Serve(h, asHTMX(request(t, nil, http.MethodPost, "/tasks/7/edit", nil), ""))
	t.Must(res.Header("HX-Retarget", ct.Exact("#errors")))
	t.Must(res.Header("HX-Reswap", ct.Exact("afterbegin")))
	t.Must(res.Body(ct.Exact(`<div class="alert alert-danger">403 Forbidden: Only the owner can do this.</div>`)))

	want := web.ErrorView{Status: 403, Title: "403 Forbidden", Message: "Only the owner can do this.", Home: "/app/", Err: forbidden}
	t.Must(be.DeepEqual([]web.ErrorView{want, want}, seen))
}

// TestTargetFor checks that TargetFor chooses the target of every htmx
// answer of the policy, with Target as the fallback.
func TestTargetFor(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var calls atomic.Int32
	dialogAlerts := map[string]string{"rename-form": "#rename-alerts"}
	errs := web.Errors{
		Target: "#errors",
		TargetFor: func(r *http.Request) string {
			calls.Add(1)
			return dialogAlerts[hx2.ParseRequestHeaders(r.Header).Trigger]
		},
	}
	fromDialog := func(method, target string) *http.Request {
		r := asHTMX(request(t, nil, method, target, nil), "https://app.example/projects")
		r.Header.Set(hx2.HeaderTrigger, "rename-form")
		return r
	}
	mux := http.NewServeMux()
	mux.Handle("POST /rename", errs.Handle(failWith(web.Conflict("", nil))))
	mux.HandleFunc("POST /boom", func(http.ResponseWriter, *http.Request) { panic("boom") })
	mux.HandleFunc("POST /plain", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "plain", http.StatusBadRequest)
	})
	h := mw.Chain(mux, errs.Recover, errs.Intercept)

	for _, path := range []string{"/rename", "/boom", "/plain"} {
		res := cthttp.Serve(h, fromDialog(http.MethodPost, path))
		t.Mustf(res.Header("HX-Retarget", ct.Exact("#rename-alerts")), "%s from the dialog", path)

		// Another element, or a forged trigger, gets the fallback.
		r := fromDialog(http.MethodPost, path)
		r.Header.Set(hx2.HeaderTrigger, `x"><i hx-get=/x>`)
		res = cthttp.Serve(h, r)
		t.Mustf(res.Header("HX-Retarget", ct.Exact("#errors")), "%s from elsewhere", path)
	}
	t.Must(be.Equal(int32(6), calls.Load()))

	// Plain requests get the error page; TargetFor is not asked.
	res := cthttp.Serve(h, request(t, nil, http.MethodPost, "/rename", nil))
	t.Must(res.Status(http.StatusConflict))
	t.Must(res.HeaderAbsent("HX-Retarget"))
	t.Must(be.Equal(int32(6), calls.Load()))

	// Without Target, an empty choice means the page's alerts region.
	errs.Target = ""
	res = cthttp.Serve(errs.Handle(failWith(web.Conflict("", nil))), asHTMX(request(t, nil, http.MethodPost, "/", nil), ""))
	t.Must(res.Header("HX-Retarget", ct.Exact("#"+page.AlertsID)))
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

// TestLogsPatternNotPath checks that the error log names the route pattern,
// through the request logger of mw.LogRequests, and never the path, which
// may hold a capability.
func TestLogsPatternNotPath(gt *testing.T) {
	t := devtest.ParallelT(gt)
	l := new(logs)
	errs := web.Errors{}
	mux := http.NewServeMux()
	mux.Handle("GET /share/{token}", errs.Handle(failWith(errors.New("lookup failed"))))
	mux.Handle("GET /panic", http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("broken") }))
	mux.Handle("/", errs.NotFound())
	// A middleware that derives a new request loses r.Pattern: the request
	// logger keeps it.
	derive := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(r.Context()))
		})
	}
	h := mw.Chain(mux, mw.LogRequests(mw.RequestLog{Routes: mux}), errs.Recover, derive)

	res := cthttp.Serve(h, request(t, l, http.MethodGet, "/share/CAPABILITY-TOKEN?x=QUERY-VALUE", nil))
	t.Must(res.Status(http.StatusInternalServerError))
	res = cthttp.Serve(h, request(t, l, http.MethodGet, "/missing/PATH-VALUE", nil))
	t.Must(res.Status(http.StatusNotFound))
	t.Must(be.False(strings.Contains(res.BodyString(), "PATH-VALUE")))
	res = cthttp.Serve(h, request(t, l, http.MethodGet, "/panic?x=QUERY-VALUE", nil))
	t.Must(res.Status(http.StatusInternalServerError))

	want := map[string]string{
		"request failed": "",
		"handler panic":  "GET /panic",
	}
	var failed []string
	for _, rec := range l.all() {
		if rec.msg == "http request" {
			continue
		}
		_, ok := want[rec.msg]
		t.Mustf(be.True(ok), "record %q", rec.msg)
		_, hasRoute := rec.attrs["route"]
		t.Must(be.False(hasRoute))
		t.Must(be.Equal("GET", rec.attrs["method"]))
		if rec.msg == "request failed" {
			failed = append(failed, rec.attrs["pattern"])
		} else {
			t.Must(be.Equal(want[rec.msg], rec.attrs["pattern"]))
		}
	}
	t.Must(be.DeepEqual([]string{"GET /share/{token}", "/"}, failed))
	logText := l.text()
	for _, leaked := range []string{"CAPABILITY-TOKEN", "QUERY-VALUE", "PATH-VALUE"} {
		t.Mustf(be.False(strings.Contains(logText, leaked)), "log contains %s", leaked)
	}
}

func TestNotFoundAnswers(gt *testing.T) {
	t := devtest.ParallelT(gt)
	h := web.Errors{}.NotFound()
	res := cthttp.Serve(h, request(t, nil, http.MethodGet, "/projects/PATH-VALUE", nil))
	t.Must(res.Status(http.StatusNotFound))
	t.Must(res.HTML().Find(ct.Role("alert")).Texts(web.DefaultMessage(http.StatusNotFound)))
	t.Must(be.False(strings.Contains(res.BodyString(), "PATH-VALUE")))

	// A boosted link to a missing page shows the alert on the current page.
	r := asHTMX(request(t, nil, http.MethodGet, "/projects/PATH-VALUE", nil), "https://app.example/tasks")
	r.Header.Set(hx2.HeaderBoosted, "true")
	htmx := cthttp.Serve(h, r)
	t.Must(htmx.Status(http.StatusNotFound))
	t.Must(htmx.Header("HX-Retarget", ct.Exact("#"+page.AlertsID)))
	t.Must(htmx.Header("HX-Reswap", ct.Exact(web.ErrorReswap)))
	t.Must(htmx.Header("HX-Reselect", ct.Exact("unset")))
	t.Must(htmx.Header("HX-Push-Url", ct.Exact("false")))
	t.Must(htmx.HTMLFragment("div").Find(ct.Class("alert-danger")).Texts(web.DefaultMessage(http.StatusNotFound)))
}

func TestMux(gt *testing.T) {
	t := devtest.ParallelT(gt)
	errs := web.Errors{}
	mux := http.NewServeMux()
	mux.Handle("GET /tasks/{id}", errs.Handle(func(w http.ResponseWriter, r *http.Request) error {
		return web.Render(w, r, http.StatusOK, text.Text("task "+r.PathValue("id")))
	}))
	mux.Handle("POST /tasks", errs.Handle(failWith(web.Conflict("", nil))))
	mux.Handle("GET /docs/", errs.Handle(func(w http.ResponseWriter, r *http.Request) error {
		return web.Render(w, r, http.StatusOK, text.Text("docs"))
	}))
	h := errs.Mux(mux)

	// Routes are served by the mux, with their path values.
	res := cthttp.Serve(h, request(t, nil, http.MethodGet, "/tasks/7", nil))
	t.Must(res.Status(http.StatusOK))
	t.Must(res.Body(ct.Exact("task 7")))
	t.Must(cthttp.Serve(h, request(t, nil, http.MethodPost, "/tasks", nil)).Status(http.StatusConflict))

	// No route: the policy's 404 page, which does not name the path.
	res = cthttp.Serve(h, request(t, nil, http.MethodGet, "/PATH-VALUE", nil))
	t.Must(res.Status(http.StatusNotFound))
	t.Must(res.Header("Content-Type", ct.Exact(web.ContentType)))
	t.Must(res.HTML().Find(ct.Role("alert")).Texts(web.DefaultMessage(http.StatusNotFound)))
	t.Must(be.False(strings.Contains(res.BodyString(), "PATH-VALUE")))

	// The wrong method: the policy's 405, with the mux's Allow.
	res = cthttp.Serve(h, request(t, nil, http.MethodDelete, "/tasks/7", nil))
	t.Must(res.Status(http.StatusMethodNotAllowed))
	t.Must(res.Header("Allow", ct.Exact("GET, HEAD")))
	t.Must(res.HTML().Find(ct.Role("alert")).Texts(web.DefaultMessage(http.StatusMethodNotAllowed)))
	res = cthttp.Serve(h, asHTMX(request(t, nil, http.MethodPut, "/tasks", nil), ""))
	t.Must(res.Status(http.StatusMethodNotAllowed))
	t.Must(res.Header("Allow", ct.Exact("POST")))
	t.Must(res.Header("HX-Retarget", ct.Exact("#"+page.AlertsID)))
	t.Must(res.HTMLFragment("div").Find(ct.Class("alert-danger")).Texts(web.DefaultMessage(http.StatusMethodNotAllowed)))

	// The mux's redirects pass, also to a cleaned path without a route.
	for target, location := range map[string]string{"/docs": "/docs/", "/a/../nope": "/nope"} {
		want := httptest.NewRecorder()
		mux.ServeHTTP(want, request(t, nil, http.MethodGet, target, nil))
		res = cthttp.Serve(h, request(t, nil, http.MethodGet, target, nil))
		t.Must(be.True(want.Code >= 300 && want.Code < 400))
		t.Must(res.Status(want.Code))
		t.Must(res.Header("Location", ct.Exact(location)))
	}

	// The request for the whole server is the mux's.
	r := request(t, nil, http.MethodOptions, "/", nil)
	r.RequestURI = "*"
	t.Must(cthttp.Serve(h, r).Status(http.StatusBadRequest))
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
	// panic is the flag of mw.LogRequests' request record too; the value
	// has a key of its own.
	t.Must(be.Equal("true", recs[0].attrs["panic"]))
	t.Must(be.Equal("broken invariant", recs[0].attrs["panic_value"]))
	t.Must(be.Substring(recs[0].attrs["stack"], "goroutine"))

	htmx := cthttp.Serve(h, asHTMX(request(t, nil, http.MethodPost, "/tasks", nil), ""))
	t.Must(htmx.Status(http.StatusInternalServerError))
	t.Must(htmx.Header("HX-Retarget", ct.Exact("#alerts")))
	t.Must(htmx.Header("HX-Reselect", ct.Exact("unset")))
	t.Must(htmx.Header("HX-Push-Url", ct.Exact("false")))
}

// TestRecoverLogsValueAsText checks that panic_value is text whatever the
// type of the panic value, so that it has one type in every record.
func TestRecoverLogsValueAsText(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, c := range []struct {
		value any
		want  string
	}{
		{"broken", "broken"},
		{errors.New("store closed"), "store closed"},
		{42, "42"},
		{struct{ ID int }{7}, "{7}"},
	} {
		l := new(logs)
		h := web.Errors{}.Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(c.value) }))
		res := cthttp.Serve(h, request(t, l, http.MethodGet, "/", nil))
		t.Must(res.Status(http.StatusInternalServerError))
		recs := l.all()
		t.Must(be.Equal(1, len(recs)))
		t.Mustf(be.Equal(c.want, recs[0].attrs["panic_value"]), "panic value %v", c.value)
		t.Must(be.Equal("true", recs[0].attrs["panic"]))
	}
}

// TestRecoverPanicKeyMatchesRequestLog checks that the panic record of
// Recover and the request record of mw.LogRequests give the key panic the
// same meaning, for a panic after the response started, which both log.
func TestRecoverPanicKeyMatchesRequestLog(gt *testing.T) {
	t := devtest.ParallelT(gt)
	l := new(logs)
	h := mw.Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "half")
		panic("late")
	}), mw.LogRequests(mw.RequestLog{Logger: slog.New(l.handler())}), web.Errors{}.Recover)
	v := recovered(func() {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	})
	t.Must(be.True(v == http.ErrAbortHandler))
	recs := l.all()
	t.Must(be.Equal(2, len(recs)))
	for _, rec := range recs {
		t.Mustf(be.Equal("true", rec.attrs["panic"]), "record %q", rec.msg)
	}
	t.Must(be.Equal("late", recs[0].attrs["panic_value"]))
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
	t.Must(be.Panic(func() { web.Errors{}.Intercept(nil) }))
	t.Must(be.Panic(func() { web.Errors{}.Answer(nil) }))
	t.Must(be.Panic(func() { web.Errors{}.Mux(nil) }))
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

// TestErrorAfterCopyOfNothing checks that a copy that copies nothing does
// not start the response: the error that follows gets its error page, on a
// real server as on a recorder.
func TestErrorAfterCopyOfNothing(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for name, copyNothing := range copiesNothing {
		h := web.Errors{}.Handle(copyNothing)
		for _, onServer := range []bool{false, true} {
			l := new(logs)
			var res *cthttp.Result
			if onServer {
				res = serveHTTP(t, l, h, request(t, nil, http.MethodGet, hostileQuery, nil))
			} else {
				res = cthttp.Serve(h, request(t, l, http.MethodGet, hostileQuery, nil))
			}
			t.Mustf(res.Status(http.StatusInternalServerError), "%s (server %t): status", name, onServer)
			t.Mustf(res.HTML().Find(ct.Role("alert")).Texts(web.DefaultMessage(500)), "%s (server %t): page", name, onServer)
			t.Mustf(be.False(strings.Contains(res.BodyString(), "hx-post")), "%s (server %t): body", name, onServer)
			recs := l.all()
			t.Mustf(be.Equal(1, len(recs)), "%s (server %t): records", name, onServer)
			t.Mustf(be.Equal("request failed", recs[0].msg), "%s (server %t): record", name, onServer)
		}
	}
}

func TestErrorAfterFlushes(gt *testing.T) {
	t := devtest.ParallelT(gt)
	// A writer that cannot flush starts nothing.
	l := new(logs)
	w := newFakeWriter()
	web.Errors{}.Handle(func(w http.ResponseWriter, r *http.Request) error {
		if err := http.NewResponseController(w).Flush(); !errors.Is(err, http.ErrNotSupported) {
			return fmt.Errorf("unexpected flush result: %w", err)
		}
		w.(http.Flusher).Flush()
		return web.NotFound("", nil)
	}).ServeHTTP(w, request(t, l, http.MethodGet, "/", nil))
	t.Must(be.Equal(http.StatusNotFound, w.status))
	t.Must(be.Substring(w.body.String(), `role="alert"`))
	t.Must(be.Equal("request failed", l.all()[0].msg))

	// A flush that fails after it wrote the status started the response.
	l = new(logs)
	rec := brokenFlusher{httptest.NewRecorder()}
	web.Errors{}.Handle(func(w http.ResponseWriter, r *http.Request) error {
		return http.NewResponseController(w).Flush()
	}).ServeHTTP(rec, request(t, l, http.MethodGet, "/", nil))
	t.Must(be.Equal(http.StatusOK, rec.Code))
	t.Must(be.Equal("", rec.Body.String()))
	t.Must(be.Equal("request failed after the response started", l.all()[0].msg))
}

// TestWriterLongCopy checks that a copy keeps the ReadFrom of the
// underlying writer for all but its start, and starts the response.
func TestWriterLongCopy(gt *testing.T) {
	t := devtest.ParallelT(gt)
	content := strings.Repeat("0123456789", 300)
	l := new(logs)
	w := &readFromRecorder{ResponseRecorder: httptest.NewRecorder()}
	var n int64
	web.Errors{}.Handle(func(w http.ResponseWriter, r *http.Request) error {
		// A LimitedReader has no WriteTo, so io.Copy uses the ReadFrom of w.
		var err error
		n, err = io.Copy(w, io.LimitReader(strings.NewReader(content), int64(len(content))))
		if err != nil {
			return err
		}
		return errors.New("late")
	}).ServeHTTP(w, request(t, l, http.MethodGet, "/", nil))
	t.Must(be.Equal(int64(len(content)), n))
	t.Must(be.True(w.copied > 0 && w.copied < n))
	t.Must(be.Equal(content, w.Body.String()))
	t.Must(be.Equal("request failed after the response started", l.all()[0].msg))
}
