package web

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/asset"
	"github.com/protolambda/webui/form"
	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/mw"
	"github.com/protolambda/webui/page"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/sse"
)

// Messages of the default classification.
const (
	// MessageForm answers a form that cannot be read.
	MessageForm = "The form is not valid."
	// MessageForged answers a request with an invalid CSRF token or from
	// another origin.
	// The advice works for an htmx alert and for an error page alike: a
	// reload of the error page would post the expired form again.
	MessageForged = "The form expired or came from another site. Open the form again and retry."
	// MessageInterrupted answers a request that failed with
	// context.Canceled or context.DeadlineExceeded, such as one that ran
	// past the deadline that a timeout middleware set on its context.
	MessageInterrupted = "The request was interrupted. Try again."
	// MessageLogin answers an unauthenticated request that cannot be
	// redirected to the login page; see [Errors.RequireAuth].
	MessageLogin = "Log in to continue."
)

// DefaultHomeLabel is the text of the link back on error pages.
const DefaultHomeLabel = "Go to the start page"

// ErrorReswap is the HX-Reswap of htmx error answers unless
// [Errors.Reswap] sets another: the alert replaces the content of the
// alerts region, and htmx then scrolls the region to the top of the
// window (the show:top modifier of hx-swap), so that the alert is seen
// also when the control that failed is further down the page, as it often
// is on a phone.
const ErrorReswap = hx2.SwapInnerHTML + " show:top"

