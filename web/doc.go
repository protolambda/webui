// Package web connects chord views to net/http for server-rendered pages
// with htmx: handlers that return errors, buffered rendering, htmx-aware
// redirects and error answers, a safety net that keeps plain-text errors
// out of htmx swaps, an open-redirect guard, an authentication guard, and
// notices for Post/Redirect/Get.
//
// # Handlers and errors
//
// A [HandlerFunc] returns its failure instead of answering it, and an
// [Errors] policy turns it into an http.Handler:
//
//	errs := web.Errors{Document: appErrorPage}
//	mux.Handle("GET /tasks/{id}", errs.Handle(showTask))
//	mux.Handle("/", errs.NotFound())
//	handler := mw.Chain(mux, mw.LogRequests(mw.RequestLog{Routes: mux}), errs.Recover, errs.Intercept, ...)
//
// A handler returns an [*Error] (see [NotFound], [BadRequest] and the
// other constructors) for an answer with a public message, or any other
// error, which the policy classifies: request-size, form, CSRF and origin
// errors get their 4xx status, an interrupted request 503, and anything
// else 500 with a fixed message. Public messages never contain request
// data; causes are logged, never the path or query (mw.LogRequests with
// RequestLog.Routes tags the records with the route pattern). A form
// handler returns the errors of form.Read or session.Forms.Read as they are
// (see the form example of [Errors]).
//
// A plain request gets a full error page in the application's layout. An
// htmx request, boosted or not, gets an alert that htmx swaps into the
// alerts region of the page (page.AlertsID), whatever hx-select the page
// uses, and scrolls into view, with the real status: webui's htmx
// configuration swaps 4xx and 5xx responses, so the message shows without
// client-side script. A client that went away gets nothing, while a
// request whose context ended on the server's side, such as past the
// deadline of a timeout middleware, still gets its answer (503 for the
// context error).
//
// [Errors.Recover] answers panics. [Errors.Intercept] answers, for htmx
// requests, the error responses not declared as HTML which code outside
// the policy writes (http.Error, the refusals of mw.HostCheck and
// mw.SameOrigin, the 404 and 405 of http.ServeMux, error bodies without a
// Content-Type), so that their raw text is not swapped into the page as
// markup (see below). It trusts error responses with Content-Type
// text/html, which must not contain request data either.
// [Errors.Answer] makes a handler of an error, for the refusal hooks of
// middleware such as the Deny handler of mw.SameOrigin.
//
// # htmx reads every answer as HTML
//
// htmx inserts the body of every answer that it swaps as HTML, whatever
// its Content-Type, and processes the hx-* attributes in it. Those
// attributes send requests without any script, so a strict
// Content-Security-Policy does not stop them. webui's htmx configuration
// (hx2.Config) swaps 4xx and 5xx answers as well, which is how error alerts
// show without script. A plain-text error such as
//
//	http.Error(w, err.Error(), http.StatusBadRequest)
//
// whose err quotes the request, as the errors of strconv, time.Parse and
// url.Parse do, is inert in a browser tab (text/plain, nosniff) but becomes
// markup in the page when htmx made the request. An element that an
// attacker gets into such a request, for example through a link that
// prefills a field of a form that a load trigger submits, such as
//
//	<i hx-post="/archive" hx-include="[name=csrf]" hx-trigger="load"></i>
//
// then sends a same-origin request with the page's CSRF token, which
// passes the same-origin and CSRF checks.
//
// On routes that htmx requests, never write request data into an error
// body: return errors to an [Errors] policy, whose messages never contain
// request data. [Errors.Intercept] is the safety net for code that writes
// its own error responses: for htmx requests it replaces 4xx and 5xx
// responses that are not declared text/html, with another Content-Type or
// none, by the policy's alert of their status. It trusts text/html; see
// its documentation for the exact rules. hx2.Config.NoErrorSwap turns
// error swapping off for pages whose htmx requests reach handlers that
// neither covers. Successful answers are swapped as HTML under any
// configuration: render them with chord, which escapes text.
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
// show:#alerts:top (page.ShowAlerts) on the hx-swap of the element that
// made the request:
//
//	hx2.Swap(hx2.SwapOuterHTML + " " + page.ShowAlerts)
//
// or in an HX-Reswap of the answer (hx2.SetReswap), which replaces the
// whole hx-swap of that element, its swap style included. Without it, the
// notice is announced, as the alerts region is a live region, but it may
// stay out of sight. The modifier only scrolls when the main swap leaves
// content in the page: an answer that removes its target (an outerHTML
// answer with only out-of-band parts, or the delete style) should target a
// container that it renders again, retarget one (hx2.SetRetarget), or use
// page.ShowWindowTop.
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
// While a modal dialog is open, the page behind it is inert: an error
// answer to a form inside the dialog must target an alerts region inside
// the dialog, or the alert is neither seen nor announced. See "Forms in
// dialogs" at [Errors] and its TargetFor field. An htmx answer does not
// close the dialog either: a successful one closes it by replacing the
// dialogs region out of band (page.DialogsOOB), and gives the focus to an
// element of the new content (page.FocusTarget), since a removed dialog
// does not return it to its opener.
//
// # Speculative requests
//
// Browsers prefetch links. [IsPrefetch] and [MayAct] tell a GET that may
// complete an action (such as a confirmation link) from a speculative or
// HEAD request, which must not.
package web
