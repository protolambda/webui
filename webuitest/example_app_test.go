package webuitest_test

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"testing/fstest"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/html/form"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/form/input"
	"github.com/protolambda/chord/html/form/label"
	"github.com/protolambda/chord/html/group/list"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"
	"github.com/protolambda/mustbe/assertion"

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

// notesPage is a page of a small notes application: page.Document around
// page.Layout, with a form that posts a new note.
func notesPage(set *asset.Set, csrf string, action string) elem.Node {
	return page.Document{
		Title:  "Notes",
		Assets: []*asset.Set{set},
		Body: page.Layout{
			Navbar: page.Navbar{Brand: page.Brand{Label: "Notes"}, Items: []page.NavItem{{Label: "Notes", Href: "/notes", Current: true}}},
			Header: page.Header{Title: text.Text("Notes")},
			Main: form.Form(form.Method(form.MethodPost), form.Action(action))(
				session.CSRFInput(csrf),
				label.Label(label.For("title"), attr.Class("form-label"))(text.Text("Title")),
				input.Input(attr.ID("title"), input.Name("title"), attr.Class("form-control")),
				bs.BtnPrimary(button.Type(button.TypeSubmit))(text.Text("Add")),
			),
		},
	}
}

// notesApp serves the notes pages and their assets behind webui's
// same-origin policy, and records the notes it is sent.
func notesApp(set *asset.Set, csrf string, notes *[]string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(set.Pattern(), set)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		web.Redirect(w, r, "/notes")
	})
	mux.HandleFunc("GET /notes", func(w http.ResponseWriter, r *http.Request) {
		_ = web.Render(w, r, http.StatusOK, notesPage(set, csrf, "/notes"))
	})
	mux.HandleFunc("GET /notes/drafts/", func(w http.ResponseWriter, r *http.Request) {
		// A form whose action is relative to the page's URL.
		_ = web.Render(w, r, http.StatusOK, notesPage(set, csrf, "publish"))
	})
	add := func(w http.ResponseWriter, r *http.Request) {
		if r.ParseForm() != nil || session.CheckCSRF(csrf, r.PostForm.Get(session.CSRFField)) != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		*notes = append(*notes, r.URL.Path+": "+r.PostForm.Get("title"))
		web.Redirect(w, r, "/notes")
	}
	mux.HandleFunc("POST /notes", add)
	mux.HandleFunc("POST /notes/drafts/publish", add)
	return mw.Chain(mux, mw.HostCheck(mw.MustParseSite(webuitest.BaseURL)), mw.SameOrigin(mw.OriginPolicy{}))
}

// PageContract checks a page.Document and page.Layout against the shell of
// its asset sets, every webui rule and the regions of the layout.
func ExamplePageContract() {
	t := webuitest.PrintT{}
	set := theme.MustAssets(theme.Options{})
	rules := append(webuitest.Rules(webuitest.WithCSRF("csrf-token")), webuitest.LayoutRegions())
	t.Must(webuitest.PageContract(ct.View(notesPage(set, "csrf-token", "/notes")), webuitest.DefaultShell(set), rules...))
	fmt.Println("the notes page passes")
	t.Must(webuitest.PageContract(ct.View(notesPage(set, "stale-token", "javascript:void(0)")), webuitest.DefaultShell(set), rules...))
	// Output:
	// the notes page passes
	// assertion failed: document rule violated in chord view:
	//   safeURLs(): action at html[1]/body[1]/div[0]/main#main[2]/div[0]/form[2] is a javascript: URL
	//   postFormsCarryCSRF(token): post form at html[1]/body[1]/div[0]/main#main[2]/div[0]/form[2] has a "csrf" field with another value
}

