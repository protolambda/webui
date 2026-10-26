package webuitest_test

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthtml"
	"github.com/protolambda/chord/html/form/button"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/text"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/bi"
	"github.com/protolambda/webui/bs"
	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/webuitest"
)

// htmlPage wraps body in a minimal document.
func htmlPage(body string) string {
	return `<!DOCTYPE html><html lang="en"><head><title>t</title></head><body>` + body + `</body></html>`
}

// sri returns a valid integrity value of data.
func sri(data string) string {
	sum := sha512.Sum384([]byte(data))
	return "sha384-" + base64.StdEncoding.EncodeToString(sum[:])
}

func sri256(data string) string {
	sum := sha256.Sum256([]byte(data))
	return "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
}

// violations returns what rule reports for subject, which must load.
func violations(t devtest.T, subject *ct.Subject, rule ct.Rule) []string {
	t.Helper()
	doc, err := subject.Load(t.Context())
	t.Must(be.NoError(err))
	return rule.Violations(doc.Root())
}

// ruleCase is one mutation of a page: the body of a document (or a
// fragment), and the substrings of the violations it must cause, in
// order. No want means the page is valid.
type ruleCase struct {
	name     string
	body     string
	fragment bool
	want     []string
	// some only requires that one violation contains each want, for
	// mutations that break more than one thing.
	some bool
}

func checkRule(gt *testing.T, rule ct.Rule, cases []ruleCase) {
	t := devtest.ParallelT(gt)
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			subject := cthtml.PageString(htmlPage(c.body))
			if c.fragment {
				subject = cthtml.FragmentString("div", c.body)
			}
			got := violations(t, subject, rule)
			if c.some {
				t.Must(be.NonEmpty(got))
				for _, w := range c.want {
					found := false
					for _, g := range got {
						if strings.Contains(g, w) {
							found = true
						}
					}
					t.Mustf(be.True(found), "want a violation with %q, got %q", w, got)
				}
				return
			}
			t.Mustf(be.Len(got, len(c.want)), "violations %q", got)
			for i, w := range c.want {
				t.Must(be.Substring(got[i], w))
			}
		})
	}
}

func TestNoInlineScripts(gt *testing.T) {
	checkRule(gt, webuitest.NoInlineScripts(), []ruleCase{
		{name: "valid", body: `<script src="/a.js"></script><script type="application/json">{"a":1}</script><script type="application/ld+json; charset=utf-8">{}</script>`},
		{name: "inline", body: `<script>alert(1)</script>`, want: []string{"inline script at html[1]/body[1]/script[0]"}},
		{name: "module", body: `<script type="module">import "/x.js"</script>`, want: []string{"inline script at"}},
		{name: "empty inline", body: `<script></script>`, want: []string{"inline script at"}},
		{name: "srcdoc", body: `<iframe srcdoc="<p>x</p>"></iframe>`, want: []string{"iframe at html[1]/body[1]/iframe[0] has an inline srcdoc document"}},
	})
}

func TestNoInlineStyles(gt *testing.T) {
	checkRule(gt, webuitest.NoInlineStyles(), []ruleCase{
		{name: "valid", body: `<p class="text-danger">x</p>`},
		{name: "style element", body: `<style>p{color:red}</style>`, want: []string{"style element at html[1]/body[1]/style[0]"}},
		{name: "style attribute", body: `<p style="color:red">x</p>`, want: []string{"style attribute at html[1]/body[1]/p[0]"}},
		{name: "empty style attribute", body: `<p style>x</p>`, want: []string{"style attribute at"}},
	})
}

func TestNoHTMXScripting(gt *testing.T) {
	checkRule(gt, webuitest.NoHTMXScripting(), []ruleCase{
		{name: "hx-on prefix", body: `<button type="button" hx-get="/x" hx-trigger="click, keyup changed delay:500ms" hx-vals='{"a":1}' hx-headers='{"X-A":"b"}' hx-on-none>x</button>`, want: []string{"hx-on-none at"}},
		{name: "valid", body: `<button type="button" hx-get="/x" hx-trigger="sse:tick" hx-vals='{"a":"js:no"}' hx-request='{"timeout":100}'>x</button>`},
		{name: "js request", body: `<button type="button" hx-get="/x" hx-request='js: {timeout: computeTimeout()}'>x</button>`, want: []string{"hx-request at html[1]/body[1]/button[0] has a script value"}},
		{name: "javascript data request", body: `<div data-hx-request="javascript:{timeout: t()}"></div>`, want: []string{"data-hx-request at"}},
		{name: "hx-on colon", body: `<button type="button" hx-on:click="alert(1)">x</button>`, want: []string{"hx-on:click at html[1]/body[1]/button[0] is an htmx script handler"}},
		{name: "hx-on double colon", body: `<form hx-on::after-request="x()"></form>`, want: []string{"hx-on::after-request at"}},
		{name: "hx-on dash", body: `<div hx-on-click="x()"></div>`, want: []string{"hx-on-click at"}},
		{name: "data prefix", body: `<div data-hx-on:click="x()"></div>`, want: []string{"data-hx-on:click at"}},
		{name: "htmx 1 hx-on", body: `<div hx-on="click: x()"></div>`, want: []string{"hx-on at"}},
		{name: "hx-vars", body: `<div hx-vars="a: b()"></div>`, want: []string{"hx-vars at html[1]/body[1]/div[0] evaluates script"}},
		{name: "js vals", body: `<div hx-vals="js:{a: b()}"></div>`, want: []string{"hx-vals at html[1]/body[1]/div[0] has a script value"}},
		{name: "javascript headers", body: `<div data-hx-headers=" JavaScript:{a: b()}"></div>`, want: []string{"data-hx-headers at"}},
		{name: "trigger filter", body: `<input name="q" hx-get="/s" hx-trigger="keyup[key=='Enter']">`, want: []string{"hx-trigger at html[1]/body[1]/input[0] has an event filter"}},
	})
}

func TestSafeURLs(gt *testing.T) {
	checkRule(gt, webuitest.SafeURLs(), []ruleCase{
		{name: "valid", body: `<a href="/x">x</a><a href="mailto:a@example.test">m</a><img src="data:image/png;base64,AAAA" alt=""><link rel="icon" href="data:,"><img srcset="/a.png 1x, data:image/png;base64,AA 2x" alt=""><picture><source srcset="data:image/webp;base64,AA"></picture><video poster="data:image/png;base64,AA"></video><input type="image" src="data:image/gif;base64,AA" alt="go">`},
		{name: "javascript link", body: `<a href="javascript:alert(1)">x</a>`, want: []string{"href at html[1]/body[1]/a[0] is a javascript: URL"}},
		{name: "obfuscated", body: `<a href=" JaVa&#x09;Script:alert(1)">x</a>`, want: []string{"is a javascript: URL"}},
		{name: "newline inside", body: "<a href=\"java\nscript:alert(1)\">x</a>", want: []string{"is a javascript: URL"}},
		{name: "vbscript action", body: `<form action="vbscript:x"></form>`, want: []string{"action at html[1]/body[1]/form[0] is a vbscript: URL"}},
		{name: "htmx", body: `<div hx-get="javascript:x"></div>`, want: []string{"hx-get at"}},
		{name: "sse", body: `<div data-sse-connect="javascript:x"></div>`, want: []string{"data-sse-connect at"}},
		{name: "data frame", body: `<iframe src="data:text/html,x"></iframe>`, want: []string{"src at html[1]/body[1]/iframe[0] is a data: URL that is not an image"}},
		{name: "data link", body: `<a href="data:image/png,x">x</a>`, want: []string{"href at"}},
		{name: "data script", body: `<script src="data:text/javascript,alert(1)"></script>`, want: []string{"is a data: URL"}},
		{name: "data stylesheet", body: `<link rel="stylesheet" href="data:text/css,p{}">`, want: []string{"is a data: URL"}},
		{name: "data svg in object", body: `<object data="data:image/svg+xml,<svg/>"></object>`, want: []string{"data at"}},
		{name: "srcset", body: `<img srcset="/a.png 1x, data:text/html,x 2x" alt="">`, want: []string{"srcset at"}},
		{name: "media source", body: `<video><source src="data:image/png,AA"></video>`, want: []string{"src at"}},
	})
}

func TestLocalResources(gt *testing.T) {
	checkRule(gt, webuitest.LocalResources(), []ruleCase{
		{name: "valid", body: `<script src="/a.js"></script><img src="a.png" alt=""><img src="data:image/png;base64,AA" alt=""><form action="/x" method="post"></form><form></form><a href="https://other.example/">x</a><a href="//other.example/">y</a><link rel="canonical" href="https://other.example/"><button type="submit" formaction="?step=2">n</button><div hx-ext="sse" sse-connect="/live" sse-close="end"></div><link rel="icon" href="data:,">`},
		{name: "cdn script", body: `<script src="https://cdn.example/x.js"></script>`, want: []string{"src at html[1]/body[1]/script[0] names a host, not a same-origin path"}},
		{name: "scheme relative", body: `<script src="//cdn.example/x.js"></script>`, want: []string{"names a host"}},
		{name: "backslash", body: `<img src="/\evil.example/x.png" alt="">`, want: []string{"names a host"}},
		{name: "special scheme without slashes", body: `<form action="https:evil.example"></form>`, want: []string{"action at"}},
		{name: "own origin absolute", body: `<button type="button" hx-post="https://app.example.test/x">x</button>`, want: []string{"hx-post at"}},
		{name: "ftp stylesheet", body: `<link rel="stylesheet" href="ftp://x/a.css">`, want: []string{"href at"}},
		{name: "mailto action", body: `<form action="mailto:a@example.test"></form>`, want: []string{"is an absolute mailto: URL, not a same-origin path"}},
		{name: "sse", body: `<div hx-ext="sse" sse-connect="//evil.example/s" sse-close="end"></div>`, want: []string{"sse-connect at"}},
		{name: "ws", body: `<div hx-ext="ws" ws-connect="wss://evil.example/s"></div>`, want: []string{"ws-connect at"}},
		{name: "frame", body: `<iframe src="https://evil.example/"></iframe>`, want: []string{"src at"}},
		{name: "ping", body: `<a href="/x" ping="/p https://tracker.example/p">x</a>`, want: []string{"ping at"}},
		{name: "preconnect", body: `<link rel="preconnect" href="https://fonts.example">`, want: []string{"href at"}},
		{name: "formaction", body: `<form method="post"><button formaction="https://evil.example/x">x</button></form>`, want: []string{"formaction at"}},
		{name: "srcset", body: `<img srcset="/a.png 1x, https://cdn.example/b.png 2x" alt="">`, want: []string{"srcset at"}},
	})
}

