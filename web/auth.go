package web

import (
	"net/http"
	"net/url"

	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/mw"
)

// NextParam is the query parameter of the login URL that carries the page
// to return to after logging in; see [Errors.RequireAuth].
const NextParam = "next"

// RequireAuth returns middleware that passes a request on when authed
// reports true for it, and otherwise sends the user to the login page at
// login, a local path such as "/login":
//
//   - an htmx request ([IsHTMX]) gets 401 with HX-Redirect to
//     login?next=PAGE, where PAGE is the path and query of the page the user
//     is on (HX-Current-URL), so that an expired session in a polled or
//     submitted fragment moves the whole page to the login;
//   - a GET or HEAD request gets 303 See Other to login?next=URI, where URI
//     is the path and query of the request;
//   - any other request (a form post without htmx) gets a 403 error page
//     with [MessageLogin] through [Errors.Write], because a redirect would
//     lose the submitted form.
//
// The next value is passed through [SafeNext] and left out when it is not
// a safe local path; the login handler must check it with SafeNext again
// before redirecting to it. Answers add HX-Request to Vary.
//
// authed decides only whether to let the request through, e.g. by looking
// up the session cookie; it must not write to the response. Do not put
// the login page or its form behind the middleware, and register
// server-sent event streams outside it: an event stream that is redirected
// to an HTML page fails and reconnects instead of ending.
//
// RequireAuth panics if login is not a local path or authed is nil.
func (e Errors) RequireAuth(login string, authed func(r *http.Request) bool) mw.Middleware {
	if SafeNext(login, "") == "" {
		panic("web: RequireAuth login must be a local path")
	}
	if authed == nil {
		panic("web: RequireAuth with a nil authed func")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authed(r) {
				next.ServeHTTP(w, r)
				return
			}
			h := w.Header()
			switch {
			case IsHTMX(r):
				hx2.AddVary(h)
				hx2.SetRedirect(h, loginURL(login, currentPage(r)))
				w.WriteHeader(http.StatusUnauthorized)
			case r.Method == http.MethodGet || r.Method == http.MethodHead:
				hx2.AddVary(h)
				http.Redirect(w, r, loginURL(login, SafeNext(r.URL.RequestURI(), "")), http.StatusSeeOther)
			default:
				e.Write(w, r, &Error{Status: http.StatusForbidden, Message: MessageLogin})
			}
		})
	}
}

// currentPage returns the path and query of the page that an htmx request
// came from (HX-Current-URL), when it is a safe local path, and "" otherwise.
func currentPage(r *http.Request) string {
	u, err := url.Parse(hx2.ParseRequestHeaders(r.Header).CurrentURL)
	if err != nil || !u.IsAbs() {
		return ""
	}
	return SafeNext(u.RequestURI(), "")
}

// loginURL returns the login URL with the next parameter, or the login URL
// alone when next is empty.
func loginURL(login, next string) string {
	if next == "" {
		return login
	}
	return WithParam(login, NextParam, next)
}
