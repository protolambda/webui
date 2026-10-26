package session

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
)

// ErrNoToken reports a render whose forms have no CSRF token: the
// outermost token node they render within, of [Forms.WithPageToken] or
// [Forms.WithFragmentToken], found no session token and no
// [Forms.PreAuth] for a visitor without a session, or that of
// [Forms.WithToken] found neither a session token nor a valid
// pre-authentication cookie (as [Forms.Token] reports false).
// It fails the render rather than letting a form out with the empty token.
var ErrNoToken = errors.New("session: no CSRF token for the form")

var (
	errNoFormToken   = fmt.Errorf("%w: no session token, and no pre-auth", ErrNoToken)
	errNoStreamToken = fmt.Errorf("%w: no session token, and no valid pre-auth cookie", ErrNoToken)
	// errNoTokenInContext reports a form rendered outside every node that
	// puts a token into the render context: a mistake of the view, not a
	// state of the visitor, so it does not wrap ErrNoToken.
	errNoTokenInContext = errors.New("session: no CSRF token in the render context: " +
		"render the form within Forms.WithPageToken, WithFragmentToken or WithToken")
)

// WithPageToken returns node with the CSRF token for the forms of a whole
// page that answers r in its render context, where [PostFormFromContext],
// [CSRFInputFromContext] and [TokenFromContext] find it anywhere below
// node. The views then render forms without a token parameter, and the
// handler states once, here, that it answers with a page. The token is
// that of [Forms.PageToken], taken when the first form below node renders
// and kept for the other forms and for later renders of the returned node,
// which belongs to the response of w and r: a page without forms sets no
// cookie, and the forms of a page share one token.
//
// One response has one token, and the outermost token node decides it: a
// node of WithPageToken, [Forms.WithFragmentToken] or [Forms.WithToken]
// below another one renders with the outer node's token and takes none of
// its own, so the response sets one cookie at most. A page may thus hold
// the node of a fragment that its handler also answers with on its own
// (see WithFragmentToken), or a layout's form within WithToken: within the
// page, their forms carry the page's token, also when the page issues a
// new cookie.
//
// PageToken may set the pre-authentication cookie on w, which only works
// while the header of the response is not written. web.Render renders the
// whole node into a buffer before it writes the header, so it works there;
// a renderer that writes to w as it goes writes the header first, and the
// cookie is lost. An event stream writes its header before it renders an
// update: its forms take [Forms.WithToken].
//
// A form below node fails the render with the error of PageToken, or with
// one wrapping [ErrNoToken] when the token is empty, as for a visitor
// without a session when PreAuth is nil.
func (f Forms[T]) WithPageToken(w http.ResponseWriter, r *http.Request, node elem.Node) elem.Node {
	source := newTokenSource(false, func() (string, bool, error) {
		token, err := f.PageToken(w, r)
		return token, false, err
	})
	return withTokenSource{node: node, source: func() *tokenSource { return source }}
}

// WithFragmentToken is [Forms.WithPageToken] for a fragment that updates
// a page of r (an htmx swap, a poll): the token is that of
// [Forms.FragmentToken], the token of the page's other forms, which it
// does not renew under them. Nothing guesses the kind from the request: a
// handler that answers with either (web.Choose) wraps the fragment with
// WithFragmentToken and the page with WithPageToken, and only the one that
// renders takes a token. A page that holds the fragment's node renders it
// with the page's token: below another token node, the node of
// WithFragmentToken takes the outer node's token (see WithPageToken), and
// [IfRenewed] below it follows the outer node, so it renders nothing
// within a page.
//
// When r carries no valid pre-authentication cookie (none, a forged one,
// or one that expired, with which the page's forms fail too), the first
// form below node issues a new cookie on w, which FragmentToken reports as
// renewed, and the forms below node carry its token. Once the browser
// stores the new cookie, it refuses the tokens of the page's other forms:
// render fresh copies of them, for out-of-band swaps, within [IfRenewed]
// below node, which renders them in that case only. The new cookie needs a
// response whose header is not written yet, as for WithPageToken.
func (f Forms[T]) WithFragmentToken(w http.ResponseWriter, r *http.Request, node elem.Node) elem.Node {
	source := newTokenSource(true, func() (string, bool, error) {
		return f.FragmentToken(w, r)
	})
	return withTokenSource{node: node, source: func() *tokenSource { return source }}
}

// WithToken returns node with the token of [Forms.Token] in its render
// context, as [Forms.WithPageToken] does: the token of the forms already on
// r's page, which sets nothing, for the updates of an event stream and any
// other response that has started. Unlike the other two, it takes the
// token anew at each render of the returned node (when its first form
// renders), so that a node built once renders each update of a stream with
// a token that is still valid: a stream outlives its pre-authentication
// cookie. Below another token node, it takes that node's token, as every
// token node does (see [Forms.WithPageToken]); the nodes of WithPageToken
// and WithFragmentToken below it take its token, and set nothing.
//
// Once there is no token, a form below node fails the render with an error
// wrapping [ErrNoToken], and the stream's Send sends nothing. The Update of
// an sse Loop must not return that error: the Loop would then end the
// stream as a failure ([github.com/protolambda/webui/sse.ReasonFailed]),
// and Run would return the error, to be logged as one. The Update checks
// errors.Is(err, session.ErrNoToken) itself and ends the stream with the
// stream's End and an end fragment that asks for a reload, instead of
// returning the error (see the example, and [Forms.Token]).
func (f Forms[T]) WithToken(r *http.Request, node elem.Node) elem.Node {
	return withTokenSource{node: node, source: func() *tokenSource {
		return newTokenSource(false, func() (string, bool, error) {
			token, ok := f.Token(r)
			if !ok {
				return "", false, errNoStreamToken
			}
			return token, false, nil
		})
	}}
}

