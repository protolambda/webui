package session

import (
	"fmt"
	"net/http"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	htmlform "github.com/protolambda/chord/html/form"

	"github.com/protolambda/webui/form"
)

// errNoTokenSource reports a form check without a session and without a
// [PreAuth] to check the token against.
var errNoTokenSource = fmt.Errorf("%w: no session and no pre-auth", ErrCSRF)

// PostForm returns a POST form to action whose first child is the hidden
// CSRF field of token ([CSRFInput]), followed by the children:
//
//	<form method="post" action="/tasks"><input type="hidden" name="csrf" value="..."/>...</form>
//
// attrs follow method and action (an id, a class, an enctype, htmx
// attributes) and must not set either again. action is a URL of the
// application, not one taken from a request; the renderer escapes it and
// the token like any attribute value. The token comes from
// [Forms.PageToken] or [Forms.FragmentToken], [Session.CSRF] or
// [PreAuth.Issue].
func PostForm(action, token string, attrs ...attr.Node) elem.Scope {
	all := make([]attr.Node, 0, 2+len(attrs))
	all = append(all, htmlform.Method(htmlform.MethodPost), htmlform.Action(action))
	all = append(all, attrs...)
	scope := htmlform.Form(all...)
	return func(children ...elem.Node) elem.Node {
		nodes := make([]elem.Node, 0, 1+len(children))
		nodes = append(nodes, CSRFInput(token))
		nodes = append(nodes, children...)
		return scope(nodes...)
	}
}

// Forms hands out and checks the CSRF tokens of an application's forms, for
// visitors with a session of type T and for visitors without one: the token
// of a form is the session's ([Session.CSRF]) when the visitor has a
// session, and a pre-authentication token ([PreAuth]) otherwise.
//
//   - [Forms.PageToken] gives the token for the forms of a whole page,
//   - [Forms.FragmentToken] the token for the forms of a fragment that
//     updates a page (an htmx swap, a poll), which must match the page's,
//   - [Forms.Token] the page's token without setting anything, for the
//     updates of an event stream,
//   - [Forms.Read] reads a submitted form within Limits and checks its
//     token, and [Forms.Check] checks a token read some other way.
//
// Render the token with [PostForm] (or [CSRFInput] in a form of one's own).
// A visitor with a session must submit the session's token, and a visitor
// without one the token of the pre-authentication cookie: a form rendered
// before a login in another tab, or before a logout, is refused, and the
// visitor reloads it.
//
// The zero Forms checks sessions attached to the request context only and
// refuses every form of visitors without a session. Forms holds no state of
// its own: it is safe to copy and for concurrent use.
type Forms[T any] struct {
	// Store is the store of the sessions, for requests that carry no
	// session of type T in their context (see [Forms.Session]). Nil means
	// the context alone counts.
	Store *Store[T]
	// PreAuth protects the forms of visitors without a session. Nil means
	// such visitors get the empty token, which every check refuses: the
	// application has no forms for anonymous visitors.
	PreAuth *PreAuth
	// Limits bounds the forms that [Forms.Read] reads ([form.Read]); the
	// zero Limits gives package form's defaults. A negative limit is a
	// programming error and makes Read panic; check limits from
	// configuration with [form.Limits.Validate] at startup.
	Limits form.Limits
}