// Errors is an error policy: how handler errors are logged and answered.
// The zero value is a complete policy with webui's defaults. An Errors is a
// value; it is safe for concurrent use as long as its fields are not
// modified meanwhile.
//
// # Classification
//
// An error is classified into a status and a public message, in this order:
//
//   - Classify, when set, maps the application's own errors first;
//   - an [*Error] (also wrapped) gives its status and message;
//   - sse.ErrWrite means the client of a stream went away: nothing is
//     written, and it is logged at debug level only;
//   - context.Canceled and context.DeadlineExceeded give 503 with
//     [MessageInterrupted];
//   - an [*http.MaxBytesError] or form.ErrTooLarge give 413;
//     form.ErrUnsupportedType gives 415, and the other form errors 400 with
//     [MessageForm];
//   - session.ErrCSRF and mw.ErrCrossOrigin give 403 with [MessageForged];
//   - anything else gives 500 with the [DefaultMessage], which never
//     reveals the error.
//
// mw.SameOrigin refuses with a plain-text 403 of its own unless its Deny
// handler answers through the policy, as
// mw.OriginPolicy{Deny: errs.Answer(mw.ErrCrossOrigin)} does ([Errors.Answer]).
// [Errors.Intercept] turns plain-text refusals that reach htmx into the
// alert of their status.
//
// # Answers
//
// A plain request gets a full error page: the Document hook, or by default
// a [page.Document] with the policy's Assets and HTMX configuration around
// [ErrorView.Layout]. An htmx request ([IsHTMX]) gets the alert of
// [ErrorView.Alert] (or the Fragment hook) with HX-Retarget set to the
// alerts region of the page ([page.AlertsID]; see Target and TargetFor),
// HX-Reswap [ErrorReswap] (innerHTML show:top), HX-Reselect unset and
// HX-Push-Url false, keeping the real status: webui's htmx configuration (hx2.Config) swaps 4xx and
// 5xx responses, so the message shows without script, htmx scrolls it into
// view, and the URL does not change. HX-Reselect unset turns off an
// hx-select that the triggering element has or inherits (such as a boosted
// body with hx-select="#main"), which would otherwise filter the alert out
// and empty the alerts region. Pages that make htmx requests must contain
// the alerts region ([page.Layout] renders it): without it htmx reports a
// target error and shows nothing. The element that made the request is
// not swapped: a form keeps what was typed into it, and the focus stays
// where it was. Both answers carry Vary: HX-Request and Cache-Control:
// no-store.
//
// With show:top, htmx scrolls the alerts region to the very top of the
// window, also when it was in view already. A page whose navbar stays in
// place at the top (sticky-top or fixed-top) should give the region a
// scroll-margin-top of the navbar's height in its stylesheet, or the
// navbar covers the alert. See [Errors.Reswap] to swap without scrolling.
//
// A boosted request (HX-Boosted) gets the alert too, although a
// successful one gets a full page (see [Choose]): the user stays on the
// page that made the request, with the error above its content, and a
// boosted form keeps what was typed into it.
//
// htmx inserts every answer that it swaps as HTML, whatever its
// Content-Type, error answers included: see "htmx reads every answer as
// HTML" in the package documentation, and [Errors.Intercept], which keeps
// the plain-text errors of other code out of the page.
//
// Headers that the handler set for the response it meant to send, which
// would contradict the error page, are removed first: the content
// headers (Content-Length, Content-Encoding, Content-Disposition,
// Content-Range, Content-Language, Content-Location), validators (ETag,
// Last-Modified), Location, Refresh, Accept-Ranges, and every htmx
// response header (HX-*). Other headers, such as Set-Cookie and the
// security headers of the middleware, stay.
//
// When an error page fails to render, the answer is the plain-text message
// instead. Nothing is written when the response has already started (see
// [Errors.Handle]), or when the client went away; the error is then logged
// only. The client went away when the request's context was canceled the
// way net/http cancels it on a disconnect: with context.Canceled and no
// cause of its own ([context.Cause]). A context that passed its deadline,
// such as one that a timeout middleware set, or that the application
// canceled with a cause of its own ([context.WithCancelCause]), ended on
// the server's side while the client still waits: it is answered like any
// other error, a context error with 503 and [MessageInterrupted]. The
// answer is then rendered with the context's values but without its
// cancellation, which would stop the render.
//
// # Forms in dialogs
//
// While a modal dialog (bs.Dialog) is open, the rest of the page is inert:
// dimmed under the backdrop and left out of the accessibility tree. An
// alert swapped into the page's alerts region is then neither seen nor
// announced, and the dialog stays open as if nothing happened. An htmx
// form inside a dialog therefore needs an alerts region of its own at the
// top of the dialog's body, such as
// <div id="rename-alerts" aria-live="polite"></div>, rendered with the
// dialog so that it is in the page before an answer arrives, and its error
// answers must target that region. Set TargetFor to map the requests of
// dialog forms to their regions (see the dialog example). It covers every
// answer of the policy, those of [Errors.Recover] and [Errors.Intercept]
// included, and routes that both a dialog and the page post to. For a
// route that only a dialog posts to, a copy of the policy (an Errors is a
// value) with that region as its Target works too, for the errors that its
// handler returns only. The show:top of [ErrorReswap] then scrolls the
// region inside the dialog. An alert stays in the region after the dialog
// closes; a successful answer that leaves the dialog in the page can empty
// the region out of band.
//
// # Logging
//
// Errors are logged to mw.Logger of the request context, with the status
// and the error, never the path or query, which may carry capabilities or
// personal data. The records carry the attributes of that logger:
// mw.LogRequests with RequestLog.Routes set (as in the package example)
// tags them with the method and the route pattern, also for errors written
// by middleware outside the mux and for requests that an inner middleware
// derived. Server errors (5xx) are logged at error level, interrupted
// requests at warn level, and client errors (4xx), departed clients and
// errors after the response started at debug level unless they are server
// errors.
type Errors struct {
	// Classify maps an error to its answer before the default
	// classification. It returns nil for an error it does not know. Its
	// Message must not contain request data.
	Classify func(err error) *Error
	// Document renders the full error page of a plain request, usually the
	// application's [page.Document] around v.Layout() with the
	// application's navbar. Nil means a page.Document with the Assets and
	// HTMX fields below.
	Document func(r *http.Request, v ErrorView) elem.Node
	// Fragment renders the content swapped into Target for an htmx
	// request; htmx swaps all of it, without the hx-select of the page.
	// Nil means v.Alert(). The policy renders it inside
	// page.AlertsContent, as content for a live region already in the
	// page: the page.ErrorAlert and page.Notice values in it render no
	// role of their own, and the region announces them once.
	Fragment func(r *http.Request, v ErrorView) elem.Node
	// Assets and HTMX configure the default error document.
	Assets []*asset.Set
	HTMX   hx2.Config
	// Home is the href of the link back on error pages. Empty means "/".
	Home string
	// Target is the CSS selector that htmx error answers are swapped into:
	// a live region (aria-live) rendered with the page, which announces
	// the alert, as the alerts region of page.Layout does. Empty means
	// "#" + page.AlertsID.
	Target string
	// TargetFor, when set, chooses the target of the htmx error answer to
	// r: the CSS selector of an alerts region that the user sees while
	// making the request, such as one inside the open dialog whose form
	// made it (see "Forms in dialogs" above). An empty result, or a nil
	// TargetFor, means Target. It is called for htmx requests only.
	//
	// It typically maps the id of the element that made the request
	// (HX-Trigger, hx2.RequestHeaders.Trigger) or of its target
	// (HX-Target) to one of a fixed set of the application's selectors.
	// Those headers are set by the client: they choose where a message
	// shows, a presentation, never an authorization. Return a selector of
	// the application's own, not the header's value.
	TargetFor func(r *http.Request) string
	// Reswap is the HX-Reswap of htmx error answers: how htmx swaps the
	// answer into Target, with the syntax of hx-swap. Empty means
	// [ErrorReswap], which scrolls Target into view. A Target that is
	// always in view, such as a container of toasts in a fixed position,
	// can do without the scroll: hx2.SwapInnerHTML, or hx2.SwapAfterBegin
	// to keep the earlier alerts below the new one.
	Reswap string
}

