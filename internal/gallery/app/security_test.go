package app_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthttp"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/asset"
	"github.com/protolambda/webui/internal/gallery/app"
	"github.com/protolambda/webui/mw"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/web"
	"github.com/protolambda/webui/webuitest"
)

// httpsHeaders are the security headers that the gallery sends at an
// https site that is not a loopback host, such as webuitest.BaseURL:
// webui's defaults, with HSTS for a year (mw.HSTSFor).
var httpsHeaders = mw.Headers{HSTS: mw.HSTS{MaxAge: mw.DefaultHSTSMaxAge}}

// TestSecurityHeaders checks that every kind of answer carries webui's
// strict security headers, with HSTS at an https site: pages, fragments,
// redirects, error pages, htmx error fragments, the replaced answer of a
// handler outside the error policy, cross-origin refusals and assets.
func TestSecurityHeaders(gt *testing.T) {
	t := devtest.ParallelT(gt)
	g := newGallery(t, app.Options{})
	c := g.anonymous()
	policy := webuitest.PolicyFor(httpsHeaders)
	targets := append(slices.Clone(pages), "/notes", "/no/such/page", "/htmx/tasks?filter=done")
	for _, target := range targets {
		t.Mustf(webuitest.SecurityHeaders(c.Get(t, target), policy), "GET %s", target)
		t.Mustf(webuitest.SecurityHeaders(c.Get(t, target, webuitest.HTMX()), policy), "htmx GET %s", target)
	}
	t.Must(c.Get(t, "/").Header("Strict-Transport-Security", ct.Exact("max-age=31536000")))
	t.Must(webuitest.SecurityHeaders(c.Post(t, "/htmx/fail", webuitest.Form(nil)), policy))
	t.Must(webuitest.SecurityHeaders(c.Post(t, "/htmx/plain-error", webuitest.Form(nil), webuitest.HTMX()), policy))
	t.Must(webuitest.SecurityHeaders(c.Post(t, "/htmx/fail", webuitest.Form(nil), webuitest.FetchSite("cross-site")), policy))
	// Assets replace the default Cache-Control: their URLs change with
	// their content.
	for _, set := range []*asset.Set{plainAssets, jsAssets} {
		for info := range set.Files() {
			t.Mustf(webuitest.SecurityHeaders(c.Get(t, info.URL), webuitest.AssetPolicyFor(httpsHeaders)), "GET %s", info.URL)
		}
	}
	// The host check refuses without HSTS, so that a host name that the
	// client chose is never pinned to https.
	res := c.Get(t, "/", webuitest.Host("evil.example"))
	t.Must(res.Status(http.StatusBadRequest))
	t.Must(webuitest.SecurityHeaders(res, webuitest.StrictPolicy()))
}

// TestRefusalsAnswerThroughPolicy checks that the refusals of the
// middleware reach htmx as the policy's alerts, never as plain text that
// htmx would insert as markup: the same-origin policy answers through the
// error policy (web.Errors.Answer), and web.Errors.Intercept replaces the
// plain-text answers of the host check and of a handler outside the
// policy. (The mux answers no 405: the catch-all route answers through
// the policy with 404.)
func TestRefusalsAnswerThroughPolicy(gt *testing.T) {
	t := devtest.ParallelT(gt)
	g := newGallery(t, app.Options{})
	c := g.signedIn(newAccount("Ada"))
	alert := func(t devtest.T, res *cthttp.Result, status int, message string) {
		t.Helper()
		t.Must(webuitest.HXRetarget(res, ct.Exact("#alerts")))
		t.Must(res.Header("Content-Type", ct.Exact(web.ContentType)))
		frag := c.Fragment(t, res, status, "div")
		t.Must(frag.Find(ct.Class("alert-danger")).Texts(message))
		t.Must(frag.Find(ct.Tag("strong")).None())
	}

	t.Run("cross-origin", func(t devtest.T) {
		res := c.Post(t, "/htmx/fail", webuitest.Form(nil), webuitest.HTMX(), webuitest.FetchSite("cross-site"), webuitest.Origin(webuitest.ForeignOrigin))
		alert(t, res, http.StatusForbidden, web.MessageForged)
		// A plain request gets the gallery's error page.
		page := c.Page(t, c.Post(t, "/htmx/fail", webuitest.Form(nil), webuitest.FetchSite("cross-site")), http.StatusForbidden)
		t.Must(page.Find(ct.ID("alerts")).Find(ct.Role("alert")).Texts(web.MessageForged))
		t.Must(page.Find(ct.ID("navbar")).Count(1))
	})

	t.Run("handler outside the policy", func(t devtest.T) {
		p := c.Page(t, c.Get(t, "/htmx"), http.StatusOK)
		res := c.SubmitHTMX(t, p, p.Find(ct.ID("plain-error-form")), nil, nil)
		alert(t, res, http.StatusBadRequest, web.DefaultMessage(http.StatusBadRequest))
		t.Must(be.False(strings.Contains(res.BodyString(), "Markup")))
		// Without htmx, the plain text stays plain text.
		res = c.Submit(t, p, p.Find(ct.ID("plain-error-form")), nil, nil)
		t.Must(res.Status(http.StatusBadRequest))
		t.Must(res.Header("Content-Type", ct.Prefix("text/plain")))
		t.Must(res.Header("X-Content-Type-Options", ct.Exact("nosniff")))
	})

	t.Run("host check", func(t devtest.T) {
		res := c.Get(t, "/htmx/poll", webuitest.HTMX(), webuitest.Host("evil.example"))
		alert(t, res, http.StatusBadRequest, web.DefaultMessage(http.StatusBadRequest))
	})
}