// Session returns the live session of r that the tokens of Forms belong
// to. That is the session attached to r's context, as [FromContext] finds
// it ([Store.Middleware] attaches it, and tests attach one with
// [WithSession]). Only when no session of type T is attached at all, as on
// a route without the middleware, and Store is set, is it the session that
// r's cookie names ([Store.Lookup], which counts as use of the session
// unless [Options.Passive] says otherwise). A nil session attached with
// WithSession hides the cookie's session from Forms as well, and an ended
// one means no session.
//
// Behind [Store.Middleware], Session finds exactly what FromContext finds,
// so a handler that acts for the session from FromContext acts for the
// session whose token [Forms.Read] checked.
//
// After [Store.New] or [Store.Rotate] in the same request, r still holds
// the session the request came with, which has now ended, or none: Session
// finds no session, and the tokens of Forms are pre-authentication ones
// (PageToken may also set a pre-authentication cookie), which the
// browser's new session cookie refuses. Answer such a request with a
// redirect (the usual way after a login), or attach the returned session
// before rendering: r = r.WithContext(WithSession(r.Context(), s)).
func (f Forms[T]) Session(r *http.Request) (*Session[T], bool) {
	if s, attached := r.Context().Value(contextKey[T]{}).(*Session[T]); attached {
		if s == nil || s.Ended() {
			return nil, false
		}
		return s, true
	}
	if f.Store == nil {
		return nil, false
	}
	return f.Store.Lookup(r)
}

// PageToken returns the CSRF token for the forms of a whole page that
// answers r: the token of r's session ([Forms.Session]), or for a visitor
// without one the pre-authentication token of [PreAuth.Issue], which sets
// or renews the pre-authentication cookie on w. Call it before writing the
// response, and only for pages that hold forms, so that pages without forms
// set no cookie.
//
// Do not call it for a fragment that updates part of a page: a renewed
// cookie would refuse the page's other forms. Fragments use
// [Forms.FragmentToken], and event streams [Forms.Token]. After
// [Store.New] or [Store.Rotate] in the same request, see [Forms.Session].
//
// For a visitor without a session it returns "" when PreAuth is nil: forms
// rendered with the empty token are refused by every check. It fails only
// when the source of randomness of PreAuth fails, without touching w.
func (f Forms[T]) PageToken(w http.ResponseWriter, r *http.Request) (string, error) {
	if s, ok := f.Session(r); ok {
		return s.CSRF(), nil
	}
	if f.PreAuth == nil {
		return "", nil
	}
	return f.PreAuth.Issue(w, r)
}

// FragmentToken returns the CSRF token for the forms of a fragment that
// updates a page of r (an htmx swap, a poll), which must match the token
// of the page's other forms. For a visitor with a session that is the
// session's token ([Forms.Session]). For a visitor without one it is the
// token of the pre-authentication cookie that the page's forms were
// rendered with ([PreAuth.Token]), which FragmentToken does not renew under
// them: past half of its lifetime, [PreAuth.Issue] would replace it.
//
// The fallback: when r carries no valid pre-authentication cookie (none, a
// forged one, or one that expired, with which the page's forms fail too),
// FragmentToken issues a new cookie on w ([PreAuth.IssueRenewed]) and
// reports renewed. The browser's new cookie then refuses the tokens of the
// page's forms, so the fragment must also replace each of them with one
// that carries the new token (out of band swaps), or ask for a reload.
//
// The fallback sets the cookie in w's header, so call FragmentToken before
// writing the response: a cookie set once the header is written is lost,
// and renewed still reports true. That rules it out for the updates of an
// event stream (a live region), which use [Forms.Token]. After
// [Store.New] or [Store.Rotate] in the same request, see [Forms.Session].
//
// renewed is false whenever w was not touched: with a session, with a
// valid cookie, without PreAuth (the token is then "", as for
// [Forms.PageToken]), and on error. It fails only when the source of
// randomness of PreAuth fails.
func (f Forms[T]) FragmentToken(w http.ResponseWriter, r *http.Request) (token string, renewed bool, err error) {
	if s, ok := f.Session(r); ok {
		return s.CSRF(), false, nil
	}
	if f.PreAuth == nil {
		return "", false, nil
	}
	if token, ok := f.PreAuth.Token(r); ok {
		return token, false, nil
	}
	return f.PreAuth.IssueRenewed(w, r)
}

