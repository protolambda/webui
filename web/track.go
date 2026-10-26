package web

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/http"

	"github.com/protolambda/webui/internal/flush"
)

// tracker records whether a handler started its response: wrote a final
// status or a byte of the body, flushed (unless the writer cannot flush), or
// took over the connection. After that, an error can only be logged. A copy
// (ReadFrom) that copies nothing, because its source is empty or fails at
// once, does not start the response, just as net/http sends nothing then.
// It is used by the handler's goroutine while the handler runs, and read
// after the handler returns (or panics).
//
// It implements Unwrap, so [http.ResponseController] reaches the underlying
// writer (deadlines, full duplex), and Flush, FlushError, Hijack and
// ReadFrom, so the common optional interfaces keep working and are tracked.
type tracker struct {
	http.ResponseWriter
	started bool
}

var (
	_ http.Flusher    = (*tracker)(nil)
	_ http.Hijacker   = (*tracker)(nil)
	_ io.ReaderFrom   = (*tracker)(nil)
	_ flush.Forwarder = (*tracker)(nil)
)

// track wraps w in a tracker, or returns w itself when it is one.
func track(w http.ResponseWriter) *tracker {
	if t, ok := w.(*tracker); ok {
		return t
	}
	return &tracker{ResponseWriter: w}
}

// maxUnwrap bounds the walk through wrapped writers, in case of a cycle.
const maxUnwrap = 32

// started reports whether the response of w started, as far as a tracker
// in the chain of wrapped writers of w knows. Without a tracker it reports
// false.
func started(w http.ResponseWriter) bool {
	for range maxUnwrap {
		if t, ok := w.(*tracker); ok {
			return t.started
		}
		u, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return false
		}
		w = u.Unwrap()
	}
	return false
}

func (t *tracker) WriteHeader(code int) {
	// Informational responses (1xx) may precede the final status, except
	// 101, which switches the protocol.
	if code >= 200 || code == http.StatusSwitchingProtocols {
		t.started = true
	}
	t.ResponseWriter.WriteHeader(code)
}

func (t *tracker) Write(b []byte) (int, error) {
	t.started = true
	return t.ResponseWriter.Write(b)
}

// ReadFrom copies src to the response, with the underlying writer's
// ReadFrom (sendfile) when it has one. It starts the response with its
// first byte only ([copyStarting]).
func (t *tracker) ReadFrom(src io.Reader) (int64, error) {
	if t.started {
		return io.Copy(t.ResponseWriter, src)
	}
	return copyStarting(t, t.ResponseWriter, src)
}

// FlushError flushes the response, which starts it, unless the underlying
// writer cannot flush. net/http writes the status before it flushes, so a
// flush that fails otherwise, such as on a broken connection, has started
// the response.
func (t *tracker) FlushError() error {
	err := http.NewResponseController(t.ResponseWriter).Flush()
	if !errors.Is(err, http.ErrNotSupported) {
		t.started = true
	}
	return err
}

// Flush is FlushError without the error, for [http.Flusher].
func (t *tracker) Flush() { _ = t.FlushError() }

// ForwardsFlush marks the tracker as a [flush.Forwarder]: it can flush
// when the underlying writer can.
func (t *tracker) ForwardsFlush() {}

// Hijack hands the connection to the handler.
func (t *tracker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	c, rw, err := http.NewResponseController(t.ResponseWriter).Hijack()
	if err == nil {
		t.started = true
	}
	return c, rw, err
}

// Unwrap returns the underlying writer, for [http.ResponseController].
func (t *tracker) Unwrap() http.ResponseWriter { return t.ResponseWriter }

// firstChunk is the length of the start of a copy that [copyStarting]
// copies through the tracking writer, the length that net/http sniffs.
const firstChunk = 512

// copyStarting copies src to the response of w, a writer that tracks when
// the response starts, for its ReadFrom while the response has not
// started. The first bytes go through w.Write, which sees the start, and
// the rest through io.Copy to next, the writer that w wraps, which keeps
// its ReadFrom (sendfile). A src that is empty, or that fails before its
// first byte, writes nothing at all, so the status stays open: net/http,
// too, starts the response of a copy with its first byte, and a status
// written after an empty copy is the real one.
func copyStarting(w, next io.Writer, src io.Reader) (int64, error) {
	// The LimitedReader hides a WriteTo of src, and writerOnly the
	// ReadFrom of w: every byte goes through w.Write, never an empty write,
	// which would start the response of net/http.
	n, err := io.Copy(writerOnly{w}, io.LimitReader(src, firstChunk))
	if err != nil || n < firstChunk {
		return n, err
	}
	m, err := io.Copy(next, src)
	return n + m, err
}

// writerOnly hides the methods of a writer other than Write.
type writerOnly struct{ io.Writer }
