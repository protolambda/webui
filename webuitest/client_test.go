package webuitest_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthtml"
	"github.com/protolambda/chord/html/form"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/form/input"
	"github.com/protolambda/chord/html/form/label"
	"github.com/protolambda/chord/html/group/list"
	"github.com/protolambda/chord/html/script"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/assertion"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/webuitest"
)

// render writes a view as an HTML response.
func render(w http.ResponseWriter, r *http.Request, status int, node elem.Node) {
	var b strings.Builder
	if err := core.Render(r.Context(), node, &b); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, b.String())
}

// taskApp is a small application: a page of tasks with a form to add one,
// behind a session that tests attach to the request context.
type taskApp struct {
	mu    sync.Mutex
	tasks []string
	// broken adds an inline script to the page.
	broken bool
}

func (a *taskApp) titles() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.tasks...)
}

func (a *taskApp) page(token string) elem.Node {
	items := make([]elem.Node, 0)
	for _, title := range a.titles() {
		items = append(items, list.LI()(text.Text(title)))
	}
	main := section.Main(attr.ID("main"))(
		section.H1()(text.Text("Tasks")),
		list.UL(attr.ID("list"))(items...),
		form.Form(form.Method(form.MethodPost), form.Action("/tasks"))(
			session.CSRFInput(token),
			label.Label(label.For("title"))(text.Text("Title")),
			input.Input(attr.ID("title"), input.Name("title"), input.Required()),
			button.Button(button.Type(button.TypeSubmit))(text.Text("Add")),
		),
		elem.If(a.broken, script.Inline("alert(1)")),
	)
	return validDoc().with(func(d *docSpec) { d.body = []elem.Node{main} }).node()
}

func (a *taskApp) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tasks", func(w http.ResponseWriter, r *http.Request) {
		s, ok := session.FromContext[string](r.Context())
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		render(w, r, http.StatusOK, a.page(s.CSRF()))
	})
	mux.HandleFunc("GET /tasks/list", func(w http.ResponseWriter, r *http.Request) {
		render(w, r, http.StatusOK, list.UL(attr.ID("list"))(list.LI()(text.Text("x"))))
	})
	mux.HandleFunc("POST /tasks", func(w http.ResponseWriter, r *http.Request) {
		s, ok := session.FromContext[string](r.Context())
		if !ok || r.ParseForm() != nil || s.CheckCSRF(r.PostForm.Get(session.CSRFField)) != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		a.mu.Lock()
		a.tasks = append(a.tasks, r.PostForm.Get("title"))
		a.mu.Unlock()
		http.Redirect(w, r, "/tasks", http.StatusSeeOther)
	})
	return mux
}

func newTaskClient(a *taskApp) (*webuitest.Client, *session.Session[string]) {
	sess := session.NewDetached("account", "csrf-token-of-the-session")
	return &webuitest.Client{
		Handler: a.handler(),
		Prepare: func(r *http.Request) *http.Request {
			return r.WithContext(session.WithSession(r.Context(), sess))
		},
		CSRF: sess.CSRF(),
	}, sess
}

