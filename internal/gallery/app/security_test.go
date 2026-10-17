package app_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"testing"

	"github.com/protolambda/chord/ct"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/asset"
	"github.com/protolambda/webui/internal/gallery/app"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/webuitest"
)

// TestSecurityHeaders checks that every kind of answer carries webui's
// strict security headers: pages, fragments, redirects, error pages, htmx
// error fragments and assets.
func TestSecurityHeaders(gt *testing.T) {
	t := devtest.ParallelT(gt)
	g := newGallery(t, app.Options{})
	c := g.anonymous()
	targets := append(slices.Clone(pages), "/notes", "/no/such/page", "/htmx/tasks?filter=done")
	for _, target := range targets {
		t.Mustf(webuitest.SecurityHeaders(c.Get(t, target), webuitest.StrictPolicy()), "GET %s", target)
		t.Mustf(webuitest.SecurityHeaders(c.Get(t, target, webuitest.HTMX()), webuitest.StrictPolicy()), "htmx GET %s", target)
	}
	t.Must(webuitest.SecurityHeaders(c.Post(t, "/htmx/fail", webuitest.Form(nil)), webuitest.StrictPolicy()))
	// Assets replace the default Cache-Control: their URLs change with
	// their content.
	for _, set := range []*asset.Set{plainAssets, jsAssets} {
		for info := range set.Files() {
			t.Mustf(webuitest.SecurityHeaders(c.Get(t, info.URL), webuitest.AssetPolicy()), "GET %s", info.URL)
		}
	}
	res := c.Get(t, "/", webuitest.Host("evil.example"))
	t.Must(res.Status(http.StatusBadRequest))
	t.Must(webuitest.SecurityHeaders(res, webuitest.StrictPolicy()))
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
