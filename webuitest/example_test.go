package webuitest_test

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing/fstest"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthtml"
	"github.com/protolambda/chord/html/form"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/form/input"
	"github.com/protolambda/chord/html/form/label"
	"github.com/protolambda/chord/html/interactive"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/asset"
	"github.com/protolambda/webui/bi"
	"github.com/protolambda/webui/bs"
	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/mw"
	"github.com/protolambda/webui/page"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/sse"
	"github.com/protolambda/webui/webuitest"
)

// PrintT prints failures, so an Example shows what an assertion reports.
// All checks every assertion and reports each failure.
func ExampleAll() {
	t := webuitest.PrintT{}
	t.Must(webuitest.All("settings", be.Equal(3, 3), be.Equal("dark", "light"), be.True(false)))
	fmt.Println("continues after a failure")
	// Output:
	// assertion failed: settings: 2 failures:
	// not equal, expected: dark, got: light
	// expected True
	// continues after a failure
}

// Rules checks a view against every webui rule at once; the failure names
// each violation with the rule and the location.
func ExampleRules() {
	t := webuitest.PrintT{}
	view := section.Main()(
		form.Form(form.Method(form.MethodPost), form.Action("/tasks"))(
			input.Input(input.Name("title"), attr.Style("width: 10em")),
			button.Button(button.Type(button.TypeSubmit))(bi.Plus),
		),
		text.A(text.Href("javascript:void(0)"))(text.Text("Help")),
	)
	t.Must(ct.View(view).Valid(webuitest.Rules(webuitest.WithCSRF("token-of-the-session"))...))

	fixed := section.Main()(
		form.Form(form.Method(form.MethodPost), form.Action("/tasks"))(
			session.CSRFInput("token-of-the-session"),
			label.Label(label.For("title"))(text.Text("Title")),
			input.Input(attr.ID("title"), input.Name("title")),
			button.Button(button.Type(button.TypeSubmit), attr.KV("aria-label", "Add"))(bi.Plus),
		),
		text.A(text.Href("/help"))(text.Text("Help")),
	)
	t.Must(ct.View(fixed).Valid(webuitest.Rules(webuitest.WithCSRF("token-of-the-session"))...))
	fmt.Println("fixed")
	// Output:
	// assertion failed: document rule violated in chord view:
	//   noInlineStyles(): style attribute at main[0]/form[0]/input[0]
	//   safeURLs(): href at main[0]/a[1] is a javascript: URL
	//   postFormsCarryCSRF(token): post form at main[0]/form[0] has no "csrf" field
	//   controlsNamed(): input at main[0]/form[0]/input[0] has no accessible name
	//   controlsNamed(): button at main[0]/form[0]/button[1] has no accessible name
	// fixed
}

// exampleAssets is a small, stable asset set for examples.
func exampleAssets() *asset.Set {
	return asset.Must(asset.New(asset.Options{
		Prefix: "/assets/app/",
		FS: fstest.MapFS{
			"app.css": {Data: []byte("main{padding:1rem}")},
			"app.js":  {Data: []byte("\"use strict\";")},
		},
		Files: []asset.File{{Name: "app.css"}, {Name: "app.js"}},
	}))
}

// examplePage is a page.Document with the htmx configuration c.
func examplePage(set *asset.Set, c hx2.Config) elem.Node {
	return page.Document{
		Title:  "Notes",
		Assets: []*asset.Set{set},
		HTMX:   c,
		Body:   section.Main(attr.ID(page.MainID))(section.H1()(text.Text("Notes"))),
	}
}

// ShellContract checks the head of a page against the application's
// shell: here an htmx configuration that allows eval.
func ExampleShellContract() {
	t := webuitest.PrintT{}
	set := exampleAssets()
	shell := webuitest.DefaultShell(set)
	t.Must(webuitest.ShellContract(ct.View(examplePage(set, hx2.Config{})), shell))
	fmt.Println("the default page passes")
	t.Must(webuitest.ShellContract(ct.View(examplePage(set, hx2.Config{AllowEval: true, HistoryCacheSize: 10})), shell))
	// Output:
	// the default page passes
	// assertion failed: document rule violated in chord view:
	//   shell(): htmx-config "allowEval" is true, want false
	//   shell(): htmx-config "historyCacheSize" is 10, want 0
}

