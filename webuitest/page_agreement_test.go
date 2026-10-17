package webuitest_test

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthtml"
	"github.com/protolambda/chord/html/form"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/form/input"
	"github.com/protolambda/chord/html/form/label"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/assertion"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/asset"
	"github.com/protolambda/webui/bs"
	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/page"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/theme"
	"github.com/protolambda/webui/web"
	"github.com/protolambda/webui/webuitest"
)

// The tests in this file run the contracts on what page and web render,
// so that the expectations of this package cannot drift from them.

const agreementToken = "agreement-token-of-the-session"

// appAssets is an application's own asset set: a stylesheet, a classic
// script, an entry module and an unlisted worker that pages do not load.
func appAssets() *asset.Set {
	return asset.Must(asset.New(asset.Options{
		Prefix: "/assets/app/",
		FS: fstest.MapFS{
			"app.css":   {Data: []byte("main{padding:1rem}")},
			"app.js":    {Data: []byte("\"use strict\";")},
			"app.mjs":   {Data: []byte("export {};")},
			"worker.js": {Data: []byte("\"use strict\";")},
		},
		Files: []asset.File{
			{Name: "app.css"},
			{Name: "app.js"},
			{Name: "app.mjs", Kind: asset.KindModule, ContentType: "text/javascript; charset=utf-8"},
			{Name: "worker.js", Unlisted: true},
		},
	}))
}

// agreementDocument is a complete page of a webui application: a navbar
// with a logout form, a sidebar, a header with a dialog opener, a form, a
// footer, and a dialog with a form of its own.
func agreementDocument(sets ...*asset.Set) page.Document {
	return page.Document{
		Title:  "Tasks",
		Assets: sets,
		Body: page.Layout{
			Navbar: page.Navbar{
				Brand: page.Brand{Label: "Tasks"},
				Items: []page.NavItem{{Label: "Tasks", Href: "/tasks", Current: true}, {Label: "Notes", Href: "/notes"}},
				End: []elem.Node{form.Form(form.Method(form.MethodPost), form.Action("/logout"))(
					session.CSRFInput(agreementToken),
					bs.BtnOutlineSecondary(button.Type(button.TypeSubmit))(text.Text("Log out")),
				)},
			},
			Sidebar: page.Sidebar{Label: "Projects", Sections: []page.SideSection{
				{Title: "Projects", Items: []page.NavItem{{Label: "Garden", Href: "/projects/1"}, {Label: "Kitchen", Href: "/projects/2"}}},
			}},
			Header: page.Header{
				Title:   text.Text("Tasks"),
				Actions: []elem.Node{bs.DialogButton("new-task", attr.Class("btn btn-primary"))(text.Text("New task"))},
			},
			Main: section.Section()(
				form.Form(form.Method(form.MethodPost), form.Action("/tasks/clear"))(
					session.CSRFInput(agreementToken),
					bs.BtnDanger(button.Type(button.TypeSubmit))(text.Text("Clear done")),
				),
				bs.BtnSecondary(button.Type(button.TypeButton), hx2.Post("/tasks/sort"), hx2.ValsJSON(map[string]any{session.CSRFField: agreementToken}), hx2.Target("#"+page.AlertsID))(text.Text("Sort")),
			),
			Footer: text.Small()(text.Text("Tasks 1.0")),
			Dialogs: []elem.Node{bs.Dialog{
				ID:    "new-task",
				Title: text.Text("New task"),
				Body: form.Form(form.Method(form.MethodPost), form.Action("/tasks"))(
					session.CSRFInput(agreementToken),
					label.Label(label.For("title"), attr.Class("form-label"))(text.Text("Title")),
					input.Input(attr.ID("title"), input.Name("title"), attr.Class("form-control")),
					bs.BtnPrimary(button.Type(button.TypeSubmit))(text.Text("Add")),
				),
			}},
		},
	}
}

// agreementRules are the rules of a complete webui page.
func agreementRules() []ct.Rule {
	return append(webuitest.Rules(webuitest.WithCSRF(agreementToken)), webuitest.LayoutRegions())
}