// ErrorView is the view model of an error answer, given to the Document
// and Fragment hooks of [Errors].
type ErrorView struct {
	// Status is the HTTP status of the answer.
	Status int
	// Title is the page title, e.g. "404 Not Found" ([StatusTitle]).
	Title string
	// Message is the public message, a fixed text without request data.
	Message string
	// Home is the href of the link back.
	Home string
}

// Header returns the page header of the error page, with the title as h1.
func (v ErrorView) Header() elem.Node {
	return page.Header{Title: text.Text(v.Title)}
}

// Alert returns the message as a danger alert ([page.ErrorAlert]): with
// role="alert" in the error page, and without a role in the htmx answer,
// which the live region that it is swapped into announces.
func (v ErrorView) Alert() elem.Node {
	return page.ErrorAlert(text.Text(v.Message))
}

// Content returns the main content of the error page: a link back to the
// start page.
func (v ErrorView) Content() elem.Node {
	return text.P()(text.A(text.Href(v.Home))(text.Text(DefaultHomeLabel)))
}

// Layout returns the body of the error page: the header, the alert in the
// alerts region, and the content. Set its Navbar (and other regions)
// before rendering it in a Document hook.
func (v ErrorView) Layout() page.Layout {
	return page.Layout{Header: v.Header(), Alerts: []elem.Node{v.Alert()}, Main: v.Content()}
}

// Handle turns fn into an [http.Handler] that answers the error fn returns
// with [Errors.Write].
//
// The response writer given to fn records whether the response started (a
// final status, a byte, a flush or a hijack), so that an error after that
// is logged only: a half-written page is never followed by an error page.
// The writer implements Unwrap, Flush and Hijack, so
// http.NewResponseController(w) keeps working through it (deadlines for
// streams, for example). Handle panics if fn is nil.
func (e Errors) Handle(fn HandlerFunc) http.Handler {
	if fn == nil {
		panic("web: Handle of a nil HandlerFunc")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := track(w)
		if err := fn(t, r); err != nil {
			e.Write(t, r, err)
		}
	})
}

// Write answers r with the error, as described at [Errors]. A nil err does
// nothing. When w is not (and does not wrap) a writer of [Errors.Handle] or
// [Errors.Recover], Write cannot know whether the response started, and
// assumes it did not.
func (e Errors) Write(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}
	a := e.classify(err)
	gone := clientGone(r.Context())
	level := a.level
	if a.silent || (gone && a.interrupted) {
		level = slog.LevelDebug
	}
	switch {
	case started(w):
		e.log(r, level, "request failed after the response started", a.status, err)
	case a.silent:
		e.log(r, level, "client went away", a.status, err)
	case gone:
		e.log(r, level, "request failed after the client went away", a.status, err)
	default:
		e.log(r, level, "request failed", a.status, err)
		e.answer(w, r, a)
	}
}

