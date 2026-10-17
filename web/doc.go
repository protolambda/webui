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
// uses, and scrolls into view, with the real status: webui's htmx
// configuration swaps 4xx and 5xx responses, so the message shows without
// client-side script. A client
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
// # Scrolling and focus
//
// htmx swaps content without moving the window, and content that replaces
// the focused element takes the focus away. Both need care on pages that
// swap parts of themselves.
//
// An error answer scrolls the alerts region into view ([ErrorReswap]). A
// notice that a successful htmx answer swaps into the alerts region out of
// band (page.AlertsOOB) cannot scroll by itself: in htmx 2, hx-swap-oob
// takes a swap style and a selector, not the modifiers of hx-swap (htmx
// reads hx-swap-oob="innerHTML show:top" as the selector "top", finds no
// such element and drops the notice). The main swap of the answer can
// scroll it instead, with the modifier
// show:#alerts:top on the hx-swap of the element that made the request:
//
//	hx2.Swap(hx2.SwapOuterHTML + " show:#" + page.AlertsID + ":top")
//
// or in an HX-Reswap of the answer (hx2.SetReswap), which replaces the
// whole hx-swap of that element, its swap style included. Without it, the
// notice is announced, as the alerts region is a live region, but it may
// stay out of sight.
//
// When a swap removes the element that has the focus, htmx gives the focus
// to the element of the new content with the same id, and restores the
// selection of a text field; without an id the focus is lost to the body,
// and a keyboard user starts over at the top of the page. A form that htmx
// swaps in place, such as one that its handler renders again with
// validation messages, therefore needs an id on its submit button, which
// has the focus after a click in most browsers, and on every control that
// can have the focus when Enter submits the form (bs.Field and the other bs form
// controls have one already). To move the focus elsewhere instead, such as
// to the first invalid field, give that element the autofocus attribute:
// htmx focuses it after the swap. An error answer swaps the alerts region
// only, so the form and its focus stay as they were.
//
// # Speculative requests
//
// Browsers prefetch links. [IsPrefetch] and [MayAct] tell a GET that may
// complete an action (such as a confirmation link) from a speculative or
// HEAD request, which must not.
package web
