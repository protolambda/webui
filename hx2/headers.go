package hx2

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

// Names of the htmx 2 request and response headers.
//
// Reference: https://htmx.org/reference/#request_headers and
// https://htmx.org/reference/#response_headers
const (
	// HeaderBoosted is sent with requests of an element with hx-boost: "true".
	HeaderBoosted = "HX-Boosted"
	// HeaderCurrentURL is sent with the current URL of the browser.
	HeaderCurrentURL = "HX-Current-URL"
	// HeaderHistoryRestoreRequest is sent as "true" when htmx requests a page
	// to restore history after a miss in its local history cache.
	HeaderHistoryRestoreRequest = "HX-History-Restore-Request"
	// HeaderPrompt is sent with the user's response to hx-prompt.
	HeaderPrompt = "HX-Prompt"
	// HeaderRequest is sent with every htmx request: "true".
	HeaderRequest = "HX-Request"
	// HeaderTarget is sent with the id of the target element, if it has one.
	HeaderTarget = "HX-Target"
	// HeaderTriggerName is sent with the name of the triggered element, if it
	// has one.
	HeaderTriggerName = "HX-Trigger-Name"
	// HeaderTrigger is sent with the id of the triggered element, if it has
	// one. As a response header, it names client-side events to trigger.
	HeaderTrigger = "HX-Trigger"

	// HeaderLocation makes htmx load a location with an ajax request, without
	// a full page load.
	HeaderLocation = "HX-Location"
	// HeaderPushURL pushes a URL into the browser history.
	HeaderPushURL = "HX-Push-Url"
	// HeaderRedirect makes the browser load a URL (a full page load).
	HeaderRedirect = "HX-Redirect"
	// HeaderRefresh makes the browser reload the page when "true".
	HeaderRefresh = "HX-Refresh"
	// HeaderReplaceURL replaces the current URL in the browser history.
	HeaderReplaceURL = "HX-Replace-Url"
	// HeaderReselect selects the part of the response that is swapped in.
	HeaderReselect = "HX-Reselect"
	// HeaderReswap overrides how the response is swapped in.
	HeaderReswap = "HX-Reswap"
	// HeaderRetarget overrides the element the response is swapped into.
	HeaderRetarget = "HX-Retarget"
	// HeaderTriggerAfterSettle names client-side events to trigger after the
	// settle step.
	HeaderTriggerAfterSettle = "HX-Trigger-After-Settle"
	// HeaderTriggerAfterSwap names client-side events to trigger after the
	// swap step.
	HeaderTriggerAfterSwap = "HX-Trigger-After-Swap"
)

// ErrInvalidHeader indicates a response header value that htmx cannot read
// as intended, such as an event without a name.
var ErrInvalidHeader = errors.New("invalid htmx header")

// RequestHeaders are the headers that htmx 2 adds to its requests.
//
// The client sets them, and anyone can forge them: use them to choose a
// representation (for example a fragment instead of a full page), never to
// make an authorization decision. A response that depends on them should say
// so in its Vary header, e.g. Vary: HX-Request, so that caches keep the
// representations apart.
//
// The string fields are decoded the way htmx encodes them. htmx sets headers
// with XMLHttpRequest, which sends characters up to U+00FF as one byte each
// (Latin-1), so a value that is not valid UTF-8 is read as Latin-1. A value
// with other characters, or a line break, cannot be sent that way: htmx
// sends it encoded with encodeURIComponent and adds the header
// <name>-URI-AutoEncoded: true, and such a value is percent-decoded (or kept
// as sent when it is malformed). Values that are valid UTF-8 are kept as
// sent, so other clients can send UTF-8; a Latin-1 value whose bytes happen
// to form UTF-8, such as "Ã©", reads as that UTF-8 ("é").
type RequestHeaders struct {
	// Request is set for every request that htmx makes (HX-Request).
	Request bool
	// Boosted is set for requests of an element with hx-boost (HX-Boosted).
	Boosted bool
	// HistoryRestoreRequest is set when htmx requests a page to restore
	// history after a miss in its local history cache
	// (HX-History-Restore-Request).
	HistoryRestoreRequest bool
	// CurrentURL is the current URL of the browser (HX-Current-URL).
	CurrentURL string
	// Prompt is the user's response to hx-prompt (HX-Prompt).
	Prompt string
	// Target is the id of the target element, if it has one (HX-Target).
	Target string
	// Trigger is the id of the triggered element, if it has one (HX-Trigger).
	Trigger string
	// TriggerName is the name of the triggered element, if it has one
	// (HX-Trigger-Name).
	TriggerName string
}

