package web_test

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthttp"
	"github.com/protolambda/chord/html/text"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/mw"
	"github.com/protolambda/webui/page"
	"github.com/protolambda/webui/web"
)

// hostile is markup that a request smuggles into an error message.
const hostile = `<i hx-post="/archive" hx-include="[name=csrf]" hx-trigger="load">x</i>`

// echoError answers with a plain-text error that quotes the request, the
// way http.Error(w, err.Error(), 400) does for a strconv error.
func echoError(w http.ResponseWriter, r *http.Request) {
	if _, err := strconv.Atoi(r.FormValue("page")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, _ = io.WriteString(w, "ok")
}

// hostileQuery is a query whose page parameter is the hostile markup.
var hostileQuery = "/tasks?" + url.Values{"page": {hostile}}.Encode()

func TestInterceptReplacesPlainError(gt *testing.T) {
	t := devtest.ParallelT(gt)
	l := new(logs)
	h := web.Errors{}.Intercept(http.HandlerFunc(echoError))
	res := cthttp.Serve(h, asHTMX(request(t, l, http.MethodGet, hostileQuery, nil), "https://app.example/tasks"))
	t.Must(res.Status(http.StatusBadRequest))
	t.Must(res.Header("Content-Type", ct.Exact(web.ContentType)))
	t.Must(res.Header("HX-Retarget", ct.Exact("#"+page.AlertsID)))
	t.Must(res.Header("HX-Reswap", ct.Exact(web.ErrorReswap)))
	t.Must(res.Header("HX-Reselect", ct.Exact("unset")))
	t.Must(res.Header("HX-Push-Url", ct.Exact("false")))
	t.Must(res.Header("Vary", ct.Exact(hx2.HeaderRequest)))
	t.Must(res.Header("Cache-Control", ct.Exact("no-store")))
	// http.Error's nosniff stays: it suits HTML too.
	t.Must(res.Header("X-Content-Type-Options", ct.Exact("nosniff")))
	t.Must(res.Body(ct.Exact(`<div class="alert alert-danger">` + web.DefaultMessage(http.StatusBadRequest) + `</div>`)))
	t.Must(be.False(strings.Contains(res.BodyString(), "hx-post")))
	t.Must(be.False(strings.Contains(res.BodyString(), "strconv")))

	recs := l.all()
	t.Must(be.Equal(1, len(recs)))
	t.Must(be.Equal(slog.LevelDebug, recs[0].level))
	t.Must(be.Equal("400", recs[0].attrs["status"]))
	t.Must(be.Equal("text/plain", recs[0].attrs["content_type"]))
	t.Must(be.False(strings.Contains(l.text(), "hx-post")))
}

func TestInterceptPassesPlainRequests(gt *testing.T) {
	t := devtest.ParallelT(gt)
	h := web.Errors{}.Intercept(http.HandlerFunc(echoError))
	res := cthttp.Serve(h, request(t, nil, http.MethodGet, "/tasks?page=x", nil))
	t.Must(res.Status(http.StatusBadRequest))
	t.Must(res.Header("Content-Type", ct.Exact("text/plain; charset=utf-8")))
	// The htmx answer differs, so caches must keep them apart.
	t.Must(res.Header("Vary", ct.Exact(hx2.HeaderRequest)))
	t.Must(res.HeaderAbsent("HX-Retarget"))
	t.Must(res.Body(ct.Prefix(`strconv.Atoi: parsing "x"`)))

	// A history restore request is not an htmx request (IsHTMX): it
	// passes as well.
	r := asHTMX(request(t, nil, http.MethodGet, "/tasks?page=x", nil), "")
	r.Header.Set(hx2.HeaderHistoryRestoreRequest, "true")
	res = cthttp.Serve(h, r)
	t.Must(res.Header("Content-Type", ct.Exact("text/plain; charset=utf-8")))
	t.Must(res.HeaderAbsent("HX-Retarget"))

	// Successes pass without a Vary of their own.
	res = cthttp.Serve(h, request(t, nil, http.MethodGet, "/tasks?page=2", nil))
	t.Must(res.Status(http.StatusOK))
	t.Must(res.HeaderAbsent("Vary"))
	t.Must(res.Body(ct.Exact("ok")))
}

// TestInterceptRules checks which responses to htmx requests are replaced.
func TestInterceptRules(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		name     string
		status   int
		header   map[string]string
		body     string
		replaced bool
	}{
		{name: "text/plain error", status: 404, header: map[string]string{"Content-Type": "text/plain"}, body: "gone", replaced: true},
		{name: "json error", status: 500, header: map[string]string{"Content-Type": "application/json"}, body: `{"error":"x"}`, replaced: true},
		{name: "error without body", status: 409, header: map[string]string{"Content-Type": "text/plain"}, replaced: true},
		{name: "media type parameters", status: 422, header: map[string]string{"Content-Type": " Text/Plain ; charset=utf-8"}, body: "x", replaced: true},
		{name: "malformed type", status: 400, header: map[string]string{"Content-Type": ";;"}, body: "x", replaced: true},
		{name: "refresh false", status: 400, header: map[string]string{"Content-Type": "text/plain", "HX-Refresh": "false"}, body: "x", replaced: true},
		{name: "first 5xx", status: 503, header: map[string]string{"Content-Type": "text/plain"}, body: "x", replaced: true},
		{name: "last 5xx", status: 599, header: map[string]string{"Content-Type": "text/plain"}, body: "x", replaced: true},

		{name: "html error", status: 422, header: map[string]string{"Content-Type": web.ContentType}, body: "<p>fix the form</p>"},
		{name: "html in capitals", status: 400, header: map[string]string{"Content-Type": "TEXT/HTML"}, body: "<p>x</p>"},
		{name: "no content type", status: 401},
		{name: "empty content type", status: 400, header: map[string]string{"Content-Type": ""}},
		{name: "success", status: 200, header: map[string]string{"Content-Type": "text/plain"}, body: "fine"},
		{name: "redirect status", status: 303, header: map[string]string{"Content-Type": "text/plain", "Location": "/"}},
		{name: "not an error class", status: 600, header: map[string]string{"Content-Type": "text/plain"}, body: "x"},
		{name: "htmx redirect", status: 401, header: map[string]string{"Content-Type": "text/plain", "HX-Redirect": "/login"}, body: "x"},
		{name: "htmx location", status: 403, header: map[string]string{"Content-Type": "text/plain", "HX-Location": "/home"}, body: "x"},
		{name: "htmx refresh", status: 409, header: map[string]string{"Content-Type": "text/plain", "HX-Refresh": "true"}, body: "x"},
		{name: "non-canonical redirect key", status: 401, header: map[string]string{"Content-Type": "text/plain", "hx-redirect": "/login"}, body: "x"},
	}
	for _, c := range cases {
		h := web.Errors{}.Intercept(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for k, v := range c.header {
				if k == "hx-redirect" {
					w.Header()[k] = []string{v} // not canonical
					continue
				}
				w.Header().Set(k, v)
			}
			w.WriteHeader(c.status)
			_, _ = io.WriteString(w, c.body)
		}))
		res := cthttp.Serve(h, asHTMX(request(t, nil, http.MethodPost, "/tasks", nil), ""))
		t.Mustf(res.Status(c.status), "%s: status", c.name)
		if c.replaced {
			t.Mustf(res.Header("Content-Type", ct.Exact(web.ContentType)), "%s: content type", c.name)
			t.Mustf(res.Header("HX-Retarget", ct.Exact("#"+page.AlertsID)), "%s: retarget", c.name)
			t.Mustf(res.HTMLFragment("div").Find(ct.Class("alert-danger")).Texts(web.DefaultMessage(c.status)), "%s: alert", c.name)
			t.Mustf(res.HeaderAbsent("HX-Refresh"), "%s: htmx headers reset", c.name)
			continue
		}
		t.Mustf(res.HeaderAbsent("HX-Retarget"), "%s: retarget", c.name)
		t.Mustf(be.Equal(c.body, res.BodyString()), "%s: body", c.name)
	}
}

