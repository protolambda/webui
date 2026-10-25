package webuitest

import (
	"errors"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthttp"
	"github.com/protolambda/mustbe"
	"github.com/protolambda/mustbe/assertion"
)

// Sender sends requests built like [Request]: [Client] and [Browser]
// implement it, for helpers that work with both, such as
// [RejectsCrossSite] and [Crawl].
type Sender interface {
	Send(t TB, method, target string, body Body, opts ...ReqOption) *cthttp.Result
}

// Contract is a reusable expectation of a page or fragment.
type Contract func(page *ct.Subject) assertion.Assertion

// Client serves requests with a handler, synchronously and without a
// network, through [cthttp.Serve]: the handler-level harness of an
// application's tests. Requests are built as [Request] builds them, on
// [BaseURL].
//
// A Client keeps no cookies: the Set-Cookie headers of a response never
// reach the next request, so flows that depend on cookies, such as a
// login form or a pre-authentication CSRF cookie, do not work through it
// as a browser would run them. Attach state to each request instead, with
// Prepare (for example a detached session) or [SendCookie] in Options, or
// use a [Browser], which keeps a cookie jar.
//
// A Client is a plain value; methods take the test, so one client can
// serve the subtests of a test. It is safe for concurrent use when its
// fields are not changed and Prepare and the handler are.
type Client struct {
	// Handler serves the requests. It is required.
	Handler http.Handler
	// Prepare, when set, adapts every request before it is served, for
	// example to attach a session to the context
	// (session.WithSession(r.Context(), s)) and so bypass cookies and the
	// login flow. It must return a request.
	Prepare func(*http.Request) *http.Request
	// CSRF, when set, is added as the CSRF field ([session.CSRFField]) to
	// the hand-built form bodies of requests other than GET and HEAD
	// ([Client.Request], [Client.Send], [Client.Post]) that do not carry
	// the field yet, as the page's forms would. [Client.Submit] and
	// [Client.SubmitHTMX] send what the page sends and never add it. It
	// is also the token of the default contracts.
	CSRF string
	// Shell is the page shell of the default contract; nil means
	// [DefaultShell](), the shell of pages that load the default theme
	// set. An application that loads other sets, such as a theme without
	// icons, sets it to their shell and keeps the default rules:
	// new(webuitest.DefaultShell(sets...)).
	Shell *Shell
	// Contract checks the pages that [Client.Page] returns; nil means
	// [PageContract] with Shell and [Rules]([WithCSRF](CSRF)).
	Contract Contract
	// FragmentContract checks the fragments that [Client.Fragment]
	// returns; nil means [FragmentContract] with
	// [Rules]([WithCSRF](CSRF)).
	FragmentContract Contract
	// Options apply to every request, before the options of each call.
	Options []ReqOption
	// StreamTimeout is the timeout of [Client.Stream] streams, see
	// [StreamTimeout]; zero means [DefaultStreamTimeout].
	StreamTimeout time.Duration
}

var _ Sender = (*Client)(nil)

// Request builds a request with the client's options and CSRF field, see
// [Client.CSRF]. Prepare is applied when it is sent ([Client.Do]).
func (c *Client) Request(t TB, method, target string, body Body, opts ...ReqOption) *http.Request {
	t.Helper()
	if c.CSRF != "" && !safeMethod(method) && !body.Has(csrfFieldName) {
		body = body.With(csrfFieldName, c.CSRF)
	}
	return Request(t, method, target, body, c.options(opts)...)
}

// options returns the client's options followed by opts.
func (c *Client) options(opts []ReqOption) []ReqOption {
	return append(append([]ReqOption(nil), c.Options...), opts...)
}

// Do prepares req (see [Client.Prepare]) and serves it. A missing handler
// or a Prepare that returns nil fails the test.
func (c *Client) Do(t TB, req *http.Request) *cthttp.Result {
	t.Helper()
	if req = c.prepare(t, req); req == nil {
		return nil
	}
	return cthttp.Serve(c.Handler, req)
}

// prepare checks the handler and req, and applies Prepare. It returns nil
// after failing the test.
func (c *Client) prepare(t TB, req *http.Request) *http.Request {
	t.Helper()
	if c.Handler == nil {
		fatalf(t, "webuitest.Client: no handler")
		return nil
	}
	if req == nil {
		fatalf(t, "webuitest.Client: nil request")
		return nil
	}
	if c.Prepare != nil {
		if req = c.Prepare(req); req == nil {
			fatalf(t, "webuitest.Client: Prepare returned no request")
			return nil
		}
	}
	return req
}

// Stream builds a GET request for target as a browser's EventSource sends
// it (the client's Options, then [EventSource], then opts), prepares it
// (see [Client.Prepare], for example to attach a session) and serves it
// with [ServeStream], whose contract applies: the stream returns once the
// response header is sent, is closed at the end of the test, and the test
// fails if the handler does not return in time or panics. Streams wait at
// most [Client.StreamTimeout]. A missing handler or a Prepare that returns
// nil fails the test.
func (c *Client) Stream(t TB, target string, opts ...ReqOption) *Stream {
	t.Helper()
	req := c.Request(t, http.MethodGet, target, Body{}, append([]ReqOption{EventSource()}, opts...)...)
	if req == nil {
		return nil
	}
	if req = c.prepare(t, req); req == nil {
		return nil
	}
	return ServeStream(t, c.Handler, req, StreamTimeout(c.StreamTimeout))
}

// Send builds a request ([Client.Request]) and serves it ([Client.Do]).
func (c *Client) Send(t TB, method, target string, body Body, opts ...ReqOption) *cthttp.Result {
	t.Helper()
	req := c.Request(t, method, target, body, opts...)
	if req == nil {
		return nil
	}
	return c.Do(t, req)
}