func TestSubresourceIntegrity(gt *testing.T) {
	checkRule(gt, webuitest.SubresourceIntegrity(), []ruleCase{
		{name: "valid", body: `<script src="/a.js" integrity="` + sri("a") + `"></script><link rel="stylesheet" href="/a.css" integrity="sha512-bad ` + sri256("b") + `"><script>x</script><link rel="icon" href="data:,"><script src=""></script>`},
		{name: "missing", body: `<script src="/a.js"></script>`, want: []string{"script at html[1]/body[1]/script[0] has no integrity attribute"}},
		{name: "sha1", body: `<link rel="stylesheet" href="/a.css" integrity="sha1-2jmj7l5rSw0yVb/vlWAYkK/YBwk=">`, want: []string{"link at html[1]/body[1]/link[0] has no well-formed sha256, sha384 or sha512 integrity value"}},
		{name: "short digest", body: `<script src="/a.js" integrity="sha384-AAAA"></script>`, want: []string{"no well-formed"}},
		{name: "not base64", body: `<script src="/a.js" integrity="sha256-!!!"></script>`, want: []string{"no well-formed"}},
		{name: "preload", body: `<link rel="preload" href="/f.woff2" as="font">`, want: []string{"link at"}},
		{name: "modulepreload", body: `<link rel="modulepreload" href="/m.js">`, want: []string{"link at"}},
		{name: "options", body: `<script src="/a.js" integrity="` + sri("a") + `?ct=application/javascript"></script>`},
	})
}

const testToken = "tok-3f9a-visible-never"

func TestPostFormsCarryCSRF(gt *testing.T) {
	csrf := `<input type="hidden" name="csrf" value="` + testToken + `">`
	cases := []ruleCase{
		{name: "valid", body: `<form method="post" action="/x">` + csrf + `</form><form method="get"></form><form method="dialog"></form>`},
		{name: "case and associated", body: `<form id="f" method="POST"></form><input form="f" type="hidden" name="csrf" value="` + testToken + `">`},
		{name: "missing", body: `<form method="post" action="/x"></form>`, want: []string{`post form at html[1]/body[1]/form[0] has no "csrf" field`}},
		{name: "wrong value", body: `<form method="post"><input type="hidden" name="csrf" value="other"></form>`, want: []string{`has a "csrf" field with another value`}},
		{name: "disabled", body: `<form method="post"><input type="hidden" name="csrf" value="` + testToken + `" disabled></form>`, want: []string{`has only a disabled "csrf" field`}},
		{name: "disabled fieldset", body: `<form method="post"><fieldset disabled>` + csrf + `</fieldset></form>`, want: []string{`only a disabled`}},
		{name: "twice", body: `<form method="post">` + csrf + csrf + `</form>`, want: []string{`has 2 "csrf" fields`}},
		{name: "owned elsewhere", body: `<form id="f" method="post"></form><form id="g" method="post">` + strings.Replace(csrf, "<input", `<input form="f"`, 1) + `</form>`, want: []string{"form at html[1]/body[1]/form#g[1] has no"}},
		{name: "formmethod post", body: `<form action="/search"><button name="save" formmethod="post">Save</button></form>`, want: []string{"button at html[1]/body[1]/form[0]/button[0] submits with POST, but its form at html[1]/body[1]/form[0] has no"}},
		{name: "formmethod post with field", body: `<form>` + csrf + `<input type="submit" formmethod="post"></form>`},
		{name: "hx-post outside forms", body: `<button type="button" hx-post="/x">x</button>`, want: []string{`hx-post at html[1]/body[1]/button[0] sends no "csrf" field with the expected value`}},
		{name: "hx-delete in form", body: `<form method="post">` + csrf + `<button type="button" hx-delete="/x">x</button></form>`},
		{name: "hx-post in get form", body: `<form>` + csrf + `<button type="button" hx-post="/x">x</button></form>`},
		{name: "hx-post on form", body: `<form hx-post="/x">` + csrf + `</form>`},
		{name: "hx-post with vals", body: `<button type="button" hx-post="/x" hx-vals='{"csrf":"` + testToken + `"}'>x</button>`},
		{name: "hx-post with inherited vals", body: `<div hx-vals='{"csrf":"` + testToken + `"}'><button type="button" hx-put="/x">x</button></div>`},
		{name: "hx-post with wrong vals", body: `<button type="button" hx-post="/x" hx-vals='{"csrf":"no"}'>x</button>`, want: []string{"hx-post at"}},
		{name: "hx-post with script vals", body: `<button type="button" hx-post="/x" hx-vals='js:{csrf: "` + testToken + `"}'>x</button>`, want: []string{"hx-post at"}},
		{name: "hx-include id", body: `<form id="f">` + csrf + `</form><button type="button" hx-patch="/x" hx-include="#f">x</button>`},
		{name: "hx-include field", body: `<div id="d">` + csrf + `</div><button type="button" hx-patch="/x" hx-include="#nothing, #d">x</button>`},
		{name: "hx-include closest", body: `<form><div hx-include="closest form"><span hx-post="/x">x</span></div>` + csrf + `</form>`},
		{name: "data prefix", body: `<div data-hx-delete="/x"></div>`, want: []string{"data-hx-delete at"}},
		// Template content is a tree of its own; datalist controls are
		// never submitted.
		{name: "field in template", body: `<form method="post" action="/tasks"><template>` + csrf + `</template><button type="submit">Save</button></form>`, want: []string{`post form at html[1]/body[1]/form[0] has no "csrf" field`}},
		{name: "form attribute in template", body: `<form id="f" method="post"></form><template>` + strings.Replace(csrf, "<input", `<input form="f"`, 1) + `</template>`, want: []string{"form at html[1]/body[1]/form#f[0] has no"}},
		{name: "form in template", body: `<template><form method="post">` + csrf + `</form><form method="post"></form></template>`, want: []string{"form at html[1]/body[1]/template[0]/form[1] has no"}},
		{name: "include into template", body: `<div id="d"><template>` + csrf + `</template></div><button type="button" hx-post="/x" hx-include="#d">x</button>`, want: []string{"hx-post at"}},
		{name: "field in datalist", body: `<form method="post"><datalist id="l">` + csrf + `</datalist></form>`, want: []string{`has no "csrf" field`}},
		// htmx merges the hx-vals of every ancestor, nearer keys first.
		{name: "merged vals", body: `<div hx-vals='{"csrf":"` + testToken + `"}'><div hx-vals='{"x":"1"}'><button type="button" hx-post="/a" hx-vals='{"op":"up"}'>x</button></div></div>`},
		{name: "merged data vals", body: `<div data-hx-vals='{"csrf":"` + testToken + `"}'><button type="button" hx-post="/a" hx-vals='{"x":1}'>x</button></div>`},
		{name: "nearer null", body: `<div hx-vals='{"csrf":"` + testToken + `"}'><button type="button" hx-post="/a" hx-vals='{"csrf":null}'>x</button></div>`},
		{name: "nearer empty", body: `<div hx-vals='{"csrf":"` + testToken + `"}'><button type="button" hx-post="/a" hx-vals="">x</button></div>`},
		{name: "vals without braces", body: `<button type="button" hx-post="/a" hx-vals='"csrf":"` + testToken + `"'>x</button>`},
		{name: "nearer wrong vals", body: `<div hx-vals='{"csrf":"` + testToken + `"}'><button type="button" hx-post="/a" hx-vals='{"csrf":"no"}'>x</button></div>`, want: []string{"hx-post at"}},
		{name: "unset vals", body: `<div hx-vals="unset"><button type="button" hx-post="/a" hx-vals='{"csrf":"` + testToken + `"}'>x</button></div>`, want: []string{"hx-post at"}},
		// hx-params filters what htmx sends.
		{name: "params none", body: `<form method="post" action="/tasks" hx-post="/tasks" hx-params="none">` + csrf + `</form>`, want: []string{"hx-post at html[1]/body[1]/form[0] sends no"}},
		{name: "data params none", body: `<form method="post" action="/tasks" hx-post="/tasks" data-hx-params="none">` + csrf + `</form>`, want: []string{"hx-post at"}},
		{name: "params not csrf", body: `<form method="post" hx-post="/tasks" hx-params="not title, csrf">` + csrf + `</form>`, want: []string{"hx-post at"}},
		{name: "params list without csrf", body: `<form method="post" hx-post="/tasks" hx-params="title">` + csrf + `</form>`, want: []string{"hx-post at"}},
		{name: "params inherited none", body: `<div hx-params="none"><form method="post">` + csrf + `<button type="button" hx-put="/x">x</button></form></div>`, want: []string{"hx-put at"}},
		{name: "params none with vals", body: `<button type="button" hx-post="/x" hx-params="none" hx-vals='{"csrf":"` + testToken + `"}'>x</button>`, want: []string{"hx-post at"}},
		{name: "params list with csrf", body: `<form method="post" hx-post="/tasks" hx-params="title, csrf">` + csrf + `</form>`},
		{name: "params not other", body: `<form method="post" hx-post="/tasks" hx-params="not title">` + csrf + `</form>`},
		{name: "params all", body: `<form method="post" hx-post="/tasks" hx-params="*">` + csrf + `</form>`},
		{name: "params unset", body: `<div hx-params="none"><form method="post" hx-post="/tasks" hx-params="unset">` + csrf + `</form></div>`},
		{name: "params disinherited", body: `<div hx-params="none"><form method="post" hx-post="/tasks" hx-disinherit="hx-params">` + csrf + `</form></div>`, want: []string{"hx-post at"}},
		{name: "params disinherited above", body: `<div hx-params="none"><div hx-disinherit="hx-params"><form method="post" hx-post="/tasks">` + csrf + `</form></div></div>`},
		{name: "params empty inherits", body: `<div hx-params="none"><form method="post" hx-post="/tasks" hx-params="">` + csrf + `</form></div>`, want: []string{"hx-post at"}},
		// "this" is the element that declares hx-include, as a whole value.
		{name: "inherited include this", body: `<div hx-include="this">` + csrf + `<button type="button" hx-post="/tasks/1/done">Done</button></div>`},
		{name: "inherited data include this", body: `<div data-hx-include="this">` + csrf + `<button type="button" hx-post="/tasks/1/done">Done</button></div>`},
		{name: "this in a list", body: `<div hx-include="this, #nope">` + csrf + `<button type="button" hx-post="/x">x</button></div>`, want: []string{"hx-post at"}},
		{name: "this with a space", body: `<div hx-include=" this">` + csrf + `<button type="button" hx-post="/x">x</button></div>`, want: []string{"hx-post at"}},
		{name: "include disinherited", body: `<div hx-include="this" hx-disinherit="hx-include">` + csrf + `<button type="button" hx-post="/x">x</button></div>`, want: []string{"hx-post at"}},
		// The server reads the first csrf value of a submission: the field
		// must come before every other control that sends one.
		{name: "select before the field", body: `<form method="post"><select name="csrf"><option>a</option></select>` + csrf + `</form>`, want: []string{`post form at html[1]/body[1]/form[0] sends a "csrf" value from the select at html[1]/body[1]/form[0]/select[0] before its "csrf" field, and the server reads the first`}},
		{name: "textarea before the field", body: `<form method="post"><textarea name="csrf"></textarea>` + csrf + `</form>`, want: []string{`from the textarea at html[1]/body[1]/form[0]/textarea[0] before`}},
		{name: "submit button before the field", body: `<form method="post"><button name="csrf" value="go">Go</button>` + csrf + `</form>`, want: []string{`from the submit button at html[1]/body[1]/form[0]/button[0] before`}},
		{name: "dirname before the field", body: `<form method="post"><input name="title" dirname="csrf">` + csrf + `</form>`, want: []string{`from the dirname of the input at html[1]/body[1]/form[0]/input[0] before`}},
		{name: "textarea dirname before the field", body: `<form method="post"><textarea name="body" dirname="csrf"></textarea>` + csrf + `</form>`, want: []string{`from the dirname of the textarea at`}},
		{name: "associated select before the field", body: `<select form="f" name="csrf"><option>a</option></select><form id="f" method="post">` + csrf + `</form>`, want: []string{`from the select at html[1]/body[1]/select[0] before`}},
		// A select sends what FormData sends for it: nothing without an
		// option to submit.
		{name: "selects that send nothing before the field", body: `<form method="post"><select name="csrf"></select><select name="csrf" multiple><option>a</option></select><select name="csrf"><option disabled>a</option></select><select name="csrf" size="2"><option>a</option></select>` + csrf + `</form>`},
		{name: "associated select without options before the field", body: `<select form="f" name="csrf"></select><form id="f" method="post">` + csrf + `</form>`},
		{name: "controls after the field", body: `<form method="post">` + csrf + `<select name="csrf"><option>a</option></select><textarea name="csrf"></textarea><button name="csrf">Go</button><input name="t" dirname="csrf"></form>`},
		{name: "controls that send nothing before the field", body: `<form method="post"><button type="button" name="csrf">x</button><button type="reset" name="csrf">x</button><select name="csrf" disabled></select><fieldset disabled><textarea name="csrf"></textarea></fieldset><datalist id="l"><select name="csrf"></select></datalist><textarea dirname="csrf"></textarea><select name="CSRF"></select>` + csrf + `</form>`},
		{name: "get submit button before the field", body: `<form method="post"><button name="csrf" formmethod="get">Search</button>` + csrf + `</form>`},
		{name: "formmethod post submitter", body: `<form><button name="csrf" formmethod="post">Save</button>` + csrf + `</form>`, want: []string{"button at html[1]/body[1]/form[0]/button[0] submits with POST, but its form at html[1]/body[1]/form[0] sends a \"csrf\" value from the submit button at html[1]/body[1]/form[0]/button[0] before"}},
		{name: "formmethod post other button", body: `<form><button name="csrf">Search</button>` + csrf + `<button formmethod="post">Save</button></form>`},
		{name: "hx-post form with a button before the field", body: `<form hx-post="/x"><button name="csrf">Go</button>` + csrf + `</form>`},
		{name: "hx-post form with a select before the field", body: `<form hx-post="/x"><select name="csrf"><option>a</option></select>` + csrf + `</form>`, want: []string{`hx-post at html[1]/body[1]/form[0] sends a "csrf" value other than the token first, and the server reads the first`}},
		{name: "hx-post form with a select without options before the field", body: `<form hx-post="/x"><select name="csrf"></select>` + csrf + `</form>`},
		// htmx sends the value of an included select, also without options.
		{name: "hx-include with a select before the field", body: `<div id="d"><select name="csrf"></select>` + csrf + `</div><button type="button" hx-post="/x" hx-include="#d">x</button>`, want: []string{`hx-post at html[1]/body[1]/button[1] sends a "csrf" value other than the token first`}},
		{name: "hx-include with a select after the field", body: `<div id="d">` + csrf + `<select name="csrf"></select><button name="csrf">x</button></div><button type="button" hx-post="/x" hx-include="#d">x</button>`},
		// The parameters of the request decide, as HTMXData collects them:
		// hx-vals replace the fields of their names, the elements of an
		// hx-include send their values in tree order, and the fields of the
		// related form replace those of the includes.
		{name: "form field and other vals", body: `<form hx-post="/x" hx-vals='{"csrf":"other"}'>` + csrf + `</form>`, want: []string{`hx-post at html[1]/body[1]/form[0] sends a "csrf" value other than the token first`}},
		{name: "post form field and other vals", body: `<form method="post" hx-post="/x" hx-vals='{"csrf":"other"}'>` + csrf + `</form>`, want: []string{`hx-post at html[1]/body[1]/form[0] sends a "csrf" value other than the token first`}},
		{name: "form field and vals of the token", body: `<form hx-post="/x" hx-vals='{"csrf":"` + testToken + `"}'><select name="csrf"><option>a</option></select>` + csrf + `</form>`},
		{name: "includes with a select first", body: `<div id="a"><select name="csrf"><option>x</option></select></div><div id="b">` + csrf + `</div><button type="button" hx-post="/x" hx-include="#a, #b">x</button>`, want: []string{`hx-post at html[1]/body[1]/button[2] sends a "csrf" value other than the token first`}},
		{name: "includes with the field first", body: `<div id="b">` + csrf + `</div><div id="a"><select name="csrf"><option>x</option></select></div><button type="button" hx-post="/x" hx-include="#a, #b">x</button>`},
		{name: "form fields over includes", body: `<form>` + csrf + `<button type="button" hx-post="/x" hx-include="#a">x</button></form><div id="a"><select name="csrf"><option>x</option></select></div>`},
		{name: "button of the request after the form's fields", body: `<form>` + csrf + `<button name="csrf" value="go" hx-post="/x">x</button></form>`},
		{name: "parts of an include", body: `<div id="d">` + csrf + `</div><button type="button" hx-post="/x" hx-include="inherit, #d, div > input">x</button>`},
	}
	checkRule(gt, webuitest.PostFormsCarryCSRF(testToken), cases)
	gt.Run("no token", func(gt *testing.T) {
		checkRule(gt, webuitest.PostFormsCarryCSRF(""), []ruleCase{
			{name: "any value", body: `<form method="post"><input type="hidden" name="csrf" value="x"></form>`},
			{name: "empty", body: `<form method="post"><input type="hidden" name="csrf" value=""></form>`, want: []string{`has an empty "csrf" field`}},
			{name: "hx-post", body: `<button type="button" hx-post="/x">x</button>`, want: []string{`sends no "csrf" field`}},
			{name: "hx-post with an empty value first", body: `<form hx-post="/x"><select name="csrf"><option value="">-</option></select><input type="hidden" name="csrf" value="x"></form>`, want: []string{`hx-post at html[1]/body[1]/form[0] sends an empty "csrf" value first`}},
		})
	})
	gt.Run("never shows the token", func(gt *testing.T) {
		t := devtest.ParallelT(gt)
		r := webuitest.PostFormsCarryCSRF(testToken)
		t.Must(be.Equal("postFormsCarryCSRF(token)", r.String()))
		t.Must(be.Equal("postFormsCarryCSRF()", webuitest.PostFormsCarryCSRF("").String()))
		got := violations(t, cthtml.PageString(htmlPage(`<form method="post"><input type="hidden" name="csrf" value="x`+testToken+`"></form>`)), r)
		t.Must(be.Len(got, 1))
		t.Must(be.False(strings.Contains(got[0], testToken)))
	})
}

