package hx2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/meta"
)

// ErrInvalidConfig indicates a [Config] that cannot be written as an htmx
// configuration, such as a negative history cache size or a [Config.Extra]
// value that is not representable as JSON.
var ErrInvalidConfig = errors.New("hx2: invalid htmx config")

// Config is the htmx configuration of a page. [Config.Meta] renders it as
// <meta name="htmx-config" content="{...}">, which htmx reads once, when it
// initializes.
//
// The zero value is webui's secure configuration, and the fields opt into
// htmx behaviors that need a weaker Content-Security-Policy or that keep
// page content in the browser. With the zero value, htmx:
//
//   - injects no inline <style> for request indicators (includeIndicatorStyles
//     false), which a strict style-src would block; style .htmx-indicator in a
//     same-origin stylesheet instead;
//   - never compiles JavaScript from the page (allowEval false): hx-on
//     attributes and "js:" and "javascript:" values of hx-vals and
//     hx-headers do nothing, and htmx drops hx-trigger event filters in
//     square brackets, so Trigger("click[ctrlKey]") triggers on every click
//     (see [Trigger]);
//   - does not run <script> elements of swapped content (allowScriptTags
//     false);
//   - keeps no snapshots of page HTML in sessionStorage (historyCacheSize 0),
//     so forms, tokens and other page content are not stored client side; on
//     back and forward navigation the browser reloads the page from the
//     server (refreshOnHistoryMiss true);
//   - sends no HX-Request header with history restore requests, if it makes
//     any (historyRestoreAsHxRequest false; see NoRefreshOnHistoryMiss), so
//     a handler that answers htmx requests with fragments answers them with
//     full pages;
//   - only requests URLs of the page's own origin (selfRequestsOnly true);
//   - puts request parameters in the URL for GET requests only
//     (methodsThatUseUrlParams ["get"]): DELETE requests carry them in the
//     body, as POST, PUT and PATCH requests do, so an hx-delete does not put
//     the CSRF field of its form in a URL (see [Config.DeleteURLParams]);
//   - settles only the class, width and height attributes of swapped
//     elements (attributesToSettle), never style: htmx writes settled
//     attributes with setAttribute, and a style attribute written that way
//     violates a style-src without 'unsafe-inline' (see
//     [Config.AttributesToSettle]);
//   - swaps the responses of [DefaultResponseHandling]: 2xx responses except
//     204, and also 4xx and 5xx responses, so a server-rendered error fragment
//     shows without client-side script. A swapped error is handled like a
//     success, history updates and an inherited hx-select included, so a
//     handler that sends such a fragment also sends HX-Push-Url: false and
//     HX-Reselect: unset (see [DefaultResponseHandling]). Every 4xx and 5xx
//     status but 422 counts as an error, which htmx logs to the browser
//     console. htmx inserts every swapped body as HTML, whatever its
//     Content-Type, so an error body that echoes request data, such as a
//     plain-text http.Error(w, err.Error(), 400), becomes markup in the
//     page whose hx-* attributes act without script: never write
//     unescaped request data into an error body on a route that htmx
//     requests (see the security note at DefaultResponseHandling, and
//     [Config.NoErrorSwap]).
//
// Each of these keys is written explicitly, so the page does not depend on
// the defaults of a particular htmx release. The keys and their meanings
// were checked against htmx 2.0.10, the release that webui's theme ships; a
// test of this package checks the keys against that script, and fails when
// the theme moves to another release.
//
// Other keys keep htmx's defaults. One of them matters to forms:
// reportValidityOfForms is false, so a request that htmx checks and stops,
// that of an element with hx-validate="true" ([Validate]) or of a form
// without novalidate that a trigger other than submit sends, shows no
// message and moves no focus: nothing seems to happen. A click on an htmx
// button inside a form skips the browser's own check too. Set the key in
// Extra to have htmx report the first invalid field, or validate on the
// server:
//
//	hx2.Config{Extra: map[string]any{"reportValidityOfForms": true}}
//
// An ordinary form submit is not affected: the browser checks the form,
// and reports, before htmx sees the submit event.
//
// A Config is a plain value; it is safe for concurrent use as long as no
// goroutine modifies its ResponseHandling, AttributesToSettle or Extra
// while another reads it.
type Config struct {
	// IndicatorStyles lets htmx inject its inline <style> for request
	// indicators (includeIndicatorStyles). That needs a style-src that allows
	// inline styles, or a nonce set through Extra ("inlineStyleNonce"). A
	// nonce protects only when it is fresh for each response, so it needs a
	// Config per response, not one shared value; it is also readable in the
	// content of the htmx-config meta element, which browsers do not hide
	// the way they hide nonce attributes.
	IndicatorStyles bool

	// AllowEval lets htmx evaluate JavaScript from the page (allowEval): hx-on
	// attributes (see [On]), "js:" values of hx-vals and hx-headers, and
	// hx-trigger event filters. The browser runs these only with a script-src
	// that allows 'unsafe-eval'. Without both, htmx drops the event filter of
	// an hx-trigger and triggers on every matching event (see [Trigger]).
	// Never combine it with untrusted content in those attributes.
	AllowEval bool

	// AllowScriptTags lets htmx run <script> elements of swapped content
	// (allowScriptTags). htmx re-creates each such script in the page, so the
	// script-src of the page applies: scripts from allowed sources run, and
	// inline scripts run only when script-src allows them.
	//
	// Setting "inlineScriptNonce" through Extra makes htmx give every swapped
	// script the page's nonce, so script-src no longer filters the scripts of
	// fragments: any <script> that reaches swapped content, for example
	// through an HTML injection bug in user content, runs. Use it only when no
	// fragment can contain user-derived HTML; the nonce must also be fresh for
	// each response, not a fixed value in a shared Config, and it is readable
	// in the htmx-config meta element.
	AllowScriptTags bool

	// HistoryCacheSize is the number of pages that htmx keeps as HTML
	// snapshots in sessionStorage for history navigation (historyCacheSize).
	// Zero keeps none. A snapshot holds everything the page showed, form
	// values and hidden tokens included: mark pages with sensitive content
	// with [History](false) when the cache is enabled. It must not be
	// negative.
	HistoryCacheSize int

	// NoRefreshOnHistoryMiss makes htmx restore a page missing from the
	// history cache with a history restore request (a GET with the header
	// HX-History-Restore-Request: true) whose response it swaps into the
	// page, instead of reloading the page (refreshOnHistoryMiss false). The
	// handler must then answer such requests with a full page.
	NoRefreshOnHistoryMiss bool

	// HistoryRestoreAsHXRequest makes htmx send HX-Request: true with history
	// restore requests too (historyRestoreAsHxRequest), as htmx does by
	// default.
	HistoryRestoreAsHXRequest bool

	// AllowCrossOrigin lets htmx request URLs of other origins
	// (selfRequestsOnly false). The connect-src of the page still applies.
	AllowCrossOrigin bool

	// DeleteURLParams makes htmx put the parameters of DELETE requests in
	// the URL query and send no body (methodsThatUseUrlParams ["get",
	// "delete"]), as htmx 2 does by default.
	//
	// Without it, a DELETE request carries its parameters in the body, as a
	// POST request does and as htmx 1 did. htmx sends the fields of the
	// element's form with every request but GET, so with DeleteURLParams an
	// hx-delete button in a form puts all of the form's fields, its CSRF
	// token included, in the URL, where server and proxy logs keep it.
	//
	// [net/http.Request.ParseForm] reads the body only for POST, PUT and
	// PATCH requests, so a handler of DELETE requests reads their bodies
	// itself, for example with webui's form package, which reads the body
	// whatever the method and never reads the URL query.
	DeleteURLParams bool

	// AttributesToSettle, when not empty, replaces the attributes that htmx
	// settles (attributesToSettle), [DefaultAttributesToSettle]. When htmx
	// swaps in an element with the tag and id of an element of the target,
	// it first gives the new element the old element's values of these
	// attributes, then its own after the settle delay, so CSS transitions
	// can animate the change. htmx matches the names exactly: the names of
	// HTML attributes are lower case, while SVG keeps the case of some, such
	// as viewBox. Each name must be of ASCII letters, digits, '-', '_', '.'
	// and ':'.
	//
	// htmx writes the values with setAttribute. A "style" attribute written
	// that way is an inline style, which a style-src (or style-src-attr)
	// without 'unsafe-inline' blocks and reports, even when it is empty, so
	// webui leaves "style" out of the default, unlike htmx: scripts such as
	// browser extensions, test drivers and Bootstrap's JavaScript leave style
	// attributes on elements that a later swap replaces.
	AttributesToSettle []string

	// NoErrorSwap makes htmx ignore the bodies of 4xx and 5xx responses, as
	// htmx does by default: the response handling is htmx's default, which is
	// [DefaultResponseHandling] without the 422 rule and with a "[45].." rule
	// that does not swap. Without it, a swapped error updates the browser
	// history and applies the inherited hx-select like a success does, as
	// described at DefaultResponseHandling. It cannot be combined with
	// ResponseHandling.
	//
	// Set it for a page whose htmx requests reach handlers that may write
	// request data into error bodies, such as handlers of other packages
	// that answer with http.Error(w, err.Error(), status): with webui's
	// rules htmx inserts such a body as HTML, and hx-* attributes injected
	// through it act without script, past a strict Content-Security-Policy
	// (see the security note at DefaultResponseHandling). The price is that
	// no error shows: htmx fires htmx:responseError, which only script
	// displays, and the alerts of [github.com/protolambda/webui/web.Errors]
	// are not swapped in either. Wrapping such handlers with
	// [github.com/protolambda/webui/web.Errors.Intercept], which answers
	// their non-HTML errors to htmx requests with the policy's alert, keeps
	// error swapping safe instead.
	NoErrorSwap bool

	// ResponseHandling, when not empty, replaces [DefaultResponseHandling]
	// (responseHandling). htmx uses the first rule whose Code matches the
	// status of a response, and swaps nothing when no rule matches.
	ResponseHandling []ResponseRule

	// DefaultSwapStyle is the swap style of elements without hx-swap
	// (defaultSwapStyle); empty leaves the htmx default, [SwapInnerHTML]. It
	// must be one of [SwapInnerHTML], [SwapOuterHTML], [SwapBeforeBegin],
	// [SwapAfterBegin], [SwapBeforeEnd], [SwapAfterEnd], [SwapDelete], or
	// [SwapNone]: htmx falls back to it for unknown swap styles, and falling
	// back to an unknown style, or to [SwapTextContent], never ends.
	DefaultSwapStyle string

	// Extra holds additional htmx config keys, e.g. "timeout",
	// "defaultSettleDelay" or "reportValidityOfForms" (see [Validate]),
	// written in sorted key order with the keys above.
	// Values are encoded with encoding/json. Keys are not checked against the
	// htmx config, so a misspelled key is silently ignored by htmx. A key that
	// a field of Config controls, such as "allowEval", is an error: use the
	// field instead. Some keys weaken the Content-Security-Policy, such as
	// "inlineScriptNonce" (see AllowScriptTags) and "inlineStyleNonce" (see
	// IndicatorStyles).
	Extra map[string]any
}