// Get serves a GET request for target.
func (c *Client) Get(t TB, target string, opts ...ReqOption) *cthttp.Result {
	t.Helper()
	return c.Send(t, http.MethodGet, target, Body{}, opts...)
}

// Post serves a POST request for target with body, which gets the CSRF
// field of [Client.CSRF].
func (c *Client) Post(t TB, target string, body Body, opts ...ReqOption) *cthttp.Result {
	t.Helper()
	return c.Send(t, http.MethodPost, target, body, opts...)
}

// Page asserts that res has the status and is a page that satisfies the
// client's contract, and returns the page. A failure fails the test.
func (c *Client) Page(t TB, res *cthttp.Result, status int) *ct.Subject {
	t.Helper()
	mustbe.Must(t, res.Status(status))
	page := res.HTML()
	contract := c.Contract
	if contract == nil {
		contract = func(p *ct.Subject) assertion.Assertion {
			return PageContract(p, shellOrDefault(c.Shell), Rules(WithCSRF(c.CSRF))...)
		}
	}
	mustbe.Must(t, contract(page))
	return page
}

// Fragment asserts that res has the status and is an HTML fragment (see
// [cthttp.Result.HTMLFragment], parsed in the context of contextTag) that
// satisfies the client's fragment contract, and returns it.
func (c *Client) Fragment(t TB, res *cthttp.Result, status int, contextTag string) *ct.Subject {
	t.Helper()
	mustbe.Must(t, res.Status(status))
	frag := res.HTMLFragment(contextTag)
	contract := c.FragmentContract
	if contract == nil {
		contract = func(f *ct.Subject) assertion.Assertion {
			return FragmentContract(f, Rules(WithCSRF(c.CSRF))...)
		}
	}
	mustbe.Must(t, contract(frag))
	return frag
}

// Submit submits a form of page as a browser would ([FormData]), with
// edits replacing the values of the named controls as a user would enter
// them, and serves the request. A nil submitter submits without one.
//
// The body is what the form submits: [Client.CSRF] is not added, so a form
// without the CSRF field is sent without one, as by a browser. A browser
// resolves the action against the page's URL: pass it with [PageURL] for a
// form whose action is relative to the page ("done", "?page=2") or empty.
// Editing a name that is no control of the form, a relative or empty
// action without PageURL, or a dialog form fails the test.
//
// Submit sends the submission of the form without script. With [HTMX] (or
// [HXTarget], [HXCurrentURL]) it adds htmx's headers to that submission,
// which models an htmx form whose request attribute (hx-post, ...) has
// the form's method and action; it fails the test when the htmx request
// of the form or of the submitter has another method or URL, such as an
// hx-delete beside a POST fallback. [Client.SubmitHTMX] sends what htmx
// sends, with hx-vals, hx-include and hx-params applied.
func (c *Client) Submit(t TB, page *ct.Subject, form, submitter *ct.Selection, edits url.Values, opts ...ReqOption) *cthttp.Result {
	t.Helper()
	all := c.options(opts)
	method, target, body, ok := submission(t, page, form, submitter, edits, all)
	if !ok {
		return nil
	}
	req := Request(t, method, target, body, all...)
	if req == nil {
		return nil
	}
	return c.Do(t, req)
}

// submission computes and edits the submission of a form; opts are the
// options of its request, which may name the page's URL ([PageURL]).
func submission(t TB, page *ct.Subject, form, submitter *ct.Selection, edits url.Values, opts []ReqOption) (method, target string, body Body, ok bool) {
	t.Helper()
	s, err := FormData(t.Context(), page, form, submitter)
	if err != nil {
		fatalf(t, "%v", prefixed(err))
		return "", "", Body{}, false
	}
	// In name order, so that fields added at the end have a stable order.
	for _, name := range slices.Sorted(maps.Keys(edits)) {
		if !s.HasControl(name) {
			fatalf(t, "webuitest: the form has no control %q to edit", name)
			return "", "", Body{}, false
		}
		s.Set(name, edits[name]...)
	}
	method, target, body, err = s.target(opts)
	if err != nil {
		fatalf(t, "%v", prefixed(err))
		return "", "", Body{}, false
	}
	return method, target, body, true
}

// prefixed returns the message of err with the package prefix, which
// [ErrFormData] already has.
func prefixed(err error) string {
	if errors.Is(err, ErrFormData) {
		return err.Error()
	}
	return "webuitest: " + err.Error()
}

// CSRFToken returns the CSRF token of a page: the value of its hidden
// [session.CSRFField] inputs. It fails the test when the page does not
// load, has no such field, or has fields with different or empty values.
// Messages never show the token.
func CSRFToken(t TB, page *ct.Subject) string {
	t.Helper()
	doc, err := page.Load(t.Context())
	if err != nil {
		fatalf(t, "webuitest.CSRFToken: %v", err)
		return ""
	}
	token, n := "", 0
	for e := range elementsByTag(doc.Root(), "input") {
		if !AttrIs(e, "name", csrfFieldName) || inputType(e) != "hidden" {
			continue
		}
		v, _ := attrOf(e, "value")
		switch {
		case v == "":
			fatalf(t, "webuitest.CSRFToken: the %q field at %s is empty", csrfFieldName, loc(e))
			return ""
		case n > 0 && v != token:
			fatalf(t, "webuitest.CSRFToken: the %q fields have different values", csrfFieldName)
			return ""
		}
		token = v
		n++
	}
	if n == 0 {
		fatalf(t, "webuitest.CSRFToken: the page has no hidden %q field", csrfFieldName)
		return ""
	}
	return token
}