func TestNoTokenLeaks(gt *testing.T) {
	csrf := `<input type="hidden" name="csrf" value="` + testToken + `">`
	cases := []ruleCase{
		{name: "valid", body: `<form method="post">` + csrf + `</form><button type="button" hx-post="/x" hx-vals='{"csrf":"` + testToken + `"}' hx-headers='{"X-T":"` + testToken + `"}'>x</button><form method="get"><input name="q"></form>`},
		{name: "get form", body: `<form action="/search">` + csrf + `</form>`, want: []string{`get form at html[1]/body[1]/form[0] owns the "csrf" field, which puts the token in the URL`}},
		{name: "disabled in get form", body: `<form><input type="hidden" name="csrf" value="` + testToken + `" disabled></form>`},
		{name: "formmethod get", body: `<form method="post">` + csrf + `<button formmethod="get">x</button></form>`, want: []string{`button at html[1]/body[1]/form[0]/button[1] submits the "csrf" field with GET`}},
		{name: "hx-get form", body: `<form method="post" hx-get="/x">` + csrf + `</form>`, want: []string{`hx-get at html[1]/body[1]/form[0] sends the "csrf" field in the URL`}},
		{name: "hx-get include", body: `<div id="d">` + csrf + `</div><button type="button" hx-get="/x" hx-include="#d">x</button>`, want: []string{`hx-get at html[1]/body[1]/button[1]`}},
		{name: "visible field", body: `<form method="post"><input type="text" name="csrf" value="` + testToken + `"></form>`, want: []string{`"csrf" field at html[1]/body[1]/form[0]/input[0] is not hidden`}},
		{name: "other controls of the name", body: `<form method="post">` + csrf + `<select name="csrf"></select><textarea name="csrf"></textarea><button type="button" name="csrf">x</button><select name="CSRF"></select></form>`, want: []string{
			`"csrf" field at html[1]/body[1]/form[0]/select[1] is not hidden`, `"csrf" field at html[1]/body[1]/form[0]/textarea[2] is not hidden`, `"csrf" field at html[1]/body[1]/form[0]/button[3] is not hidden`,
		}},
		{name: "link", body: `<a href="/x?csrf=` + testToken + `">x</a>`, want: []string{"href at html[1]/body[1]/a[0] has the token in a URL"}},
		{name: "encoded", body: `<a href="/x?t=` + strings.ReplaceAll(testToken, "-", "%2D") + `">x</a>`},
		{name: "attribute", body: `<div data-x="` + testToken + `"></div>`, want: []string{"data-x at html[1]/body[1]/div[0] has the token"}},
		{name: "text", body: `<p>` + testToken + `</p>`, want: []string{"text at html[1]/body[1]/p[0]/[0] shows the token"}},
		{name: "comment", body: `<!-- ` + testToken + ` -->`, want: []string{"comment at"}},
		{name: "hx-get url", body: `<div hx-get="/x?c=` + testToken + `"></div>`, want: []string{"hx-get at html[1]/body[1]/div[0] has the token in a URL"}},
		// htmx puts the parameters of GET and DELETE requests in the URL, and
		// includes the related form for every method but GET.
		{name: "hx-delete in post form", body: `<form method="post">` + csrf + `<button type="button" hx-delete="/tasks/1">x</button></form>`, want: []string{`hx-delete at html[1]/body[1]/form[0]/button[1] sends the "csrf" field in the URL`}},
		{name: "hx-delete with vals", body: `<button type="button" hx-delete="/tasks/1" hx-vals='{"csrf":"` + testToken + `"}'>x</button>`, want: []string{`hx-delete at html[1]/body[1]/button[0] sends the "csrf" field in the URL`}},
		{name: "data-hx-delete form", body: `<form method="post" data-hx-delete="/tasks/1">` + csrf + `</form>`, want: []string{`data-hx-delete at html[1]/body[1]/form[0] sends the "csrf" field in the URL`}},
		{name: "hx-get under vals", body: `<div hx-vals='{"csrf":"` + testToken + `"}'><button type="button" hx-get="/search">x</button></div>`, want: []string{`hx-get at html[1]/body[1]/div[0]/button[0] sends the "csrf" field in the URL`}},
		{name: "hx-get under vals with the token", body: `<div hx-vals='{"t":"` + testToken + `"}'><button type="button" hx-get="/search">x</button></div>`, want: []string{`hx-get at html[1]/body[1]/div[0]/button[0] sends the token in the URL (hx-vals)`}},
		{name: "hx-get in post form", body: `<form method="post">` + csrf + `<button type="button" hx-get="/x">x</button></form>`},
		{name: "vals unset", body: `<div hx-vals='{"csrf":"` + testToken + `"}'><div hx-vals="unset"><button type="button" hx-get="/search">x</button></div></div>`},
		{name: "params drop the field", body: `<form method="post">` + csrf + `<button type="button" hx-delete="/tasks/1" hx-params="none">x</button></form>`},
		{name: "include this", body: `<div hx-include="this">` + csrf + `<button type="button" hx-get="/x">x</button></div>`, want: []string{`hx-get at html[1]/body[1]/div[0]/button[1] sends the "csrf" field in the URL`}},
		// hx-include and hx-params inherit as in htmx.
		{name: "include disinherited", body: `<div hx-include="this" hx-disinherit="hx-include">` + csrf + `<button type="button" hx-get="/x">x</button></div>`},
		{name: "include unset", body: `<div hx-include="this">` + csrf + `<button type="button" hx-get="/x" hx-include="unset">x</button></div>`},
		{name: "params disinherited", body: `<div id="d">` + csrf + `</div><div hx-params="none"><div hx-disinherit="hx-params"><button type="button" hx-get="/x" hx-include="#d">x</button></div></div>`, want: []string{`hx-get at html[1]/body[1]/div[1]/div[0]/button[0]`}},
		{name: "template field in get form", body: `<form action="/search"><template>` + csrf + `</template></form>`},
	}
	checkRule(gt, webuitest.NoTokenLeaks(testToken), cases)
	gt.Run("methodsThatUseUrlParams", func(gt *testing.T) {
		t := devtest.ParallelT(gt)
		body := `<form method="post">` + csrf + `<button type="button" hx-delete="/tasks/1">x</button><button type="button" hx-post="/tasks/1">y</button></form>`
		page := func(config string) *ct.Subject {
			return cthtml.PageString(`<!DOCTYPE html><html lang="en"><head><title>t</title><meta name="htmx-config" content='` + config + `'></head><body>` + body + `</body></html>`)
		}
		t.Must(be.Len(violations(t, page(`{"methodsThatUseUrlParams":["get"]}`), webuitest.NoTokenLeaks(testToken)), 0))
		got := violations(t, page(`{"methodsThatUseUrlParams":["get","delete","post"]}`), webuitest.NoTokenLeaks(testToken))
		t.Must(be.Len(got, 2))
		t.Must(be.Substring(got[1], "hx-post at"))
		t.Must(be.Len(violations(t, page(`{"selfRequestsOnly":true}`), webuitest.NoTokenLeaks(testToken)), 1))
		// htmx compares the verbs case-sensitively: "DELETE" is not the
		// "delete" of hx-delete.
		t.Must(be.Len(violations(t, page(`{"methodsThatUseUrlParams":["GET","DELETE"]}`), webuitest.NoTokenLeaks(testToken)), 0))
	})
	gt.Run("encoded token", func(gt *testing.T) {
		checkRule(gt, webuitest.NoTokenLeaks("a b/c"), []ruleCase{
			{name: "query escaped", body: `<a href="/x?t=a+b%2Fc">x</a>`, want: []string{"href at"}},
			{name: "path escaped", body: `<a href="/x/a%20b%2Fc">x</a>`, want: []string{"href at"}},
		})
	})
	gt.Run("never shows the token", func(gt *testing.T) {
		t := devtest.ParallelT(gt)
		got := violations(t, cthtml.PageString(htmlPage(`<a href="/x?csrf=`+testToken+`">`+testToken+`</a>`)), webuitest.NoTokenLeaks(testToken))
		t.Must(be.Len(got, 2))
		for _, g := range got {
			t.Must(be.False(strings.Contains(g, testToken)))
		}
	})
}