// Recover returns a handler that answers a panic of next with a 500 error
// page. It logs the record "handler panic" at error level with the
// attributes panic (the bool true, as in the request record of
// mw.LogRequests), panic_value (the recovered value as text, fmt.Sprint)
// and stack, so that every key has one type in every record. It re-panics
// [http.ErrAbortHandler], which net/http uses to abort a response
// quietly. When the response had started before the panic, it logs and
// then aborts the response with http.ErrAbortHandler, so the client sees a
// broken response rather than a truncated one that looks complete.
//
// Place it inside mw.LogRequests (after it in mw.Chain), so the request
// log records the 500 and the panic log carries the request's logger.
// Recover panics if next is nil.
func (e Errors) Recover(next http.Handler) http.Handler {
	if next == nil {
		panic("web: Recover of a nil handler")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := track(w)
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if v == http.ErrAbortHandler {
				panic(v)
			}
			ctx := r.Context()
			mw.Logger(ctx).LogAttrs(ctx, slog.LevelError, "handler panic",
				slog.Bool("panic", true),
				slog.String("panic_value", fmt.Sprint(v)),
				slog.String("stack", string(debug.Stack())),
			)
			if t.started {
				panic(http.ErrAbortHandler)
			}
			e.answer(t, r, internalAnswer())
		}()
		next.ServeHTTP(t, r)
	})
}

// NotFound returns a handler that answers every request with a 404 error
// page, without naming the path. Register it for the patterns that no
// other route matches, e.g. mux.Handle("/", errs.NotFound()).
func (e Errors) NotFound() http.Handler {
	return e.Answer(&Error{Status: http.StatusNotFound})
}

// Answer returns a handler that answers every request with err, through
// [Errors.Write]: an error page, or the alert for htmx requests. It suits
// the refusal hooks of middleware, such as the Deny handler of
// mw.SameOrigin, which then answers with [MessageForged]:
//
//	mw.SameOrigin(mw.OriginPolicy{Deny: errs.Answer(mw.ErrCrossOrigin)})
//
// The same err answers every request: it is fixed when the handler is
// made, without request data. Answer panics if err is nil.
func (e Errors) Answer(err error) http.Handler {
	if err == nil {
		panic("web: Answer of a nil error")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.Write(w, r, err)
	})
}

// answer is the classification of an error.
type answer struct {
	status  int
	message string
	level   slog.Level
	// silent means nothing is written: the client of a stream went away.
	silent bool
	// interrupted means the error is a context error.
	interrupted bool
}

// clientGone reports whether ctx was canceled the way net/http cancels the
// context of a request whose client went away: with context.Canceled and
// no cause of its own. A context derived from it reports the same. A
// deadline, or a cancellation with a cause of the application's own, ended
// on the server's side, and its client may still wait for the answer.
func clientGone(ctx context.Context) bool {
	return errors.Is(ctx.Err(), context.Canceled) && errors.Is(context.Cause(ctx), context.Canceled)
}

func internalAnswer() answer {
	return answer{
		status:  http.StatusInternalServerError,
		message: DefaultMessage(http.StatusInternalServerError),
		level:   slog.LevelError,
	}
}

func (e Errors) classify(err error) answer {
	if e.Classify != nil {
		if pe := e.Classify(err); pe != nil {
			return errorAnswer(pe)
		}
	}
	var pe *Error
	if errors.As(err, &pe) && pe != nil {
		return errorAnswer(pe)
	}
	var tooLarge *http.MaxBytesError
	switch {
	case errors.Is(err, sse.ErrWrite):
		return answer{level: slog.LevelDebug, silent: true}
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return answer{
			status:      http.StatusServiceUnavailable,
			message:     MessageInterrupted,
			level:       slog.LevelWarn,
			interrupted: true,
		}
	case errors.As(err, &tooLarge), errors.Is(err, form.ErrTooLarge):
		return clientAnswer(http.StatusRequestEntityTooLarge, "")
	case errors.Is(err, form.ErrUnsupportedType):
		return clientAnswer(http.StatusUnsupportedMediaType, "")
	case errors.Is(err, form.ErrMalformed), errors.Is(err, form.ErrTooManyFields), errors.Is(err, form.ErrFile):
		return clientAnswer(http.StatusBadRequest, MessageForm)
	case errors.Is(err, session.ErrCSRF), errors.Is(err, mw.ErrCrossOrigin):
		return clientAnswer(http.StatusForbidden, MessageForged)
	}
	return internalAnswer()
}