// ResponseRule is an htmx response handling rule: it decides what htmx does
// with a response whose status matches Code. See [Config.ResponseHandling].
type ResponseRule struct {
	// Code selects the statuses that the rule applies to: exactly three
	// positions, each a decimal digit, '.' for any digit, or a bracket class
	// of digits and ascending digit ranges, e.g. "404", "[45]..",
	// "[0-4]0[1-3]", or "..." for any status. It is required.
	//
	// htmx compiles Code as a JavaScript regular expression for every
	// response, and tests it, unanchored, against the status as three
	// digits; an invalid expression breaks the response handling of the
	// whole page. So [Config.JSON] fails with [ErrInvalidConfig] for any
	// other form, alternation and negated classes included: write one rule
	// per alternative instead.
	Code string `json:"code"`
	// Swap is whether htmx swaps the response in.
	Swap bool `json:"swap"`
	// Error is whether htmx treats the response as an error: it then fires
	// htmx:responseError (after the swap, if any) and logs the status.
	Error bool `json:"error,omitempty"`
	// IgnoreTitle makes htmx ignore a <title> in the response.
	IgnoreTitle bool `json:"ignoreTitle,omitempty"`
	// Select is a CSS selector for the part of the response that is swapped
	// in, as hx-select.
	Select string `json:"select,omitempty"`
	// Target is a CSS selector for the element that the response is swapped
	// into, as hx-target.
	Target string `json:"target,omitempty"`
	// SwapOverride is the swap style of the response, as hx-swap.
	SwapOverride string `json:"swapOverride,omitempty"`
	// Event is the name of an event that htmx triggers before the swap;
	// a listener can cancel the swap.
	Event string `json:"event,omitempty"`
}

