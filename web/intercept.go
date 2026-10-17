package web

import (
	"bufio"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/protolambda/webui/hx2"
	"github.com/protolambda/webui/mw"
)

// Intercept is middleware that keeps error responses that are not HTML out
// of htmx swaps. htmx inserts every answer that it swaps as HTML, whatever
// its Content-Type, and webui's htmx configuration swaps 4xx and 5xx
// answers (see "htmx reads every answer as HTML" in the package
// documentation). A plain-text error that code outside the policy writes
// would become markup in the target of the request: an http.Error whose
// message quotes the request, the refusals of mw.HostCheck and
// mw.SameOrigin, the 404 and 405 answers of http.ServeMux.
//
// For an htmx request ([IsHTMX]), Intercept decides when the final status
// is written (by WriteHeader, or as 200 by the first Write, a ReadFrom
// that copies at least one byte, or a Flush of a writer that can flush),
// and replaces the response when all of these hold:
//
//   - the status is 4xx or 5xx;
//   - the response does not make htmx navigate instead of swapping: it has
//     no HX-Redirect or HX-Location header, and no HX-Refresh: true;
//   - its Content-Type, when the status is written, is not text/html: any
//     other media type (http.Error sets text/plain), or none at all.
//
// The replacement is the policy's htmx answer to the status, as described
// at [Errors]: the alert of [DefaultMessage] (or the Fragment hook) with
// the same status, swapped into Target (or TargetFor's choice) with
// HX-Reswap, HX-Reselect unset and HX-Push-Url false. The handler's headers
// are reset as for every error answer, and its body is dropped: later
// writes report success and discard their bytes. The replacement is logged
// at debug level with the status and the media type.
//
// Everything else passes through unchanged. Only an explicit text/html is
// trusted: such a response is taken to be markup made for the page, as
// [Render] and the policy write it, so a handler that writes an HTML error
// fragment of its own (with html/template, say) must set Content-Type
// text/html before it writes the status. An error without a Content-Type
// (absent or empty when the status is written), such as
//
//	w.WriteHeader(http.StatusBadRequest)
//	fmt.Fprintf(w, "invalid page: %s", value)
//
// is replaced: net/http would sniff the type of its body
// (http.DetectContentType), which tells markup from text poorly, and htmx
// swaps the body whatever its type. An empty error body is replaced too,
// since htmx would swap it into the target and empty it. The empty 401 of
// [Errors.RequireAuth] passes because of its HX-Redirect. A request that
// is not an htmx request gets its response unchanged, except that an
// error response that would be replaced for htmx gets HX-Request added to
// its Vary header, since the two answers differ.
//
// The response writer given to next implements Unwrap, Flush, FlushError,
// Hijack and ReadFrom, so http.NewResponseController(w) keeps working
// through it.
//
// Place it in mw.Chain inside mw.LogRequests, so that its log records
// carry the request's logger, and outside the middleware whose refusals it
// answers (mw.HostCheck, mw.SameOrigin) and the mux:
//
//	handler := mw.Chain(mux,
//		mw.LogRequests(mw.RequestLog{Routes: mux}),
//		errs.Recover,
//		errs.Intercept,
//		mw.BodyDeadline(mw.DefaultBodyTimeout),
//		mw.WriteDeadline(mw.DefaultWriteTimeout),
//		mw.SecurityHeaders(mw.Headers{}),
//		mw.HostCheck(site),
//		mw.SameOrigin(mw.OriginPolicy{Deny: errs.Answer(mw.ErrCrossOrigin)}),
//		mw.MaxBody(1<<20),
//	)
//
// With that Deny ([Errors.Answer]), SameOrigin's refusals get the policy's
// answer with [MessageForged], plain requests included; Intercept answers
// the plain-text refusals of HostCheck, and of SameOrigin without a Deny.
//
// Intercept is a safety net, not the error policy: handlers answer their
// errors through [Errors.Handle], with the application's messages.
// Intercept panics if next is nil.
func (e Errors) Intercept(next http.Handler) http.Handler {
	if next == nil {
		panic("web: Intercept of a nil handler")
	}
	policy := &e
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&interceptor{ResponseWriter: w, e: policy, r: r, htmx: IsHTMX(r)}, r)
	})
}

// interceptState is the decision of an interceptor about its response.
type interceptState uint8

const (
	// interceptPending means that no final status was written yet.
	interceptPending interceptState = iota
	// interceptPassed means that the response is the handler's own.
	interceptPassed
	// interceptReplaced means that the policy answered instead: the
	// handler's status and body are dropped.
	interceptReplaced
)

// interceptor is the response writer of [Errors.Intercept]. It is used by
// the handler's goroutine only.
type interceptor struct {
	http.ResponseWriter
	e     *Errors
	r     *http.Request
	htmx  bool
	state interceptState
}