// A fragment runs under the htmx configuration of its page: the zero
// hx2.Config, whose DELETE requests carry their parameters in the body,
// unless WithHTMXConfig names another.
func TestFragmentHTMXConfig(gt *testing.T) {
	t := devtest.ParallelT(gt)
	csrf := `<input type="hidden" name="csrf" value="` + testToken + `">`
	deleteForm := `<form method="post" action="/notes/1/delete">` + csrf + `<button type="button" hx-delete="/notes/1">Delete</button></form>`
	deleteVals := `<button type="button" hx-delete="/notes/1" hx-vals='{"csrf":"` + testToken + `"}'>Delete</button>`
	leaky := webuitest.WithHTMXConfig(hx2.Config{DeleteURLParams: true})
	fragment := func(body string) *ct.Subject { return cthtml.FragmentString("div", body) }
	t.Run("default config", func(t devtest.T) {
		for _, body := range []string{deleteForm, deleteVals} {
			t.Must(be.Len(violations(t, fragment(body), webuitest.NoTokenLeaks(testToken)), 0))
			t.Must(webuitest.FragmentContract(fragment(body), webuitest.Rules(webuitest.WithCSRF(testToken))...))
		}
	})
	t.Run("delete URL params", func(t devtest.T) {
		err := webuitest.FragmentContract(fragment(deleteForm), webuitest.Rules(webuitest.WithCSRF(testToken), leaky)...).Check(t.Context())
		t.Must(be.ErrorContains(err, `noTokenLeaks(token): hx-delete at form[0]/button[1] sends the "csrf" field in the URL`))
		err = webuitest.FragmentContract(fragment(deleteVals), webuitest.Rules(webuitest.WithCSRF(testToken), leaky)...).Check(t.Context())
		t.Must(be.ErrorContains(err, `hx-delete at button[0] sends the "csrf" field in the URL`))
		// The zero Config again.
		t.Must(webuitest.FragmentContract(fragment(deleteForm), webuitest.Rules(webuitest.WithCSRF(testToken), leaky, webuitest.WithHTMXConfig(hx2.Config{}))...))
	})
	t.Run("hx-get leaks under any config", func(t devtest.T) {
		got := violations(t, fragment(`<form method="post">`+csrf+`<button type="button" hx-get="/x" hx-include="closest form">x</button></form>`), webuitest.NoTokenLeaks(testToken))
		t.Must(be.Len(got, 1))
		t.Must(be.Substring(got[0], `hx-get at form[0]/button[1] sends the "csrf" field in the URL`))
	})
	t.Run("a meta element in a fragment is not the config", func(t devtest.T) {
		body := `<meta name="htmx-config" content='{"methodsThatUseUrlParams":["get","delete"]}'>` + deleteForm
		t.Must(be.Len(violations(t, fragment(body), webuitest.NoTokenLeaks(testToken)), 0))
	})
	t.Run("documents keep their own config", func(t devtest.T) {
		// Without a meta element a document runs htmx's defaults, whatever
		// the option says.
		page := cthtml.PageString(htmlPage(deleteForm))
		err := page.Valid(webuitest.Rules(webuitest.WithCSRF(testToken), webuitest.WithHTMXConfig(hx2.Config{}))...).Check(t.Context())
		t.Must(be.ErrorContains(err, `hx-delete at html[1]/body[1]/form[0]/button[1] sends the "csrf" field in the URL`))
	})
	t.Run("secrets", func(t devtest.T) {
		const secret = "docs/report.txt"
		body := `<button type="button" hx-delete="/files" hx-vals='{"path":"` + secret + `"}'>x</button>`
		t.Must(fragment(body).Valid(webuitest.Rules(webuitest.WithSecrets(secret), webuitest.Without("secretsHidden", "postFormsCarryCSRF"))...))
		err := fragment(body).Valid(webuitest.Rules(webuitest.WithSecrets(secret), webuitest.Without("secretsHidden", "postFormsCarryCSRF"), leaky)...).Check(t.Context())
		t.Must(be.ErrorContains(err, "notInURLs(1 values): value 1 in the hx-vals that hx-delete at button[0] sends in the URL"))
		t.Must(be.Len(violations(t, fragment(body), webuitest.NotInURLs(secret)), 0))
	})
	t.Run("invalid config", func(t devtest.T) {
		t.Must(be.Panic(func() {
			webuitest.WithHTMXConfig(hx2.Config{Extra: map[string]any{"methodsThatUseUrlParams": []string{"get"}}})
		}))
	})
}

func TestSecretsHidden(gt *testing.T) {
	const s1, s2 = "first-secret-value", "second-secret-value"
	checkRule(gt, webuitest.SecretsHidden(s1, s2), []ruleCase{
		{name: "valid", body: `<template><p>` + s1 + `</p></template><details><summary>Show</summary><p>` + s2 + `</p><details open><p>` + s1 + `</p></details></details>`},
		{name: "visible", body: `<p>` + s1 + `</p>`, want: []string{"secret 1 in visible text at html[1]/body[1]/p[0]/[0]"}},
		{name: "second", body: `<p>x ` + s2 + `</p>`, want: []string{"secret 2 in visible text at"}},
		{name: "attribute", body: `<input name="k" value="` + s1 + `">`, want: []string{"secret 1 in attribute value at html[1]/body[1]/input[0]"}},
		{name: "attribute in template", body: `<template><a href="/x/` + s1 + `">x</a></template>`, want: []string{"secret 1 in attribute href at"}},
		{name: "open details", body: `<details open><summary>s</summary>` + s1 + `</details>`, want: []string{"secret 1 in visible text"}},
		{name: "summary", body: `<details><summary>` + s1 + `</summary>x</details>`, want: []string{"secret 1 in visible text"}},
		{name: "comment", body: `<!-- ` + s2 + ` -->`, want: []string{"secret 2 in a comment at"}},
		{name: "textarea", body: `<textarea name="t">` + s1 + `</textarea>`, want: []string{"secret 1 in visible text"}},
	})
	gt.Run("names, never values", func(gt *testing.T) {
		t := devtest.ParallelT(gt)
		r := webuitest.SecretsHidden(s1, s2)
		t.Must(be.Equal("secretsHidden(2 values)", r.String()))
		got := violations(t, cthtml.PageString(htmlPage(`<p title="`+s1+`">`+s2+`</p>`)), r)
		t.Must(be.Len(got, 2))
		for _, g := range got {
			t.Must(be.False(strings.Contains(g, "secret-value")))
		}
		t.Must(be.Panic(func() { webuitest.SecretsHidden("x", "") }))
	})
}