func TestClient(gt *testing.T) {
	t := devtest.ParallelT(gt)
	app := &taskApp{}
	c, sess := newTaskClient(app)

	page := c.Page(t, c.Get(t, "/tasks"), http.StatusOK)
	t.Must(be.Equal(sess.CSRF(), webuitest.CSRFToken(t, page)))

	res := c.Submit(t, page, page.Find(ct.Tag("form")), nil, url.Values{"title": {"Write the docs"}})
	t.Must(webuitest.Location(res, http.StatusSeeOther, ct.Exact("/tasks")))
	t.Must(be.DeepEqual([]string{"Write the docs"}, app.titles()))

	// Post adds the CSRF field of the client.
	res = c.Post(t, "/tasks", webuitest.Form(url.Values{"title": {"Review"}}))
	t.Must(webuitest.Location(res, http.StatusSeeOther, ct.Exact("/tasks")))
	page = c.Page(t, c.Get(t, "/tasks"), http.StatusOK)
	t.Must(page.Find(ct.ID("list")).Find(ct.Role("listitem")).Texts("Write the docs", "Review"))

	// An explicit field is kept, so tests can send a wrong token.
	res = c.Post(t, "/tasks", webuitest.Form(url.Values{"title": {"x"}, session.CSRFField: {"wrong"}}))
	t.Must(res.Status(http.StatusForbidden))
	noToken := *c
	noToken.CSRF = ""
	t.Must(noToken.Post(t, "/tasks", webuitest.Form(url.Values{"title": {"x"}})).Status(http.StatusForbidden))
	t.Must(be.Len(app.titles(), 2))

	frag := c.Fragment(t, c.Get(t, "/tasks/list", webuitest.HTMX()), http.StatusOK, "div")
	t.Must(frag.Find(ct.Role("listitem")).Texts("x"))
}

func TestClientSubmitSendsTheFormOnly(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var got url.Values
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tasks", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil && !errors.Is(err, http.ErrNotMultipart) {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		got = r.PostForm
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /projects/7/done", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	c := &webuitest.Client{Handler: mux, CSRF: "client-token"}
	// A form that forgot its CSRF field is sent without one, as a browser
	// sends it, so the end-to-end test sees the missing field.
	page := cthtml.PageString(htmlPage(`<form method="post" action="/tasks"><input name="title" value="x"><button type="submit">Add</button></form>` +
		`<form id="multi" method="post" action="/tasks" enctype="multipart/form-data"><input name="title" value="y"></form>` +
		`<form id="list" method="post" action="/tasks"><datalist id="l"><input type="hidden" name="csrf" value="client-token"></datalist></form>`))
	t.Must(c.Submit(t, page, page.Find(ct.Tag("form")).First(), nil, nil).Status(http.StatusNoContent))
	t.Must(be.DeepEqual(url.Values{"title": {"x"}}, got))
	t.Must(c.Submit(t, page, page.Find(ct.ID("multi")), nil, nil).Status(http.StatusNoContent))
	t.Must(be.DeepEqual(url.Values{"title": {"y"}}, got))
	t.Must(c.Submit(t, page, page.Find(ct.ID("list")), nil, nil).Status(http.StatusNoContent))
	t.Must(be.False(got.Has(session.CSRFField)))
	// Hand-built posts still get the client's field.
	t.Must(c.Post(t, "/tasks", webuitest.Form(url.Values{"title": {"z"}})).Status(http.StatusNoContent))
	t.Must(be.Equal("client-token", got.Get(session.CSRFField)))

	// A relative action is resolved against the page's URL.
	rel := cthtml.PageString(htmlPage(`<form method="post" action="done"></form>`))
	t.Must(c.Submit(t, rel, rel.Find(ct.Tag("form")), nil, nil, webuitest.PageURL("/projects/7/")).Status(http.StatusAccepted))
	r := newRecordT(t.Context())
	t.Must(be.True(c.Submit(r, rel, rel.Find(ct.Tag("form")), nil, nil) == nil))
	t.Must(be.Substring(r.output(), "webuitest: cannot compute form data: the action is relative to the page's URL; pass PageURL with it"))
	t.Must(be.False(strings.Contains(r.output(), "webuitest: webuitest:")))
}

func TestClientOptions(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var seen http.Header
	c := &webuitest.Client{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seen = r.Header.Clone() }),
		Options: []webuitest.ReqOption{webuitest.HTMX(), webuitest.Header("X-A", "1")},
	}
	c.Get(t, "/", webuitest.Header("X-A", "2"))
	t.Must(be.Equal("true", seen.Get("HX-Request")))
	t.Must(be.Equal("2", seen.Get("X-A")))
	res := c.Send(t, http.MethodDelete, "/x", webuitest.Body{})
	t.Must(res.Status(http.StatusOK))
	t.Must(be.Equal(webuitest.BaseURL, seen.Get("Origin")))

	custom := 0
	c.Contract = func(page *ct.Subject) assertion.Assertion {
		custom++
		return page
	}
	c.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		render(w, r, http.StatusOK, section.Section()())
	})
	c.Page(t, c.Get(t, "/"), http.StatusOK)
	t.Must(be.Equal(1, custom))
}