// errorAnswer classifies an [*Error].
func errorAnswer(pe *Error) answer {
	status := pe.Status
	if status < 400 || status > 599 {
		status = http.StatusInternalServerError
	}
	a := clientAnswer(status, pe.Message)
	if status >= 500 {
		a.level = slog.LevelError
	}
	return a
}

func clientAnswer(status int, message string) answer {
	if message == "" {
		message = DefaultMessage(status)
	}
	return answer{status: status, message: message, level: slog.LevelDebug}
}

func (e Errors) log(r *http.Request, level slog.Level, msg string, status int, err error) {
	ctx := r.Context()
	attrs := make([]slog.Attr, 0, 2)
	if status != 0 {
		attrs = append(attrs, slog.Int("status", status))
	}
	attrs = append(attrs, slog.Any("err", err))
	mw.Logger(ctx).LogAttrs(ctx, level, msg, attrs...)
}

// target returns the selector that the htmx error answer to r is swapped
// into: TargetFor's choice, Target, or the alerts region of the page.
func (e Errors) target(r *http.Request) string {
	if e.TargetFor != nil {
		if t := e.TargetFor(r); t != "" {
			return t
		}
	}
	if e.Target != "" {
		return e.Target
	}
	return "#" + page.AlertsID
}

// answer writes the error answer of a.
func (e Errors) answer(w http.ResponseWriter, r *http.Request, a answer) {
	home := e.Home
	if home == "" {
		home = "/"
	}
	v := ErrorView{Status: a.status, Title: StatusTitle(a.status), Message: a.message, Home: home}
	h := w.Header()
	resetHeaders(h)
	hx2.AddVary(h)
	h.Set("Cache-Control", "no-store")

	var node elem.Node
	if IsHTMX(r) {
		target := e.target(r)
		reswap := e.Reswap
		if reswap == "" {
			reswap = ErrorReswap
		}
		hx2.SetRetarget(h, target)
		hx2.SetReswap(h, reswap)
		// Swap the whole alert, whatever hx-select the trigger has or
		// inherits: a selection would empty the alerts region.
		hx2.SetReselect(h, "unset")
		hx2.SetPushURL(h, "false")
		if e.Fragment != nil {
			node = e.Fragment(r, v)
		} else {
			node = v.Alert()
		}
		// Target is a live region already in the page, which announces
		// what is swapped into it: an alert with a role of its own would
		// be a second live region inside it.
		node = page.AlertsContent(node)
	} else if e.Document != nil {
		node = e.Document(r, v)
	} else {
		node = page.Document{Title: v.Title, Assets: e.Assets, HTMX: e.HTMX, Body: v.Layout()}
	}

	// The request's context may have passed its deadline, which is often the
	// error being answered: render without its cancellation, which would
	// stop the render, keeping its values.
	ctx := r.Context()
	if ctx.Err() != nil {
		ctx = context.WithoutCancel(ctx)
	}
	var buf bytes.Buffer
	if err := core.Render(ctx, node, &buf); err != nil {
		e.log(r, slog.LevelError, "error page failed to render", a.status, err)
		writePlain(w, r, a.status, a.message)
		return
	}
	h.Set("Content-Type", ContentType)
	w.WriteHeader(a.status)
	_, _ = w.Write(buf.Bytes())
}

// writePlain writes message as a plain-text answer. For htmx, which reads
// every answer as HTML, the message is escaped.
func writePlain(w http.ResponseWriter, r *http.Request, status int, message string) {
	h := w.Header()
	h.Set("Content-Type", "text/plain; charset=utf-8")
	h.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if IsHTMX(r) {
		message = html.EscapeString(message)
	}
	_, _ = io.WriteString(w, message)
}

// replacedHeaders are the headers of the response that a handler meant to
// send that an error answer drops.
var replacedHeaders = [...]string{
	"Content-Length", "Content-Encoding", "Content-Disposition", "Content-Range",
	"Content-Language", "Content-Location", "Etag", "Last-Modified", "Location",
	"Refresh", "Accept-Ranges",
}

// resetHeaders drops the headers of replacedHeaders and every htmx response
// header (HX-*) from h.
func resetHeaders(h http.Header) {
	for _, name := range replacedHeaders {
		h.Del(name)
	}
	for name := range h {
		if len(name) >= 3 && strings.EqualFold(name[:3], "hx-") {
			delete(h, name)
		}
	}
}