// ParseEvents reads a finished event stream as a browser does.
func ExampleParseEvents() {
	events, comments, err := webuitest.ParseEvents([]byte(": keep-alive\r\n\r\nevent: tick\ndata:1s\n\ndata: a\ndata: b\nid: 7\n\n"))
	fmt.Println(comments, err)
	for _, e := range events {
		fmt.Printf("%s %q %q\n", e.Name, e.Data, e.ID)
	}
	_, _, err = webuitest.ParseEvents([]byte("event: end\ndata: <p>bye</p>\n"))
	fmt.Println(err)
	// Output:
	// 1 <nil>
	// tick "1s" ""
	// message "a\nb" "7"
	// webuitest: event stream ends inside an event
}

// SecurityHeaders compares a response with the headers of webui's
// middleware, directive by directive.
func ExampleSecurityHeaders() {
	t := webuitest.PrintT{}
	page := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/legacy" {
			mw.SetCSP(w, mw.StrictCSP().With(mw.ScriptSrc, mw.SourceSelf, "'unsafe-eval'"))
		}
	})
	h := mw.SecurityHeaders(mw.Headers{})(page)
	t.Must(webuitest.SecurityHeaders(webuitest.Serve(t, h, webuitest.Get(t, "/")), webuitest.StrictPolicy()))
	fmt.Println("strict")
	t.Must(webuitest.SecurityHeaders(webuitest.Serve(t, h, webuitest.Get(t, "/legacy")), webuitest.StrictPolicy()))
	// Output:
	// strict
	// assertion failed: selected nodes do not match: security headers of GET /legacy:
	//   CSP script-src allows 'unsafe-eval'
	//   CSP script-src is 'self' 'unsafe-eval', want 'self'
}

// Cookie checks the attributes of a cookie, and NoCookie its absence,
// never showing its value.
func ExampleCookie() {
	t := webuitest.PrintT{}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "__Host-session", Value: "secret-session-token", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
		http.SetCookie(w, &http.Cookie{Name: "theme", Value: "dark", MaxAge: 3600})
	})
	res := webuitest.Serve(t, h, webuitest.Get(t, "/"))
	t.Must(webuitest.Cookie(res, "__Host-session", webuitest.CookieWant{
		HostPrefix: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, Session: true,
	}))
	fmt.Println("session cookie ok")
	t.Must(webuitest.Cookie(res, "theme", webuitest.CookieWant{HttpOnly: true, Session: true}))
	t.Must(webuitest.Cookie(res, "theme", webuitest.CookieWant{NotSecure: true, NotHttpOnly: true}))
	t.Must(webuitest.NoCookie(res, "flash"))
	fmt.Println("theme cookie ok, no flash cookie")
	t.Must(webuitest.NoCookie(res, "theme"))
	t.Must(webuitest.NoCookies(res))
	// Output:
	// session cookie ok
	// assertion failed: selected nodes do not match: cookie "theme" of GET /: not HttpOnly; has Max-Age or Expires, want a session cookie
	// theme cookie ok, no flash cookie
	// assertion failed: selected nodes do not match: GET / sets cookie "theme"
	// assertion failed: selected nodes do not match: GET / sets 2 cookies: "__Host-session", "theme"
}

// FormData computes what a browser submits for a form and a submit
// button.
func ExampleFormData() {
	t := webuitest.PrintT{}
	page := cthtml.PageString(`<!DOCTYPE html><html><head><title>Edit</title></head><body>
<form id="edit" action="/notes/3" method="post">
  <input type="hidden" name="csrf" value="token">
  <input name="title" value="Groceries">
  <textarea name="body">milk
eggs</textarea>
  <input type="checkbox" name="pinned" checked>
  <input type="checkbox" name="archived">
  <select name="color"><option>none</option><option value="blue" selected>Blue</option></select>
  <button name="action" value="save">Save</button>
  <button name="action" value="delete" formaction="/notes/3/delete">Delete</button>
</form></body></html>`)
	sub, err := webuitest.FormData(t.Context(), page, page.Find(ct.ID("edit")), page.Find(ct.Tag("button"), ct.Text("Delete")))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(sub.Method, sub.Action, sub.Enctype)
	for _, f := range sub.Fields {
		fmt.Printf("%s=%q\n", f.Name, f.Value)
	}
	sub.Set("title", "Shopping")
	req := sub.Request(t)
	body, _ := io.ReadAll(req.Body)
	fmt.Println(string(body))
	// Output:
	// POST /notes/3/delete application/x-www-form-urlencoded
	// csrf="token"
	// title="Groceries"
	// body="milk\r\neggs"
	// pinned="on"
	// color="blue"
	// action="delete"
	// csrf=token&title=Shopping&body=milk%0D%0Aeggs&pinned=on&color=blue&action=delete
}