// LayoutRegions checks the regions of page.Layout: the main landmark, the
// alerts region inside it, and the dialogs region outside main and every
// form, which holds every dialog.
func ExampleLayoutRegions() {
	t := webuitest.PrintT{}
	dialog := bs.Dialog{ID: "help", Title: text.Text("Help"), Body: text.P()(text.Text("Notes are private."))}
	layout := page.Layout{
		Header:  page.Header{Title: text.Text("Notes")},
		Main:    bs.DialogButton("help", attr.Class("btn btn-link"))(text.Text("Help")),
		Dialogs: []elem.Node{dialog},
	}
	t.Must(ct.View(page.Document{Title: "Notes", Body: layout}).Valid(webuitest.LayoutRegions()))
	fmt.Println("the layout passes")
	// A dialog in the content instead of the dialogs region.
	layout.Main, layout.Dialogs = elem.Bundle{layout.Main, dialog}, nil
	t.Must(ct.View(page.Document{Title: "Notes", Body: layout}).Valid(webuitest.LayoutRegions()))
	// Output:
	// the layout passes
	// assertion failed: document rule violated in chord view:
	//   layoutRegions(): dialog at html[1]/body[1]/div[0]/main#main[1]/div[0]/dialog#help[3] is outside the "dialogs" region
}

// FragmentContract checks what htmx swaps into a page: here a list that
// refreshes the navbar of page.Layout, with a notice for its alerts region
// out of band.
func ExampleFragmentContract() {
	t := webuitest.PrintT{}
	frag := elem.Bundle{
		list.UL()(list.LI()(text.Text("Groceries"))),
		bs.BtnLink(button.Type(button.TypeButton), hx2.Get("/navbar"), hx2.Target("#"+page.NavbarID))(text.Text("Refresh")),
		page.AlertsOOB(page.Notice(bs.VariantSuccess, text.Text("Saved."))),
	}
	t.Must(webuitest.FragmentContract(ct.View(frag)))
	fmt.Println("the fragment passes")
	broken := elem.Bundle{
		list.UL()(list.LI()(text.Text("Groceries"))),
		bs.BtnLink(button.Type(button.TypeButton), hx2.Get("/list"), hx2.Target("#notes-list"))(text.Text("Refresh")),
	}
	t.Must(webuitest.FragmentContract(ct.View(broken)))
	// Output:
	// the fragment passes
	// assertion failed: document rule violated in chord view:
	//   hxTargets("main", "alerts", "dialogs", "page-header", "navbar", "sidebar"): hx-target at button[1] targets missing id "notes-list"
}

// Submit submits a page's own form as a browser would. A form whose
// action is relative to the page needs the page's URL.
func ExampleClient_Submit() {
	t := webuitest.PrintT{}
	var notes []string
	c := &webuitest.Client{Handler: notesApp(theme.MustAssets(theme.Options{}), "csrf-token", &notes), CSRF: "csrf-token"}
	p := c.Page(t, c.Get(t, "/notes"), http.StatusOK)
	res := c.Submit(t, p, p.Find(ct.Tag("form"), ct.Attr("method", "post")), nil, url.Values{"title": {"Groceries"}})
	t.Must(webuitest.Location(res, http.StatusSeeOther, ct.Exact("/notes")))

	const at = "/notes/drafts/"
	draft := c.Page(t, c.Get(t, at), http.StatusOK)
	res = c.Submit(t, draft, draft.Find(ct.Tag("form"), ct.Attr("method", "post")), nil, url.Values{"title": {"Plans"}}, webuitest.PageURL(at))
	t.Must(webuitest.Location(res, http.StatusSeeOther, ct.Exact("/notes")))
	fmt.Println(notes)
	// Output:
	// [/notes: Groceries /notes/drafts/publish: Plans]
}

// RejectsCrossSite sends the probes of CrossSiteProbes for a
// state-changing request, and asserts that each was refused without
// changing the state.
func ExampleRejectsCrossSite() {
	t := webuitest.PrintT{}
	var notes []string
	c := &webuitest.Client{Handler: notesApp(theme.MustAssets(theme.Options{}), "csrf-token", &notes), CSRF: "csrf-token"}
	t.Must(webuitest.RejectsCrossSite(t, c, http.MethodPost, "/notes", webuitest.Form(url.Values{"title": {"x"}}), func() error {
		if len(notes) > 0 {
			return fmt.Errorf("%d notes were added", len(notes))
		}
		return nil
	}))
	for _, p := range webuitest.CrossSiteProbes() {
		fmt.Println(p.Name)
	}
	// Output:
	// cross-site
	// same-site
	// foreign origin without fetch metadata
	// lookalike origin
	// suffix origin
	// other port origin
	// null origin
	// http origin
	// foreign referer only
	// lookalike referer only
	// no origin headers
	// dns rebinding
}

