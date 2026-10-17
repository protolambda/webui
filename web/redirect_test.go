package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthttp"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/page"
	"github.com/protolambda/webui/sse"
	"github.com/protolambda/webui/web"
)

func redirectTo(target string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		web.Redirect(w, r, target)
	})
}

func TestRedirectPlain(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, method := range []string{http.MethodPost, http.MethodGet, http.MethodHead} {
		res := cthttp.Serve(redirectTo("/tasks?page=2#new"), request(t, nil, method, "/tasks", nil))
		t.Must(res.Status(http.StatusSeeOther))
		t.Must(res.Header("Location", ct.Exact("/tasks?page=2#new")))
		t.Must(res.Header("Vary", ct.Exact("HX-Request")))
		t.Must(res.HeaderAbsent("HX-Redirect"))
		t.Must(res.HeaderAbsent("HX-Refresh"))
	}
}

func TestRedirectHTMX(gt *testing.T) {
	t := devtest.ParallelT(gt)
	res := cthttp.Serve(redirectTo("/notes"), asHTMX(request(t, nil, http.MethodPost, "/tasks", nil), "https://app.example/tasks"))
	t.Must(res.Status(http.StatusOK))
	t.Must(res.Header("HX-Redirect", ct.Exact("/notes")))
	t.Must(res.HeaderAbsent("HX-Refresh"))
	t.Must(res.HeaderAbsent("Location"))
	t.Must(res.Header("Vary", ct.Exact("HX-Request")))
	t.Must(res.Body(ct.Exact("")))
}

// TestRedirectSameDocument checks when an htmx redirect also refreshes: a
// target that is a fragment of the current page only scrolls.
func TestRedirectSameDocument(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		current, target string
		refresh         bool
	}{
		{"https://app.example/tasks", "/tasks#new", true},
		{"https://app.example/tasks", "#new", true},
		{"https://app.example/tasks#old", "#new", true},
		{"https://app.example/tasks", "https://app.example/tasks#new", true},
		{"https://app.example/tasks", "https://APP.example/tasks#new", true},
		{"https://app.example/tasks?x=1", "?x=1#new", true},
		{"https://app.example/tasks?x=1", "/tasks?x=1#new", true},
		{"https://app.example/tasks", "/tasks", false},
		{"https://app.example/tasks", "/tasks#", true},
		{"https://app.example/tasks?x=1", "/tasks#new", false},
		{"https://app.example/tasks", "/tasks?x=1#new", false},
		{"https://app.example/tasks", "/notes#new", false},
		{"https://app.example/tasks", "https://other.example/tasks#new", false},
		{"https://app.example/tasks", "http://app.example/tasks#new", false},
		{"https://app.example/a/tasks", "tasks#new", true},
		{"/tasks", "/tasks#new", false},
		{"", "/tasks#new", false},
		{"https://app.example/tasks", "/ta%zz#new", false},
		{"%zz", "/tasks#new", false},
	}
	for _, c := range cases {
		res := cthttp.Serve(redirectTo(c.target), asHTMX(request(t, nil, http.MethodPost, "/x", nil), c.current))
		t.Mustf(res.Header("HX-Redirect", ct.Exact(c.target)), "%q from %q", c.target, c.current)
		if c.refresh {
			t.Mustf(res.Header("HX-Refresh", ct.Exact("true")), "%q from %q", c.target, c.current)
		} else {
			t.Mustf(res.HeaderAbsent("HX-Refresh"), "%q from %q", c.target, c.current)
		}
	}
}

func TestRedirectHistoryRestoreIsPlain(gt *testing.T) {
	t := devtest.ParallelT(gt)
	r := asHTMX(request(t, nil, http.MethodGet, "/old", nil), "https://app.example/old")
	r.Header.Set("HX-History-Restore-Request", "true")
	res := cthttp.Serve(redirectTo("/new"), r)
	t.Must(res.Status(http.StatusSeeOther))
	t.Must(res.HeaderAbsent("HX-Redirect"))
}

// safeNextCases returns values that SafeNext accepts and refuses.
func safeNextCases() (good, bad []string) {
	good = []string{
		"/",
		"/tasks",
		"/tasks/",
		"/tasks?page=2",
		"/tasks?next=%2F%2Fevil.example",
		"/tasks#new",
		"/a/b/../c",
		"/./tasks",
		"/.//evil.example",
		"/%2F%2Fevil.example",
		"/%5Cevil.example",
		"/caf%C3%A9",
		"/a:b",
		"/tasks?q=a+b&x=%20",
		"/" + strings.Repeat("a", web.MaxNextLen-1),
	}
	bad = []string{
		"",
		"tasks",
		"./tasks",
		"../tasks",
		"?x=1",
		"#top",
		"//evil.example",
		"//evil.example/tasks",
		"///evil.example",
		"/\\evil.example",
		"\\\\evil.example",
		"/./\\evil.example",
		"\\/evil.example",
		"/tasks?q=%zz",
		"/tasks?q=100%",
		"/\t/evil.example",
		"/\n/evil.example",
		"/\r\n/evil.example",
		"/\x00",
		"/\x7f",
		"/ /evil.example",
		" /tasks",
		"/tasks ",
		"https://evil.example/",
		"http:/evil.example",
		"https:evil.example",
		"javascript:alert(1)",
		"JAVASCRIPT:alert(1)",
		"data:text/html,x",
		"//user@evil.example",
		"/café",
		"/∕evil.example",
		"/%zz",
		"/tasks%",
		"/" + strings.Repeat("a", web.MaxNextLen),
	}
	return good, bad
}