// TestInterceptRequireAuth checks that the empty 401 of RequireAuth, which
// htmx follows to the login page, passes.
func TestInterceptRequireAuth(gt *testing.T) {
	t := devtest.ParallelT(gt)
	errs := web.Errors{}
	h := mw.Chain(http.NotFoundHandler(), errs.Intercept, errs.RequireAuth("/login", func(*http.Request) bool { return false }))
	res := cthttp.Serve(h, asHTMX(request(t, nil, http.MethodPost, "/tasks", nil), "https://app.example/tasks"))
	t.Must(res.Status(http.StatusUnauthorized))
	t.Must(res.Header("HX-Redirect", ct.Prefix("/login")))
	t.Must(res.HeaderAbsent("HX-Retarget"))
}

// TestInterceptServeMux checks the 404 and 405 answers of http.ServeMux.
func TestInterceptServeMux(gt *testing.T) {
	t := devtest.ParallelT(gt)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tasks", echoError)
	h := web.Errors{}.Intercept(mux)

	res := cthttp.Serve(h, asHTMX(request(t, nil, http.MethodGet, "/nowhere", nil), ""))
	t.Must(res.Status(http.StatusNotFound))
	t.Must(res.HTMLFragment("div").Find(ct.Class("alert-danger")).Texts(web.DefaultMessage(http.StatusNotFound)))

	res = cthttp.Serve(h, asHTMX(request(t, nil, http.MethodDelete, "/tasks", nil), ""))
	t.Must(res.Status(http.StatusMethodNotAllowed))
	t.Must(res.HTMLFragment("div").Find(ct.Class("alert-danger")).Texts(web.DefaultMessage(http.StatusMethodNotAllowed)))
	// The allowed methods stay.
	t.Must(res.Header("Allow", ct.Contains("GET")))
}

