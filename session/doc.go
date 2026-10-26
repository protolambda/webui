// Package session keeps server-side browser sessions behind cookies, and
// provides the CSRF tokens of their forms: a synchronizer token per session,
// and a stateless token for anonymous forms such as a login form.
//
// # Sessions
//
// A [Store] holds the live sessions of an application, each with a value of
// the application's type T (an account ID, preferences, a cart).
// Sessions exist only when the application creates one with [Store.New],
// typically at a login: anonymous requests cost the server no state, and no
// number of them can push a live session out of the store. Logins can: the
// store holds at most [Options.MaxSessions] sessions and ends the least
// recently used to make room, so a client that can log in, even to a
// single account, can push the other sessions out by logging in again and
// again. Throttle logins, successful ones included ([Throttle], per client
// and per account), and bound the sessions per account ([Store.Replace];
// see [Options.MaxSessions]).
//
// The cookie carries a session token of 32 bytes from crypto/rand, encoded
// as base64url. The store keys sessions by the SHA-256 digest of the token
// and never keeps the token itself, so a dump of its memory does not reveal
// cookies that could be replayed. [Session.ID] is a short public identifier
// derived from the digest, for logs.
//
// A session ends when the application ends it ([Store.EndSession],
// [Store.End], [Session.End], [Store.EndAll], [Store.EndFunc],
// [Store.Replace]), after [DefaultIdleTTL] without use, at
// [DefaultAbsoluteTTL] after its creation, when [Store.New] creates a new
// session for the same browser (the session ID rotates at login, which
// prevents session fixation), or when the store is full and it is the
// least recently used. When it ends, the
// [Options.OnEnd] hook runs with its value (to clear secrets, for example),
// its value and CSRF token are dropped, and then its [Session.Done] channel
// closes (long requests such as event streams select on it).
// [Session.EndReason] and [Options.OnEndReason] tell why it ended. The
// lifetimes count time the computer spends asleep.
//
// [Store.Rotate] gives a live session a new token and CSRF token and keeps
// its value, for changes of privilege within a session such as a
// re-authentication: the old session ends without the OnEnd hook, which
// runs when the new one ends.
//
// Expired sessions end when a request presents them, or at the next
// [Store.Sweep]. The package starts no goroutines: call Sweep periodically
// (every minute, say) from a goroutine you own, so that the Done channels of
// idle sessions close on time.
//
// Requests for which [Options.Passive] reports true, such as polls and event
// streams, find their session without counting as use of it: a page left
// open with a live region does not keep its session alive forever.
//
// # Request flow
//
// [Store.Middleware] looks up the session of each request's cookie and
// attaches it to the request context, where [FromContext] finds it; a
// request without a live session goes on without one. The middleware never
// sets or clears cookies: only [Store.New] and [Store.Rotate] (the cookie of
// a new session) and [Store.EndSession], [Store.End] or [Store.ClearCookie]
// (its removal) do. The context holds one session per value type, so stores
// mounted on the same routes need distinct value types (see [FromContext]).
// Handler tests can skip the cookie with [NewDetached] and [WithSession].
//
// Mount the middleware innermost, right around the router: after the
// middleware that refuses requests (the host check, the same-origin policy
// and the body limit of package mw), and before the router, so that every
// handler finds the session (see [Store.Middleware]). Without it, or
// [WithSession] in tests, FromContext finds nothing, and a guard built on
// it, such as the authed function of web.Errors.RequireAuth, takes every
// request for a signed-out one.
//
// A logout handler takes its session from the context, checks the form's
// CSRF token against it ([Forms.Read], or [Session.CheckCSRF]), and calls
// [Store.EndSession], which ends the session and clears the cookie (see
// its example). That works the same with a detached session in a test.
// [Store.End] ends the session that the request's cookie names instead,
// and finds nothing when the session came from the context alone.
//
// # Cookies
//
// Cookies are HttpOnly, have Path=/ and no Domain (they go to the exact
// host only), and the session cookie has no Max-Age or Expires: it is a
// browser-session cookie, and the server-side lifetimes above decide when a
// session ends. Unless [Options.Insecure] is set, cookies are Secure and
// their names carry the __Host- prefix ("__Host-session" by default), which
// makes browsers refuse the cookie from plain http and from other hosts, so
// that a sibling subdomain cannot plant or shadow it. Insecure is meant for
// development over plain http on localhost only.
//
// Cookies do not tell ports apart: the applications on one host, such as
// several on localhost or 127.0.0.1 with a port each, share its cookies.
// With the default names ("session" and "preauth"), each one overwrites
// the cookies of the others, which signs their users out and expires
// their forms. Give each application on a shared host names of its own
// ([Options.CookieName], [PreAuthOptions.CookieName]). Distinct names end
// the overwriting, not the sharing: each application still receives the
// cookies of the others and can set them (the __Host- prefix does not
// tell ports apart either), so a host should serve only applications
// that trust each other.
//
// Cookies are SameSite=Lax by default. Lax sends the cookie with top-level
// navigations from other sites (a link in a mail or another application
// opens the page signed in), but not with cross-site POST requests,
// subresources or frames. That adds no cross-site request forgery as long as
// GET requests never change state and every state-changing request passes
// both an origin check (a same-origin policy middleware) and a CSRF token
// check: the token defends even when a browser does send the cookie.
// [Options.StrictSameSite] selects SameSite=Strict, which also withholds the
// cookie from cross-site navigations: links from elsewhere then arrive
// signed out, in exchange for less exposure of GET pages to cross-site
// probing.
//
// # CSRF tokens
//
// Each session has a synchronizer token ([Session.CSRF]): 32 random bytes,
// base64url-encoded. Render it into every state-changing form with
// [CSRFInput], as the hidden field [CSRFField], and check the submitted
// value with [Session.CheckCSRF] (or [CheckCSRF]) before acting on the form.
// The comparison takes constant time for tokens of equal length, and an
// empty expected token never matches. Keep the token out of URLs and GET
// forms: it would leak through history, logs and the Referer header.
//
// A token is fixed for the life of its session (or pre-authentication
// cookie) and is not masked per response. That is safe as long as HTML is
// not compressed: behind a proxy that compresses pages, a page that shows
// a form next to text the request chose (a search form with its query,
// say) lets another site recover the token through the response sizes (a
// BREACH attack). Leave compression off for HTML, as webui does (package
// asset compresses the embedded assets only), or keep such pages free of
// reflected text.
//
// Forms shown before a session exists, such as the login form, use
// [PreAuth]: a signed cookie and a form token derived from it, which need no
// state on the server. See [PreAuth] for the construction and its
// trade-offs. Issue its token with [PreAuth.Issue] when rendering a whole
// page only: fragments that update the page (htmx swaps, polls) read the
// page's token with [PreAuth.Token], because a renewed cookie refuses the
// forms already on the page.
//
// [Forms] puts these rules together for applications with both kinds of
// visitors: [Forms.PageToken] and [Forms.FragmentToken] give the token for
// a page or a fragment (the session's, or the pre-authentication one),
// [Forms.Token] gives the page's token to the updates of an event stream,
// which cannot set cookies, and [Forms.Read] reads a submitted form within
// bounds ([form.Read]) and checks its token against the request's session
// or pre-authentication cookie. [PostForm] renders a POST form with the
// token as its first field. Return the errors of Read unchanged to the
// application's error policy: the Errors policy of package web answers form
// errors with 400, 413 or 415, and [ErrCSRF] with 403, without saying more.
// Forms that are answered again with what was typed when their token
// expired are the exception: read them with [form.Read] and check them with
// [Forms.Check] (see [Forms.Read] and [PreAuth]). After [Store.New] or
// [Store.Rotate], redirect, or attach the new session to the request
// before rendering (see [Forms.Session]). Read forms with package
// form rather than [http.Request.ParseForm] or [http.Request.PostFormValue],
// which accept far larger bodies (up to 10 MB of urlencoded text, more for
// multipart) unless a body limit middleware runs first.
//
// Views whose forms sit deep in their tree need not pass the token down:
// the handler wraps what it renders with [Forms.WithPageToken],
// [Forms.WithFragmentToken] or [Forms.WithToken], stating the kind of
// answer, and the views render [PostFormFromContext], which takes the token
// from the render context when it renders. The token is taken once, for
// the first form, so a page without forms sets no cookie; a form without a
// token fails the render instead of going out with an empty one. A token
// node below another one takes the outer node's token: the outermost node
// decides the one token of a response. The pre-authentication cookie of a
// page or fragment is set during the render, so render into a buffer
// before the header is written, as web.Render does; event streams take
// WithToken, which sets nothing. A fragment that had to issue a new cookie
// renders the fresh copies of the page's other forms within [IfRenewed].
// Error pages render outside the token node of the handler (the Errors
// policy of package web renders them itself): the forms of a layout that
// they share take an explicit token, or WithToken (see
// [PostFormFromContext]).
//
// # Concurrency
//
// [Store], [Session], [PreAuth] and [Forms] are safe for concurrent use. A session's
// value is guarded by the session: [Session.Data] returns a copy, and
// [Session.Update] runs a function with exclusive access to it. The
// callbacks [Options.OnEnd], [Options.OnEndReason] and those given to
// [Session.Update], [Store.EndFunc] and [Store.Each] run while their
// session is locked: they must not call methods of that session or of its
// store, and should return quickly ([Store.Rotate] waits for the session's
// lock while holding the store's).
//
// # Logging
//
// Formatting a [Store], [Session] or [PreAuth] with any verb of package fmt
// prints a short description (a cookie name, a log ID) and never a token,
// a key or a session's value. A copy of one of them formatted by value
// (fmt.Sprintf("%+v", *s), which go vet reports for a Store or Session,
// as they hold a mutex) prints its fields, but secrets stay behind opaque
// handles, including in diagnostic output for unsupported format verbs.
package session