// A Client serves an application's handler, adds the CSRF field to its
// posts, and checks pages against the page contract.
func ExampleClient() {
	t := webuitest.PrintT{}
	sess := session.NewDetached("account", "csrf-token")
	mux := http.NewServeMux()
	forms := session.Forms[string]{} // checks the token of the context's session
	mux.HandleFunc("POST /notes", func(w http.ResponseWriter, r *http.Request) {
		f, err := forms.Read(w, r)
		if err != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		defer f.Zero()
		http.Redirect(w, r, "/notes/1", http.StatusSeeOther)
	})
	c := &webuitest.Client{
		Handler: mux,
		Prepare: func(r *http.Request) *http.Request {
			return r.WithContext(session.WithSession(r.Context(), sess))
		},
		CSRF: sess.CSRF(),
	}
	res := c.Post(t, "/notes", webuitest.Form(url.Values{"title": {"Groceries"}}))
	t.Must(webuitest.Location(res, http.StatusSeeOther, ct.Exact("/notes/1")))
	fmt.Println("created")
	forged := c.Post(t, "/notes", webuitest.Form(url.Values{session.CSRFField: {"guess"}}))
	t.Must(forged.Status(http.StatusForbidden))
	fmt.Println("forged post refused")
	// Output:
	// created
	// forged post refused
}

// ServeStream reads a live event stream from a handler, without a
// network; its events parse as HTML fragments.
func ExampleServeStream() {
	t := webuitest.PrintT{}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := sse.New(w)
		_ = s.Send(r.Context(), "count", text.Strong()(text.Text("3 notes")))
		_ = s.Comment("keep-alive")
		_ = s.End(r.Context(), text.P()(text.Text("Signed out.")))
	})
	st := webuitest.ServeStream(t, h, webuitest.Get(t, "/notes/live", webuitest.EventSource()))
	defer st.Close()
	t.Must(st.Header("Content-Type", ct.Prefix("text/event-stream")))
	e := st.Expect(t, "count")
	fmt.Println(e.Name, e.Data)
	t.Must(e.Fragment("div").Find(ct.Tag("strong")).Texts("3 notes"))
	fmt.Println(st.Expect(t, sse.EventEnd).Data)
	st.ExpectEnd(t)
	fmt.Println("comments:", st.Comments())
	// Output:
	// count <strong>3 notes</strong>
	// <p>Signed out.</p>
	// comments: 1
}

// ModalDialogs checks the no-JS dialog markup: bs.Dialog and its buttons
// satisfy it, a hand-written dialog shown with open does not.
func ExampleModalDialogs() {
	t := webuitest.PrintT{}
	good := elem.Bundle{
		bs.DialogButton("rename", attr.Class("btn btn-primary"))(text.Text("Rename")),
		bs.Dialog{ID: "rename", Title: text.Text("Rename the note"), Body: text.P()(text.Text("..."))},
	}
	t.Must(ct.View(good).Valid(webuitest.ModalDialogs()))
	fmt.Println("bs.Dialog passes")
	bad := elem.Bundle{
		button.Button(attr.KV("commandfor", "rename"), attr.KV("command", "show-modal"))(text.Text("Rename")),
		interactive.Dialog(attr.ID("rename"), attr.Class("modal"), attr.Bool("open"))(text.P()(text.Text("..."))),
	}
	t.Must(ct.View(bad).Valid(webuitest.ModalDialogs()))
	// Output:
	// bs.Dialog passes
	// assertion failed: document rule violated in chord view:
	//   modalDialogs(): dialog at dialog#rename[1] has no popover attribute, which browsers without invoker commands need
	//   modalDialogs(): dialog at dialog#rename[1] has no aria-labelledby or aria-label
	//   modalDialogs(): dialog at dialog#rename[1] is rendered open
	//   modalDialogs(): dialog at dialog#rename[1] has no autofocus control
	//   modalDialogs(): commandfor at button[0] is not on a button of type button
	//   modalDialogs(): commandfor at button[0] has no popovertarget naming the same dialog
	//   modalDialogs(): dialog at dialog#rename[1] has no close button (command="close")
}