// DefaultResponseHandling returns webui's response handling rules, in order:
//
//	{"code":"204","swap":false}
//	{"code":"[23]..","swap":true}
//	{"code":"422","swap":true}
//	{"code":"[45]..","swap":true,"error":true}
//	{"code":"...","swap":false}
//
// htmx's own default differs in not swapping 4xx and 5xx responses and in
// having no 422 rule (see [Config.NoErrorSwap]); the last rule spells out
// what htmx does without a matching rule. Swapping errors lets a handler
// answer a failed htmx request with a fragment that shows the error, keeping
// the real status code. A 204 response swaps nothing, which suits polling
// that found no change. A 286 response ([StatusStopPolling]) swaps as a 2xx
// one and stops the poll of the element that made the request; no other
// status stops a poll, so an error answer to a poll, swapped like any
// other, comes again on every tick (see [Trigger]).
//
// A 422 (Unprocessable Content) response is the usual answer to a form with
// problems: an expected outcome of user input that the server explains in
// the swapped fragment, not a failure. So it swaps without the error flag.
// For a response with the error flag, htmx logs "Response Status Error
// Code ..." as an error to the browser console, fires htmx:responseError,
// and reports the request as failed in the details of its events; for every
// validation answer, that would bury real errors in noise. Without the
// flag, the detail of htmx:afterRequest reports a 422 response as
// successful: a listener that needs to tell validation answers apart reads
// the status of detail.xhr. Every other 4xx and 5xx status stays an error.
// The browser logs a console error of its own, "Failed to load resource:
// the server responded with a status of 422 (Unprocessable Entity)", for
// a 422 as for every 4xx and 5xx response, which no rule removes: browser
// checks that watch the console allow those (see "Browser console" at
// [github.com/protolambda/webui/web.Errors]).
//
// htmx handles a swapped error response like a successful one, whatever its
// status. It updates the browser history for boosted elements, elements
// with hx-push-url or hx-replace-url, and HX-Location requests, so the
// failed URL would end up in the location bar and in the history, where a
// reload or a forward step loads the full error page. And it swaps only the
// part that the hx-select of the triggering element, or of an ancestor,
// selects, which a fragment retargeted elsewhere usually does not contain:
// the target is then emptied and the error never shows. A handler that
// retargets an error fragment, for example into an alerts region,
// therefore sets four headers:
//
//	hx2.SetRetarget(h, "#alerts")
//	hx2.SetReswap(h, hx2.SwapInnerHTML)
//	hx2.SetReselect(h, "unset") // swap the whole fragment, whatever hx-select
//	hx2.SetPushURL(h, "false")  // no history update, push or replace
//
// Security: with these rules, every 4xx and 5xx body becomes page markup.
// htmx inserts a swapped response as HTML whatever its Content-Type, and
// processes the hx-* attributes in it. Those attributes send requests
// without any script, so a strict Content-Security-Policy does not stop
// them. Error bodies are where Go code most often echoes the request: a
// plain-text error such as
//
//	http.Error(w, err.Error(), http.StatusBadRequest)
//
// whose err quotes the input, as the errors of strconv, time.Parse and
// url.Parse do, is inert in a browser tab (text/plain, nosniff) but becomes
// part of the page once htmx swaps it. A value that an attacker gets into
// such a request, for example through a link that prefills a field of a
// form that a load trigger submits, can then be an element such as
//
//	<i hx-post="/archive" hx-include="[name=csrf]" hx-trigger="load"></i>
//
// which sends a same-origin request carrying the page's CSRF token, so
// same-origin checks and the CSRF check both pass. On every route that htmx
// requests, an error body therefore never contains request data that is not
// HTML-escaped:
//
//   - answer errors with [github.com/protolambda/webui/web.Errors], whose
//     messages never contain request data;
//   - wrap handlers that write their own error bodies, such as handlers of
//     other packages that call http.Error, with
//     [github.com/protolambda/webui/web.Errors.Intercept], which answers
//     their 4xx and 5xx responses to htmx requests whose Content-Type is
//     not text/html (http.Error sets text/plain; a missing one counts too)
//     with the alert of their status instead (see its documentation for
//     what it lets through);
//   - or set [Config.NoErrorSwap] for pages whose htmx requests reach
//     handlers that neither covers.
//
// htmx's own default does not swap error bodies, so the hazard is specific
// to these rules; a 2xx answer that htmx swaps is page markup under any
// configuration, and must not echo request data unescaped either.
//
// Each call returns a new slice, which the caller may modify, for example to
// add a rule in front.
func DefaultResponseHandling() []ResponseRule {
	return []ResponseRule{
		{Code: "204", Swap: false},
		{Code: "[23]..", Swap: true},
		{Code: "422", Swap: true},
		{Code: "[45]..", Swap: true, Error: true},
		{Code: "...", Swap: false},
	}
}