// TestInterceptRefusals checks the plain-text refusals of mw.HostCheck and
// mw.SameOrigin, and SameOrigin with a Deny of the policy.
func TestInterceptRefusals(gt *testing.T) {
	t := devtest.ParallelT(gt)
	errs := web.Errors{}
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "ok") })
	site := mw.MustParseSite("https://example.com")
	h := mw.Chain(ok, errs.Intercept, mw.HostCheck(site), mw.SameOrigin(mw.OriginPolicy{}))

	r := asHTMX(request(t, nil, http.MethodPost, "/tasks", nil), "")
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	res := cthttp.Serve(h, r)
	t.Must(res.Status(http.StatusForbidden))
	t.Must(res.Header("HX-Retarget", ct.Exact("#"+page.AlertsID)))
	t.Must(res.HTMLFragment("div").Find(ct.Class("alert-danger")).Texts(web.DefaultMessage(http.StatusForbidden)))

	r = asHTMX(request(t, nil, http.MethodGet, "/tasks", nil), "")
	r.Host = "elsewhere.example"
	res = cthttp.Serve(h, r)
	t.Must(res.Status(http.StatusBadRequest))
	t.Must(res.HTMLFragment("div").Find(ct.Class("alert-danger")).Texts(web.DefaultMessage(http.StatusBadRequest)))

	// A Deny of the policy answers with the classified message.
	h = mw.Chain(ok, errs.Intercept, mw.HostCheck(site), mw.SameOrigin(mw.OriginPolicy{Deny: errs.Answer(mw.ErrCrossOrigin)}))
	r = asHTMX(request(t, nil, http.MethodPost, "/tasks", nil), "")
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	res = cthttp.Serve(h, r)
	t.Must(res.Status(http.StatusForbidden))
	t.Must(res.HTMLFragment("div").Find(ct.Class("alert-danger")).Texts(web.MessageForged))

	r = request(t, nil, http.MethodPost, "/tasks", nil)
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	res = cthttp.Serve(h, r)
	t.Must(res.Status(http.StatusForbidden))
	t.Must(res.HTML().Find(ct.Role("alert")).Texts(web.MessageForged))
}

