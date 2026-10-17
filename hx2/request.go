package hx2

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/protolambda/chord/core/attr"
)

// ErrInvalidValue indicates an attribute value that htmx cannot read as
// intended, such as hx-vals that is not a JSON object.
var ErrInvalidValue = errors.New("hx2: invalid htmx attribute value")

// Include sets the hx-include attribute to include additional elements in request.
func Include(v string) attr.Node { return attr.Name("hx-include").Value(v) }

// Params sets the hx-params attribute to filter request parameters:
// "*", "none", "not a,b", or a comma-separated list of names.
func Params(v string) attr.Node { return attr.Name("hx-params").Value(v) }

// Vals sets the hx-vals attribute to add values to the request. v is JSON
// object text; prefer [ValsJSON], which encodes it.
//
// The value is HTML-escaped, but never build it from untrusted input with
// string formatting: a quote in the input can add or override values. A
// value that starts with "js:" or "javascript:" is a JavaScript expression,
// which htmx evaluates only with [Config.AllowEval].
func Vals(v string) attr.Node { return attr.Name("hx-vals").Value(v) }

// ValsJSON sets the hx-vals attribute to the JSON encoding of v, which must
// encode as a JSON object, such as a struct or a map with string keys. htmx
// submits each member as a request parameter, in addition to the form values
// (overriding those with the same name), and merges the hx-vals of ancestors
// in, the nearest element winning. A string, number or boolean member is sent
// as its text, an array as one parameter per element, and an object as its
// JSON text.
//
// A null member, which is how encoding/json writes a nil slice, map,
// pointer or interface, is sent as the text "null" (replacing a form value
// of the same name), and does not override a member of the same name in an
// ancestor's hx-vals. To leave a parameter out, tag the field omitempty (or
// omitzero); an empty non-nil slice sends no parameter.
//
// htmx merges the members into a plain JavaScript object, so a top-level
// member named after a property of Object.prototype ("constructor",
// "toString", "hasOwnProperty", "__proto__", ...) would be dropped, or,
// for "hasOwnProperty", make htmx throw and send no request; rendering
// fails with [ErrInvalidValue] instead. Names are compared exactly, as
// JavaScript does. Validate member names built from user data before
// rendering, or one value can fail the whole page.
//
// v is encoded when ValsJSON is called, with encoding/json (maps in sorted
// key order); later changes to v do not show. When v does not encode as a
// JSON object (nil, a slice, a string...) rendering fails with
// [ErrInvalidValue]; when it cannot be encoded at all, rendering fails with
// an error that wraps both ErrInvalidValue and the encoding error.
func ValsJSON(v any) attr.Node {
	data, err := json.Marshal(v)
	if err != nil {
		return failAttr(fmt.Errorf("%w: hx-vals: %w", ErrInvalidValue, err))
	}
	if !bytes.HasPrefix(data, []byte("{")) {
		return failAttr(fmt.Errorf("%w: hx-vals is not a JSON object", ErrInvalidValue))
	}
	// Decode the member names: Marshal may write a name with escapes, from a
	// json.RawMessage or a json.Marshaler, and htmx reads the decoded name.
	var members map[string]json.RawMessage
	if err := json.Unmarshal(data, &members); err != nil { // Marshal output always decodes
		return failAttr(fmt.Errorf("%w: hx-vals: %w", ErrInvalidValue, err))
	}
	for name := range members {
		if prototypeName(name) {
			return failAttr(fmt.Errorf("%w: hx-vals member %q is a JavaScript Object.prototype name", ErrInvalidValue, name))
		}
	}
	return attr.Name("hx-vals").Value(string(data))
}

// Headers sets the hx-headers attribute to add headers to the request. v is
// JSON object text; prefer [HeadersJSON], which encodes it. The cautions of
// [Vals] apply.
func Headers(v string) attr.Node { return attr.Name("hx-headers").Value(v) }

// HeadersJSON sets the hx-headers attribute to the JSON encoding of h: htmx
// sends each entry as a request header, and merges the hx-headers of
// ancestors in, the nearest element winning. Names are written in sorted
// order; a nil or empty h gives "{}". Browsers silently drop forbidden
// request headers such as Cookie or Host. htmx sends a value that a browser
// refuses in a header (characters above U+00FF, line breaks) percent-encoded,
// with the extra header <name>-URI-AutoEncoded: true.
//
// h is encoded when HeadersJSON is called; later changes to h do not show.
// Rendering fails with [ErrInvalidValue] when a name is not a valid HTTP
// header name (a non-empty token), or is a property of JavaScript's
// Object.prototype, such as "constructor" or "hasOwnProperty", which htmx
// would drop or fail on as described at [ValsJSON]. Names are compared
// exactly: "Constructor" is a valid name.
//
// The value sits in the page: anyone who can read the page can read it. It
// suits a per-session CSRF token, not a credential the page must not see.
func HeadersJSON(h map[string]string) attr.Node {
	for name := range h {
		if !headerName(name) {
			return failAttr(fmt.Errorf("%w: hx-headers name %q is not a header name", ErrInvalidValue, name))
		}
		if prototypeName(name) {
			return failAttr(fmt.Errorf("%w: hx-headers name %q is a JavaScript Object.prototype name", ErrInvalidValue, name))
		}
	}
	if len(h) == 0 {
		return attr.Name("hx-headers").Value("{}")
	}
	data, err := json.Marshal(h) // a map of strings always encodes
	if err != nil {
		return failAttr(fmt.Errorf("%w: hx-headers: %w", ErrInvalidValue, err))
	}
	return attr.Name("hx-headers").Value(string(data))
}

// Request sets the hx-request attribute to configure request behavior.
func Request(v string) attr.Node { return attr.Name("hx-request").Value(v) }

// Sync sets the hx-sync attribute to synchronize AJAX requests, e.g.
// Sync("this:replace") or Sync("closest form:abort").
func Sync(v string) attr.Node { return attr.Name("hx-sync").Value(v) }

// failAttr returns an attribute node whose evaluation fails with err, so
// that rendering a page that uses it fails as a whole.
func failAttr(err error) attr.Node {
	return attr.Fn(func(context.Context) (attr.Node, error) { return nil, err })
}

// prototypeName reports whether name is a property of JavaScript's
// Object.prototype. htmx merges hx-vals and hx-headers members into a plain
// object and skips a member whose name is already set there, so it drops
// these; a member named "hasOwnProperty" shadows the method that htmx calls
// while it copies them, so htmx throws and sends nothing.
func prototypeName(name string) bool {
	switch name {
	case "__proto__", "__defineGetter__", "__defineSetter__", "__lookupGetter__", "__lookupSetter__",
		"constructor", "hasOwnProperty", "isPrototypeOf", "propertyIsEnumerable",
		"toLocaleString", "toString", "valueOf":
		return true
	}
	return false
}

// headerName reports whether name is an HTTP field name: a token of the
// characters that RFC 9110 allows.
func headerName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		case c == '!', c == '#', c == '$', c == '%', c == '&', c == '\'', c == '*',
			c == '+', c == '-', c == '.', c == '^', c == '_', c == '`', c == '|', c == '~':
		default:
			return false
		}
	}
	return true
}