// htmxResponseHandling returns htmx's default response handling rules, with
// the rule that htmx applies when no rule matches spelled out.
func htmxResponseHandling() []ResponseRule {
	return []ResponseRule{
		{Code: "204", Swap: false},
		{Code: "[23]..", Swap: true},
		{Code: "[45]..", Swap: false, Error: true},
		{Code: "...", Swap: false},
	}
}

// DefaultAttributesToSettle returns the attributes that webui's htmx
// configuration settles: "class", "width" and "height". htmx's own default
// also has "style" (see [Config.AttributesToSettle]). Each call returns a
// new slice, which the caller may modify, for example to add a name.
func DefaultAttributesToSettle() []string {
	return []string{"class", "width", "height"}
}

// Config keys that Config fields control. Extra must not contain them.
const (
	keyIncludeIndicatorStyles    = "includeIndicatorStyles"
	keyAllowEval                 = "allowEval"
	keyAllowScriptTags           = "allowScriptTags"
	keyHistoryCacheSize          = "historyCacheSize"
	keyRefreshOnHistoryMiss      = "refreshOnHistoryMiss"
	keyHistoryRestoreAsHxRequest = "historyRestoreAsHxRequest"
	keySelfRequestsOnly          = "selfRequestsOnly"
	keyMethodsThatUseURLParams   = "methodsThatUseUrlParams"
	keyAttributesToSettle        = "attributesToSettle"
	keyResponseHandling          = "responseHandling"
	keyDefaultSwapStyle          = "defaultSwapStyle"
)

