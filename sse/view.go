package sse

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/protolambda/chord/core/attr"

	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/internal/urlpath"
)

// ErrInvalidURL reports a stream URL that [Region] or [Connect] rejects.
var ErrInvalidURL = errors.New("sse: invalid stream URL")

// Region returns the attributes of a live region's container:
//
//	hx-ext="sse" sse-connect="URL" sse-swap="end" sse-close="end"
//
// The container opens the stream at url, its content is replaced by the
// fragment of the [EventEnd] event, and that event closes the connection
// for good. Mark children that other events update with [Swap]. Add other
// htmx extensions on an ancestor (hx-ext is inherited): a second hx-ext on
// the container fails to render.
//
// url must be a same-origin path, with an optional query and fragment: it
// starts with one '/', and consists of printable ASCII other than space
// and backslash (URL parsers drop tabs and line breaks, and read a
// backslash as a slash, which could turn the path into another host).
// Percent-encode other characters, for example with [net/url.PathEscape]
// or [net/url.QueryEscape]; percent escapes must be complete, in the query
// too (a raw "100%" in a query value fails). This is the rule of
// web.SafeNext, without its length limit (web.MaxNextLen): the stream URL
// is written by the application, not taken from a request. Otherwise the
// attributes fail to render with an error wrapping [ErrInvalidURL]. Use
// [hx2.SSEConnect] directly for anything else.
func Region(url string) attr.Node {
	if err := checkURL(url); err != nil {
		return failed{err}
	}
	return attr.Bundle{hx2.Ext("sse"), hx2.SSEConnect(url), hx2.SSESwap(EventEnd), hx2.SSEClose(EventEnd)}
}

// Connect returns the attributes that open a stream without swapping the
// end event into the element:
//
//	hx-ext="sse" sse-connect="URL" sse-close="end"
//
// Use it when the end fragment belongs elsewhere: mark that element with
// Swap(EventEnd). url is checked as for [Region].
func Connect(url string) attr.Node {
	if err := checkURL(url); err != nil {
		return failed{err}
	}
	return attr.Bundle{hx2.Ext("sse"), hx2.SSEConnect(url), hx2.SSEClose(EventEnd)}
}

// Swap returns the sse-swap attribute of an element inside a live region:
// the data of each named event replaces the element's content (or what its
// hx-target and hx-swap select). With more events, the attribute lists them
// all. A name that [CheckEventName] rejects makes the attribute fail to
// render with an error wrapping [ErrInvalidEventName].
//
// Put aria-live="polite" (or role="status") on targets whose changes people
// should hear, and leave it off targets that change every second.
func Swap(event string, more ...string) attr.Node {
	if err := CheckEventName(event); err != nil {
		return failed{err}
	}
	if len(more) == 0 {
		return hx2.SSESwap(event)
	}
	for _, name := range more {
		if err := CheckEventName(name); err != nil {
			return failed{err}
		}
	}
	return hx2.SSESwap(event + "," + strings.Join(more, ","))
}

// checkURL accepts same-origin paths, with the rule of web.SafeNext and
// page links (internal/urlpath) but without SafeNext's length limit, and
// says why it refuses others without repeating the URL, which may carry a
// token.
func checkURL(s string) error {
	switch {
	case s == "" || s[0] != '/':
		return fmt.Errorf("%w: not a path starting with '/'", ErrInvalidURL)
	case len(s) > 1 && s[1] == '/':
		return fmt.Errorf("%w: starts with '//', which names another host", ErrInvalidURL)
	case !urlpath.Printable(s):
		return fmt.Errorf("%w: not printable ASCII without spaces or backslashes", ErrInvalidURL)
	case !queryEscaped(s):
		return fmt.Errorf("%w: incomplete percent escape in the query", ErrInvalidURL)
	case !urlpath.Local(s):
		return fmt.Errorf("%w: not a valid path, query and fragment", ErrInvalidURL)
	}
	return nil
}

// queryEscaped reports whether the percent escapes in the query of s are
// complete. url.Parse checks those of the path and fragment but keeps the
// raw query unchecked; the browser would send a broken escape unchanged
// and net/url's Query would then drop the pair. url.QueryUnescape checks
// the escapes without url.ParseQuery's refusal of ';'. urlpath.Local
// checks the same; this check runs first for a more specific error.
func queryEscaped(s string) bool {
	beforeFragment, _, _ := strings.Cut(s, "#")
	_, query, _ := strings.Cut(beforeFragment, "?")
	_, err := url.QueryUnescape(query)
	return err == nil
}

var _ attr.Node = failed{}

// failed is an attribute that fails to render with err.
type failed struct{ err error }

func (f failed) Eval(context.Context) (attr.Obj, error) { return attr.Obj{}, f.err }