func TestInterceptDropsLaterWrites(gt *testing.T) {
	t := devtest.ParallelT(gt)
	h := web.Errors{}.Intercept(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "first", http.StatusInternalServerError)
		w.Header().Set("HX-Trigger", "late")
		w.WriteHeader(http.StatusOK)
		n, err := io.WriteString(w, "second")
		if err != nil || n != len("second") {
			panic("a dropped write must report success")
		}
		m, err := w.(io.ReaderFrom).ReadFrom(strings.NewReader("third"))
		if err != nil || m != int64(len("third")) {
			panic("a dropped copy must report success")
		}
		if err := http.NewResponseController(w).Flush(); err != nil {
			panic(err)
		}
	}))
	res := cthttp.Serve(h, asHTMX(request(t, nil, http.MethodGet, "/", nil), ""))
	t.Must(res.Status(http.StatusInternalServerError))
	t.Must(res.Body(ct.Exact(`<div class="alert alert-danger">` + web.DefaultMessage(500) + `</div>`)))
	t.Must(res.HeaderAbsent("HX-Trigger"))
}

func TestInterceptAfterStart(gt *testing.T) {
	t := devtest.ParallelT(gt)
	// Once the response started, a later error status is the handler's:
	// net/http ignores it.
	for name, start := range map[string]func(w http.ResponseWriter){
		"write": func(w http.ResponseWriter) { _, _ = io.WriteString(w, "a") },
		"copy":  func(w http.ResponseWriter) { _, _ = w.(io.ReaderFrom).ReadFrom(strings.NewReader("a")) },
		"flush": func(w http.ResponseWriter) { _ = http.NewResponseController(w).Flush() },
	} {
		h := web.Errors{}.Intercept(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start(w)
			http.Error(w, "late", http.StatusInternalServerError)
		}))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, asHTMX(request(t, nil, http.MethodGet, "/", nil), ""))
		t.Mustf(be.Equal(http.StatusOK, rec.Code), "%s: status", name)
		t.Mustf(be.Substring(rec.Body.String(), "late"), "%s: body", name)
		t.Mustf(be.Equal("", rec.Header().Get("HX-Retarget")), "%s: retarget", name)
	}
}

// statusWriter records every status written to it.
type statusWriter struct {
	*fakeWriter
	statuses []int
}

func (w *statusWriter) WriteHeader(code int) {
	w.statuses = append(w.statuses, code)
	w.fakeWriter.WriteHeader(code)
}

func TestInterceptInformational(gt *testing.T) {
	t := devtest.ParallelT(gt)
	w := &statusWriter{fakeWriter: newFakeWriter()}
	web.Errors{}.Intercept(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", "</assets/app.css>; rel=preload; as=style")
		w.WriteHeader(http.StatusEarlyHints)
		http.Error(w, "late", http.StatusServiceUnavailable)
	})).ServeHTTP(w, asHTMX(request(t, nil, http.MethodGet, "/", nil), ""))
	t.Must(be.DeepEqual([]int{http.StatusEarlyHints, http.StatusServiceUnavailable}, w.statuses))
	t.Must(be.Substring(w.body.String(), web.DefaultMessage(http.StatusServiceUnavailable)))
}

func TestInterceptKeepsResponseController(gt *testing.T) {
	t := devtest.ParallelT(gt)
	w := newFakeWriter()
	deadline := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	web.Errors{}.Intercept(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := http.NewResponseController(w)
		if err := rc.SetWriteDeadline(deadline); err != nil {
			panic(err)
		}
		if err := rc.EnableFullDuplex(); err != nil {
			panic(err)
		}
		_, _ = io.WriteString(w, "ok")
	})).ServeHTTP(w, asHTMX(request(t, nil, http.MethodGet, "/", nil), ""))
	t.Must(be.Equal(deadline, w.deadline))
	t.Must(be.True(w.duplex))
	t.Must(be.Equal("ok", w.body.String()))
}