func TestPageAgreement(gt *testing.T) {
	t := devtest.ParallelT(gt)
	themeSet, app := theme.MustAssets(theme.Options{}), appAssets()
	doc := agreementDocument(themeSet, app)
	shell := webuitest.DefaultShell(themeSet, app)
	var html strings.Builder
	t.Must(be.NoError(core.Render(t.Context(), doc, &html)))
	for name, subject := range map[string]*ct.Subject{
		"view":   ct.View(doc),
		"parsed": cthtml.PageString(html.String()),
	} {
		t.Run(name, func(t devtest.T) {
			t.Must(webuitest.ShellContract(subject, shell))
			t.Must(webuitest.PageContract(subject, shell, agreementRules()...))
			// The page's own targets and regions.
			t.Must(subject.Find(ct.ID(page.NavbarID)).Matches(ct.Tag("nav")))
			t.Must(subject.Find(ct.ID(page.SidebarID)).Matches(ct.Tag("nav")))
			t.Must(subject.Find(ct.ID(page.HeaderID)).Count(1))
			// The unlisted worker is served, but no page loads it.
			t.Must(subject.Find(ct.Tag("script"), ct.Attr("src", app.URL("worker.js"))).None())
			t.Must(subject.Find(ct.Tag("script"), ct.Attr("type", "module")).Count(1))
		})
	}
	t.Run("the shell tells sets apart", func(t devtest.T) {
		err := webuitest.ShellContract(ct.View(doc), webuitest.DefaultShell(themeSet)).Check(t.Context())
		t.Must(be.ErrorContains(err, `(href "/assets/app/`))
		t.Must(be.ErrorContains(err, "is not part of the shell"))
		classic := webuitest.DefaultShell(themeSet, app)
		for i := range classic.Scripts {
			classic.Scripts[i].Module = false
		}
		err = webuitest.ShellContract(ct.View(doc), classic).Check(t.Context())
		t.Must(be.ErrorContains(err, "script app.mjs at"))
		t.Must(be.ErrorContains(err, "is a module, want a classic script"))
	})
	t.Run("htmx configuration", func(t devtest.T) {
		eval := agreementDocument(themeSet)
		eval.HTMX = hx2.Config{AllowEval: true}
		err := webuitest.ShellContract(ct.View(eval), webuitest.DefaultShell()).Check(t.Context())
		t.Must(be.ErrorContains(err, `htmx-config "allowEval" is true, want false`))
	})
	t.Run("deprecated ids are the page ids", func(t devtest.T) {
		t.Must(be.Equal(page.MainID, webuitest.MainID))
		t.Must(be.Equal(page.AlertsID, webuitest.AlertsID))
		t.Must(be.Equal(page.DialogsID, webuitest.DialogsID))
		t.Must(be.Equal(page.HeaderID, webuitest.HeaderID))
		t.Must(be.DeepEqual([]string{page.MainID, page.AlertsID, page.DialogsID, page.HeaderID, page.NavbarID, page.SidebarID}, webuitest.DocumentTargets()))
	})
	t.Run("fragments may target the layout", func(t devtest.T) {
		badge := page.Navbar{
			Items: []page.NavItem{{Label: "Tasks", Href: "/tasks"}},
			Attrs: hx2.SwapOOB("true"),
		}
		frag := ct.View(elem.Bundle{
			page.Notice(bs.VariantSuccess, text.Text("Saved.")),
			badge,
			bs.BtnLink(button.Type(button.TypeButton), hx2.Get("/sidebar"), hx2.Target("#"+page.SidebarID))(text.Text("Refresh")),
		})
		t.Must(webuitest.FragmentContract(frag, webuitest.Rules(webuitest.WithCSRF(agreementToken))...))
	})
}

func TestPageAgreementClient(gt *testing.T) {
	t := devtest.ParallelT(gt)
	themeSet := theme.MustAssets(theme.Options{})
	var posted url.Values
	mux := http.NewServeMux()
	mux.Handle(themeSet.Pattern(), themeSet)
	mux.HandleFunc("GET /tasks", func(w http.ResponseWriter, r *http.Request) {
		_ = web.Render(w, r, http.StatusOK, agreementDocument(themeSet))
	})
	mux.HandleFunc("POST /tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.ParseForm() != nil || r.PostForm.Get(session.CSRFField) != agreementToken {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		posted = r.PostForm
		web.Redirect(w, r, "/tasks")
	})
	c := &webuitest.Client{Handler: mux, CSRF: agreementToken, Contract: func(p *ct.Subject) assertion.Assertion {
		return webuitest.PageContract(p, webuitest.DefaultShell(themeSet), agreementRules()...)
	}}
	p := c.Page(t, c.Get(t, "/tasks"), http.StatusOK)
	// The dialog's form, as a user fills it in.
	res := c.Submit(t, p, p.Find(ct.Tag("form"), ct.Attr("action", "/tasks")), nil, url.Values{"title": {"Water the plants"}})
	t.Must(webuitest.Location(res, http.StatusSeeOther, ct.Exact("/tasks")))
	t.Must(be.Equal("Water the plants", posted.Get("title")))

	report := webuitest.Crawl(t, c, webuitest.CrawlOptions{
		Start:    []string{"/tasks"},
		Routes:   webuitest.MuxRoutes(mux),
		Skip:     func(path string) bool { return path != "/tasks" && !strings.HasPrefix(path, "/assets/") },
		Assets:   true,
		Contract: c.Contract,
	})
	t.Must(report)
	t.Must(report.Covers("/tasks"))
	t.Must(report.Uses(http.MethodPost, "/tasks", "/logout", "/tasks/clear", "/tasks/sort"))
}