// Token returns the CSRF token of the forms already on r's page, and true,
// without setting anything: the token of r's session ([Forms.Session]), or
// for a visitor without one the token of r's pre-authentication cookie
// ([PreAuth.Token]). It returns "" and false when there is no such token:
// no session and no valid cookie (none, a forged one, or one that expired,
// with which the page's forms fail too), no session and a nil PreAuth, or
// a session that ended meanwhile.
//
// Token is for the updates of an event stream (a live region of package
// sse), and for any other response that has started. Such a response
// cannot set a cookie, which rules out [Forms.FragmentToken]. A stream's
// request also keeps the cookies the stream opened with for the stream's
// whole life, so once its pre-authentication cookie expires (a stream may
// outlive the TTL of [PreAuth]), Token reports false on every later
// update. Render each update's forms with the token while Token reports
// true; otherwise end the stream with an end event whose fragment asks for
// a reload (the Update of an sse Loop calls the stream's End). A stream
// Lifetime of at most half the pre-authentication TTL makes that rare, as
// a page's cookie is less than half its TTL old when the page renders. A
// signed-in visitor's stream ends with the session instead (the Done of
// the Loop set to [Session.Done]).
func (f Forms[T]) Token(r *http.Request) (string, bool) {
	if s, ok := f.Session(r); ok {
		token := s.CSRF()
		return token, token != ""
	}
	if f.PreAuth == nil {
		return "", false
	}
	return f.PreAuth.Token(r)
}

// Check checks a submitted CSRF token against r: with the token of r's
// session ([Forms.Session], [Session.CheckCSRF]) when the visitor has one,
// and otherwise with r's pre-authentication cookie ([PreAuth.Verify]). It
// returns nil or an error wrapping [ErrCSRF], which is also the answer for
// a visitor without a session when PreAuth is nil. Use it for forms read
// some other way than [Forms.Read], such as a streamed upload, or a form
// that is answered again with its values when its token fails (see
// [Forms.Read]).
func (f Forms[T]) Check(r *http.Request, token string) error {
	if s, ok := f.Session(r); ok {
		return s.CheckCSRF(token)
	}
	if f.PreAuth == nil {
		return errNoTokenSource
	}
	return f.PreAuth.Verify(r, token)
}

// Read reads the form in the body of r within Limits ([form.Read]) and
// checks its [CSRFField] with [Forms.Check], before the handler acts on
// any of it. The token is part of the form, so the whole form is read
// first, within the limits.
//
// On success the caller owns the values and clears them with
// [form.Values.Zero] (typically deferred). On failure Read returns nil and
// an error from package form (malformed, too large, unsupported type) or one
// wrapping [ErrCSRF], after zeroing every value it read. Return the error
// unchanged to the application's error policy: the Errors policy of package
// web answers form errors with 400, 413 or 415 and ErrCSRF with 403.
//
// Because Read zeroes the form when its token fails, its handler cannot
// answer with the form again. Forms that should come back with what was
// typed when their token fails, such as a sign-up or a long form for
// anonymous visitors (a pre-authentication form can expire half a TTL
// after it was rendered, see [PreAuth]), are read with [form.Read] and
// checked with [Forms.Check]. On ErrCSRF the handler acts on nothing and
// renders the form again with a fresh token from [Forms.PageToken], the
// typed values except secrets such as a password, and a 4xx status such
// as 403. That is safe only behind a same-origin policy such as
// mw.SameOrigin, which refuses cross-site posts: without one, another site
// could post values of its choosing and have them shown back in a form
// with a valid token.
//
// Read uses the first CSRF field of the form. A login handler that must
// accept its form from a visitor who meanwhile signed in from another tab
// reads it with [form.Read] and checks it with [PreAuth.Verify] itself.
func (f Forms[T]) Read(w http.ResponseWriter, r *http.Request) (*form.Values, error) {
	v, err := form.Read(w, r, f.Limits)
	if err != nil {
		return nil, err
	}
	if err := f.checkForm(r, v); err != nil {
		return nil, err
	}
	return v, nil
}

// checkForm checks the CSRF field of v against r, and zeroes v when the
// check fails.
func (f Forms[T]) checkForm(r *http.Request, v *form.Values) error {
	if err := f.Check(r, v.String(CSRFField)); err != nil {
		v.Zero()
		return err
	}
	return nil
}