func TestNotInURLs(gt *testing.T) {
	checkRule(gt, webuitest.NotInURLs("docs/report.txt"), []ruleCase{
		{name: "valid", body: `<p>docs/report.txt</p><form method="post"><input type="hidden" name="path" value="docs/report.txt"></form>`},
		{name: "query", body: `<a href="/files?path=docs%2Freport.txt">x</a>`, want: []string{"value 1 in href at html[1]/body[1]/a[0]"}},
		{name: "path", body: `<a href="/files/docs/report.txt">x</a>`, want: []string{"value 1 in href"}},
		{name: "action", body: `<form action="/f?p=docs/report.txt"></form>`, want: []string{"value 1 in action"}},
		{name: "htmx", body: `<div hx-get="/f?p=docs%2Freport.txt"></div>`, want: []string{"value 1 in hx-get"}},
		// A GET submission puts the fields in the query.
		{name: "get form", body: `<form action="/files"><input type="hidden" name="path" value="docs/report.txt"></form>`, want: []string{"value 1 in a field of input at html[1]/body[1]/form[0]/input[0], which the form at html[1]/body[1]/form[0] submits in the URL"}},
		{name: "get text input", body: `<form action="/files"><input name="path" value="docs/report.txt"></form>`, want: []string{"value 1 in a field of input"}},
		{name: "get textarea", body: `<form action="/files"><textarea name="path">docs/report.txt</textarea></form>`, want: []string{"value 1 in a field of textarea"}},
		{name: "get option", body: `<form action="/files"><select name="path"><option>a</option><option value="docs/report.txt">r</option></select></form>`, want: []string{"value 1 in a field of select"}},
		{name: "formmethod get", body: `<form method="post" action="/files"><input type="hidden" name="path" value="docs/report.txt"><button formmethod="get">Find</button></form>`, want: []string{"value 1 in a field of input"}},
		{name: "hx-get form", body: `<form hx-get="/files"><input type="hidden" name="path" value="docs/report.txt"></form>`, want: []string{"value 1 in a field of input"}},
		{name: "hx-get vals", body: `<div hx-vals='{"path":"docs/report.txt"}'><button type="button" hx-get="/files">x</button></div>`, want: []string{"value 1 in the hx-vals that hx-get at html[1]/body[1]/div[0]/button[0] sends in the URL"}},
		{name: "disabled and nameless", body: `<form action="/files"><input type="hidden" name="path" value="docs/report.txt" disabled><input value="docs/report.txt"></form>`},
		{name: "post vals", body: `<button type="button" hx-post="/files" hx-vals='{"path":"docs/report.txt"}'>x</button>`},
	})
	gt.Run("empty", func(gt *testing.T) {
		t := devtest.ParallelT(gt)
		t.Must(be.Panic(func() { webuitest.NotInURLs("") }))
	})
}

func TestHXTargets(gt *testing.T) {
	checkRule(gt, webuitest.HXTargets(webuitest.DocumentTargets()...), []ruleCase{
		{name: "valid", body: `<div id="out"></div><form><input name="q"><button type="button" hx-get="/x" hx-target="#out" hx-include="closest form, [name=q]" hx-indicator="find .spin" hx-disabled-elt="next .after"><span class="spin"></span></button><button type="button" hx-get="/y" hx-target="this" hx-disabled-elt="this">y</button></form><p class="after"></p><table><tr><td><button type="button" hx-delete="/r" hx-target="closest tr" hx-swap="outerHTML">x</button></td></tr></table><div hx-target="body" hx-include="previous #out"></div><div hx-target="<#out/>"></div><div hx-target="div > p"></div><div hx-target=".after"></div>`},
		{name: "missing id", body: `<button type="button" hx-get="/x" hx-target="#nope">x</button>`, want: []string{`hx-target at html[1]/body[1]/button[0] targets missing id "nope"`}},
		{name: "document target in a document", body: `<button type="button" hx-get="/x" hx-target="#main">x</button>`, want: []string{`targets missing id "main"`}},
		{name: "closest", body: `<button type="button" hx-post="/x" hx-include="closest form">x</button>`, want: []string{`hx-include at html[1]/body[1]/button[0] finds no "form" ancestor`}},
		{name: "find", body: `<button type="button" hx-get="/x" hx-indicator="find .spin">x</button>`, want: []string{`finds no ".spin" descendant`}},
		{name: "next", body: `<p class="a"></p><button type="button" hx-get="/x" hx-target="next .a">x</button>`, want: []string{`finds no following ".a"`}},
		{name: "next excludes descendants", body: `<button type="button" hx-get="/x" hx-target="next .a"><span class="a"></span></button>`, want: []string{`finds no following ".a"`}},
		{name: "previous", body: `<button type="button" hx-get="/x" hx-target="previous .a">x</button><p class="a"></p>`, want: []string{`finds no preceding ".a"`}},
		{name: "selector", body: `<button type="button" hx-get="/x" hx-target=".results">x</button>`, want: []string{`selects no element with ".results"`}},
		{name: "this in a list", body: `<p class="after"></p><button type="button" hx-get="/x" hx-disabled-elt="this, previous .after">x</button>`, want: []string{`hx-disabled-elt at html[1]/body[1]/button[1] selects no element with "this"`}},
		{name: "list part", body: `<button id="b" type="button" hx-get="/x" hx-disabled-elt="#b, #c">x</button>`, want: []string{`hx-disabled-elt at html[1]/body[1]/button#b[0] targets missing id "c"`}},
		{name: "data prefix", body: `<div data-hx-target="#gone"></div>`, want: []string{"data-hx-target at"}},
		{name: "attribute selector", body: `<div hx-include="[name='q']"></div>`, want: []string{`selects no element with "[name='q']"`}},
		{name: "fragment document target", fragment: true, body: `<button type="button" hx-get="/x" hx-target="#main" hx-include="closest form">x</button>`},
		{name: "fragment layout targets", fragment: true, body: `<button type="button" hx-get="/badge" hx-target="#navbar">x</button><button type="button" hx-get="/side" hx-target="#sidebar">y</button><div hx-target="#page-header"></div><div hx-target="#dialogs"></div><div hx-target="#alerts"></div>`},
		{name: "fragment missing", fragment: true, body: `<button type="button" hx-get="/x" hx-target="#nope">x</button>`, want: []string{`targets missing id "nope"`}},
		{name: "fragment find", fragment: true, body: `<button type="button" hx-get="/x" hx-indicator="find .spin">x</button>`, want: []string{`finds no ".spin" descendant`}},
		// Inherited relative selectors start from the requesting element.
		{name: "inherited closest", body: `<table><tbody hx-target="closest tr" hx-swap="outerHTML"><tr><td><button type="button" hx-delete="/r/1">x</button></td></tr><tr><td><button type="button" data-hx-delete="/r/2">y</button></td></tr></tbody></table>`},
		{name: "inherited next", body: `<div hx-include="next input"><button type="button" hx-get="/x">x</button><input name="q"></div>`},
		{name: "inherited find", body: `<div hx-indicator="find .spin"><span class="spin"></span><button type="button" hx-get="/x">x</button></div>`, want: []string{`hx-indicator at html[1]/body[1]/div[0], inherited by html[1]/body[1]/div[0]/button[1], finds no ".spin" descendant`}},
		{name: "inherited closest missing", body: `<div hx-target="closest section"><button type="button" hx-get="/x">x</button></div>`, want: []string{`inherited by html[1]/body[1]/div[0]/button[0], finds no "section" ancestor`}},
		{name: "nearer attribute wins", body: `<div hx-target="closest section"><p hx-target="#out"><button type="button" hx-get="/x">x</button></p></div><div id="out"></div>`},
		{name: "sse swap target", body: `<div hx-target="closest li"><p sse-swap="tick">x</p></div>`, want: []string{"inherited by html[1]/body[1]/div[0]/p[0]"}},
		// "unset" and hx-disinherit stop the inheritance, as in htmx.
		{name: "unset", body: `<form hx-post="/f" hx-disabled-elt="find button"><button type="submit">Save</button><button type="button" hx-get="/pick" hx-target="unset" hx-include="unset" hx-indicator="unset" hx-disabled-elt="unset">Pick</button></form><div hx-include="unset"></div>`},
		{name: "unset stops inheritance", body: `<div hx-target="closest section"><p hx-target="unset"><button type="button" hx-get="/x">x</button></p></div>`},
		{name: "unset in a list", body: `<input id="q" name="q"><button type="button" hx-get="/x" hx-include="unset, #q">x</button>`, want: []string{`hx-include at html[1]/body[1]/button[1] selects no element with "unset"`}},
		{name: "empty inherits", body: `<div hx-target="closest section"><button type="button" hx-get="/x" hx-target="">x</button></div>`, want: []string{`hx-target at html[1]/body[1]/div[0], inherited by html[1]/body[1]/div[0]/button[0], finds no "section" ancestor`}},
		{name: "disinherit", body: `<div hx-target="closest section" hx-disinherit="hx-target"><button type="button" hx-get="/x">x</button></div>`},
		{name: "disinherit all", body: `<div hx-indicator="find .spin" data-hx-disinherit="*"><p><button type="button" hx-get="/x">x</button></p></div>`},
		{name: "disinherit others", body: `<div hx-target="closest section" hx-disinherit="hx-include hx-select"><button type="button" hx-get="/x">x</button></div>`, want: []string{`inherited by html[1]/body[1]/div[0]/button[0], finds no "section" ancestor`}},
		{name: "disinherit on the requester", body: `<div hx-get="/x" hx-target="closest section" hx-disinherit="hx-target"></div>`, want: []string{`hx-target at html[1]/body[1]/div[0] finds no "section" ancestor`}},
	})
	gt.Run("description", func(gt *testing.T) {
		t := devtest.ParallelT(gt)
		t.Must(be.Equal(`hxTargets("main", "alerts")`, webuitest.HXTargets("main", "alerts").String()))
	})
}