func TestWebErrorsAgreement(gt *testing.T) {
	t := devtest.ParallelT(gt)
	themeSet := theme.MustAssets(theme.Options{})
	errs := web.Errors{Assets: []*asset.Set{themeSet}}
	mux := http.NewServeMux()
	mux.Handle("GET /tasks/{id}", errs.Handle(func(w http.ResponseWriter, r *http.Request) error {
		return web.Conflict("The task changed meanwhile.", errors.New("version mismatch"))
	}))
	mux.Handle("GET /boom", errs.Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})))
	mux.Handle("/", errs.NotFound())
	logs := webuitest.NewLogs()
	c := &webuitest.Client{Handler: mux, Options: []webuitest.ReqOption{webuitest.LogTo(logs.Logger())}}

	t.Run("htmx fragment", func(t devtest.T) {
		res := c.Get(t, "/tasks/7", webuitest.HTMX(), webuitest.HXTarget("list"))
		frag := c.Fragment(t, res, http.StatusConflict, "div")
		t.Must(frag.Find(ct.Role("alert")).Matches(ct.InnerTextMatches(ct.Contains("The task changed meanwhile."))))
		t.Must(webuitest.HXRetarget(res, ct.Exact("#"+page.AlertsID)))
		t.Must(res.Header(hx2.HeaderReswap, ct.Exact("innerHTML")))
		t.Must(be.False(strings.Contains(string(res.Bytes()), "version mismatch")))
	})
	t.Run("error pages", func(t devtest.T) {
		for _, c2 := range []struct {
			path   string
			status int
		}{
			{"/tasks/7", http.StatusConflict},
			{"/missing", http.StatusNotFound},
			{"/boom", http.StatusInternalServerError},
		} {
			p := c.Page(t, c.Get(t, c2.path), c2.status)
			t.Must(p.Valid(webuitest.LayoutRegions()))
			t.Must(p.Find(ct.Role("heading", ct.Level(1))).Count(1))
		}
		// The logs have the cause; the pages do not.
		t.Must(be.Substring(logs.String(), "handler panic"))
		t.Must(be.Substring(logs.String(), "version mismatch"))
	})
	t.Run("htmx not found", func(t devtest.T) {
		res := c.Get(t, "/missing", webuitest.HTMX())
		c.Fragment(t, res, http.StatusNotFound, "div")
		t.Must(webuitest.HXRetarget(res, ct.Exact("#"+page.AlertsID)))
	})
}

// TestWebRecoverStream checks that a stream handler that panics after its
// first event fails the stream assertions, as a real connection does:
// web.Recover aborts the started response.
func TestWebRecoverStream(gt *testing.T) {
	t := devtest.ParallelT(gt)
	h := web.Errors{}.Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: tick\ndata: 1\n\n"))
		w.(http.Flusher).Flush()
		panic("boom")
	}))
	logs := webuitest.NewLogs()
	r := newRecordT(t.Context())
	st := webuitest.ServeStream(r, h, webuitest.Get(t, "/live", webuitest.EventSource(), webuitest.LogTo(logs.Logger())))
	st.Expect(r, "tick")
	t.Must(be.False(r.failed))
	st.ExpectEnd(r)
	r.cleanup()
	t.Must(be.True(r.failed))
	t.Must(be.Substring(r.output(), "GET /live: want the end of the stream: the handler aborted the response"))
	t.Must(be.Substring(logs.String(), "handler panic"))
}