// JSON returns the htmx configuration as JSON text: one object with the keys
// in sorted order, so that the same Config always gives the same text.
// It fails with [ErrInvalidConfig] when HistoryCacheSize is negative,
// DefaultSwapStyle is not one of the swap styles it lists, an
// AttributesToSettle name is not of the form it describes, NoErrorSwap is
// combined with ResponseHandling, a response rule has a Code not of the
// form that [ResponseRule.Code] describes, or an Extra key is empty,
// controlled by a Config field, or has a value that encoding/json cannot
// encode.
func (c Config) JSON() (string, error) {
	if c.HistoryCacheSize < 0 {
		return "", fmt.Errorf("%w: negative history cache size %d", ErrInvalidConfig, c.HistoryCacheSize)
	}
	settle := c.AttributesToSettle
	if len(settle) == 0 {
		settle = DefaultAttributesToSettle()
	}
	for i, name := range settle {
		if !attributeName(name) {
			return "", fmt.Errorf("%w: attribute to settle %d: %q is not an attribute name",
				ErrInvalidConfig, i, name)
		}
	}
	urlMethods := []string{"get"}
	if c.DeleteURLParams {
		urlMethods = append(urlMethods, "delete")
	}
	rules := c.ResponseHandling
	if len(rules) == 0 {
		rules = DefaultResponseHandling()
		if c.NoErrorSwap {
			rules = htmxResponseHandling()
		}
	} else if c.NoErrorSwap {
		return "", fmt.Errorf("%w: NoErrorSwap combined with ResponseHandling", ErrInvalidConfig)
	}
	for i, r := range rules {
		if r.Code == "" {
			return "", fmt.Errorf("%w: response rule %d without a code", ErrInvalidConfig, i)
		}
		if !responseCode(r.Code) {
			return "", fmt.Errorf("%w: response rule %d: code %q is not three digits, '.' or digit classes",
				ErrInvalidConfig, i, r.Code)
		}
	}
	values := map[string]any{
		keyIncludeIndicatorStyles:    c.IndicatorStyles,
		keyAllowEval:                 c.AllowEval,
		keyAllowScriptTags:           c.AllowScriptTags,
		keyHistoryCacheSize:          c.HistoryCacheSize,
		keyRefreshOnHistoryMiss:      !c.NoRefreshOnHistoryMiss,
		keyHistoryRestoreAsHxRequest: c.HistoryRestoreAsHXRequest,
		keySelfRequestsOnly:          !c.AllowCrossOrigin,
		keyMethodsThatUseURLParams:   urlMethods,
		keyAttributesToSettle:        settle,
		keyResponseHandling:          rules,
	}
	if c.DefaultSwapStyle != "" {
		if !defaultSwapStyle(c.DefaultSwapStyle) {
			return "", fmt.Errorf("%w: default swap style %q", ErrInvalidConfig, c.DefaultSwapStyle)
		}
		values[keyDefaultSwapStyle] = c.DefaultSwapStyle
	}
	for k, v := range c.Extra {
		if k == "" {
			return "", fmt.Errorf("%w: empty extra key", ErrInvalidConfig)
		}
		if _, ok := values[k]; ok || k == keyDefaultSwapStyle {
			return "", fmt.Errorf("%w: extra key %q is controlled by a Config field", ErrInvalidConfig, k)
		}
		// Encode each value on its own so that an error names its key.
		data, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("%w: extra key %q: %w", ErrInvalidConfig, k, err)
		}
		values[k] = json.RawMessage(data)
	}
	// encoding/json writes map keys in sorted order.
	data, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}
	return string(data), nil
}

