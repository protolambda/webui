package webuitest

import (
	"net/http"
	"net/url"

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
	// the form bodies of requests other than GET and HEAD that do not
	// carry the field yet, as the page's forms would. It is also the token
	// of the default Contract.
	CSRF string
	// Contract checks the pages that [Client.Page] returns; nil means
	// [PageContract] with [DefaultShell]() and [Rules]([WithCSRF](CSRF)).
	Contract Contract
	// FragmentContract checks the fragments that [Client.Fragment]
	// returns; nil means [FragmentContract] with
	// [Rules]([WithCSRF](CSRF)).
	FragmentContract Contract
	// Options apply to every request, before the options of each call.
	Options []ReqOption
}

var _ Sender = (*Client)(nil)

// Request builds a request with the client's options and CSRF field, see
// [Client.CSRF]. Prepare is applied when it is sent ([Client.Do]).
func (c *Client) Request(t TB, method, target string, body Body, opts ...ReqOption) *http.Request {
	t.Helper()
	if c.CSRF != "" && !safeMethod(method) && !body.Has(csrfFieldName) {
		body = body.With(csrfFieldName, c.CSRF)
	}
	all := append(append([]ReqOption(nil), c.Options...), opts...)
	return Request(t, method, target, body, all...)
}

// Do prepares req (see [Client.Prepare]) and serves it. A missing handler
// or a Prepare that returns nil fails the test.
func (c *Client) Do(t TB, req *http.Request) *cthttp.Result {
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
	return cthttp.Serve(c.Handler, req)
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
			return PageContract(p, DefaultShell(), Rules(WithCSRF(c.CSRF))...)
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
// Editing a name that is no control of the form, a form without an
// action, or a dialog form fails the test.
func (c *Client) Submit(t TB, page *ct.Subject, form, submitter *ct.Selection, edits url.Values, opts ...ReqOption) *cthttp.Result {
	t.Helper()
	method, target, body, ok := submission(t, page, form, submitter, edits)
	if !ok {
		return nil
	}
	return c.Send(t, method, target, body, opts...)
}

// submission computes and edits the submission of a form.
func submission(t TB, page *ct.Subject, form, submitter *ct.Selection, edits url.Values) (method, target string, body Body, ok bool) {
	t.Helper()
	s, err := FormData(t.Context(), page, form, submitter)
	if err != nil {
		fatalf(t, "webuitest: %v", err)
		return "", "", Body{}, false
	}
	for name, values := range edits {
		if !s.HasControl(name) {
			fatalf(t, "webuitest: the form has no control %q to edit", name)
			return "", "", Body{}, false
		}
		s.Set(name, values...)
	}
	method, target, body, err = s.target()
	if err != nil {
		fatalf(t, "webuitest: %v", err)
		return "", "", Body{}, false
	}
	return method, target, body, true
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
	for e := range Elements(doc.Root(), "input") {
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