var (
	_ http.Flusher  = (*interceptor)(nil)
	_ http.Hijacker = (*interceptor)(nil)
	_ io.ReaderFrom = (*interceptor)(nil)
)

func (w *interceptor) WriteHeader(code int) {
	if w.state == interceptReplaced {
		return
	}
	// Informational responses (1xx) may precede the final status, except
	// 101, which switches the protocol.
	if w.state == interceptPending && (code >= 200 || code == http.StatusSwitchingProtocols) {
		w.state = interceptPassed
		h := w.Header()
		if mediaType, ok := swappedAsHTML(h, code); ok {
			hx2.AddVary(h)
			if w.htmx {
				w.state = interceptReplaced
				w.e.replace(w.ResponseWriter, w.r, code, mediaType)
				return
			}
		}
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *interceptor) Write(b []byte) (int, error) {
	switch w.state {
	case interceptReplaced:
		return len(b), nil
	case interceptPending:
		// The implicit status is 200, which passes.
		w.state = interceptPassed
	}
	return w.ResponseWriter.Write(b)
}

// ReadFrom copies src to the response, with the underlying writer's
// ReadFrom (sendfile) when it has one, or discards it when the response
// was replaced. While no status was written, its first byte decides, as
// a Write does ([copyStarting]): a copy of nothing leaves the status open.
func (w *interceptor) ReadFrom(src io.Reader) (int64, error) {
	switch w.state {
	case interceptReplaced:
		return io.Copy(io.Discard, src)
	case interceptPending:
		return copyStarting(w, w.ResponseWriter, src)
	}
	return io.Copy(w.ResponseWriter, src)
}

// FlushError flushes the response, which starts it with status 200 when
// no status was written, unless the underlying writer cannot flush.
// net/http writes the status before it flushes, so a flush that fails
// otherwise, such as on a broken connection, has started the response.
func (w *interceptor) FlushError() error {
	err := http.NewResponseController(w.ResponseWriter).Flush()
	if w.state == interceptPending && !errors.Is(err, http.ErrNotSupported) {
		w.state = interceptPassed
	}
	return err
}

// Flush is FlushError without the error, for [http.Flusher].
func (w *interceptor) Flush() { _ = w.FlushError() }

// Hijack hands the connection to the handler.
func (w *interceptor) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	c, rw, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err == nil && w.state == interceptPending {
		w.state = interceptPassed
	}
	return c, rw, err
}

// Unwrap returns the underlying writer, for [http.ResponseController].
func (w *interceptor) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// swappedAsHTML reports whether htmx would insert the response with the
// header h and the final status code as HTML although it is an error that
// is not declared HTML: [Errors.Intercept] replaces such a response. It
// returns the media type of the response for the log, [noMediaType] when
// it has none.
func swappedAsHTML(h http.Header, code int) (mediaType string, ok bool) {
	if code < 400 || code > 599 {
		return "", false
	}
	// htmx follows these headers instead of swapping the body.
	if headerValue(h, hx2.HeaderRedirect) != "" || headerValue(h, hx2.HeaderLocation) != "" ||
		headerValue(h, hx2.HeaderRefresh) == "true" {
		return "", false
	}
	// Without a Content-Type, net/http sniffs one from the body, which
	// does not make the body markup made for the page: it is replaced.
	mediaType, _, _ = strings.Cut(h.Get("Content-Type"), ";")
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	switch mediaType {
	case "text/html":
		return "", false
	case "":
		return noMediaType, true
	}
	return mediaType, true
}

// noMediaType is the media type logged for a replaced response without
// one. It is no valid media type, so it cannot be confused with one.
const noMediaType = "(none)"

// headerValue returns the first value of the header name in h, whatever
// the letter case of its key: htmx reads response headers that way, and a
// handler may have set a key that is not canonical directly in the map.
func headerValue(h http.Header, name string) string {
	if v := h.Get(name); v != "" {
		return v
	}
	for key, values := range h {
		if len(values) > 0 && strings.EqualFold(key, name) {
			return values[0]
		}
	}
	return ""
}

// replace answers r with the policy's answer to status in place of a
// response that [Errors.Intercept] keeps out of the page.
func (e *Errors) replace(w http.ResponseWriter, r *http.Request, status int, mediaType string) {
	ctx := r.Context()
	if len(mediaType) > maxLoggedMediaType {
		mediaType = mediaType[:maxLoggedMediaType]
	}
	mw.Logger(ctx).LogAttrs(ctx, slog.LevelDebug, "replaced an error response that htmx would insert as HTML",
		slog.Int("status", status),
		slog.String("content_type", mediaType),
	)
	e.answer(w, r, clientAnswer(status, ""))
}

// maxLoggedMediaType bounds the media type in the log record of a
// replaced response.
const maxLoggedMediaType = 64