func TestSSEWiring(gt *testing.T) {
	checkRule(gt, webuitest.SSEWiring(), []ruleCase{
		{name: "valid", body: `<div hx-ext="sse" sse-connect="/live" sse-swap="end" sse-close="end"><p sse-swap="tick, tock"></p><button type="button" hx-get="/x" hx-trigger="sse:tick">r</button></div><section hx-ext="morph, sse"><div data-sse-connect="/b" data-sse-close="end"></div></section>`},
		{name: "outside", body: `<p sse-swap="tick"></p>`, want: []string{"sse-swap at html[1]/body[1]/p[0] is outside every sse-connect element"}},
		{name: "no extension", body: `<div sse-connect="/live" sse-close="end"></div>`, want: []string{`sse-connect at html[1]/body[1]/div[0] is not inside hx-ext="sse"`}},
		{name: "ignored extension", body: `<div hx-ext="sse"><div hx-ext="ignore:sse" sse-connect="/live" sse-close="end"></div></div>`, want: []string{`is not inside hx-ext="sse"`}},
		{name: "no close", body: `<div hx-ext="sse" sse-connect="/live"></div>`, want: []string{"sse-connect at html[1]/body[1]/div[0] has no sse-close event, so the browser reconnects forever"}},
		{name: "empty close", body: `<div hx-ext="sse" sse-connect="/live" sse-close=" "></div>`, want: []string{"has no sse-close event"}},
		{name: "no url", body: `<div hx-ext="sse" sse-connect="" sse-close="end"></div>`, want: []string{"has no URL"}},
		{name: "empty event", body: `<div hx-ext="sse" sse-connect="/l" sse-close="end"><p sse-swap="a,,b"></p></div>`, want: []string{"sse-swap at html[1]/body[1]/div[0]/p[0] names an empty event"}},
		{name: "trigger outside", body: `<button type="button" hx-get="/x" hx-trigger="load, sse:tick">x</button>`, want: []string{"hx-trigger at html[1]/body[1]/button[0] waits for an SSE event outside every sse-connect element"}},
		{name: "fragment", fragment: true, body: `<p sse-swap="tick"></p><div sse-connect="/x" sse-close="end"></div>`},
		{name: "fragment close", fragment: true, body: `<div sse-connect="/x"></div>`, want: []string{"has no sse-close event"}},
	})
}

func TestHXPreserveIDs(gt *testing.T) {
	checkRule(gt, webuitest.HXPreserveIDs(), []ruleCase{
		{name: "valid", body: `<div id="player" hx-preserve></div><video id="clip-1" data-hx-preserve="true"></video><p id="--x" hx-preserve></p><p id="_y" hx-preserve></p><p id="-z" hx-preserve></p><p id="é" hx-preserve></p><p>no preserve</p>`},
		{name: "no id", body: `<div hx-preserve></div>`, want: []string{"hx-preserve at html[1]/body[1]/div[0] has no id, so htmx keeps no element"}},
		{name: "data prefix", body: `<div data-hx-preserve></div>`, want: []string{"data-hx-preserve at html[1]/body[1]/div[0] has no id"}},
		{name: "empty id", body: `<div id="" hx-preserve="true"></div>`, want: []string{"has no id"}},
		{name: "false is preserved too", body: `<div hx-preserve="false"></div>`, want: []string{"has no id"}},
		{name: "data-id", body: `<div data-id="x" hx-preserve></div>`, want: []string{"has no id"}},
		{name: "digit", body: `<div id="1row" hx-preserve></div>`, want: []string{`hx-preserve at html[1]/body[1]/div#1row[0] has the id "1row", which is not a CSS identifier`}},
		{name: "dot", body: `<div id="a.b" hx-preserve></div>`, want: []string{`the id "a.b"`}},
		{name: "dash digit", body: `<div id="-1" hx-preserve></div>`, want: []string{`the id "-1"`}},
		{name: "space", body: `<div id="a b" hx-preserve></div>`, want: []string{`the id "a b"`}},
		{name: "fragment", fragment: true, body: `<audio hx-preserve></audio>`, want: []string{"hx-preserve at audio[0] has no id"}},
	})
	gt.Run("view", func(gt *testing.T) {
		t := devtest.ParallelT(gt)
		t.Must(ct.View(div.Div(attr.KV("ID", "kept"), attr.KV("HX-Preserve", ""))()).Valid(webuitest.HXPreserveIDs()))
		got := violations(t, ct.View(div.Div(hx2.Preserve())()), webuitest.HXPreserveIDs())
		t.Must(be.Len(got, 1))
	})
}

// dialogPage is a valid page with a no-JS dialog, in the markup of the
// dialog contract.
const dialogPage = `<button type="button" commandfor="d" command="show-modal" popovertarget="d">Open</button>` +
	`<div id="dialogs"><dialog id="d" class="modal" popover aria-labelledby="d-title"><div class="modal-dialog"><div class="modal-content">` +
	`<div class="modal-header"><h2 class="modal-title h5" id="d-title">Title</h2>` +
	`<button class="btn-close" type="button" aria-label="Close" commandfor="d" command="close" popovertarget="d" popovertargetaction="hide" autofocus></button></div>` +
	`<div class="modal-footer"><button type="button" class="btn" commandfor="d" command="request-close" popovertarget="d" popovertargetaction="hide">Cancel</button></div>` +
	`</div></div></dialog></div>`

func mutate(old, replacement string) string {
	if !strings.Contains(dialogPage, old) {
		panic("mutation does not apply: " + old)
	}
	return strings.Replace(dialogPage, old, replacement, 1)
}

func TestModalDialogs(gt *testing.T) {
	checkRule(gt, webuitest.ModalDialogs(), []ruleCase{
		{name: "valid", body: dialogPage},
		{name: "no id", body: `<dialog class="modal" popover aria-label="x"><button type="button" autofocus>x</button></dialog>`, want: []string{"dialog at html[1]/body[1]/dialog[0] has no id"}},
		{name: "no class", body: mutate(`class="modal" popover`, `popover`), want: []string{"dialog at html[1]/body[1]/div#dialogs[1]/dialog#d[0] lacks the class modal"}},
		{name: "no popover", body: mutate(`class="modal" popover`, `class="modal"`), want: []string{"has no popover attribute"}},
		{name: "no name", body: mutate(` aria-labelledby="d-title"`, ``), want: []string{"has no aria-labelledby or aria-label"}},
		{name: "aria-label name", body: mutate(` aria-labelledby="d-title"`, ` aria-label="Edit"`)},
		{name: "open", body: mutate(`popover aria`, `popover open aria`), want: []string{"is rendered open"}},
		{name: "in a form", body: mutate(`<div id="dialogs">`, `<form id="dialogs">`), want: []string{"dialog at html[1]/body[1]/form#dialogs[1]/dialog#d[0] is inside a form"}},
		{name: "no autofocus", body: mutate(` autofocus`, ``), want: []string{"has no autofocus control"}},
		{name: "opener type", body: mutate(`<button type="button" commandfor="d" command="show-modal"`, `<button commandfor="d" command="show-modal"`), want: []string{"commandfor at html[1]/body[1]/button[0] is not on a button of type button"}},
		{name: "opener element", body: mutate(`<button type="button" commandfor="d" command="show-modal" popovertarget="d">Open</button>`, `<a href="#" type="button" commandfor="d" command="show-modal" popovertarget="d">Open</a>`), want: []string{"commandfor at html[1]/body[1]/a[0] is not on a button"}},
		{name: "missing target", body: mutate(`commandfor="d" command="show-modal" popovertarget="d"`, `commandfor="e" command="show-modal" popovertarget="e"`), want: []string{`commandfor at html[1]/body[1]/button[0] targets missing id "e"`, "has no opener"}},
		{name: "not a dialog", body: dialogPage + `<p id="p"></p><button type="button" commandfor="p" command="show-modal" popovertarget="p">x</button>`, want: []string{"targets a p, not a dialog"}},
		{name: "no popovertarget", body: mutate(`command="show-modal" popovertarget="d"`, `command="show-modal"`), want: []string{"commandfor at html[1]/body[1]/button[0] has no popovertarget naming the same dialog"}},
		{name: "other popovertarget", body: mutate(`command="show-modal" popovertarget="d"`, `command="show-modal" popovertarget="x"`), want: []string{"has no popovertarget naming the same dialog"}},
		{name: "opener hides", body: mutate(`popovertarget="d">Open`, `popovertarget="d" popovertargetaction="hide">Open`), want: []string{"opens the dialog but its popovertargetaction hides it"}},
		{name: "closer outside", body: dialogPage + `<button type="button" commandfor="d" command="close" popovertarget="d" popovertargetaction="hide">x</button>`, want: []string{"commandfor at html[1]/body[1]/button[2] closes a dialog it is not inside"}},
		{name: "closer without hide", body: mutate(` popovertargetaction="hide" autofocus`, ` autofocus`), want: []string{`closes the dialog without popovertargetaction="hide"`}},
		{name: "unknown command", body: mutate(`command="request-close"`, `command="toggle"`), want: []string{`has command "toggle", not show-modal, close or request-close`}},
		{name: "popover without commandfor", body: dialogPage + `<button type="button" popovertarget="d">x</button>`, want: []string{`popovertarget at html[1]/body[1]/button[2] opens dialog "d" without commandfor, as a non-modal popover`}},
		{name: "popover of a menu", body: dialogPage + `<button type="button" popovertarget="m">x</button><div id="m" popover>menu</div>`},
		{name: "popover missing", body: dialogPage + `<button type="button" popovertarget="m">x</button>`, want: []string{`popovertarget at html[1]/body[1]/button[2] targets missing id "m"`}},
		{name: "no opener", body: mutate(`<button type="button" commandfor="d" command="show-modal" popovertarget="d">Open</button>`, ``), want: []string{`dialog at html[1]/body[1]/div#dialogs[0]/dialog#d[0] has no opener (command="show-modal")`}},
		{name: "no closer", body: `<button type="button" commandfor="d" command="show-modal" popovertarget="d">Open</button><dialog id="d" class="modal" popover aria-label="x"><input name="a" autofocus></dialog>`, want: []string{`has no close button (command="close")`}},
		{name: "fragment opener", fragment: true, body: `<button type="button" commandfor="d" command="show-modal" popovertarget="d">Open</button>`},
		{name: "fragment dialog", fragment: true, body: strings.TrimPrefix(dialogPage, `<button type="button" commandfor="d" command="show-modal" popovertarget="d">Open</button>`)},
		{name: "fragment checks", fragment: true, body: `<button commandfor="d" command="show-modal">Open</button>`, want: []string{"is not on a button of type button", "has no popovertarget naming the same dialog"}},
	})
	// A dialog that every page renders for the openers that fragments
	// bring needs no opener on the page; the other checks stay.
	noOpener := mutate(`<button type="button" commandfor="d" command="show-modal" popovertarget="d">Open</button>`, ``)
	gt.Run("shared", func(gt *testing.T) {
		checkRule(gt, webuitest.ModalDialogs("d"), []ruleCase{
			{name: "no opener", body: noOpener},
			{name: "with opener", body: dialogPage},
			{name: "other dialog", body: noOpener + `<dialog id="e" class="modal" popover aria-label="E"><button type="button" commandfor="e" command="close" popovertarget="e" popovertargetaction="hide" autofocus>x</button></dialog>`, want: []string{`dialog at html[1]/body[1]/dialog#e[1] has no opener (command="show-modal")`}},
			{name: "other checks", body: strings.Replace(noOpener, ` autofocus`, ``, 1), want: []string{"has no autofocus control"}},
			{name: "no close button", body: `<div id="dialogs"><dialog id="d" class="modal" popover aria-label="D"><input name="a" autofocus></dialog></div>`, want: []string{`has no close button (command="close")`}},
		})
	})
	gt.Run("shared in Rules", func(gt *testing.T) {
		t := devtest.ParallelT(gt)
		page := func() *ct.Subject { return cthtml.PageString(htmlPage(noOpener)) }
		t.Must(be.ErrorContains(page().Valid(webuitest.Rules()...).Check(t.Context()), "has no opener"))
		t.Must(page().Valid(webuitest.Rules(webuitest.WithSharedDialogs("x"), webuitest.WithSharedDialogs("d"))...))
		var desc string
		for _, r := range webuitest.Rules(webuitest.WithSharedDialogs("x", "d")) {
			if webuitest.RuleName(r) == "modalDialogs" {
				desc = r.String()
			}
		}
		t.Must(be.Equal(`modalDialogs("x", "d")`, desc))
		t.Must(be.Equal("modalDialogs()", webuitest.ModalDialogs().String()))
	})
}