func TestInterceptHijack(gt *testing.T) {
	t := devtest.ParallelT(gt)
	server, client := net.Pipe()
	defer func() { _ = client.Close() }()
	w := hijacker{fakeWriter: newFakeWriter(), conn: server}
	var hijacked bool
	web.Errors{}.Intercept(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			panic(err)
		}
		hijacked = true
		_ = conn.Close()
	})).ServeHTTP(w, asHTMX(request(t, nil, http.MethodGet, "/ws", nil), ""))
	t.Must(be.True(hijacked))
	t.Must(be.Equal(0, w.status))
}

// TestInterceptWithRecoverAndHandle checks the documented stack: the
// answers of Recover and Handle are HTML and pass, and a panic after a
// replaced answer aborts the response.
func TestInterceptWithRecoverAndHandle(gt *testing.T) {
	t := devtest.ParallelT(gt)
	errs := web.Errors{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /boom", func(http.ResponseWriter, *http.Request) { panic("boom") })
	mux.HandleFunc("GET /late", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "plain", http.StatusBadRequest)
		panic("late")
	})
	mux.Handle("GET /tasks", errs.Handle(func(w http.ResponseWriter, r *http.Request) error {
		return web.Conflict("", errors.New("stale"))
	}))
	mux.Handle("GET /plain", errs.Handle(func(w http.ResponseWriter, r *http.Request) error {
		http.Error(w, "plain", http.StatusBadRequest)
		return errors.New("after the plain answer")
	}))
	h := mw.Chain(mux, errs.Recover, errs.Intercept)

	res := cthttp.Serve(h, asHTMX(request(t, nil, http.MethodGet, "/boom", nil), ""))
	t.Must(res.Status(http.StatusInternalServerError))
	t.Must(res.HTMLFragment("div").Find(ct.Class("alert-danger")).Texts(web.DefaultMessage(500)))

	res = cthttp.Serve(h, asHTMX(request(t, nil, http.MethodGet, "/tasks", nil), ""))
	t.Must(res.Status(http.StatusConflict))
	t.Must(res.HTMLFragment("div").Find(ct.Class("alert-danger")).Texts(web.DefaultMessage(http.StatusConflict)))

	// Handle sees the started response and logs its error only.
	l := new(logs)
	res = cthttp.Serve(h, asHTMX(request(t, l, http.MethodGet, "/plain", nil), ""))
	t.Must(res.Status(http.StatusBadRequest))
	t.Must(res.HTMLFragment("div").Find(ct.Class("alert-danger")).Texts(web.DefaultMessage(http.StatusBadRequest)))
	t.Must(be.Equal("request failed after the response started", l.all()[1].msg))

	v := recovered(func() {
		h.ServeHTTP(httptest.NewRecorder(), asHTMX(request(t, nil, http.MethodGet, "/late", nil), ""))
	})
	t.Must(be.True(v == http.ErrAbortHandler))

	// In the other order, Recover's answer passes Intercept too.
	h = mw.Chain(mux, errs.Intercept, errs.Recover)
	res = cthttp.Serve(h, asHTMX(request(t, nil, http.MethodGet, "/boom", nil), ""))
	t.Must(res.Status(http.StatusInternalServerError))
	t.Must(res.Header("HX-Retarget", ct.Exact("#"+page.AlertsID)))
}

func TestInterceptUsesHooks(gt *testing.T) {
	t := devtest.ParallelT(gt)
	errs := web.Errors{
		Target: "#errors",
		Reswap: hx2.SwapAfterBegin,
		Fragment: func(r *http.Request, v web.ErrorView) elem.Node {
			return text.P()(text.Text(fmt.Sprintf("%d: %s", v.Status, v.Message)))
		},
	}
	res := cthttp.Serve(errs.Intercept(http.HandlerFunc(echoError)), asHTMX(request(t, nil, http.MethodGet, "/tasks?page=x", nil), ""))
	t.Must(res.Header("HX-Retarget", ct.Exact("#errors")))
	t.Must(res.Header("HX-Reswap", ct.Exact(hx2.SwapAfterBegin)))
	t.Must(res.Body(ct.Exact("<p>400: " + web.DefaultMessage(400) + "</p>")))
}
