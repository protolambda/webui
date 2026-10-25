package webuitest_test

import (
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthtml"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/text"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/webuitest"
)

// seenRequest is what a handler received.
type seenRequest struct {
	method, uri, body string
	header            http.Header
}

// requestRecorder is a handler that records the last request it served.
type requestRecorder struct {
	mu   sync.Mutex
	last seenRequest
}

func (rr *requestRecorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	rr.mu.Lock()
	rr.last = seenRequest{method: r.Method, uri: r.RequestURI, body: string(b), header: r.Header.Clone()}
	rr.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (rr *requestRecorder) seen() seenRequest {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.last
}

// webuiPage wraps body in a minimal document with webui's htmx
// configuration, as page.Document renders it.
func webuiPage(body string) string {
	return `<!DOCTYPE html><html lang="en"><head><title>t</title><meta name="htmx-config" content='` + webuitest.DefaultHTMXConfig() + `'></head><body>` + body + `</body></html>`
}

// htmxClient returns a client of a recorder.
func htmxClient() (*webuitest.Client, *requestRecorder) {
	rr := &requestRecorder{}
	return &webuitest.Client{Handler: rr}, rr
}

func TestSubmitHTMX(gt *testing.T) {
	t := devtest.ParallelT(gt)
	const token = "csrf-token-of-the-session"
	csrf := `<input type="hidden" name="csrf" value="` + token + `">`
	t.Run("form with a fallback action", func(t devtest.T) {
		c, rr := htmxClient()
		page := cthtml.PageString(webuiPage(`<ul id="list"></ul><form id="f" method="post" action="/notes/1/delete" hx-delete="/notes/1" hx-target="#list">` +
			csrf + `<input name="title" value="a b"><button name="op" value="del">Delete</button></form>`))
		form, button := page.Find(ct.ID("f")), page.Find(ct.Tag("button"))
		t.Must(c.SubmitHTMX(t, page, form, button, nil).Status(http.StatusNoContent))
		got := rr.seen()
		t.Must(be.Equal("DELETE", got.method))
		t.Must(be.Equal("/notes/1", got.uri))
		t.Must(be.Equal("csrf="+token+"&title=a+b&op=del", got.body))
		t.Must(be.Equal("application/x-www-form-urlencoded", got.header.Get("Content-Type")))
		t.Must(be.Equal("true", got.header.Get("HX-Request")))
		t.Must(be.Equal("f", got.header.Get("HX-Trigger")))
		t.Must(be.Equal("list", got.header.Get("HX-Target")))
		t.Must(be.Equal("", got.header.Get("HX-Trigger-Name")))
		t.Must(be.Equal("", got.header.Get("HX-Current-URL")))
		t.Must(be.Equal(webuitest.BaseURL, got.header.Get("Origin")))
		t.Must(be.Equal("cors", got.header.Get("Sec-Fetch-Mode")))

		// Submit sends the submission without script; with HTMX it would mix
		// the two requests, so it fails.
		t.Must(c.Submit(t, page, form, button, nil).Status(http.StatusNoContent))
		t.Must(be.Equal("POST /notes/1/delete", rr.seen().method+" "+rr.seen().uri))
		r := newRecordT(t.Context())
		t.Must(be.True(c.Submit(r, page, form, button, nil, webuitest.HTMX()) == nil))
		t.Must(be.Substring(r.output(), `webuitest: cannot compute form data: htmx sends DELETE "/notes/1" for the hx-delete of the form, not the form's POST "/notes/1/delete"; send it with SubmitHTMX`))
		r = newRecordT(t.Context())
		t.Must(be.True(c.Submit(r, page, form, button, nil, webuitest.HXTarget("list")) == nil))
		t.Must(be.Substring(r.output(), "send it with SubmitHTMX"))
	})
	t.Run("a matching form submits with HTMX", func(t devtest.T) {
		c, rr := htmxClient()
		page := cthtml.PageString(webuiPage(`<form method="post" action=" /tasks" hx-post="/tasks">` + csrf + `<button formaction="/tasks/2" hx-post="/tasks/2">Save</button></form>`))
		t.Must(c.Submit(t, page, page.Find(ct.Tag("form")), nil, nil, webuitest.HTMX()).Status(http.StatusNoContent))
		t.Must(be.Equal("/tasks", rr.seen().uri))
		t.Must(c.Submit(t, page, page.Find(ct.Tag("form")), page.Find(ct.Tag("button")), nil, webuitest.HTMX()).Status(http.StatusNoContent))
		t.Must(be.Equal("/tasks/2", rr.seen().uri))
	})
	t.Run("button in a form", func(t devtest.T) {
		c, rr := htmxClient()
		page := cthtml.PageString(webuiPage(`<ul><li id="task-1"><form method="post" action="/tasks/1">` + csrf +
			`<input name="title" value="x"><button type="button" id="del" name="act" value="rm" hx-delete="/tasks/1" hx-target="closest li">Delete</button></form></li></ul>`))
		t.Must(c.SubmitHTMX(t, page, page.Find(ct.ID("del")), nil, url.Values{"title": {"edited"}}).Status(http.StatusNoContent))
		got := rr.seen()
		t.Must(be.Equal("DELETE /tasks/1", got.method+" "+got.uri))
		t.Must(be.Equal("csrf="+token+"&title=edited&act=rm", got.body))
		t.Must(be.Equal("del", got.header.Get("HX-Trigger")))
		t.Must(be.Equal("act", got.header.Get("HX-Trigger-Name")))
		t.Must(be.Equal("task-1", got.header.Get("HX-Target")))
	})
	t.Run("get puts the parameters in the URL", func(t devtest.T) {
		c, rr := htmxClient()
		page := cthtml.PageString(webuiPage(`<div id="results"></div><form action="/search"><input id="q" name="q" value="go lang" hx-get="/search?scope=all" hx-target="#results"><input name="o" value="1"></form>`))
		t.Must(c.SubmitHTMX(t, page, page.Find(ct.ID("q")), nil, nil, webuitest.Header("HX-Target", "")).Status(http.StatusNoContent))
		got := rr.seen()
		t.Must(be.Equal("GET /search?scope=all&q=go+lang", got.method+" "+got.uri))
		t.Must(be.Equal("", got.body))
		t.Must(be.Equal("", got.header.Get("Content-Type")))
		t.Must(be.Equal("", got.header.Get("Origin")))
		// Header options apply after htmx's headers.
		t.Must(be.Equal("", got.header.Get("HX-Target")))
		t.Must(be.Equal("q", got.header.Get("HX-Trigger")))
	})
	t.Run("a form included after its control sends it once", func(t devtest.T) {
		page := cthtml.PageString(webuiPage(`<form><input id="q" name="q" value="v" hx-get="/s" hx-include="closest form"><input name="o" value="1"></form>`))
		r, err := webuitest.HTMXData(t.Context(), page, page.Find(ct.ID("q")), nil, nil)
		t.Must(be.NoError(err))
		t.Must(be.DeepEqual(url.Values{"q": {"v"}, "o": {"1"}}, r.Values()))
		t.Must(be.DeepEqual([][2]string{{"q", "v"}, {"o", "1"}}, formFields(r.Fields)))
	})
	t.Run("the fields of a form win over includes", func(t devtest.T) {
		page := cthtml.PageString(webuiPage(`<input id="extra" name="title" value="outside"><input id="more" name="more" value="m">` +
			`<form id="f" hx-post="/x" hx-include="#extra, #more"><input name="title" value="inside"></form>`))
		r, err := webuitest.HTMXData(t.Context(), page, page.Find(ct.ID("f")), nil, nil)
		t.Must(be.NoError(err))
		t.Must(be.DeepEqual([][2]string{{"more", "m"}, {"title", "inside"}}, formFields(r.Fields)))
	})
	t.Run("vals, includes and params", func(t devtest.T) {
		page := cthtml.PageString(webuiPage(`<input id="extra" name="extra" value="e"><div hx-vals='{"page": 2, "tags": ["a", "b"], "obj": {"x": 1}, "n": null, "extra": "v"}'>` +
			`<button type="button" id="go" hx-post="/items" hx-include="#extra" hx-vals='{"page": 3}' hx-params="not obj">Go</button>` +
			`<button type="button" id="only" hx-post="/items" hx-include="#extra" hx-params="page, extra">Only</button>` +
			`<button type="button" id="none" hx-post="/items" hx-include="#extra" hx-params="none">None</button>` +
			`<div hx-vals="unset"><button type="button" id="unset" hx-post="/items" hx-vals='{"a": "1"}'>Unset</button></div></div>`))
		data := func(id string) [][2]string {
			r, err := webuitest.HTMXData(t.Context(), page, page.Find(ct.ID(id)), nil, nil)
			t.Must(be.NoError(err))
			return formFields(r.Fields)
		}
		t.Must(be.DeepEqual([][2]string{{"page", "3"}, {"tags", "a"}, {"tags", "b"}, {"n", "null"}, {"extra", "v"}}, data("go")))
		t.Must(be.DeepEqual([][2]string{{"page", "2"}, {"extra", "v"}}, data("only")))
		t.Must(be.Len(data("none"), 0))
		t.Must(be.Len(data("unset"), 0))
	})
	t.Run("vals as JavaScript converts them", func(t devtest.T) {
		page := cthtml.PageString(webuiPage(`<button type="button" hx-post="/x" hx-vals='{"b": "1", "2": "x", "a": 1e21, "1": 0.0000001, "c": 1.50, "d": true, "e": -0, "f": [1, [2, 3], {"k": 1}, null], "g": 12345678901234567890}'>x</button>`))
		r, err := webuitest.HTMXData(t.Context(), page, page.Find(ct.Tag("button")), nil, nil)
		t.Must(be.NoError(err))
		t.Must(be.DeepEqual([][2]string{
			{"1", "1e-7"}, {"2", "x"}, {"b", "1"}, {"a", "1e+21"}, {"c", "1.5"}, {"d", "true"}, {"e", "0"},
			{"f", "1"}, {"f", "2,3"}, {"f", "[object Object]"}, {"f", "null"}, {"g", "12345678901234567000"},
		}, formFields(r.Fields)))
	})
	t.Run("vals that htmx ignores", func(t devtest.T) {
		page := cthtml.PageString(webuiPage(`<div hx-vals='"c": "3"'><div hx-vals='{"b": "2", "b": "4"}'><div hx-vals='js:{d: 1}'>` +
			`<button type="button" hx-post="/x" hx-vals='{"a": "1"} x'>x</button></div></div></div>`))
		r, err := webuitest.HTMXData(t.Context(), page, page.Find(ct.Tag("button")), nil, nil)
		t.Must(be.NoError(err))
		t.Must(be.DeepEqual([][2]string{{"b", "4"}, {"c", "3"}}, formFields(r.Fields)))
	})
	t.Run("controls as htmx reads them", func(t devtest.T) {
		page := cthtml.PageString(webuiPage(`<div id="box"><input type="checkbox" name="c1" checked><input type="checkbox" name="c2">` +
			`<input type="radio" name="r" value="1" checked><input type="radio" name="r" value="2" checked>` +
			`<select name="s" multiple><option selected disabled>x</option><option value="y" selected>Y</option><option value="z">Z</option></select>` +
			`<select name="one"><option disabled>a</option><option>b</option></select>` +
			`<input type="file" name="f"><input type="submit" name="sub" value="s"><input type="text" name="t" value="a&#10;b"><textarea name="ta">line</textarea>` +
			`<fieldset disabled><legend><input name="dis" value="d"></legend></fieldset><input name="off" value="o" disabled><template><input name="tpl" value="t"></template></div>` +
			`<button type="button" hx-get="/x" hx-include="#box">x</button>`))
		r, err := webuitest.HTMXData(t.Context(), page, page.Find(ct.Tag("button")), nil, nil)
		t.Must(be.NoError(err))
		t.Must(be.DeepEqual([][2]string{{"c1", "on"}, {"r", "2"}, {"s", "x"}, {"s", "y"}, {"one", "b"}, {"t", "ab"}, {"ta", "line"}}, formFields(r.Fields)))
	})
	t.Run("delete URL params", func(t devtest.T) {
		c, rr := htmxClient()
		body := `<form method="post" action="/notes/1/delete">` + csrf + `<button type="button" id="del" hx-delete="/notes/1">Delete</button></form>`
		cfg, err := hx2.Config{DeleteURLParams: true}.JSON()
		t.Must(be.NoError(err))
		page := cthtml.PageString(`<!DOCTYPE html><html lang="en"><head><title>t</title><meta name="htmx-config" content='` + cfg + `'></head><body>` + body + `</body></html>`)
		t.Must(c.SubmitHTMX(t, page, page.Find(ct.ID("del")), nil, nil).Status(http.StatusNoContent))
		got := rr.seen()
		t.Must(be.Equal("DELETE /notes/1?csrf="+token, got.method+" "+got.uri))
		t.Must(be.Equal("", got.body))
		// htmx sets the content type also without a body.
		t.Must(be.Equal("application/x-www-form-urlencoded", got.header.Get("Content-Type")))

		frag := cthtml.FragmentString("div", body)
		t.Must(c.SubmitHTMX(t, frag, frag.Find(ct.ID("del")), nil, nil).Status(http.StatusNoContent))
		t.Must(be.Equal("/notes/1", rr.seen().uri))
		t.Must(be.Equal("csrf="+token, rr.seen().body))
		t.Must(c.SubmitHTMX(t, frag, frag.Find(ct.ID("del")), nil, nil, webuitest.PageHTMXConfig(hx2.Config{DeleteURLParams: true})).Status(http.StatusNoContent))
		t.Must(be.Equal("/notes/1?csrf="+token, rr.seen().uri))
		// A page without the meta element runs htmx's defaults.
		bare := cthtml.PageString(htmlPage(body))
		r, err := webuitest.HTMXData(t.Context(), bare, bare.Find(ct.ID("del")), nil, nil)
		t.Must(be.NoError(err))
		t.Must(be.True(r.URLParams))
		t.Must(be.Panic(func() {
			webuitest.PageHTMXConfig(hx2.Config{HistoryCacheSize: -1})
		}))
	})
	t.Run("multipart", func(t devtest.T) {
		c, rr := htmxClient()
		page := cthtml.PageString(webuiPage(`<form id="m" hx-post="/upload" hx-encoding="multipart/form-data"><textarea name="t">a` + "\n" + `b</textarea><input type="file" name="f"></form>` +
			`<form id="u" hx-post="/upload"><textarea name="t">a` + "\n" + `b</textarea></form>`))
		t.Must(c.SubmitHTMX(t, page, page.Find(ct.ID("m")), nil, nil).Status(http.StatusNoContent))
		got := rr.seen()
		mt, params, err := mime.ParseMediaType(got.header.Get("Content-Type"))
		t.Must(be.NoError(err))
		t.Must(be.Equal("multipart/form-data", mt))
		form, err := multipart.NewReader(strings.NewReader(got.body), params["boundary"]).ReadForm(1 << 20)
		t.Must(be.NoError(err))
		t.Must(be.DeepEqual(map[string][]string{"t": {"a\r\nb"}}, form.Value))
		t.Must(be.Equal(0, len(form.File)))
		t.Must(c.SubmitHTMX(t, page, page.Find(ct.ID("u")), nil, nil).Status(http.StatusNoContent))
		t.Must(be.Equal("t=a%0Ab", rr.seen().body))
	})
	t.Run("submitter", func(t devtest.T) {
		c, rr := htmxClient()
		page := cthtml.PageString(webuiPage(`<form id="f" method="post" action="/a" hx-post="/a"><button id="put" name="b" formaction="/b" formmethod="PUT">Put</button>` +
			`<button id="dialog" formmethod="dialog">Close</button><button id="plain" type="button">x</button><input id="img" type="image" alt="Go"></form><div id="d" hx-post="/x"></div>`))
		t.Must(c.SubmitHTMX(t, page, page.Find(ct.ID("f")), page.Find(ct.ID("put")), nil).Status(http.StatusNoContent))
		got := rr.seen()
		t.Must(be.Equal("PUT /b", got.method+" "+got.uri))
		t.Must(be.Equal("b=", got.body))
		cases := map[string][2]*ct.Selection{
			`has formmethod "dialog", for which htmx sends no request`: {page.Find(ct.ID("f")), page.Find(ct.ID("dialog"))},
			`is not a submit button or submit input`:                   {page.Find(ct.ID("f")), page.Find(ct.ID("plain"))},
			`id("img") in html page) is not a submit button`:           {page.Find(ct.ID("f")), page.Find(ct.ID("img"))},
			`is a div; only a form element has a submitter`:            {page.Find(ct.ID("d")), page.Find(ct.ID("put"))},
		}
		for want, sel := range cases {
			_, err := webuitest.HTMXData(t.Context(), page, sel[0], sel[1], nil)
			t.Must(be.ErrorIs(err, webuitest.ErrFormData))
			t.Must(be.ErrorContains(err, want))
		}
	})
	t.Run("targets", func(t devtest.T) {
		c, rr := htmxClient()
		frag := cthtml.FragmentString("tbody", `<tr id="row-1"><td><button type="button" id="b1" hx-delete="/rows/1" hx-target="closest table">x</button>`+
			`<button type="button" id="b2" hx-delete="/rows/1" hx-target="closest tr">y</button><button type="button" id="b3" hx-get="/rows" hx-target="#main">z</button></td></tr>`)
		r := newRecordT(t.Context())
		t.Must(be.True(c.SubmitHTMX(r, frag, frag.Find(ct.ID("b1")), nil, nil) == nil))
		t.Must(be.Substring(r.output(), `webuitest: cannot compute form data: the hx-target "closest table" cannot be resolved within the subject; pass HXTarget with the id of the target`))
		t.Must(c.SubmitHTMX(t, frag, frag.Find(ct.ID("b1")), nil, nil, webuitest.HXTarget("rows")).Status(http.StatusNoContent))
		t.Must(be.Equal("rows", rr.seen().header.Get("HX-Target")))
		t.Must(c.SubmitHTMX(t, frag, frag.Find(ct.ID("b2")), nil, nil).Status(http.StatusNoContent))
		t.Must(be.Equal("row-1", rr.seen().header.Get("HX-Target")))
		t.Must(c.SubmitHTMX(t, frag, frag.Find(ct.ID("b3")), nil, nil).Status(http.StatusNoContent))
		t.Must(be.Equal("main", rr.seen().header.Get("HX-Target")))

		page := cthtml.PageString(webuiPage(`<button type="button" id="gone" hx-get="/x" hx-target="#nope">x</button><div id="out"><button type="button" id="self" hx-get="/x" hx-target="this">x</button></div>` +
			`<button type="button" id="next" hx-get="/x" hx-target="next .r, #out">x</button><p class="r" id="r1"></p><section hx-target="this" id="sec"><button type="button" id="inh" hx-get="/x">x</button>` +
			`<div hx-disinherit="hx-target"><button type="button" id="dis" hx-get="/x">x</button></div><button type="button" id="unset" hx-get="/x" hx-target="unset">x</button></section>`))
		_, err := webuitest.HTMXData(t.Context(), page, page.Find(ct.ID("gone")), nil, nil)
		t.Must(be.ErrorContains(err, `the hx-target "#nope" selects no element, so htmx sends no request`))
		target := func(id string) string {
			r, err := webuitest.HTMXData(t.Context(), page, page.Find(ct.ID(id)), nil, nil)
			t.Must(be.NoError(err))
			return r.Target
		}
		t.Must(be.Equal("self", target("self")))
		t.Must(be.Equal("r1", target("next")))
		t.Must(be.Equal("sec", target("inh")))
		t.Must(be.Equal("dis", target("dis")))
		t.Must(be.Equal("unset", target("unset")))
	})
	t.Run("headers", func(t devtest.T) {
		c, rr := htmxClient()
		page := cthtml.PageString(webuiPage(`<div hx-headers='{"X-Mode": "fast", "X-Keep": "k", "Origin": "https://other.example", "HX-Request": "false", "Sec-Fetch-Site": "cross-site", "X-Num": 5, "X-Text": "Grüße €"}'>` +
			`<button type="button" id="b" hx-post="/x" hx-headers='{"X-Mode": "slow", "HX-Trigger": "spoofed"}'>x</button>` +
			`<div hx-headers="unset"><button type="button" id="u" name="u" hx-post="/x" hx-headers='{"X-B": "2", "HX-Trigger-Name": "n"}'>x</button></div></div>`))
		t.Must(c.SubmitHTMX(t, page, page.Find(ct.ID("b")), nil, nil).Status(http.StatusNoContent))
		h := rr.seen().header
		t.Must(be.Equal("slow", h.Get("X-Mode")))
		t.Must(be.Equal("k", h.Get("X-Keep")))
		t.Must(be.Equal(webuitest.BaseURL, h.Get("Origin")))
		t.Must(be.Equal("true", h.Get("HX-Request")))
		t.Must(be.Equal("same-origin", h.Get("Sec-Fetch-Site")))
		t.Must(be.Equal("b", h.Get("HX-Trigger")))
		t.Must(be.Equal("5", h.Get("X-Num")))
		t.Must(be.Equal("Gr%C3%BC%C3%9Fe%20%E2%82%AC", h.Get("X-Text")))
		t.Must(be.Equal("true", h.Get("X-Text-URI-AutoEncoded")))
		t.Must(c.SubmitHTMX(t, page, page.Find(ct.ID("u")), nil, nil).Status(http.StatusNoContent))
		h = rr.seen().header
		t.Must(be.Equal("2", h.Get("X-B")))
		t.Must(be.Equal("", h.Get("X-Mode")))
		t.Must(be.DeepEqual([]string{"u"}, h.Values("HX-Trigger-Name")))
		t.Must(be.Equal("u", h.Get("HX-Trigger")))
	})
	t.Run("page URL", func(t devtest.T) {
		c, rr := htmxClient()
		page := cthtml.PageString(webuiPage(`<button type="button" id="rel" hx-post="done">x</button><input id="here" name="q" value="v" hx-get="">`))
		r := newRecordT(t.Context())
		t.Must(be.True(c.SubmitHTMX(r, page, page.Find(ct.ID("rel")), nil, nil) == nil))
		t.Must(be.Substring(r.output(), "the request URL is relative to the page's URL; pass PageURL with it"))
		r = newRecordT(t.Context())
		t.Must(be.True(c.SubmitHTMX(r, page, page.Find(ct.ID("here")), nil, nil) == nil))
		t.Must(be.Substring(r.output(), "the request URL is empty, which is the page's URL; pass PageURL with it"))
		t.Must(c.SubmitHTMX(t, page, page.Find(ct.ID("rel")), nil, nil, webuitest.PageURL("/notes/")).Status(http.StatusNoContent))
		got := rr.seen()
		t.Must(be.Equal("POST /notes/done", got.method+" "+got.uri))
		t.Must(be.Equal(webuitest.BaseURL+"/notes/", got.header.Get("HX-Current-URL")))
		t.Must(c.SubmitHTMX(t, page, page.Find(ct.ID("here")), nil, nil, webuitest.PageURL("/notes?page=2#top")).Status(http.StatusNoContent))
		got = rr.seen()
		t.Must(be.Equal("GET /notes?page=2&q=v", got.method+" "+got.uri))
		t.Must(be.Equal(webuitest.BaseURL+"/notes?page=2#top", got.header.Get("HX-Current-URL")))
		t.Must(c.SubmitHTMX(t, page, page.Find(ct.ID("rel")), nil, nil, webuitest.PageURL("/notes/"), webuitest.HXCurrentURL("/elsewhere")).Status(http.StatusNoContent))
		t.Must(be.Equal(webuitest.BaseURL+"/elsewhere", rr.seen().header.Get("HX-Current-URL")))
	})
	t.Run("elements htmx does not use", func(t devtest.T) {
		page := cthtml.PageString(webuiPage(`<button type="button" id="off" hx-post="/x" disabled>x</button><div hx-disable><button type="button" id="dis" hx-post="/x">x</button></div>` +
			`<template><button type="button" id="tpl" hx-post="/x">x</button></template><button type="button" id="none">x</button><button type="button" id="two" hx-get="/x" hx-post="/x">x</button>` +
			`<button type="button" id="inc" hx-post="/x" hx-include="div > input">x</button><button type="button" id="inh" hx-post="/x" hx-include="inherit">x</button><input id="edit" name="q" hx-get="/x">`))
		cases := map[string]string{
			"off":  `id("off") in html page) is disabled`,
			"dis":  `id("dis") in html page) is inside hx-disable, which htmx does not process`,
			"tpl":  `id("tpl") in html page) is template content, which htmx does not process`,
			"none": `id("none") in html page) has no hx-get, hx-post, hx-put, hx-patch or hx-delete`,
			"two":  `id("two") in html page) has several htmx request attributes`,
			"inc":  `cannot resolve the hx-include "div > input" within the subject`,
			"inh":  `the hx-include "inherit" inherits, which is not supported`,
		}
		for id, want := range cases {
			_, err := webuitest.HTMXData(t.Context(), page, page.Find(ct.ID(id)), nil, nil)
			t.Must(be.ErrorIs(err, webuitest.ErrFormData))
			t.Must(be.ErrorContains(err, want))
		}
		_, err := webuitest.HTMXData(t.Context(), page, page.Find(ct.ID("edit")), nil, url.Values{"other": {"x"}})
		t.Must(be.ErrorContains(err, `includes no control "other" to edit`))
		r, err := webuitest.HTMXData(t.Context(), page, page.Find(ct.ID("edit")), nil, url.Values{"q": {"a", "b"}})
		t.Must(be.NoError(err))
		t.Must(be.DeepEqual(url.Values{"q": {"a", "b"}}, r.Values()))
	})
	t.Run("request", func(t devtest.T) {
		page := cthtml.PageString(webuiPage(`<form hx-put="/settings"><input name="theme" value="dark"></form>`))
		r, err := webuitest.HTMXData(t.Context(), page, page.Find(ct.Tag("form")), nil, nil)
		t.Must(be.NoError(err))
		req := r.Request(t, webuitest.HXCurrentURL("/settings"))
		t.Must(be.Equal("PUT /settings", req.Method+" "+req.RequestURI))
		t.Must(be.Equal(webuitest.BaseURL+"/settings", req.Header.Get("HX-Current-URL")))
		b, err := io.ReadAll(req.Body)
		t.Must(be.NoError(err))
		t.Must(be.Equal("theme=dark", string(b)))
		bad := r
		bad.Method = "TRACE"
		rec := newRecordT(t.Context())
		t.Must(be.True(bad.Request(rec) == nil))
		t.Must(be.Substring(rec.output(), `htmx sends no "TRACE" requests`))
	})
}

func TestBrowserSubmitHTMX(gt *testing.T) {
	t := devtest.ParallelT(gt)
	rr := &requestRecorder{}
	b := webuitest.NewBrowser(t, rr, webuitest.BrowserOptions{})
	b.Options = []webuitest.ReqOption{webuitest.Header("X-Client", "1")}
	page := cthtml.PageString(webuiPage(`<form method="post" action="/notes/1/delete"><input type="hidden" name="csrf" value="tok"><button type="button" id="del" hx-delete="/notes/1">Delete</button></form>`))
	t.Must(b.SubmitHTMX(t, page, page.Find(ct.ID("del")), nil, nil, webuitest.PageURL("/notes")).Status(http.StatusNoContent))
	got := rr.seen()
	t.Must(be.Equal("DELETE /notes/1", got.method+" "+got.uri))
	t.Must(be.Equal("csrf=tok", got.body))
	t.Must(be.Equal("1", got.header.Get("X-Client")))
	t.Must(be.Equal("del", got.header.Get("HX-Trigger")))
	t.Must(be.Equal(webuitest.BaseURL+"/notes", got.header.Get("HX-Current-URL")))
	form := cthtml.PageString(webuiPage(`<form method="post" action="/notes/1/delete" hx-delete="/notes/1"><button>Delete</button></form>`))
	r := newRecordT(t.Context())
	t.Must(be.True(b.Submit(r, form, form.Find(ct.Tag("form")), nil, nil, webuitest.HTMX()) == nil))
	t.Must(be.Substring(r.output(), "send it with SubmitHTMX"))
	t.Must(b.SubmitHTMX(t, form, form.Find(ct.Tag("form")), form.Find(ct.Tag("button")), nil).Status(http.StatusNoContent))
	t.Must(be.Equal("DELETE /notes/1", rr.seen().method+" "+rr.seen().uri))
}

func TestHTMXSubmitterWithOwnRequest(gt *testing.T) {
	t := devtest.ParallelT(gt)
	// A note form with a Save button and buttons of their own requests:
	// htmx sends the request of a clicked button that has one, not the
	// form's, unless htmx does not handle its click.
	page := cthtml.PageString(webuiPage(`<div id="out"></div><form id="f" method="post" action="/notes/1" hx-post="/notes/1"><input name="title" value="a">` +
		`<button id="save" name="op" value="save">Save</button>` +
		`<button id="del" name="op" value="del" hx-delete="/notes/1" hx-target="#out">Delete</button>` +
		`<button id="search" name="q" value="x" hx-get="/search">Search</button>` +
		`<button id="ctrl" name="op" value="ctrl" hx-post="/copy" hx-trigger="mouseenter, click[ctrlKey]">Copy</button>` +
		`<button id="hover" name="op" value="hover" hx-post="/preview" hx-trigger="mouseenter">Preview</button>` +
		`<div hx-disable><button id="off" name="op" value="off" hx-post="/off">Off</button></div></form>`))
	form := page.Find(ct.ID("f"))
	for id, attr := range map[string]string{"del": "hx-delete", "search": "hx-get", "ctrl": "hx-post"} {
		_, err := webuitest.HTMXData(t.Context(), page, form, page.Find(ct.ID(id)), nil)
		t.Must(be.ErrorIs(err, webuitest.ErrFormData))
		t.Must(be.ErrorContains(err, `has its own `+attr+`, which htmx sends instead of the form's request when it is clicked; pass it as elt with a nil submitter`))
	}
	data := func(elt, submitter *ct.Selection) webuitest.HTMXRequest {
		r, err := webuitest.HTMXData(t.Context(), page, elt, submitter, nil)
		t.Must(be.NoError(err))
		return r
	}
	r := data(page.Find(ct.ID("del")), nil)
	t.Must(be.Equal("DELETE /notes/1", r.Method+" "+r.Path))
	t.Must(be.DeepEqual([][2]string{{"title", "a"}, {"op", "del"}}, formFields(r.Fields)))
	t.Must(be.Equal("del", r.Trigger))
	t.Must(be.Equal("op", r.TriggerName))
	t.Must(be.Equal("out", r.Target))
	r = data(page.Find(ct.ID("search")), nil)
	t.Must(be.Equal("GET /search", r.Method+" "+r.Path))
	t.Must(be.True(r.URLParams))
	t.Must(be.DeepEqual([][2]string{{"q", "x"}}, formFields(r.Fields)))
	// htmx does not handle these clicks, so they submit the form.
	for _, id := range []string{"hover", "off"} {
		r = data(form, page.Find(ct.ID(id)))
		t.Must(be.Equal("POST /notes/1", r.Method+" "+r.Path))
		t.Must(be.DeepEqual([][2]string{{"title", "a"}, {"op", id}}, formFields(r.Fields)))
		t.Must(be.Equal("f", r.Trigger))
	}

	// Submit with HTMX names the call that sends the button's request.
	c, rr := htmxClient()
	rec := newRecordT(t.Context())
	t.Must(be.True(c.Submit(rec, page, form, page.Find(ct.ID("del")), nil, webuitest.HTMX()) == nil))
	t.Must(be.Substring(rec.output(), `htmx sends DELETE "/notes/1" for the hx-delete of the submitter, not the form's POST "/notes/1"; send it with SubmitHTMX, with the submitter as the element and a nil submitter`))
	t.Must(c.SubmitHTMX(t, page, page.Find(ct.ID("del")), nil, nil).Status(http.StatusNoContent))
	got := rr.seen()
	t.Must(be.Equal("DELETE /notes/1", got.method+" "+got.uri))
	t.Must(be.Equal("title=a&op=del", got.body))
	t.Must(be.Equal("del", got.header.Get("HX-Trigger")))
	t.Must(be.Equal("out", got.header.Get("HX-Target")))
	rec = newRecordT(t.Context())
	t.Must(be.True(c.SubmitHTMX(rec, page, form, page.Find(ct.ID("del")), nil) == nil))
	t.Must(be.Substring(rec.output(), "pass it as elt with a nil submitter"))
	// A button whose click htmx does not handle submits the form.
	t.Must(c.Submit(t, page, form, page.Find(ct.ID("hover")), nil, webuitest.HTMX()).Status(http.StatusNoContent))
	t.Must(be.Equal("POST /notes/1", rr.seen().method+" "+rr.seen().uri))
	t.Must(be.Equal("title=a&op=hover", rr.seen().body))
}

func TestHTMXUntrackedSubmitter(gt *testing.T) {
	t := devtest.ParallelT(gt)
	// htmx tracks a submit button outside of form elements only when its
	// type attribute is exactly "submit" and it has a form attribute.
	page := cthtml.PageString(webuiPage(`<form id="f" method="post" action="/x" hx-post="/x"><input name="a" value="1">` +
		`<button id="inside" name="op" value="v" formaction="/y">In</button></form>` +
		`<form id="g"><button id="other" form="f" name="op" value="v" formaction="/y">Other</button></form>` +
		`<button id="none" form="f" name="op" value="v" formaction="/y">None</button>` +
		`<button id="upper" type="SUBMIT" form="f" name="op" value="v" formaction="/y">Upper</button>` +
		`<input id="input-upper" type="SUBMIT" form="f" name="op" value="v" formaction="/y">` +
		`<button id="invalid" type="foo" form="f" name="op" value="v" formaction="/y">Invalid</button>` +
		`<button id="method" form="f" formmethod="put">Method</button>` +
		`<div hx-disable><button id="disabled" type="submit" form="f" name="op" value="v">Off</button></div>` +
		`<button id="plain" form="f">Plain</button>` +
		`<button id="exact" type="submit" form="f" name="op" value="v" formaction="/y">Exact</button>` +
		`<input id="input" type="submit" form="f" name="op" value="v" formaction="/y">`))
	form := page.Find(ct.ID("f"))
	for _, id := range []string{"none", "upper", "input-upper", "invalid", "method", "disabled"} {
		_, err := webuitest.HTMXData(t.Context(), page, form, page.Find(ct.ID(id)), nil)
		t.Must(be.ErrorIs(err, webuitest.ErrFormData))
		t.Mustf(be.ErrorContains(err, `id("`+id+`") in html page) as the submitter of the form, so it would send the form's request without its name, value, formaction and formmethod`), "%s", id)
	}
	for _, id := range []string{"inside", "other", "exact", "input"} {
		r, err := webuitest.HTMXData(t.Context(), page, form, page.Find(ct.ID(id)), nil)
		t.Must(be.NoError(err))
		t.Must(be.Equal("POST /y", r.Method+" "+r.Path))
		t.Must(be.DeepEqual([][2]string{{"a", "1"}, {"op", "v"}}, formFields(r.Fields)))
	}
	// A submitter that would add nothing is the form's request.
	r, err := webuitest.HTMXData(t.Context(), page, form, page.Find(ct.ID("plain")), nil)
	t.Must(be.NoError(err))
	t.Must(be.Equal("POST /x", r.Method+" "+r.Path))
	t.Must(be.DeepEqual([][2]string{{"a", "1"}}, formFields(r.Fields)))

	// Submit with HTMX: the browser's submission goes to the formaction,
	// htmx's to the form's URL.
	c, _ := htmxClient()
	rec := newRecordT(t.Context())
	t.Must(be.True(c.Submit(rec, page, form, page.Find(ct.ID("none")), nil, webuitest.HTMX()) == nil))
	t.Must(be.Substring(rec.output(), `htmx sends POST "/x" for the hx-post of the form, not the form's POST "/y"; send it with SubmitHTMX`))
	t.Must(c.Submit(t, page, form, page.Find(ct.ID("exact")), nil, webuitest.HTMX()).Status(http.StatusNoContent))
}

func TestHTMXHeadersAsBrowsersSend(gt *testing.T) {
	t := devtest.ParallelT(gt)
	c, rr := htmxClient()
	page := cthtml.PageString(webuiPage(`<button type="button" id="b" hx-post="/x" hx-headers='{"X-HTTP-Method-Override": "PUT", "X-Method-Override": " get, Track ", ` +
		`"X-HTTP-Method": "TRACE\nx", "X-Nl": "v\n", "X-Sp": " sp ", "X-Latin": "café", "X-Enc": " a\nb ", "Cookie": "a\nb", "Sec-Thing": "1"}'>x</button>` +
		`<button type="button" id="q" hx-post="/x" hx-headers='{"X-HTTP-Method-Override": "\"x, TRACE\"", "X-Method-Override": "get,\tconnect"}'>x</button>` +
		`<button type="button" id="bad" hx-post="/x" hx-headers='{"X Bad": "1"}'>x</button>` +
		`<button type="button" id="null" hx-post="/x" hx-headers='{"X Bad": null, "X-Ok": "1"}'>x</button>`))
	t.Must(c.SubmitHTMX(t, page, page.Find(ct.ID("b")), nil, nil).Status(http.StatusNoContent))
	h := rr.seen().header
	t.Must(be.Equal("PUT", h.Get("X-HTTP-Method-Override")))
	t.Must(be.DeepEqual([]string(nil), h.Values("X-Method-Override")))
	t.Must(be.Equal("TRACE%0Ax", h.Get("X-HTTP-Method")))
	t.Must(be.Equal("true", h.Get("X-HTTP-Method-URI-AutoEncoded")))
	t.Must(be.Equal("v", h.Get("X-Nl")))
	t.Must(be.Equal("", h.Get("X-Nl-URI-AutoEncoded")))
	t.Must(be.Equal("sp", h.Get("X-Sp")))
	t.Must(be.Equal("caf\xe9", h.Get("X-Latin")))
	// htmx encodes the value as written, white space included.
	t.Must(be.Equal("%20a%0Ab%20", h.Get("X-Enc")))
	t.Must(be.Equal("true", h.Get("X-Enc-URI-AutoEncoded")))
	// The browser drops the forbidden Cookie, not the marker of its
	// encoding.
	t.Must(be.Equal("", h.Get("Cookie")))
	t.Must(be.Equal("true", h.Get("Cookie-URI-AutoEncoded")))
	t.Must(be.Equal("", h.Get("Sec-Thing")))
	// The bytes travel over HTTP as they are.
	br := webuitest.NewBrowser(t, rr, webuitest.BrowserOptions{})
	t.Must(br.SubmitHTMX(t, page, page.Find(ct.ID("b")), nil, nil).Status(http.StatusNoContent))
	t.Must(be.Equal("caf\xe9", rr.seen().header.Get("X-Latin")))
	t.Must(be.Equal("sp", rr.seen().header.Get("X-Sp")))

	r, err := webuitest.HTMXData(t.Context(), page, page.Find(ct.ID("q")), nil, nil)
	t.Must(be.NoError(err))
	// A quoted string is one value; a tab separates like a space.
	t.Must(be.DeepEqual(http.Header{"X-Http-Method-Override": {`"x, TRACE"`}}, r.Header))

	_, err = webuitest.HTMXData(t.Context(), page, page.Find(ct.ID("bad")), nil, nil)
	t.Must(be.ErrorIs(err, webuitest.ErrFormData))
	t.Must(be.ErrorContains(err, `the hx-headers name "X Bad" is not a valid header name, so htmx sends no request`))
	rec := newRecordT(t.Context())
	t.Must(be.True(c.SubmitHTMX(rec, page, page.Find(ct.ID("bad")), nil, nil) == nil))
	t.Must(be.Substring(rec.output(), "so htmx sends no request"))
	// htmx sets no header for null, so the name does not matter.
	r, err = webuitest.HTMXData(t.Context(), page, page.Find(ct.ID("null")), nil, nil)
	t.Must(be.NoError(err))
	t.Must(be.DeepEqual(http.Header{"X-Ok": {"1"}}, r.Header))
}

func TestHTMXCrossOrigin(gt *testing.T) {
	t := devtest.ParallelT(gt)
	body := `<button type="button" id="other" hx-post="https://other.example/x">x</button>` +
		`<button type="button" id="relative" hx-post="//other.example/x">x</button>` +
		`<button type="button" id="scheme" hx-post="http://app.example.test/x">x</button>` +
		`<button type="button" id="port" hx-post="https://app.example.test:8443/x">x</button>` +
		`<button type="button" id="script" hx-post="javascript:void(0)">x</button>` +
		`<button type="button" id="same" hx-post="https://app.example.test/x">x</button>` +
		`<button type="button" id="default-port" hx-post="https://APP.example.test:443/x">x</button>` +
		`<button type="button" id="same-relative" hx-post="//app.example.test/x">x</button>` +
		`<button type="button" id="local" hx-post="//localhost/x">x</button>`
	page := cthtml.PageString(webuiPage(body))
	frag := cthtml.FragmentString("div", body)
	c, rr := htmxClient()
	for _, subject := range []*ct.Subject{page, frag} {
		for _, id := range []string{"other", "relative", "scheme", "port", "script", "local"} {
			rec := newRecordT(t.Context())
			t.Must(be.True(c.SubmitHTMX(rec, subject, subject.Find(ct.ID(id)), nil, nil) == nil))
			t.Mustf(be.Substring(rec.output(), `is not of the page's origin https://app.example.test: htmx refuses it (selfRequestsOnly), and otherwise sends it to another server`), "%s", id)
		}
		for _, id := range []string{"same", "default-port", "same-relative"} {
			t.Must(c.SubmitHTMX(t, subject, subject.Find(ct.ID(id)), nil, nil).Status(http.StatusNoContent))
			t.Must(be.Equal("POST /x", rr.seen().method+" "+rr.seen().uri))
		}
	}
	r, err := webuitest.HTMXData(t.Context(), page, page.Find(ct.ID("other")), nil, nil)
	t.Must(be.NoError(err))
	rec := newRecordT(t.Context())
	t.Must(be.True(r.Request(rec) == nil))
	t.Must(be.Substring(rec.output(), `the request URL "https://other.example/x" is not of the page's origin`))
	// The page's URL names the origin.
	r, err = webuitest.HTMXData(t.Context(), page, page.Find(ct.ID("scheme")), nil, nil)
	t.Must(be.NoError(err))
	req := r.Request(t, webuitest.PageURL("http://app.example.test/notes"))
	t.Must(be.Equal("POST /x", req.Method+" "+req.RequestURI))
	t.Must(be.Equal("http://app.example.test/notes", req.Header.Get("HX-Current-URL")))

	// A browser of plain http has its own origin.
	b := webuitest.NewBrowser(t, rr, webuitest.BrowserOptions{Insecure: true})
	t.Must(b.SubmitHTMX(t, page, page.Find(ct.ID("local")), nil, nil).Status(http.StatusNoContent))
	t.Must(be.Equal("POST /x", rr.seen().method+" "+rr.seen().uri))
	rec = newRecordT(t.Context())
	t.Must(be.True(b.SubmitHTMX(rec, page, page.Find(ct.ID("same")), nil, nil) == nil))
	t.Must(be.Substring(rec.output(), `is not of the page's origin http://localhost`))
}

func TestHTMXMethodCase(gt *testing.T) {
	t := devtest.ParallelT(gt)
	c, rr := htmxClient()
	// htmx keeps a formmethod as written and compares it case-sensitively.
	body := `<form id="f" hx-post="/a" hx-headers='{"Content-Type": "text/plain"}'><input name="q" value="1">` +
		`<button id="GET" formmethod="GET">x</button><button id="get" formmethod="get">x</button>` +
		`<button id="DELETE" formmethod="DELETE">x</button><button id="delete" formmethod="delete">x</button></form>` +
		`<button type="button" id="hx-get" name="b" value="2" hx-get="/s">x</button>`
	doc := func(config string) *ct.Subject {
		return cthtml.PageString(`<!DOCTYPE html><html lang="en"><head><title>t</title><meta name="htmx-config" content='` + config + `'></head><body>` + body + `</body></html>`)
	}
	page := doc(`{"methodsThatUseUrlParams":["get","delete"]}`)
	send := func(p *ct.Subject, elt, submitter string) seenRequest {
		var sub *ct.Selection
		if submitter != "" {
			sub = p.Find(ct.ID(submitter))
		}
		t.Must(c.SubmitHTMX(t, p, p.Find(ct.ID(elt)), sub, nil).Status(http.StatusNoContent))
		return rr.seen()
	}
	// The fields go in a body, which a GET request drops, with htmx's
	// urlencoded Content-Type over the one of hx-headers.
	got := send(page, "f", "GET")
	t.Must(be.Equal("GET /a", got.method+" "+got.uri))
	t.Must(be.Equal("", got.body))
	t.Must(be.Equal("application/x-www-form-urlencoded", got.header.Get("Content-Type")))
	r, err := webuitest.HTMXData(t.Context(), page, page.Find(ct.ID("f")), page.Find(ct.ID("GET")), nil)
	t.Must(be.NoError(err))
	t.Must(be.Equal("GET", r.Method))
	t.Must(be.False(r.URLParams))
	t.Must(be.DeepEqual(url.Values{"q": {"1"}}, r.Values()))
	got = send(page, "f", "get")
	t.Must(be.Equal("GET /a?q=1", got.method+" "+got.uri))
	t.Must(be.Equal("text/plain", got.header.Get("Content-Type")))
	got = send(page, "f", "DELETE")
	t.Must(be.Equal("DELETE /a", got.method+" "+got.uri))
	t.Must(be.Equal("q=1", got.body))
	got = send(page, "f", "delete")
	t.Must(be.Equal("DELETE /a?q=1", got.method+" "+got.uri))
	t.Must(be.Equal("", got.body))

	// A configuration names methods case-sensitively too.
	upper := doc(`{"methodsThatUseUrlParams":["GET"]}`)
	got = send(upper, "hx-get", "")
	t.Must(be.Equal("GET /s", got.method+" "+got.uri))
	t.Must(be.Equal("", got.header.Get("Content-Type")))
	got = send(upper, "f", "GET")
	t.Must(be.Equal("GET /a?q=1", got.method+" "+got.uri))
}

func TestHTMXValsObjects(gt *testing.T) {
	t := devtest.ParallelT(gt)
	// htmx sends an object member as JSON.stringify(JSON.parse(...)).
	page := cthtml.PageString(webuiPage(`<button type="button" hx-post="/x" hx-vals='{"o": {"10": 1, "2": 2, "x": {"1": 0}}, "d": {"a": 1, "b": 2, "a": 3}, ` +
		`"n": {"id": 9007199254740993, "h": 1e400, "z": -0, "f": 1.50, "e": 1E2, "s": 1e-7}, "s": {"q": "a&amp;b\/\u0001\n é\ud800😀\uDBFF"}, ` +
		`"l": {"l": [1, {"b": 1, "0": 2}, [], {}]}, "nil": null}'>x</button>`))
	r, err := webuitest.HTMXData(t.Context(), page, page.Find(ct.Tag("button")), nil, nil)
	t.Must(be.NoError(err))
	t.Must(be.DeepEqual([][2]string{
		{"o", `{"2":2,"10":1,"x":{"1":0}}`},
		{"d", `{"a":3,"b":2}`},
		{"n", `{"id":9007199254740992,"h":null,"z":0,"f":1.5,"e":100,"s":1e-7}`},
		{"s", `{"q":"a&b/\u0001\n` + " é" + `\ud800😀\udbff"}`},
		{"l", `{"l":[1,{"0":2,"b":1},[],{}]}`},
		{"nil", "null"},
	}, formFields(r.Fields)))

	// hx2.ValsJSON writes Go's JSON: sorted keys and escaped HTML
	// characters, which the browser sends as JavaScript writes them.
	view := ct.View(button.Button(button.Type(button.TypeButton), hx2.Post("/x"),
		hx2.ValsJSON(map[string]any{"m": map[int]string{10: "x", 2: "a&b<c>"}}))(text.Text("x")))
	r, err = webuitest.HTMXData(t.Context(), view, view.Find(ct.Tag("button")), nil, nil)
	t.Must(be.NoError(err))
	t.Must(be.DeepEqual([][2]string{{"m", `{"2":"a&b<c>","10":"x"}`}}, formFields(r.Fields)))
}