// ParseRequestHeaders reads the htmx request headers from h, usually the
// header of an [http.Request]. Absent headers read as false or "". The
// boolean headers are true only for the value "true", which htmx sends. The
// string headers are decoded as described at [RequestHeaders].
func ParseRequestHeaders(h http.Header) RequestHeaders {
	return RequestHeaders{
		Request:               h.Get(HeaderRequest) == "true",
		Boosted:               h.Get(HeaderBoosted) == "true",
		HistoryRestoreRequest: h.Get(HeaderHistoryRestoreRequest) == "true",
		CurrentURL:            requestValue(h, HeaderCurrentURL),
		Prompt:                requestValue(h, HeaderPrompt),
		Target:                requestValue(h, HeaderTarget),
		Trigger:               requestValue(h, HeaderTrigger),
		TriggerName:           requestValue(h, HeaderTriggerName),
	}
}

// requestValue reads a string header that htmx sets: percent-decoded when
// htmx marked it as encoded, and decoded from Latin-1 when it is not valid
// UTF-8. See [RequestHeaders].
func requestValue(h http.Header, name string) string {
	v := h.Get(name)
	if h.Get(name+"-URI-AutoEncoded") == "true" {
		// url.PathUnescape is the inverse of encodeURIComponent: unlike
		// QueryUnescape, it keeps '+'.
		if decoded, err := url.PathUnescape(v); err == nil {
			return decoded
		}
		return v
	}
	if utf8.ValidString(v) {
		return v
	}
	var b strings.Builder
	b.Grow(2 * len(v))
	for i := 0; i < len(v); i++ {
		b.WriteRune(rune(v[i]))
	}
	return b.String()
}

// SetRedirect sets HX-Redirect: the browser loads url with a full page load.
func SetRedirect(h http.Header, url string) {
	h.Set(HeaderRedirect, url)
}

// SetRefresh sets HX-Refresh: the browser reloads the page.
func SetRefresh(h http.Header) {
	h.Set(HeaderRefresh, "true")
}

// SetPushURL sets HX-Push-Url: htmx pushes url into the browser history.
// The value "false" prevents the history update that the request would
// otherwise make.
func SetPushURL(h http.Header, url string) {
	h.Set(HeaderPushURL, url)
}

// SetReplaceURL sets HX-Replace-Url: htmx replaces the current URL in the
// browser history with url. The value "false" prevents the history update
// that the request would otherwise make.
func SetReplaceURL(h http.Header, url string) {
	h.Set(HeaderReplaceURL, url)
}

// SetReswap sets HX-Reswap: how the response is swapped in, with the syntax
// of hx-swap, e.g. "outerHTML" or "innerHTML show:top".
func SetReswap(h http.Header, swap string) {
	h.Set(HeaderReswap, swap)
}

// SetRetarget sets HX-Retarget: the CSS selector of the element that the
// response is swapped into, instead of the request's target.
func SetRetarget(h http.Header, selector string) {
	h.Set(HeaderRetarget, selector)
}

// SetReselect sets HX-Reselect: the CSS selector of the part of the response
// that is swapped in, overriding hx-select of the triggering element.
func SetReselect(h http.Header, selector string) {
	h.Set(HeaderReselect, selector)
}

// Location is the value of the HX-Location response header: htmx requests
// Path with an ajax GET, as htmx.ajax does, swaps the response in, and pushes
// Path into the browser history. The client-side event and handler options
// of htmx.ajax cannot be sent in a header and are not supported.
type Location struct {
	// Path is the URL to load. It is required.
	Path string `json:"path"`
	// Source is the CSS selector of the source element of the request.
	Source string `json:"source,omitempty"`
	// Target is the CSS selector of the element to swap the response into.
	Target string `json:"target,omitempty"`
	// Swap is how the response is swapped in, with the syntax of hx-swap.
	Swap string `json:"swap,omitempty"`
	// Select is the CSS selector of the part of the response to swap in.
	Select string `json:"select,omitempty"`
	// Values are submitted with the request.
	Values map[string]any `json:"values,omitempty"`
	// Headers are sent with the request.
	Headers map[string]string `json:"headers,omitempty"`
	// Push is the URL to push into the history instead of Path, or "false"
	// to push nothing. Only recent htmx 2 releases read it; others push Path.
	Push string `json:"push,omitempty"`
	// Replace is the URL that replaces the current one in the history,
	// instead of pushing one. Only recent htmx 2 releases read it.
	Replace string `json:"replace,omitempty"`
}

// SetLocation sets HX-Location. A location with only a Path is sent as the
// plain path; otherwise it is sent as a JSON object, with non-ASCII
// characters escaped so that the browser reads them intact. It fails with
// [ErrInvalidHeader] when Path is empty, and when Values cannot be encoded
// as JSON; h is then unchanged.
func SetLocation(h http.Header, loc Location) error {
	if loc.Path == "" {
		return fmt.Errorf("%w: %s without a path", ErrInvalidHeader, HeaderLocation)
	}
	pathOnly := loc.Source == "" && loc.Target == "" && loc.Swap == "" && loc.Select == "" &&
		len(loc.Values) == 0 && len(loc.Headers) == 0 && loc.Push == "" && loc.Replace == ""
	// htmx reads a value that starts with "{" as JSON.
	if pathOnly && !strings.HasPrefix(loc.Path, "{") {
		h.Set(HeaderLocation, loc.Path)
		return nil
	}
	data, err := json.Marshal(loc)
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrInvalidHeader, HeaderLocation, err)
	}
	h.Set(HeaderLocation, asciiJSON(data))
	return nil
}