// TestModalDialogsMatchBS checks that the dialog of the bs package, its
// opener and its close buttons satisfy the rule, and every rule of the
// bundle: the component and the rule implement one contract.
func TestModalDialogsMatchBS(gt *testing.T) {
	t := devtest.ParallelT(gt)
	view := elem.Bundle{
		bs.DialogButton("edit", attr.Class("btn btn-primary"))(text.Text("Edit")),
		div.Div(attr.ID("dialogs"))(bs.Dialog{
			ID:     "edit",
			Title:  text.Text("Edit the note"),
			Body:   text.P()(text.Text("Body")),
			Footer: bs.BtnSecondary(bs.DialogClose("edit"))(text.Text("Cancel")),
		}),
	}
	t.Must(ct.View(view).Valid(webuitest.Rules()...))
	t.Must(ct.View(view).Valid(webuitest.ModalDialogs()))
}

func TestNoBootstrapJSHooks(gt *testing.T) {
	checkRule(gt, webuitest.NoBootstrapJSHooks(), []ruleCase{
		{name: "valid", body: `<div data-bs-theme="light" data-x="1"></div>`},
		{name: "toggle", body: `<button type="button" data-bs-toggle="modal" data-bs-target="#m">x</button>`, want: []string{"data-bs-toggle at html[1]/body[1]/button[0] needs Bootstrap's script", "data-bs-target at"}},
		{name: "dismiss", body: `<button type="button" data-bs-dismiss="alert">x</button>`, want: []string{"data-bs-dismiss at"}},
	})
	gt.Run("bs.Modal needs it", func(gt *testing.T) {
		t := devtest.ParallelT(gt)
		got := violations(t, ct.View(bs.ModalTrigger("m")(text.Text("Open"))), webuitest.NoBootstrapJSHooks())
		t.Must(be.NonEmpty(got))
	})
}

func TestIconsHidden(gt *testing.T) {
	checkRule(gt, webuitest.IconsHidden(), []ruleCase{
		{name: "valid", body: `<i class="bi bi-x" aria-hidden="true"></i><span aria-hidden="true"><i class="bi bi-y"></i></span><i class="bi bi-z" role="img" aria-label="Warning"></i><span class="bin"></span>`},
		{name: "visible", body: `<i class="bi bi-x"></i>`, want: []string{`icon at html[1]/body[1]/i[0] is neither aria-hidden="true" nor role="img" with a label`}},
		{name: "hidden false", body: `<i class="bi bi-x" aria-hidden="false"></i>`, want: []string{"icon at"}},
		{name: "unnamed image", body: `<i class="bi bi-x" role="img"></i>`, want: []string{"icon at"}},
	})
	gt.Run("bi icons", func(gt *testing.T) {
		t := devtest.ParallelT(gt)
		t.Must(ct.View(elem.Bundle{bi.Alarm, bi.Alarm.Label("Alarm set")}).Valid(webuitest.IconsHidden()))
	})
}

func TestControlsNamed(gt *testing.T) {
	checkRule(gt, webuitest.ControlsNamed(), []ruleCase{
		{name: "valid", body: `<button type="button">Save</button><button type="button" aria-label="Close"><i class="bi bi-x" aria-hidden="true"></i></button><label>Name <input name="n"></label><label for="e">Email</label><input id="e" name="e"><input type="submit"><input type="hidden" name="h"><a href="/x">Home</a><a>no href</a><select aria-label="Size"><option>S</option></select><textarea title="Notes"></textarea><div role="button" aria-label="Go"></div><span id="l">Label</span><input aria-labelledby="l">`},
		{name: "icon button", body: `<button type="button"><i class="bi bi-x" aria-hidden="true"></i></button>`, want: []string{"button at html[1]/body[1]/button[0] has no accessible name"}},
		{name: "empty link", body: `<a href="/x"></a>`, want: []string{"a at html[1]/body[1]/a[0] has no accessible name"}},
		{name: "unlabelled input", body: `<input name="q">`, want: []string{"input at"}},
		{name: "image input", body: `<input type="image" src="/a.png">`, want: []string{"input at"}},
		{name: "select", body: `<select name="s"><option>a</option></select>`, want: []string{"select at"}},
		{name: "role", body: `<div role="checkbox"></div>`, want: []string{"div at html[1]/body[1]/div[0] has no accessible name"}},
		{name: "broken reference", body: `<button type="button" aria-labelledby="gone">x</button>`, want: []string{"button at html[1]/body[1]/button[0] has a name that cannot be computed"}},
	})
}

func TestNoNestedInteractive(gt *testing.T) {
	checkRule(gt, webuitest.NoNestedInteractive(), []ruleCase{
		{name: "valid", body: `<a href="/x"><span>t</span></a><button type="button"><i class="bi" aria-hidden="true"></i> t</button><a><button type="button">b</button></a><label>x <input name="a"></label>`},
		{name: "button in link", body: `<a href="/x"><button type="button">b</button></a>`, want: []string{"button at html[1]/body[1]/a[0]/button[0] is inside the a at html[1]/body[1]/a[0]"}},
		{name: "input in button", body: `<button type="button"><input name="x"></button>`, want: []string{"input at html[1]/body[1]/button[0]/input[0] is inside the button at"}},
		{name: "tabindex", body: `<a href="/x"><span tabindex="0">t</span></a>`, want: []string{"span at"}},
		{name: "link in button", body: `<button type="button"><a href="/x">x</a></button>`, want: []string{"a at"}},
		{name: "deep", body: `<a href="/x"><span><select aria-label="s"></select></span></a>`, want: []string{"select at"}},
		{name: "hidden input", body: `<button type="button"><input type="hidden" name="x"></button>`},
	})
}

func TestRectangularTables(gt *testing.T) {
	checkRule(gt, webuitest.RectangularTables(), []ruleCase{
		{name: "valid", body: `<table><thead><tr><th>a</th><th>b</th><th>c</th></tr></thead><tbody><tr><td colspan="2">ab</td><td>c</td></tr><tr><td rowspan="2">a</td><td>b</td><td>c</td></tr><tr><td>b</td><td>c</td></tr></tbody><tfoot><tr><td colspan="3">f</td></tr></tfoot></table>`},
		{name: "no head", body: `<table><tr><td>a</td><td>b</td></tr><tr><td>a</td><td>b</td></tr></table>`},
		{name: "short row", body: `<table><thead><tr><th>a</th><th>b</th><th>c</th></tr></thead><tbody><tr><td>a</td><td>b</td></tr></tbody></table>`, want: []string{"row at html[1]/body[1]/table[0]/tbody[1]/tr[0] spans 2 columns, the first row at html[1]/body[1]/table[0]/thead[0]/tr[0] spans 3"}},
		{name: "rowspan overflow", body: `<table><thead><tr><th>a</th><th>b</th></tr></thead><tbody><tr><td rowspan="2">a</td><td>b</td></tr><tr><td>a</td><td>b</td></tr></tbody></table>`, want: []string{"spans 3 columns"}},
		{name: "rowspan zero", body: `<table><thead><tr><th>a</th><th>b</th></tr></thead><tbody><tr><td rowspan="0">a</td><td>b</td></tr><tr><td>b</td></tr><tr><td>b</td></tr></tbody></table>`},
		{name: "rowspan stays in group", body: `<table><thead><tr><th rowspan="3">a</th><th>b</th></tr></thead><tbody><tr><td>a</td><td>b</td></tr></tbody></table>`},
		{name: "wide colspan", body: `<table><tr><td>a</td></tr><tr><td colspan="99999999">x</td></tr></table>`, want: []string{"spans 1000 columns"}},
	})
}

func TestExternalLinksSafe(gt *testing.T) {
	checkRule(gt, webuitest.ExternalLinksSafe(), []ruleCase{
		{name: "valid", body: `<a href="https://x.example" target="_blank" rel="noopener noreferrer">x</a><a href="/y" target="_self">y</a><a href="https://z.example" target="_blank" rel="NoReferrer">z</a><a href="/w">w</a>`},
		{name: "blank", body: `<a href="https://x.example" target="_blank">x</a>`, want: []string{`a at html[1]/body[1]/a[0] opens a new browsing context without rel="noopener"`}},
		{name: "named", body: `<form target="other"></form>`, want: []string{"form at"}},
		{name: "opener", body: `<a href="/x" target="_BLANK" rel="opener">x</a>`, want: []string{"a at"}},
	})
}