func TestClientFailures(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Run("contract", func(t devtest.T) {
		app := &taskApp{broken: true}
		c, _ := newTaskClient(app)
		r := newRecordT(t.Context())
		c.Page(r, c.Get(t, "/tasks"), http.StatusOK)
		t.Must(be.True(r.failed))
		t.Must(be.Substring(r.output(), "noInlineScripts(): inline script at"))
		r = newRecordT(t.Context())
		c.Page(r, c.Get(t, "/tasks"), http.StatusTeapot)
		t.Must(be.Substring(r.output(), "418"))
	})
	t.Run("submit", func(t devtest.T) {
		c, _ := newTaskClient(&taskApp{})
		page := c.Page(t, c.Get(t, "/tasks"), http.StatusOK)
		r := newRecordT(t.Context())
		t.Must(be.True(c.Submit(r, page, page.Find(ct.Tag("form")), nil, url.Values{"titel": {"x"}}) == nil))
		t.Must(be.Substring(r.output(), `the form has no control "titel" to edit`))
		r = newRecordT(t.Context())
		t.Must(be.True(c.Submit(r, page, page.Find(ct.Tag("nav")), nil, nil) == nil))
		t.Must(be.Substring(r.output(), "matches 0 elements"))
	})
	t.Run("handler", func(t devtest.T) {
		r := newRecordT(t.Context())
		var c webuitest.Client
		t.Must(be.True(c.Get(r, "/") == nil))
		t.Must(be.Substring(r.output(), "no handler"))
		c.Handler = http.NotFoundHandler()
		c.Prepare = func(*http.Request) *http.Request { return nil }
		r = newRecordT(t.Context())
		t.Must(be.True(c.Get(r, "/") == nil))
		t.Must(be.Substring(r.output(), "Prepare returned no request"))
		r = newRecordT(t.Context())
		t.Must(be.True(c.Get(r, "%zz") == nil))
		t.Must(be.True(c.Do(r, nil) == nil))
	})
}

func TestCSRFToken(gt *testing.T) {
	t := devtest.ParallelT(gt)
	const token = "csrf-token-never-printed"
	page := func(body string) *ct.Subject { return cthtml.PageString(htmlPage(body)) }
	t.Must(be.Equal(token, webuitest.CSRFToken(t, page(`<form method="post"><input type="hidden" name="csrf" value="`+token+`"></form><form method="post"><input type="hidden" name="csrf" value="`+token+`"></form><input name="csrf" value="visible">`))))
	cases := []struct{ name, body, want string }{
		{"none", `<p>x</p>`, `the page has no hidden "csrf" field`},
		{"different", `<input type="hidden" name="csrf" value="` + token + `"><input type="hidden" name="csrf" value="other">`, `the "csrf" fields have different values`},
		{"empty", `<input type="hidden" name="csrf" value="">`, `the "csrf" field at html[1]/body[1]/input[0] is empty`},
	}
	for _, c := range cases {
		r := newRecordT(t.Context())
		t.Must(be.Equal("", webuitest.CSRFToken(r, page(c.body))))
		t.Must(be.True(r.failed))
		t.Must(be.Substring(r.output(), c.want))
		t.Must(be.False(strings.Contains(r.output(), token)))
	}
	r := newRecordT(t.Context())
	webuitest.CSRFToken(r, ct.View(elem.Fn(func(context.Context) (elem.Node, error) { return nil, errors.New("boom") })))
	t.Must(be.Substring(r.output(), "webuitest.CSRFToken"))
}