// TestRejectsCrossSite sends the cross-site probes of webuitest to every
// state-changing route, with a valid CSRF token, so that the same-origin
// policy is tested alone: each is refused, and nothing changes.
func TestRejectsCrossSite(gt *testing.T) {
	t := devtest.ParallelT(gt)
	g := newGallery(t, app.Options{})
	s := newAccount("Ada")
	c := g.signedIn(s)

	// The state that the probes must not change: the task list, the
	// project name, the notes and the session.
	snapshot := func() []byte {
		var b bytes.Buffer
		b.Write(c.Get(t, "/htmx/tasks?filter=all", webuitest.HTMX()).Bytes())
		b.Write(c.Get(t, "/dialogs").Bytes())
		acc, _ := s.Data()
		for _, n := range acc.Notes {
			b.WriteString(n)
		}
		return b.Bytes()
	}
	// A renamed project, so that a reset would change it.
	t.Must(c.Post(t, "/dialogs/rename", webuitest.Form(url.Values{"name": {"Renamed"}})).Status(http.StatusSeeOther))
	before := snapshot()
	unchanged := func() error {
		if s.Ended() {
			return errors.New("the session ended")
		}
		if !bytes.Equal(before, snapshot()) {
			return errors.New("the state changed")
		}
		return nil
	}

	// The probes as htmx requests too, whose refusals are fragments.
	hc := *c
	hc.Options = []webuitest.ReqOption{webuitest.HTMX()}
	for _, route := range []struct {
		target string
		form   url.Values
	}{
		{"/forms", url.Values{"title": {"Cross-site"}, "priority": {"low"}, "visibility": {"team"}}},
		{"/dialogs/rename", url.Values{"name": {"Cross-site"}}},
		{"/dialogs/reset", nil},
		{"/htmx/tasks", url.Values{"title": {"Cross-site"}}},
		{"/htmx/tasks/1/delete", nil},
		{"/htmx/fail", nil},
		{"/notes", url.Values{"note": {"Cross-site"}}},
		{"/logout", nil},
	} {
		t.Mustf(webuitest.RejectsCrossSite(t, c, http.MethodPost, route.target, webuitest.Form(route.form), unchanged), "POST %s", route.target)
		t.Mustf(webuitest.RejectsCrossSite(t, &hc, http.MethodPost, route.target, webuitest.Form(route.form), unchanged), "htmx POST %s", route.target)
	}

	// The same requests from the site itself go through.
	t.Must(c.Post(t, "/htmx/tasks", webuitest.Form(url.Values{"title": {"Same-site"}})).Status(http.StatusSeeOther))
	t.Must(c.Post(t, "/notes", webuitest.Form(url.Values{"note": {"Same-site"}})).Status(http.StatusSeeOther))
	t.Must(be.NotDeepEqual(before, snapshot()))
	// The logout ends the detached session too, which carries no cookie,
	// and clears the session cookie all the same.
	res := c.Post(t, "/logout", webuitest.Form(nil))
	t.Must(webuitest.Location(res, http.StatusSeeOther, ct.Exact("/?notice=signed-out")))
	t.Must(webuitest.Cookie(res, "__Host-session", webuitest.CookieWant{HostPrefix: true, Cleared: true}))
	t.Must(be.True(s.Ended()))
}

// TestRejectsCrossSiteLogin probes the login form with a valid pair of
// pre-authentication cookie and token: none of the probes logs in.
func TestRejectsCrossSiteLogin(gt *testing.T) {
	t := devtest.ParallelT(gt)
	g := newGallery(t, app.Options{})
	b := webuitest.NewBrowser(t, g, webuitest.BrowserOptions{})
	token := webuitest.CSRFToken(t, b.Get(t, "/login").HTML())
	body := webuitest.Form(url.Values{session.CSRFField: {token}, "name": {"Ada"}, "password": {app.DemoPassword}})
	signedOut := func() error {
		if b.Get(t, "/notes").StatusCode() != http.StatusSeeOther {
			return errors.New("signed in")
		}
		return nil
	}
	t.Must(webuitest.RejectsCrossSite(t, b, http.MethodPost, "/login", body, signedOut))
	// The pair itself is valid.
	t.Must(webuitest.Location(b.Post(t, "/login", body), http.StatusSeeOther, ct.Prefix("/notes")))
	t.Must(b.Get(t, "/notes").Status(http.StatusOK))
}