// Event is a client-side event that a response triggers with HX-Trigger,
// HX-Trigger-After-Swap, or HX-Trigger-After-Settle.
type Event struct {
	// Name is the event name, e.g. "itemAdded". It is required.
	Name string
	// Detail, when not nil, is encoded as JSON and becomes the detail of the
	// event. htmx uses a JSON object as the detail, and triggers the event on
	// the element that its "target" member selects (a CSS selector), if any;
	// any other value becomes {"value": Detail}.
	Detail any
}

// SetTrigger sets HX-Trigger: htmx triggers the events once the response is
// received. See [SetTriggerAfterSwap] for the encoding and the errors.
func SetTrigger(h http.Header, events ...Event) error {
	return setTrigger(h, HeaderTrigger, events)
}

// SetTriggerAfterSwap sets HX-Trigger-After-Swap: htmx triggers the events
// after the swap step.
//
// Events without details, with names that htmx can read from a list, are
// sent as a comma-separated list; otherwise the events are sent as a JSON
// object, in order, with non-ASCII characters escaped so that the browser
// reads them intact. Without events, the header is removed. It fails with
// [ErrInvalidHeader] for an empty or repeated name, and for a detail that
// cannot be encoded as JSON; h is then unchanged.
func SetTriggerAfterSwap(h http.Header, events ...Event) error {
	return setTrigger(h, HeaderTriggerAfterSwap, events)
}

// SetTriggerAfterSettle sets HX-Trigger-After-Settle: htmx triggers the
// events after the settle step. See [SetTriggerAfterSwap] for the encoding
// and the errors.
func SetTriggerAfterSettle(h http.Header, events ...Event) error {
	return setTrigger(h, HeaderTriggerAfterSettle, events)
}

func setTrigger(h http.Header, header string, events []Event) error {
	if len(events) == 0 {
		h.Del(header)
		return nil
	}
	list := true
	seen := make(map[string]bool, len(events))
	for _, e := range events {
		if e.Name == "" {
			return fmt.Errorf("%w: %s event without a name", ErrInvalidHeader, header)
		}
		if seen[e.Name] {
			return fmt.Errorf("%w: %s event %q repeated", ErrInvalidHeader, header, e.Name)
		}
		seen[e.Name] = true
		if e.Detail != nil || !listName(e.Name) {
			list = false
		}
	}
	if list {
		names := make([]string, len(events))
		for i, e := range events {
			names[i] = e.Name
		}
		h.Set(header, strings.Join(names, ", "))
		return nil
	}
	// A JSON object, built by hand to keep the order of the events.
	var b strings.Builder
	b.WriteByte('{')
	for i, e := range events {
		if i > 0 {
			b.WriteByte(',')
		}
		name, _ := json.Marshal(e.Name) // a string always encodes
		detail, err := json.Marshal(e.Detail)
		if err != nil {
			return fmt.Errorf("%w: %s event %q detail: %w", ErrInvalidHeader, header, e.Name, err)
		}
		b.Write(name)
		b.WriteByte(':')
		b.Write(detail)
	}
	b.WriteByte('}')
	h.Set(header, asciiJSON([]byte(b.String())))
	return nil
}

// listName reports whether htmx reads name intact from a comma-separated
// event list: it splits on commas, trims whitespace, and reads a value that
// starts with "{" as JSON.
func listName(name string) bool {
	if strings.ContainsRune(name, ',') || strings.HasPrefix(name, "{") || strings.TrimSpace(name) != name {
		return false
	}
	for i := 0; i < len(name); i++ {
		if c := name[i]; c < 0x20 || c >= 0x7f {
			return false
		}
	}
	return true
}

// asciiJSON escapes the non-ASCII characters of JSON text as \u sequences.
// Browsers read header values as Latin-1 bytes, so UTF-8 would arrive
// garbled. Outside strings JSON is ASCII, so the escapes only land in
// strings, where they mean the same characters.
func asciiJSON(data []byte) string {
	var b strings.Builder
	b.Grow(len(data))
	for len(data) > 0 {
		r, size := utf8.DecodeRune(data)
		data = data[size:]
		switch {
		case r < utf8.RuneSelf:
			b.WriteRune(r)
		case r > 0xFFFF:
			r -= 0x10000
			fmt.Fprintf(&b, `\u%04x\u%04x`, 0xD800+(r>>10), 0xDC00+(r&0x3FF))
		default:
			fmt.Fprintf(&b, `\u%04x`, r)
		}
	}
	return b.String()
}