// Crawl follows the links of an application and checks every page, target
// and asset it finds.
func ExampleCrawl() {
	t := webuitest.PrintT{}
	var notes []string
	c := &webuitest.Client{Handler: notesApp(theme.MustAssets(theme.Options{}), "csrf-token", &notes)}
	report := webuitest.Crawl(t, c, webuitest.CrawlOptions{Start: []string{"/"}, Assets: true})
	t.Must(report)
	t.Must(report.Uses(http.MethodPost, "/notes"))
	fmt.Println(report.Pages)
	assets := 0
	for _, tg := range report.Targets {
		if tg.Kind == webuitest.TargetAsset {
			assets++
		}
	}
	fmt.Println(assets, "assets")
	// Output:
	// [/notes]
	// 5 assets
}

// A Browser serves the application over TLS and keeps cookies, for
// full-stack tests: the URLs, the Host, the Origin and the TLS server name
// are those of BaseURL.
func ExampleNewBrowser() {
	t := webuitest.PrintT{}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie("visited"); err == nil {
			fmt.Fprintf(w, "welcome back (%s, %s)", r.Host, r.TLS.ServerName)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "visited", Value: "1", Path: "/", Secure: true, HttpOnly: true})
		fmt.Fprint(w, "hello")
	})
	b := webuitest.NewBrowser(t, h, webuitest.BrowserOptions{})
	defer b.Server().Close()
	fmt.Println(b.URL())
	fmt.Println(string(b.Get(t, "/").Bytes()))
	fmt.Println(string(b.Get(t, "/").Bytes()))
	fmt.Println(string(b.Fork().Get(t, "/").Bytes()))
	// Output:
	// https://app.example.test
	// hello
	// welcome back (app.example.test, app.example.test)
	// hello
}

// NotLogged checks that no secret reached the logs, also in the escaped
// and percent-encoded forms in which loggers write values.
func ExampleLogs_NotLogged() {
	t := webuitest.PrintT{}
	logs := webuitest.NewLogs()
	h := mw.LogRequests(mw.RequestLog{Logger: logs.Logger(), Path: true})(http.NotFoundHandler())
	webuitest.Serve(t, h, webuitest.Get(t, "/notes"))
	t.Must(logs.NotLogged("Medical results.pdf"))
	fmt.Println("nothing leaked")
	webuitest.Serve(t, h, webuitest.Get(t, "/files/"+url.PathEscape("Medical results.pdf")))
	t.Must(logs.NotLogged("Medical results.pdf"))
	// Output:
	// nothing leaked
	// assertion failed: webuitest: secret leaked: the logs leak secrets: secret 1 appears 1 times
}

// Fragment checks the answer to an htmx request: web.Errors retargets an
// error to the alerts region of page.Layout, keeping the status. The alert
// has no role of its own: the region, a live region, announces it.
func ExampleClient_Fragment() {
	t := webuitest.PrintT{}
	errs := web.Errors{}
	h := errs.Handle(func(w http.ResponseWriter, r *http.Request) error {
		return web.Conflict("The note changed meanwhile.", nil)
	})
	c := &webuitest.Client{Handler: h}
	res := c.Get(t, "/notes/3", webuitest.HTMX())
	frag := c.Fragment(t, res, http.StatusConflict, "div")
	t.Must(webuitest.HXRetarget(res, ct.Exact("#"+page.AlertsID)))
	t.Must(frag.Find(ct.Class("alert-danger")).Count(1))
	fmt.Println(res.Headers().Get("HX-Retarget"), res.Headers().Get("HX-Reswap"))
	// Output:
	// #alerts innerHTML show:top
}

// The request options build what a browser and htmx send: here an htmx
// request for a fragment of a page.
func ExampleHTMX() {
	t := webuitest.PrintT{}
	req := webuitest.Get(t, "/notes/list", webuitest.HTMX(), webuitest.HXTarget("list"), webuitest.HXCurrentURL("/notes"))
	names := make([]string, 0, len(req.Header))
	for name := range req.Header {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		fmt.Printf("%s: %s\n", name, req.Header.Get(name))
	}
	fmt.Println(req.Host, req.RequestURI)
	// Output:
	// Hx-Current-Url: https://app.example.test/notes
	// Hx-Request: true
	// Hx-Target: list
	// Sec-Fetch-Dest: empty
	// Sec-Fetch-Mode: cors
	// Sec-Fetch-Site: same-origin
	// app.example.test /notes/list
}