func TestSafeNext(gt *testing.T) {
	t := devtest.ParallelT(gt)
	const fallback = "/home"
	good, bad := safeNextCases()
	for _, v := range good {
		t.Mustf(be.Equal(v, web.SafeNext(v, fallback)), "SafeNext(%q)", v)
	}
	for _, v := range bad {
		t.Mustf(be.Equal(fallback, web.SafeNext(v, fallback)), "SafeNext(%q)", v)
	}
}

// SafeNext, the stream URLs of sse.Region and the favicon of page.Document
// share one rule for a path on the page's own origin (internal/urlpath);
// only SafeNext adds a length limit.
func TestLocalPathRuleIsShared(gt *testing.T) {
	t := devtest.ParallelT(gt)
	good, bad := safeNextCases()
	render := func(n elem.Node) error {
		var b strings.Builder
		return core.Render(t.Context(), n, &b)
	}
	for _, v := range append(good, bad...) {
		if len(v) > web.MaxNextLen {
			continue
		}
		want := web.SafeNext(v, "") != ""
		streamErr := render(div.Div(sse.Region(v))())
		t.Mustf(be.Equal(want, streamErr == nil), "sse.Region(%q): %v", v, streamErr)
		if !want {
			t.Mustf(be.ErrorIs(streamErr, sse.ErrInvalidURL), "sse.Region(%q)", v)
		}
		if v == "" || strings.HasPrefix(v, "data:") {
			continue // the default icon, or a data: URL, which a favicon may be
		}
		iconErr := render(page.Document{Title: "x", Favicon: v})
		t.Mustf(be.Equal(want, iconErr == nil), "page.Document{Favicon: %q}: %v", v, iconErr)
		if !want {
			t.Mustf(be.ErrorIs(iconErr, page.ErrInvalid), "page.Document{Favicon: %q}", v)
		}
	}
	// Only SafeNext limits the length: stream and icon URLs are written by
	// the application.
	long := "/" + strings.Repeat("a", web.MaxNextLen)
	t.Must(be.Equal("", web.SafeNext(long, "")))
	t.Must(be.NoError(render(div.Div(sse.Region(long))())))
	t.Must(be.NoError(render(page.Document{Title: "x", Favicon: long})))
}

// FuzzSafeNext checks the property that matters: an accepted value, resolved
// like a browser resolves a link on the site, stays on the site.
func FuzzSafeNext(f *testing.F) {
	for _, seed := range []string{"/", "/tasks?x=1#y", "//evil", "/\\evil", "/\t/evil", "/.//evil", "https://evil"} {
		f.Add(seed)
	}
	base, err := url.Parse("https://app.example/dir/page")
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(gt *testing.T, value string) {
		got := web.SafeNext(value, "/fallback")
		if got != value && got != "/fallback" {
			gt.Fatalf("SafeNext returned neither the value nor the fallback: %q", got)
		}
		if got == "/fallback" {
			return
		}
		u, err := url.Parse(got)
		if err != nil {
			gt.Fatalf("accepted %q does not parse: %v", got, err)
		}
		resolved := base.ResolveReference(u)
		if resolved.Host != base.Host || resolved.Scheme != base.Scheme || resolved.User != nil {
			gt.Fatalf("accepted %q leaves the site: %s", got, resolved)
		}
	})
}

func TestWithParam(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct{ target, key, value, want string }{
		{"/tasks", "notice", "saved", "/tasks?notice=saved"},
		{"/tasks?", "notice", "saved", "/tasks?notice=saved"},
		{"/tasks?page=2", "notice", "saved", "/tasks?page=2&notice=saved"},
		{"/tasks?notice=old&page=2", "notice", "saved", "/tasks?page=2&notice=saved"},
		{"/tasks?notice=a&notice=b", "notice", "saved", "/tasks?notice=saved"},
		{"/tasks?%6Eotice=old", "notice", "saved", "/tasks?notice=saved"},
		{"/tasks?b=%zz&&a=1", "notice", "saved", "/tasks?b=%zz&a=1&notice=saved"},
		{"/tasks#list", "notice", "saved", "/tasks?notice=saved#list"},
		{"/tasks?page=2#list?x", "notice", "saved", "/tasks?page=2&notice=saved#list?x"},
		{"/login", "next", "/tasks?x=1&y=2", "/login?next=%2Ftasks%3Fx%3D1%26y%3D2"},
		{"/a", "k y", "v&w", "/a?k+y=v%26w"},
	}
	for _, c := range cases {
		t.Mustf(be.Equal(c.want, web.WithParam(c.target, c.key, c.value)), "WithParam(%q, %q, %q)", c.target, c.key, c.value)
	}
}
