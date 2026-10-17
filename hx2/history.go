package hx2

import (
	"github.com/protolambda/chord/core/attr"
)

// PushURL sets the hx-push-url attribute to push a URL to browser history:
// "true" pushes the request URL, "false" pushes nothing, and any other value
// is the URL to push. Before it pushes, htmx stores a snapshot of the current
// page in its history cache, unless [Config.HistoryCacheSize] is zero or the
// page has [History](false).
func PushURL(v string) attr.Node { return attr.Name("hx-push-url").Value(v) }

// ReplaceURL sets the hx-replace-url attribute to replace browser history URL,
// with the values of [PushURL].
func ReplaceURL(v string) attr.Node { return attr.Name("hx-replace-url").Value(v) }

// HistoryElt sets the hx-history-elt boolean attribute: htmx snapshots and
// restores this element, instead of the body, for history navigation.
func HistoryElt() attr.Node { return attr.Name("hx-history-elt").Bool() }

// History sets the hx-history attribute. History(false) renders
// hx-history="false": while any element of the page carries it, htmx stores
// no snapshot of the page in its history cache (sessionStorage), and history
// navigation back to the page loads it from the server. Put it on pages with
// content that must not stay in the browser, such as forms with tokens or
// personal data, whenever the cache is enabled ([Config.HistoryCacheSize]
// above zero; webui's default Config disables it for all pages).
//
// History(true) renders hx-history="true", which htmx treats like no
// attribute: it is the default.
func History(enabled bool) attr.Node {
	if enabled {
		return attr.Name("hx-history").Value("true")
	}
	return attr.Name("hx-history").Value("false")
}