// DefaultShell follows how page.Document renders asset sets: stylesheets,
// then scripts and modules, without the files a set leaves unlisted.
func ExampleDefaultShell() {
	set := appAssets()
	shell := webuitest.DefaultShell(set)
	for _, a := range shell.Stylesheets {
		fmt.Println("stylesheet", a.Name)
	}
	for _, a := range shell.Scripts {
		fmt.Println("script", a.Name, "module:", a.Module)
	}
	t := webuitest.PrintT{}
	t.Must(webuitest.ShellContract(ct.View(page.Document{
		Title:  "Notes",
		Assets: []*asset.Set{set},
		Body:   section.Main()(section.H1()(text.Text("Notes"))),
	}), shell))
	// Output:
	// stylesheet app.css
	// script app.js module: false
	// script app.mjs module: true
}

// AssetPolicy expects the security headers of webui's middleware on the
// files of an asset set, which replace the Cache-Control with their own.
func ExampleAssetPolicy() {
	t := webuitest.PrintT{}
	set := appAssets()
	mux := http.NewServeMux()
	mux.Handle(set.Pattern(), set)
	h := mw.SecurityHeaders(mw.Headers{})(mux)
	res := webuitest.Serve(t, h, webuitest.Get(t, set.URL("app.css")))
	t.Must(webuitest.SecurityHeaders(res, webuitest.AssetPolicy()))
	fmt.Println(res.Headers().Get("Cache-Control") == asset.CacheImmutable)
	// The policy of pages does not fit.
	t.Must(webuitest.SecurityHeaders(res, webuitest.StrictPolicy()))
	// Output:
	// true
	// assertion failed: selected nodes do not match: security headers of GET /assets/app/94e344c36323b020/app.css:
	//   has Cache-Control "public, max-age=31536000, immutable", want "no-store"
}

// ContractFor picks the contract of each page by its URL: here the report
// page loads a second asset set, which the shell of the other pages does
// not expect.
func ExampleCrawl_contractFor() {
	t := webuitest.PrintT{}
	set := appAssets()
	charts := asset.Must(asset.New(asset.Options{
		Prefix: "/assets/charts/",
		FS:     fstest.MapFS{"charts.js": {Data: []byte("\"use strict\";")}},
		Files:  []asset.File{{Name: "charts.js"}},
	}))
	doc := func(title string, sets []*asset.Set, body ...elem.Node) elem.Node {
		return page.Document{Title: title, Assets: sets, Body: section.Main()(append([]elem.Node{section.H1()(text.Text(title))}, body...)...)}
	}
	mux := http.NewServeMux()
	mux.Handle(set.Pattern(), set)
	mux.Handle(charts.Pattern(), charts)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		_ = web.Render(w, r, http.StatusOK, doc("Projects", []*asset.Set{set}, text.A(text.Href("/report"))(text.Text("Report"))))
	})
	mux.HandleFunc("GET /report", func(w http.ResponseWriter, r *http.Request) {
		_ = web.Render(w, r, http.StatusOK, doc("Report", []*asset.Set{set, charts}))
	})
	c := &webuitest.Client{Handler: mux}

	plain := webuitest.DefaultShell(set)
	contract := func(p *ct.Subject) assertion.Assertion { return webuitest.PageContract(p, plain) }
	report := webuitest.Crawl(t, c, webuitest.CrawlOptions{Contract: contract})
	fmt.Println(len(report.Problems), "problem without ContractFor")

	withCharts := webuitest.DefaultShell(set, charts)
	report = webuitest.Crawl(t, c, webuitest.CrawlOptions{
		Contract: contract,
		ContractFor: func(u *url.URL) webuitest.Contract {
			if u.Path == "/report" {
				return func(p *ct.Subject) assertion.Assertion { return webuitest.PageContract(p, withCharts) }
			}
			return nil // Contract
		},
		Assets: true,
	})
	t.Must(report)
	fmt.Println(report.Pages)
	// Output:
	// 1 problem without ContractFor
	// [/ /report]
}