func TestLayoutRegions(gt *testing.T) {
	const valid = `<nav>n</nav><main id="main"><div class="container"><div id="alerts" aria-live="polite"></div><header id="page-header"><h1>T</h1></header></div></main><div id="dialogs"><dialog id="d"></dialog></div>`
	checkRule(gt, webuitest.LayoutRegions(), []ruleCase{
		{name: "valid", body: valid},
		{name: "no main", body: strings.Replace(valid, `<main id="main">`, `<main>`, 1), want: []string{`no element with id "main"`}, some: true},
		{name: "main not main", body: `<div id="main"><div id="alerts" role="status"></div></div><div id="dialogs"></div>`, want: []string{`element with id "main" at html[1]/body[1]/div#main[0] is a div, not main`}},
		{name: "alerts outside", body: `<div id="alerts" aria-live="polite"></div><main id="main"></main><div id="dialogs"></div>`, want: []string{`"alerts" region at html[1]/body[1]/div#alerts[0] is outside main`}},
		{name: "alerts silent", body: strings.Replace(valid, ` aria-live="polite"`, ``, 1), want: []string{`"alerts" region at html[1]/body[1]/main#main[1]/div[0]/div#alerts[0] has no aria-live`}},
		{name: "header outside", body: `<header id="page-header"></header>` + valid[len(`<nav>n</nav>`):], want: []string{`"page-header" at`}, some: true},
		{name: "dialogs in main", body: `<main id="main"><div id="alerts" aria-live="polite"></div><div id="dialogs"></div></main>`, want: []string{`"dialogs" region at html[1]/body[1]/main#main[0]/div#dialogs[1] is inside main`}},
		{name: "dialog elsewhere", body: valid + `<dialog id="e"></dialog>`, want: []string{`dialog at html[1]/body[1]/dialog#e[3] is outside the "dialogs" region`}},
		{name: "fragment", fragment: true, body: `<dialog></dialog>`},
		{name: "navbar and sidebar", body: `<nav id="navbar"></nav><div><nav id="sidebar"></nav></div>` + valid[len(`<nav>n</nav>`):]},
		{name: "navbar not nav", body: `<div id="navbar"></div>` + valid[len(`<nav>n</nav>`):], want: []string{`element with id "navbar" at html[1]/body[1]/div#navbar[0] is a div, not nav`}},
		{name: "sidebar in main", body: strings.Replace(valid, `<div class="container">`, `<div class="container"><nav id="sidebar"></nav>`, 1), want: []string{`"sidebar" at html[1]/body[1]/main#main[1]/div[0]/nav#sidebar[0] is inside main`}},
	})
}

func TestRules(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Run("names", func(t devtest.T) {
		names := map[string]bool{}
		for _, r := range webuitest.Rules() {
			name := webuitest.RuleName(r)
			t.Mustf(be.False(names[name]), "duplicate rule %s", name)
			names[name] = true
		}
		for _, want := range []string{"uniqueIDs", "noRaw", "noInlineScripts", "postFormsCarryCSRF", "noTokenLeaks", "hxTargets", "hxPreserveIDs", "modalDialogs", "noBootstrapJSHooks", "externalLinksSafe"} {
			t.Mustf(be.True(names[want]), "missing rule %s", want)
		}
		t.Must(be.False(names["secretsHidden"]))
	})
	t.Run("options", func(t devtest.T) {
		has := func(rules []ct.Rule, name string) bool {
			for _, r := range rules {
				if webuitest.RuleName(r) == name {
					return true
				}
			}
			return false
		}
		all := webuitest.Rules()
		t.Must(be.False(has(webuitest.Rules(webuitest.WithBootstrapJS()), "noBootstrapJSHooks")))
		without := webuitest.Rules(webuitest.Without("imagesHaveAlt", "controlsNamed", "unknown"))
		t.Must(be.Len(without, len(all)-2))
		t.Must(be.False(has(without, "imagesHaveAlt")))
		secrets := webuitest.Rules(webuitest.WithSecrets("s1"))
		t.Must(be.True(has(secrets, "secretsHidden")))
		t.Must(be.True(has(secrets, "notInURLs")))
		for _, r := range webuitest.Rules(webuitest.WithCSRF(testToken)) {
			t.Must(be.False(strings.Contains(r.String(), testToken)))
		}
		var targets string
		for _, r := range webuitest.Rules(webuitest.WithDocumentTargets("content")) {
			if webuitest.RuleName(r) == "hxTargets" {
				targets = r.String()
			}
		}
		t.Must(be.Equal(`hxTargets("content")`, targets))
	})
	t.Run("a valid page", func(t devtest.T) {
		page := cthtml.PageString(htmlPage(`<main><form method="post" action="/tasks"><input type="hidden" name="csrf" value="` + testToken + `"><label for="t">Title</label><input id="t" name="title"><button type="submit">Add</button></form>` +
			`<div id="list" hx-ext="sse" sse-connect="/tasks/live" sse-swap="end" sse-close="end"><ul sse-swap="tasks"></ul></div>` +
			`<button type="button" hx-post="/tasks/clear" hx-target="#list" hx-vals='{"csrf":"` + testToken + `"}'><i class="bi bi-trash" aria-hidden="true"></i> Clear</button></main>` + dialogPage))
		t.Must(page.Valid(webuitest.Rules(webuitest.WithCSRF(testToken))...))
	})
	t.Run("view attribute case", func(t devtest.T) {
		view := div.Div(attr.KV("HX-On:Click", "x()"), attr.KV("Style", "color: red"))()
		got := violations(t, ct.View(view), webuitest.NoHTMXScripting())
		t.Must(be.Len(got, 1))
		got = violations(t, ct.View(view), webuitest.NoInlineStyles())
		t.Must(be.Len(got, 1))
	})
}

// TestRulesAttributeCase checks that the rules read attributes whose
// names are not lower case: views keep the case in which attributes were
// written, and the HTML parser writes SVG attributes such as viewBox in
// mixed case, so the other attributes of an SVG element are looked up
// case-insensitively too.
func TestRulesAttributeCase(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		name    string
		subject *ct.Subject
		rule    ct.Rule
		want    []string
	}{
		{"view target found", ct.View(elem.Bundle{
			div.Div(attr.KV("ID", "list"))(),
			button.Button(button.Type(button.TypeButton), attr.KV("HX-Get", "/items"), attr.KV("HX-Target", "#list"))(text.Text("Load")),
		}), webuitest.HXTargets(), nil},
		{"view target missing", ct.View(
			button.Button(button.Type(button.TypeButton), attr.KV("Data-HX-Get", "/items"), attr.KV("HX-Target", "#gone"))(text.Text("Load")),
		), webuitest.HXTargets(), []string{`hx-target at button[0] targets missing id "gone"`}},
		{"view inherited target", ct.View(
			div.Div(attr.KV("HX-Target", "find tr"))(button.Button(button.Type(button.TypeButton), attr.KV("HX-Get", "/x"))(text.Text("x"))),
		), webuitest.HXTargets(), []string{`hx-target at div[0], inherited by div[0]/button[0], finds no "tr" descendant`}},
		{"view post without csrf", ct.View(
			button.Button(button.Type(button.TypeButton), attr.KV("HX-Post", "/x"))(text.Text("x")),
		), webuitest.PostFormsCarryCSRF(""), []string{`hx-post at button[0] sends no "csrf" field`}},
		{"view csrf in vals", ct.View(
			div.Div(attr.KV("HX-Vals", `{"csrf":"t"}`))(button.Button(button.Type(button.TypeButton), attr.KV("HX-Post", "/x"))(text.Text("x"))),
		), webuitest.PostFormsCarryCSRF("t"), nil},
		{"view sse", ct.View(
			div.Div(attr.KV("HX-Ext", "sse"), attr.KV("SSE-Connect", "/live"))(div.Div(attr.KV("SSE-Swap", "tick"))()),
		), webuitest.SSEWiring(), []string{"sse-connect at div[0] has no sse-close event"}},
		{"view icon class", ct.View(text.Span(attr.KV("Class", "bi bi-x"))()), webuitest.IconsHidden(), []string{"icon at span[0] is neither"}},
		{"view dialog opener", ct.View(button.Button(attr.KV("CommandFor", "d"), attr.KV("Command", "show-modal"))(text.Text("x"))), webuitest.ModalDialogs(), []string{
			"commandfor at button[0] is not on a button of type button", "commandfor at button[0] has no popovertarget naming the same dialog",
		}},
		{"svg request host", cthtml.PageString(htmlPage(`<svg viewBox="0 0 1 1" hx-get="https://other.example/x"><path d="M0 0"/></svg>`)), webuitest.LocalResources(), []string{"hx-get at html[1]/body[1]/svg[0] names a host"}},
		{"svg post without csrf", cthtml.PageString(htmlPage(`<svg viewBox="0 0 1 1" data-hx-post="/x"></svg>`)), webuitest.PostFormsCarryCSRF(""), []string{`data-hx-post at html[1]/body[1]/svg[0] sends no "csrf" field`}},
		{"svg icon", cthtml.PageString(htmlPage(`<svg viewBox="0 0 1 1" class="bi"></svg><svg viewBox="0 0 1 1" class="bi" aria-hidden="true"></svg>`)), webuitest.IconsHidden(), []string{"icon at html[1]/body[1]/svg[0] is neither"}},
		{"svg style", cthtml.PageString(htmlPage(`<svg viewBox="0 0 1 1" style="fill: red"></svg>`)), webuitest.NoInlineStyles(), []string{"style attribute at html[1]/body[1]/svg[0]"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			got := violations(t, c.subject, c.rule)
			t.Mustf(be.Len(got, len(c.want)), "violations %q", got)
			for i, w := range c.want {
				t.Must(be.Substring(got[i], w))
			}
		})
	}
}

func TestNodeHelpers(gt *testing.T) {
	t := devtest.ParallelT(gt)
	doc, err := cthtml.PageString(htmlPage(`<div id="a" class="x y"><p>one</p><p>two</p></div>`)).Load(t.Context())
	t.Must(be.NoError(err))
	var p ct.Node
	for e := range ct.Elements(doc.Root()) {
		if e.Tag() == "p" {
			p = e
			break
		}
	}
	div := ct.Closest(p, func(n ct.Node) bool { return webuitest.HasClass(n, "y") })
	t.Must(be.True(div != nil && webuitest.AttrIs(div, "ID", "a")))
	t.Must(be.False(webuitest.HasClass(div, "z")))
}

// FuzzRules checks that no rule, contract or form computation panics on
// hostile markup, parsed as a page and as a fragment.
func FuzzRules(f *testing.F) {
	f.Add(dialogPage)
	f.Add(`<form id="f" method="post"><select name="s" size="x"><option selected>a</option></select><textarea name="t">x</textarea><button formmethod="post" hx-include="closest form, #f, [name=s], <#f/>">b</button></form>`)
	f.Add(`<table><tr><td rowspan="0" colspan="1001">a</td></tr><tr><td rowspan="-1">b</td></tr></table><img srcset=",,, x 1x,(,), y" alt="">`)
	f.Add(`<div hx-target="closest" hx-indicator="find [" hx-disabled-elt="next .a, previous #" sse-swap="," hx-trigger="sse:" hx-ext="ignore:sse"></div>`)
	f.Fuzz(func(gt *testing.T, body string) {
		t := devtest.SerialT(gt)
		for _, subject := range []*ct.Subject{cthtml.PageString(htmlPage(body)), cthtml.FragmentString("div", body)} {
			_ = subject.Valid(append(webuitest.Rules(webuitest.WithCSRF("t"), webuitest.WithSecrets("s")), webuitest.LayoutRegions(), webuitest.DefaultShell().Rule(), webuitest.FragmentRule())...).Check(t.Context())
			for _, sel := range []*ct.Selection{subject.Find(ct.Tag("form")).First(), subject.Find(ct.Tag("form")).Last()} {
				_, _ = webuitest.FormData(t.Context(), subject, sel, nil)
			}
		}
	})
}