// TokenFromContext returns the CSRF token for a form that renders with
// ctx: the token of the outermost node of [Forms.WithPageToken],
// [Forms.WithFragmentToken] or [Forms.WithToken] that the form renders
// within, taken on first use. It fails outside such a node, with the error
// of the token's method when that fails, and with an error wrapping
// [ErrNoToken] when there is no token: it never returns the empty token.
// Nodes of one's own call it from an [elem.Fn], with the context of their
// render.
func TokenFromContext(ctx context.Context) (string, error) {
	source, ok := ctx.Value(tokenKey{}).(*tokenSource)
	if !ok {
		return "", errNoTokenInContext
	}
	token, _, err := source.get()
	if err != nil {
		return "", err
	}
	return token, nil
}

// CSRFInputFromContext returns the hidden input of [CSRFInput] with the
// token of its render context ([TokenFromContext]). Without a token it
// fails the render, so that no form goes out with an empty token.
func CSRFInputFromContext() elem.Node {
	return elem.Fn(func(ctx context.Context) (elem.Node, error) {
		token, err := TokenFromContext(ctx)
		if err != nil {
			return nil, err
		}
		return CSRFInput(token), nil
	})
}

// PostFormFromContext is [PostForm] with the token of its render context
// ([CSRFInputFromContext]): views render it without a token parameter,
// below a node of [Forms.WithPageToken], [Forms.WithFragmentToken] or
// [Forms.WithToken], and it fails the render elsewhere.
//
// Error pages render outside the token node of the handler that failed:
// the Document and Fragment hooks of
// [github.com/protolambda/webui/web.Errors] render the error answer on
// their own. A form of a layout that error pages share, such as a logout
// form, therefore fails every error page, which web.Errors then answers
// as plain text. Render such a form with an explicit token ([PostForm]
// with [Session.CSRF], for a logout form), or within [Forms.WithToken]:
// it sets nothing, takes the page's token within a page, and on an error
// page the token of the request, failing only for a request without one.
func PostFormFromContext(action string, attrs ...attr.Node) elem.Scope {
	return postForm(action, CSRFInputFromContext(), attrs)
}

// IfRenewed renders children only when the response replaces the
// pre-authentication cookie whose token the page's forms carry: below a
// node of [Forms.WithFragmentToken], when the request carried no valid
// cookie and the fragment issued a new one (the renewed result of
// [Forms.FragmentToken]). The browser's new cookie refuses the page's other
// forms, so put fresh copies of them in it, for out-of-band swaps. It takes
// the token itself when no form did yet. Without a token, as for a visitor
// without a session when [Forms.PreAuth] is nil, nothing was renewed: it
// renders nothing, and the forms below the node still fail the render.
// Elsewhere it renders nothing: below [Forms.WithPageToken], whose page
// renders all of its forms with its own token, below [Forms.WithToken],
// which sets no cookie, and below a node of WithFragmentToken within one
// of those, which takes their token. Outside such a node it fails the
// render, as the forms do.
func IfRenewed(children ...elem.Node) elem.Node {
	return elem.Fn(func(ctx context.Context) (elem.Node, error) {
		source, ok := ctx.Value(tokenKey{}).(*tokenSource)
		if !ok {
			return nil, errNoTokenInContext
		}
		if !source.fragment {
			return elem.Noop(), nil
		}
		_, renewed, err := source.get()
		if errors.Is(err, ErrNoToken) {
			return elem.Noop(), nil // nothing to renew; the forms fail closed
		}
		if err != nil {
			return nil, err
		}
		if !renewed {
			return elem.Noop(), nil
		}
		return elem.Bundle(children), nil
	})
}

// tokenKey is the context key of the token source of a render.
type tokenKey struct{}

// tokenSource takes the CSRF token of a render, once, on first use.
type tokenSource struct {
	// fragment marks the source of WithFragmentToken, whose renewal
	// IfRenewed reports.
	fragment bool
	take     func() tokenResult
}

// tokenResult is what a [tokenSource] took.
type tokenResult struct {
	token   string
	renewed bool
	err     error
}

// newTokenSource returns a source that calls take once, on first use.
func newTokenSource(fragment bool, take func() (token string, renewed bool, err error)) *tokenSource {
	return &tokenSource{fragment: fragment, take: sync.OnceValue(func() tokenResult {
		token, renewed, err := take()
		if err == nil && token == "" {
			err = errNoFormToken
		}
		if err != nil {
			return tokenResult{err: err}
		}
		return tokenResult{token: token, renewed: renewed}
	})}
}

// get returns the token of s, taking it on first use: never the empty
// token without an error.
func (s *tokenSource) get() (token string, renewed bool, err error) {
	res := s.take()
	return res.token, res.renewed, res.err
}

// withTokenSource is a node whose subtree renders with a token source in
// its context: its own, unless the context holds one already, the source
// of an outer token node, which decides the token of the response.
type withTokenSource struct {
	node elem.Node
	// source returns the source of one render.
	source func() *tokenSource
}

var _ elem.Node = withTokenSource{}

func (n withTokenSource) Eval(ctx context.Context) (elem.Obj, error) {
	if n.node == nil {
		return elem.Obj{}, fmt.Errorf("%w: the node of Forms.WithPageToken, WithFragmentToken or WithToken", elem.ErrNilNode)
	}
	if _, nested := ctx.Value(tokenKey{}).(*tokenSource); nested {
		return n.node.Eval(ctx)
	}
	return n.node.Eval(context.WithValue(ctx, tokenKey{}, n.source()))
}