// Meta returns the <meta name="htmx-config"> element for the configuration,
// with the [Config.JSON] text as its content attribute value, which chord
// HTML-escapes. The configuration is encoded when Meta is called; when that
// fails, rendering the returned node fails with the [ErrInvalidConfig]
// error.
//
// Render it once, in the head of every full page that loads htmx, before
// any content derived from user input: htmx reads the first such element
// when it initializes, and ignores the element in swapped content.
func (c Config) Meta() elem.Node {
	data, err := c.JSON()
	if err != nil {
		return elem.Fn(func(context.Context) (elem.Node, error) { return nil, err })
	}
	return meta.Meta(meta.Name("htmx-config"), meta.Content(data))
}

// defaultSwapStyle reports whether htmx can use s as its default swap style:
// a style that htmx swaps without falling back to the default.
func defaultSwapStyle(s string) bool {
	switch s {
	case SwapInnerHTML, SwapOuterHTML, SwapBeforeBegin, SwapAfterBegin,
		SwapBeforeEnd, SwapAfterEnd, SwapDelete, SwapNone:
		return true
	}
	return false
}

// responseCode reports whether code has the form that [ResponseRule.Code]
// describes: exactly three positions, each a decimal digit, '.', or a
// non-empty bracket class of digits and ascending digit ranges such as [45]
// or [0-4]. Such a code is a valid JavaScript regular expression that
// matches only whole three-digit statuses.
func responseCode(code string) bool {
	positions := 0
	for i := 0; i < len(code); positions++ {
		if positions == 3 {
			return false
		}
		switch c := code[i]; {
		case c == '.' || isDigit(c):
			i++
		case c == '[':
			i++
			start := i
			for i < len(code) && code[i] != ']' {
				lo := code[i]
				if !isDigit(lo) {
					return false
				}
				i++
				if i+1 < len(code) && code[i] == '-' && code[i+1] != ']' {
					if hi := code[i+1]; !isDigit(hi) || hi < lo {
						return false
					}
					i += 2
				}
			}
			if i == start || i == len(code) {
				return false
			}
			i++ // the closing ']'
		default:
			return false
		}
	}
	return positions == 3
}

func isDigit(c byte) bool { return '0' <= c && c <= '9' }

// attributeName reports whether name is a non-empty attribute name of the
// form that [Config.AttributesToSettle] describes, which excludes the
// spaces, quotes and other characters that no attribute name has.
func attributeName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		switch c := name[i]; {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', isDigit(c), c == '-', c == '_', c == '.', c == ':':
		default:
			return false
		}
	}
	return true
}
