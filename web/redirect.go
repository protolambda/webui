package web

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/internal/urlpath"
)

// Redirect answers r with a redirect to target, after a successful form
// post (Post/Redirect/Get) or to send the user elsewhere.
//
// A plain request gets 303 See Other with Location: the browser loads the
// target with a GET, and a reload does not post the form again. An htmx
// request ([IsHTMX]) gets 200 with HX-Redirect: htmx then loads the target
// as a full page. A 303 would not work there, because the browser's
// XMLHttpRequest follows it by itself and htmx would swap the target page
// into the request's target element.
//
// A browser that navigates to a fragment of the page it shows (from
// /tasks to /tasks#new) only scrolls, without loading the page again. When
// the target is such a fragment of the current page, which htmx sends as
// HX-Current-URL, the htmx answer also carries HX-Refresh, and htmx
// reloads the page after the navigation.
//
// Pass target as an absolute path, such as "/tasks#new", or an absolute
// URL. The two answers resolve a relative target against different URLs:
// http.Redirect against the directory of the request's path, and htmx
// against the page that the browser shows, which the server does not know
// for a plain request. So "#new", posted to /tasks/7/done from the page
// /tasks, would end at /tasks/7/#new without htmx and at /tasks#new with
// htmx.
//
// A boosted request (HX-Boosted) is an htmx request: HX-Redirect makes
// htmx load the target as a full page, as for other htmx requests.
//
// Both answers add HX-Request to Vary. Redirect does not check target:
// pass a value from the request (such as a "next" parameter) through
// [SafeNext] first.
func Redirect(w http.ResponseWriter, r *http.Request, target string) {
	h := w.Header()
	hx2.AddVary(h)
	if IsHTMX(r) {
		hx2.SetRedirect(h, target)
		if sameDocument(hx2.ParseRequestHeaders(r.Header).CurrentURL, target) {
			hx2.SetRefresh(h)
		}
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// sameDocument reports whether target has a fragment and otherwise is the
// current URL, so that a browser navigates to it by scrolling only. The
// current URL must be absolute (as htmx sends it). A relative target is
// resolved against it, as the browser resolves HX-Redirect.
func sameDocument(current, target string) bool {
	if !strings.Contains(target, "#") {
		return false
	}
	cur, err := url.Parse(current)
	if err != nil || !cur.IsAbs() {
		return false
	}
	dst, err := url.Parse(target)
	if err != nil {
		return false
	}
	dst = cur.ResolveReference(dst)
	return strings.EqualFold(dst.Scheme, cur.Scheme) && strings.EqualFold(dst.Host, cur.Host) &&
		dst.EscapedPath() == cur.EscapedPath() && dst.RawQuery == cur.RawQuery && dst.ForceQuery == cur.ForceQuery
}

// MaxNextLen is the longest value that [SafeNext] accepts, in bytes.
const MaxNextLen = 2048

// SafeNext returns value when it is a safe post-action destination, and
// fallback otherwise. A safe destination is a local absolute path, with an
// optional query and fragment: it starts with a single "/", has no scheme,
// host or user information, is at most [MaxNextLen] bytes, and consists of
// printable ASCII other than space and backslash.
//
// Use it for every redirect target that comes from a request, such as a
// "next" query parameter or form field, so that a crafted link cannot send
// the user to another site after logging in (an open redirect). The
// character rules matter: browsers read a backslash as a slash and drop
// tabs and newlines inside URLs, and net/http cleans the path of a
// redirect, so "/\host", "/<TAB>/host" or "/./\host" could each turn into
// "//host", which is another host. Non-ASCII characters must arrive
// percent-encoded, as browsers send them.
func SafeNext(value, fallback string) string {
	if len(value) > MaxNextLen || !urlpath.Local(value) {
		return fallback
	}
	return value
}

// WithParam returns target with the query parameter key set to value:
// earlier values of key are removed, other parameters keep their order and
// encoding, and a fragment stays at the end. The key and value are
// query-escaped.
//
//	WithParam("/tasks?page=2#list", "notice", "saved") == "/tasks?page=2&notice=saved#list"
func WithParam(target, key, value string) string {
	base, fragment, hasFragment := strings.Cut(target, "#")
	path, query, _ := strings.Cut(base, "?")
	var b strings.Builder
	b.Grow(len(target) + len(key) + len(value) + 2)
	b.WriteString(path)
	b.WriteByte('?')
	for part := range strings.SplitSeq(query, "&") {
		if part == "" {
			continue
		}
		name, _, _ := strings.Cut(part, "=")
		if decoded, err := url.QueryUnescape(name); err == nil && decoded == key {
			continue
		}
		b.WriteString(part)
		b.WriteByte('&')
	}
	b.WriteString(url.QueryEscape(key))
	b.WriteByte('=')
	b.WriteString(url.QueryEscape(value))
	if hasFragment {
		b.WriteByte('#')
		b.WriteString(fragment)
	}
	return b.String()
}
