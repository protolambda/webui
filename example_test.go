package webui_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/asset"
	"github.com/protolambda/webui/bs"
	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/mw"
	"github.com/protolambda/webui/page"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/theme"
	"github.com/protolambda/webui/web"
	"github.com/protolambda/webui/webuitest"
)

// assets are the embedded browser stack and theme, served from the
// application's own origin with integrity values.
var assets = theme.MustAssets(theme.Options{})

// layout renders a page in the standard layout of the application.
func layout(title string, main elem.Node) elem.Node {
	return page.Document{
		Title:  title + " · Tasks",
		Assets: []*asset.Set{assets},
		Body: page.Layout{
			Navbar: page.Navbar{
				Brand: page.Brand{Label: "Tasks", Href: "/"},
				Items: []page.NavItem{{Label: "Today", Href: "/", Current: true}},
			},
			Header: page.Header{Title: text.Text(title)},
			Main:   main,
		},
	}
}

// account is the value of a signed-in visitor's session.
type account struct{ Name string }

// newHandler returns the application served at site, the origin that
// browsers use, behind the recommended middleware stack.
func newHandler(site mw.Site, logger *slog.Logger) (http.Handler, error) {
	store, err := session.NewStore(session.Options[account]{})
	if err != nil {
		return nil, err
	}
	preauth, err := session.NewPreAuth(session.PreAuthOptions{})
	if err != nil {
		return nil, err
	}
	// The CSRF tokens of every form: the session's, or a pre-authentication
	// token for visitors without a session.
	forms := session.Forms[account]{PreAuth: preauth}
	// The error policy answers the errors that handlers return with error
	// pages, or with alerts in the page for htmx requests.
	errs := web.Errors{Assets: []*asset.Set{assets}}

	mux := http.NewServeMux()
	mux.Handle(assets.Pattern(), assets)
	mux.Handle("GET /{$}", errs.Handle(func(w http.ResponseWriter, r *http.Request) error {
		token, err := forms.PageToken(w, r)
		if err != nil {
			return err
		}
		return web.Render(w, r, http.StatusOK, layout("Today", taskForm(token, "", "")))
	}))
	mux.Handle("POST /tasks", errs.Handle(func(w http.ResponseWriter, r *http.Request) error {
		f, err := forms.Read(w, r) // bounded, and the CSRF token checked
		if err != nil {
			return err // 400, 403, 413 or 415, through errs
		}
		defer f.Zero()
		title := strings.TrimSpace(f.String("title"))
		if title == "" {
			form := taskForm(f.String(session.CSRFField), title, "Give the task a title.")
			return web.Choose(w, r, http.StatusUnprocessableEntity, form, layout("Today", form))
		}
		// ... save the task
		web.Redirect(w, r, "/")
		return nil
	}))
	mux.Handle("/", errs.NotFound())

	return mw.Chain(mux,
		mw.LogRequests(mw.RequestLog{Logger: logger, Routes: mux}),
		errs.Recover,
		errs.Intercept, // plain-text errors of other code reach htmx as alerts
		mw.BodyDeadline(mw.DefaultBodyTimeout),
		mw.WriteDeadline(mw.DefaultWriteTimeout),
		mw.SecurityHeaders(mw.Headers{HSTS: mw.HSTSFor(site)}),
		mw.HostCheck(site),
		mw.SameOrigin(mw.OriginPolicy{Deny: errs.Answer(mw.ErrCrossOrigin)}),
		mw.MaxBody(1<<20),
		store.Middleware, // last: refused requests never touch a session
	), nil
}

// taskForm renders the form that adds a task, with the title and the
// problem of a submission. It works as a plain form, and htmx swaps it in
// place.
func taskForm(token, title, problem string) elem.Node {
	var msg elem.Node
	if problem != "" {
		msg = text.Text(problem)
	}
	return session.PostForm("/tasks", token, attr.ID("task-form"), hx2.Post("/tasks"), hx2.Swap(hx2.SwapOuterHTML))(
		bs.Field{ID: "task-title", Name: "title", Label: text.Text("New task"), Value: title, Error: msg},
		bs.BtnPrimary(button.Type(button.TypeSubmit))(text.Text("Add")),
	)
}

// The quick start: an application with a form, behind the recommended
// middleware stack, used once through webuitest as a browser on
// webuitest.BaseURL uses it.
//
// A main function serves the same handler at the origin that browsers
// use:
//
//	handler, err := newHandler(mw.MustParseSite("https://tasks.example.com"), logger)
//	...
//	ln, err := net.Listen("tcp", "127.0.0.1:8080") // behind the site's TLS proxy
//	...
//	err = server.Run(ctx, server.New(handler, server.Options{Logger: logger}), ln)
func Example() {
	site := mw.MustParseSite(webuitest.BaseURL)
	handler, err := newHandler(site, slog.New(slog.DiscardHandler))
	if err != nil {
		fmt.Println(err)
		return
	}

	// The page, as a browser gets it.
	t := webuitest.PrintT{}
	b := webuitest.NewBrowser(t, handler, webuitest.BrowserOptions{})
	res := b.Get(t, "/")
	fmt.Println(res.StatusCode(), res.Headers().Get("Content-Type"))
	csp, err := mw.ParseCSP(res.Headers().Get("Content-Security-Policy"))
	if err != nil {
		fmt.Println(err)
		return
	}
	scripts, _ := csp.Values(mw.ScriptSrc)
	fmt.Println("script-src", strings.Join(scripts, " "))
	fmt.Println(res.Headers().Get("Strict-Transport-Security"))

	// It has the document head of its assets, passes every webui rule (no
	// inline script or style, integrity on every asset, named controls, a
	// CSRF field in every post form, ...) and has the regions of the
	// layout.
	headers := mw.Headers{HSTS: mw.HSTSFor(site)}
	rules := append(webuitest.Rules(), webuitest.LayoutRegions())
	p := res.HTML()
	t.Must(webuitest.PageContract(p, webuitest.DefaultShell(assets), rules...))
	t.Must(webuitest.SecurityHeaders(res, webuitest.PolicyFor(headers)))
	fmt.Println("page contract passes")

	// The form, submitted as the browser submits it: with a problem, as a
	// valid task, and with a forged token.
	form := p.Find(ct.ID("task-form"))
	res = b.Submit(t, p, form, nil, url.Values{"title": {" "}})
	problem, _ := res.HTML().Find(ct.ID("task-title-error")).Text(context.Background())
	fmt.Println(res.StatusCode(), problem)
	res = b.Submit(t, p, form, nil, url.Values{"title": {"Water the plants"}})
	fmt.Println(res.StatusCode(), res.Headers().Get("Location"))
	res = b.Submit(t, p, form, nil, url.Values{"title": {"Water the plants"}, session.CSRFField: {"forged"}})
	fmt.Println(res.StatusCode())

	// The assets are cached for good: their URLs change with their
	// content.
	res = b.Get(t, assets.URL(theme.HTMXJS))
	t.Must(webuitest.SecurityHeaders(res, webuitest.AssetPolicyFor(headers)))
	fmt.Println(res.Headers().Get("Cache-Control"))

	// Unknown paths get the error page, with the same headers.
	fmt.Println(b.Get(t, "/nowhere").StatusCode())
	// Output:
	// 200 text/html; charset=utf-8
	// script-src 'self'
	// max-age=31536000
	// page contract passes
	// 422 Give the task a title.
	// 303 /
	// 403
	// public, max-age=31536000, immutable
	// 404
}
