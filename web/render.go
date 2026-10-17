package web

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/elem"

	"github.com/protolambda/webui/hx2"
)

// ContentType is the Content-Type of rendered HTML responses.
const ContentType = "text/html; charset=utf-8"

// Render renders node and writes it as an HTML response with the status.
//
// The node is rendered into a buffer first, with the request's context.
// When rendering fails, nothing is written, so a failure never produces half
// a page: Render returns the error (wrapping the render error) and the
// caller answers it, e.g. by returning it from a [HandlerFunc], which turns
// it into a 500 page. On success Render sets Content-Type to [ContentType],
// writes the status and the body, and returns the error of the write, if
// any (usually a client that went away). The status must allow a body
// (not 1xx, 204 or 304). Other headers, such as Cache-Control, are left to
// the caller and the middleware.
//
// Render does not add Vary: use [Choose], or call hx2.AddVary, when the
// content depends on whether htmx made the request.
func Render(w http.ResponseWriter, r *http.Request, status int, node elem.Node) error {
	var buf bytes.Buffer
	if err := core.Render(r.Context(), node, &buf); err != nil {
		return fmt.Errorf("web: %w", err)
	}
	w.Header().Set("Content-Type", ContentType)
	w.WriteHeader(status)
	if _, err := w.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("web: write response: %w", err)
	}
	return nil
}

// Choose renders fragment for htmx requests ([IsHTMX]) and full otherwise,
// with [Render], and adds HX-Request to the Vary header of both, so that
// caches (the browser's own included) keep the two representations apart.
//
// The fragment is usually the part of the page that the htmx request swaps,
// and full the page that embeds the same fragment, so that the URL also
// works without htmx (a reload, a bookmark, a browser without script).
func Choose(w http.ResponseWriter, r *http.Request, status int, fragment, full elem.Node) error {
	hx2.AddVary(w.Header())
	if IsHTMX(r) {
		return Render(w, r, status, fragment)
	}
	return Render(w, r, status, full)
}

// IsHTMX reports whether r is an htmx request that expects a fragment: it
// carries HX-Request: true, and is not a history restore request
// (HX-History-Restore-Request: true), which needs a full page because htmx
// swaps the response into the whole body.
//
// The headers come from the client, and anyone can forge them: use IsHTMX
// to choose a representation, never to make an authorization decision.
func IsHTMX(r *http.Request) bool {
	h := hx2.ParseRequestHeaders(r.Header)
	return h.Request && !h.HistoryRestoreRequest
}

// IsPrefetch reports whether r is a speculative request that the browser
// makes before the user follows a link (a prefetch or prerender): it
// carries a Sec-Purpose header (the standard one), or a Purpose,
// X-Purpose or X-Moz header naming prefetch, prerender or preview (older
// browsers).
//
// A user may never see the response of such a request, so it must not
// complete an action; see [MayAct].
func IsPrefetch(r *http.Request) bool {
	if r.Header.Get("Sec-Purpose") != "" {
		// Every purpose that the standard defines is speculative.
		return true
	}
	for _, name := range [...]string{"Purpose", "X-Purpose", "X-Moz"} {
		v := strings.ToLower(r.Header.Get(name))
		if strings.Contains(v, "prefetch") || strings.Contains(v, "prerender") || strings.Contains(v, "preview") {
			return true
		}
	}
	return false
}

// MayAct reports whether r is a GET request that may complete an action,
// such as the link in an email that confirms an account: a GET that is not
// a prefetch ([IsPrefetch]). HEAD requests and every other method report
// false. A GET that may not act should show a page with a button that
// posts the action instead.
func MayAct(r *http.Request) bool {
	return r.Method == http.MethodGet && !IsPrefetch(r)
}
