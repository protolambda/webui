// Package web connects chord views to net/http for server-rendered pages
// with htmx: handlers that return errors, buffered rendering, htmx-aware
// redirects and error answers, an open-redirect guard, an authentication
// guard, and notices for Post/Redirect/Get.
//
// # Handlers and errors
//
// A [HandlerFunc] returns its failure instead of answering it, and an
// [Errors] policy turns it into an http.Handler:
//
//	errs := web.Errors{Document: appErrorPage}
//	mux.Handle("GET /tasks/{id}", errs.Handle(showTask))
//	mux.Handle("/", errs.NotFound())
//	handler := mw.Chain(mux, mw.LogRequests(mw.RequestLog{Routes: mux}), errs.Recover, ...)
//
// A handler returns an [*Error] (see [NotFound], [BadRequest] and the
// other constructors) for an answer with a public message, or any other
// error, which the policy classifies: request-size, form, CSRF and origin
// errors get their 4xx status, an interrupted request 503, and anything
// else 500 with a fixed message. Public messages never contain request
// data; causes are logged, never the path or query (mw.LogRequests with
// RequestLog.Routes tags the records with the route pattern).
//
// A plain request gets a full error page in the application's layout. An
// htmx request, boosted or not, gets an alert that htmx swaps into the
// alerts region of the page (page.AlertsID), whatever hx-select the page
// uses, with the real status: webui's htmx configuration swaps 4xx and 5xx
// responses, so the message shows without client-side script. A client
// that went away gets nothing, while a request whose context ended on the
// server's side, such as past the deadline of a timeout middleware, still
// gets its answer (503 for the context error).
//
// # Rendering
//
// [Render] renders a view into a buffer before it writes anything, so a
// failing view never produces half a page. [Choose] picks the fragment for
// htmx requests and the full page otherwise, and sets Vary: HX-Request,
// HX-Boosted. [WantsFragment] reports whether a request wants a fragment:
// an htmx request ([IsHTMX]) that is not boosted, since htmx swaps a
// boosted answer into the whole body, as it does for history restore
// requests, which IsHTMX excludes. htmx headers are set by the client: they
// choose a representation, never an authorization.
//
// # Redirects
//
// [Redirect] ends a successful form post with 303 See Other, or with
// HX-Redirect for htmx requests (which would otherwise follow a 303 inside
// XMLHttpRequest and swap the target page into a fragment). [SafeNext]
// checks a redirect target that comes from a request, such as a "next"
// parameter, and accepts local paths only. [Errors.RequireAuth] sends
// unauthenticated requests to the login page, with the page to return to.
//
// # Notices
//
// [Notices] maps short codes to fixed messages: an action redirects with
// the code of its outcome in the query, and the page shows the registered
// message. Nothing from the URL is reflected into the page.
//
// # Speculative requests
//
// Browsers prefetch links. [IsPrefetch] and [MayAct] tell a GET that may
// complete an action (such as a confirmation link) from a speculative or
// HEAD request, which must not.
package web
