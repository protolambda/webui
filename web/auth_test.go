package web_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthttp"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/web"
)

// guarded is a page behind RequireAuth that is authenticated when the
// request carries the header X-Test-User.
func guarded(login string) http.Handler {
	authed := func(r *http.Request) bool { return r.Header.Get("X-Test-User") != "" }
	return web.Errors{}.RequireAuth(login, authed)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("secret page"))
	}))
}

func TestRequireAuthPasses(gt *testing.T) {
	t := devtest.ParallelT(gt)
	r := request(t, nil, http.MethodPost, "/tasks", nil)
	r.Header.Set("X-Test-User", "ada")
	res := cthttp.Serve(guarded("/login"), r)
	t.Must(res.Status(http.StatusOK))
	t.Must(res.Body(ct.Exact("secret page")))
	t.Must(res.HeaderAbsent("Vary"))
}

func TestRequireAuthRedirects(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		name, login, method, target, current string
		htmx, boosted                        bool
		status                               int
		location                             string
	}{
		{"get", "/login", http.MethodGet, "/tasks?page=2", "", false, false, http.StatusSeeOther, "/login?next=%2Ftasks%3Fpage%3D2"},
		{"head", "/login", http.MethodHead, "/tasks", "", false, false, http.StatusSeeOther, "/login?next=%2Ftasks"},
		{"root", "/login", http.MethodGet, "/", "", false, false, http.StatusSeeOther, "/login?next=%2F"},
		{"login with query", "/login?mode=short", http.MethodGet, "/tasks", "", false, false, http.StatusSeeOther, "/login?mode=short&next=%2Ftasks"},
		{"scheme relative path", "/login", http.MethodGet, "//evil.example/x", "", false, false, http.StatusSeeOther, "/login"},
		{"htmx", "/login", http.MethodPost, "/tasks/7/done", "https://app.example/tasks?page=2#t7", true, false, http.StatusUnauthorized, "/login?next=%2Ftasks%3Fpage%3D2"},
		{"htmx get", "/login", http.MethodGet, "/live/tasks", "https://app.example/inbox", true, false, http.StatusUnauthorized, "/login?next=%2Finbox"},
		{"htmx without current", "/login", http.MethodGet, "/live/tasks", "", true, false, http.StatusUnauthorized, "/login"},
		{"htmx relative current", "/login", http.MethodGet, "/live/tasks", "/inbox", true, false, http.StatusUnauthorized, "/login"},
		{"htmx hostile current", "/login", http.MethodGet, "/live/tasks", "https://app.example//evil.example/", true, false, http.StatusUnauthorized, "/login"},
		// A boosted link or GET form navigates to the request's URL, like a
		// plain one; a boosted post returns to the page it came from.
		{"htmx boosted", "/login", http.MethodGet, "/tasks?page=2", "https://app.example/", true, true, http.StatusUnauthorized, "/login?next=%2Ftasks%3Fpage%3D2"},
		{"htmx boosted head", "/login", http.MethodHead, "/tasks", "https://app.example/", true, true, http.StatusUnauthorized, "/login?next=%2Ftasks"},
		{"htmx boosted post", "/login", http.MethodPost, "/tasks/7/done", "https://app.example/tasks?page=2", true, true, http.StatusUnauthorized, "/login?next=%2Ftasks%3Fpage%3D2"},
		{"htmx boosted hostile", "/login", http.MethodGet, "//evil.example/x", "https://app.example/", true, true, http.StatusUnauthorized, "/login"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			r := request(t, nil, c.method, c.target, nil)
			if c.htmx {
				asHTMX(r, c.current)
			}
			if c.boosted {
				r.Header.Set(hx2.HeaderBoosted, "true")
			}
			res := cthttp.Serve(guarded(c.login), r)
			t.Must(res.Status(c.status))
			t.Must(res.Header("Vary", ct.Exact("HX-Request")))
			t.Must(res.Header("Vary", ct.Exact("HX-History-Restore-Request")))
			if c.htmx {
				t.Must(res.Header("HX-Redirect", ct.Exact(c.location)))
				t.Must(res.HeaderAbsent("Location"))
			} else {
				t.Must(res.Header("Location", ct.Exact(c.location)))
			}
			t.Must(be.False(strings.Contains(res.BodyString(), "secret page")))
		})
	}
}

func TestRequireAuthRefusesPosts(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		res := cthttp.Serve(guarded("/login"), request(t, nil, method, "/tasks", nil))
		t.Must(res.Status(http.StatusForbidden))
		t.Must(res.HeaderAbsent("Location"))
		t.Must(res.HTML().Find(ct.Role("alert")).Texts(web.MessageLogin))
	}
}
