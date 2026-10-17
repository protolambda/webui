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
// again. Throttle logins, successful ones included, and bound the sessions
// per account (see [Options.MaxSessions]).
//
// The cookie carries a session token of 32 bytes from crypto/rand, encoded
// as base64url. The store keys sessions by the SHA-256 digest of the token
// and never keeps the token itself, so a dump of its memory does not reveal
// cookies that could be replayed. [Session.ID] is a short public identifier
// derived from the digest, for logs.
//
// A session ends when the application ends it ([Store.EndSession],
// [Store.End], [Session.End], [Store.EndAll], [Store.EndFunc]), after
// [DefaultIdleTTL] without use, at [DefaultAbsoluteTTL] after its creation,
// when [Store.New] creates a new session for the same browser (the session
// ID rotates at login, which prevents session fixation), or when the store
// is full and it is the least recently used. When it ends, the
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
// A logout handler takes its session from the context, checks the form's
// CSRF token against it, and calls [Store.EndSession], which ends the
// session and clears the cookie (see its example). That works the same
// with a detached session in a test. [Store.End] ends the session that the
// request's cookie names instead, and finds nothing when the session came
// from the context alone.
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
// Forms shown before a session exists, such as the login form, use
// [PreAuth]: a signed cookie and a form token derived from it, which need no
// state on the server. See [PreAuth] for the construction and its
// trade-offs. Issue its token with [PreAuth.Issue] when rendering a whole
// page only: fragments that update the page (htmx swaps, polls) read the
// page's token with [PreAuth.Token], because a renewed cookie refuses the
// forms already on the page.
//
// # Concurrency
//
// [Store], [Session] and [PreAuth] are safe for concurrent use. A session's
// value is guarded by the session: [Session.Data] returns a copy, and
// [Session.Update] runs a function with exclusive access to it. The
// callbacks [Options.OnEnd], [Options.OnEndReason] and those given to
// [Session.Update] and [Store.EndFunc] run while their session is locked:
// they must not call methods of that session or of its store, and should
// return quickly ([Store.Rotate] waits for the session's lock while holding
// the store's).
//
// # Logging
//
// Formatting a [Store], [Session] or [PreAuth] with any verb of package fmt
// prints a short description (a cookie name, a log ID) and never a token,
// a key or a session's value.
package session
