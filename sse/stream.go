package sse

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/elem"
)

const (
	// ContentType is the media type of an event stream response.
	ContentType = "text/event-stream; charset=utf-8"

	// EventEnd names the event that ends a stream for good: [Stream.End]
	// sends it, and [Region] names it in sse-swap and sse-close.
	EventEnd = "end"

	// DefaultWriteWindow is how long one write of a stream may take, unless
	// [WithWriteWindow] sets another window.
	DefaultWriteWindow = 30 * time.Second

	// MaxEventNameLen is the maximum length of an event name in bytes.
	MaxEventNameLen = 128
)

var (
	// ErrEventName reports an event name that [CheckEventName] rejects.
	ErrEventName = errors.New("sse: invalid event name")

	// ErrField reports a field value that the event stream format cannot
	// carry: a comment or ID with a line break, or a negative retry delay.
	ErrField = errors.New("sse: invalid field")

	// ErrEnded reports a write to a stream after [Stream.End].
	ErrEnded = errors.New("sse: stream ended")

	// ErrWrite reports that the connection failed: setting the write
	// deadline, writing, or flushing returned an error. The client left or
	// stalled; the stream cannot be used any more. Handlers normally treat
	// it as the end of the stream rather than as a failure to log.
	//
	// A response writer that cannot flush at all is not a connection
	// failure: that error wraps [http.ErrNotSupported] instead, so it is
	// logged (see [Stream]).
	ErrWrite = errors.New("sse: write failed")
)

// Option configures a [Stream].
type Option func(*Stream)

// WithWriteWindow sets how long each write of the stream may take: every
// write first sets the connection's write deadline to now plus d. Values of
// zero or less select [DefaultWriteWindow]. Keep-alive comments must come
// more often than d (see [Timing]).
func WithWriteWindow(d time.Duration) Option {
	return func(s *Stream) {
		if d > 0 {
			s.window = d
		}
	}
}

// Stream writes server-sent events to an HTTP response.
//
// The response starts lazily, at the first write: [Stream.Start] or the
// first event or comment sets Content-Type to [ContentType], Cache-Control
// to no-store and X-Accel-Buffering to no, and writes status 200. Before
// that, a failing event (an invalid name or field, or a fragment that fails
// to render) writes nothing, so the handler can still answer with an error.
//
// Every write sets a write deadline of now plus the write window, writes one
// whole event, and flushes. A failed write makes the stream unusable: every
// later write returns the same error, which wraps [ErrWrite].
//
// An event stream that is not flushed never reaches the browser, so the
// ResponseWriter must support flushing through [http.ResponseController]
// (a FlushError or Flush method, possibly behind Unwrap methods). When it
// does not, typically because middleware wraps the writer without passing
// flushes through, the first write fails before anything is written, with
// an error wrapping [http.ErrNotSupported] and not [ErrWrite]: it is a
// configuration bug worth logging, and the handler can still answer with an
// error status. Later writes return the same error. A writer that reports
// [http.ErrNotSupported] only when it is flushed fails the same way, after
// the header. Writers without write deadlines, such as
// [net/http/httptest.ResponseRecorder] and CGI, are tolerated: the deadline
// is then skipped silently.
//
// The stream keeps its buffers for reuse and clears them after every event,
// so rendered fragments do not stay in its memory. This is best effort: the
// HTTP server and the runtime may hold other copies.
//
// A Stream is not safe for concurrent use, like the ResponseWriter it
// writes to.
type Stream struct {
	w       http.ResponseWriter
	rc      *http.ResponseController
	window  time.Duration
	started bool
	ended   bool
	err     error // sticky write failure
	data    buffer
	frame   buffer
}

// New returns a stream that writes to w. It writes nothing yet.
func New(w http.ResponseWriter, opts ...Option) *Stream {
	s := &Stream{w: w, rc: http.NewResponseController(w), window: DefaultWriteWindow}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// WriteWindow returns how long each write may take.
func (s *Stream) WriteWindow() time.Duration { return s.window }

// Started reports whether the response header was written.
func (s *Stream) Started() bool { return s.started }

// Ended reports whether [Stream.End] was called.
func (s *Stream) Ended() bool { return s.ended }

// Start writes the response header and flushes it, if the stream has not
// started yet, so the browser's connection opens before the first event is
// ready. Events and comments start the stream themselves.
func (s *Stream) Start() error {
	if s.started {
		return s.err
	}
	return s.write(nil)
}

// Send renders node and sends it as the data of one event named event,
// followed by the fields. The HTML is split at CR, LF and CRLF into data
// fields, which the browser joins with LF.
//
// Nothing is written when the event name or a field is invalid (wrapping
// [ErrEventName] or [ErrField]) or when node fails to render (wrapping the
// render error). ctx is the render context. Send returns [ErrEnded] after
// [Stream.End], and an error wrapping [ErrWrite] when the connection fails
// (see [Stream] for writers that cannot flush).
func (s *Stream) Send(ctx context.Context, event string, node elem.Node, fields ...Field) error {
	if err := s.check(event, fields); err != nil {
		return err
	}
	defer s.data.reset()
	if err := core.Render(ctx, node, &s.data); err != nil {
		return fmt.Errorf("sse: render event: %w", err)
	}
	return s.event(event, s.data.b, fields)
}

// SendText sends text verbatim as the data of one event named event,
// followed by the fields. The text is split at CR, LF and CRLF into data
// fields, which the browser joins with LF, so CR and CRLF arrive as LF.
//
// The text is not escaped. An sse-swap target swaps event data in as HTML:
// never send untrusted text with SendText to one; use [Stream.Send] with an
// [elem.Text] node instead. SendText suits data that scripts or hx-trigger
// listeners read, such as a counter or JSON.
//
// Errors are as for [Stream.Send].
func (s *Stream) SendText(event, text string, fields ...Field) error {
	if err := s.check(event, fields); err != nil {
		return err
	}
	defer s.data.reset()
	_, _ = s.data.WriteString(text)
	return s.event(event, s.data.b, fields)
}

// Comment sends a comment line, which browsers ignore. An empty text sends
// a bare colon. Comments keep idle connections open through proxies and
// keep a write deadline from passing; [Loop] sends "keep-alive" comments.
//
// It returns an error wrapping [ErrField] when text contains CR or LF,
// [ErrEnded] after [Stream.End], and an error wrapping [ErrWrite] when the
// connection fails.
func (s *Stream) Comment(text string) error {
	if s.ended {
		return ErrEnded
	}
	if strings.ContainsAny(text, "\r\n") {
		return fmt.Errorf("%w: comment contains a line break", ErrField)
	}
	defer s.frame.reset()
	s.frame.grow(len(": \n\n") + len(text))
	s.frame.b = appendField(s.frame.b, "", text)
	s.frame.b = append(s.frame.b, '\n')
	return s.write(s.frame.b)
}

// End sends the [EventEnd] event with the rendered node as its data, and
// marks the stream as ended: later sends return [ErrEnded]. With [Region],
// the fragment replaces the content of the live region and the browser
// closes the connection without reconnecting. The handler should return
// after End.
//
// When node fails to render, End still sends the end event, with empty
// data, so the browser stops reconnecting; it then returns the render
// error. It also returns an error wrapping [ErrWrite] when the connection
// fails, and [ErrEnded] when the stream already ended. The response has
// started when End returns (unless the connection failed before), so a
// handler must not write an error page after it: such errors are only
// worth logging.
func (s *Stream) End(ctx context.Context, node elem.Node) error {
	if s.ended {
		return ErrEnded
	}
	renderErr, writeErr := s.end(ctx, node)
	return errors.Join(renderErr, writeErr)
}

// end implements End, keeping its two failures apart.
func (s *Stream) end(ctx context.Context, node elem.Node) (renderErr, writeErr error) {
	s.ended = true
	defer s.data.reset()
	if err := core.Render(ctx, node, &s.data); err != nil {
		s.data.reset()
		renderErr = fmt.Errorf("sse: render end event: %w", err)
	}
	return renderErr, s.event(EventEnd, s.data.b, nil)
}

// check validates an event before anything is rendered or written.
func (s *Stream) check(event string, fields []Field) error {
	if s.ended {
		return ErrEnded
	}
	if err := CheckEventName(event); err != nil {
		return err
	}
	for _, f := range fields {
		if f.err != nil {
			return f.err
		}
	}
	return nil
}

// event frames and writes one event.
func (s *Stream) event(name string, data []byte, fields []Field) error {
	defer s.frame.reset()
	size := len("event: \n\n") + len(name)
	for _, f := range fields {
		if f.name != "" {
			size += len(f.name) + len(": \n") + len(f.value)
		}
	}
	lines := 1 + bytes.Count(data, []byte{'\n'}) + bytes.Count(data, []byte{'\r'})
	size += len(data) + lines*len("data: \n")
	s.frame.grow(size) // one allocation at most: appends below do not reallocate

	b := appendField(s.frame.b, "event", name)
	for _, f := range fields {
		if f.name != "" {
			b = appendField(b, f.name, f.value)
		}
	}
	b = appendData(b, data)
	b = append(b, '\n')
	s.frame.b = b
	return s.write(b)
}

// appendField appends one "name: value" line. An empty name makes a
// comment line; an empty value omits the space after the colon.
func appendField(b []byte, name, value string) []byte {
	b = append(b, name...)
	b = append(b, ':')
	if value != "" {
		b = append(b, ' ')
		b = append(b, value...)
	}
	return append(b, '\n')
}

// appendData appends one data field per line of data. The event stream
// format ends a line at CR, LF or CRLF, so each of these splits the data.
// Empty data still gets one (empty) data field, so the browser dispatches
// the event.
func appendData(b, data []byte) []byte {
	for {
		i := bytes.IndexAny(data, "\r\n")
		line := data
		if i >= 0 {
			line = data[:i]
		}
		b = append(b, "data:"...)
		if len(line) > 0 {
			b = append(b, ' ')
			b = append(b, line...)
		}
		b = append(b, '\n')
		if i < 0 {
			return b
		}
		if data[i] == '\r' && i+1 < len(data) && data[i+1] == '\n' {
			i++
		}
		data = data[i+1:]
	}
}

// write sets the write deadline, starts the response if needed, writes b
// and flushes. An empty b only starts and flushes.
func (s *Stream) write(b []byte) error {
	if s.err != nil {
		return s.err
	}
	if !s.started && !canFlush(s.w) {
		return s.fail(errNoFlush(http.ErrNotSupported))
	}
	// The deadline comes first, before the header: middleware that sets a
	// default deadline when the response starts sees that the stream took
	// over.
	if err := s.rc.SetWriteDeadline(time.Now().Add(s.window)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return s.fail(fmt.Errorf("%w: set deadline: %w", ErrWrite, err))
	}
	if !s.started {
		s.started = true
		setHeader(s.w.Header())
		s.w.WriteHeader(http.StatusOK)
	}
	if len(b) > 0 {
		if _, err := s.w.Write(b); err != nil {
			return s.fail(fmt.Errorf("%w: %w", ErrWrite, err))
		}
	}
	if err := s.rc.Flush(); err != nil {
		if errors.Is(err, http.ErrNotSupported) {
			return s.fail(errNoFlush(err))
		}
		return s.fail(fmt.Errorf("%w: flush: %w", ErrWrite, err))
	}
	return nil
}

// errNoFlush is the error of a writer that cannot flush. It does not wrap
// ErrWrite: the client did not leave, the server is misconfigured.
func errNoFlush(err error) error {
	return fmt.Errorf("sse: response writer cannot flush: %w", err)
}

// maxUnwrap bounds the walk through wrapped writers, in case of a cycle.
const maxUnwrap = 32

// canFlush reports whether [http.ResponseController.Flush] finds a flush
// method on w: it takes the same walk through Unwrap methods. It looks at
// types only, so it writes nothing.
func canFlush(w http.ResponseWriter) bool {
	for range maxUnwrap {
		switch t := w.(type) {
		case interface{ FlushError() error }, http.Flusher:
			return true
		case interface{ Unwrap() http.ResponseWriter }:
			w = t.Unwrap()
		default:
			return false
		}
	}
	return false // a cycle would make the controller loop forever
}

// fail records a write failure: a partly written event would corrupt any
// event written after it.
func (s *Stream) fail(err error) error {
	s.err = err
	return err
}

// setHeader sets the header fields of an event stream response.
func setHeader(h http.Header) {
	h.Set("Content-Type", ContentType)
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
}

// HandleHEAD answers a HEAD request with the header of an event stream and
// status 200, and reports whether it did. [http.ServeMux] routes HEAD
// requests to GET patterns, and a stream handler should not start streaming
// for one:
//
//	if sse.HandleHEAD(w, r) {
//		return
//	}
//
// For other methods it writes nothing and returns false.
func HandleHEAD(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodHead {
		return false
	}
	setHeader(w.Header())
	w.WriteHeader(http.StatusOK)
	return true
}

// Field is an optional field of an event: [ID] or [Retry]. The zero Field
// adds nothing.
type Field struct {
	name  string
	value string
	err   error
}

// ID returns the id field of an event, which sets the browser's last event
// ID: an EventSource sends it in the Last-Event-ID header when it
// reconnects. An empty id resets it. Streams of whole fragments need no IDs;
// this package does not read Last-Event-ID. An id with CR, LF or NUL makes
// the send fail with [ErrField].
func ID(id string) Field {
	if strings.ContainsAny(id, "\r\n\x00") {
		return Field{err: fmt.Errorf("%w: id contains CR, LF or NUL", ErrField)}
	}
	return Field{name: "id", value: id}
}

// Retry returns the retry field of an event, which sets how long the
// browser waits before it reconnects after the connection closes, in whole
// milliseconds. A negative d makes the send fail with [ErrField].
func Retry(d time.Duration) Field {
	if d < 0 {
		return Field{err: fmt.Errorf("%w: negative retry delay", ErrField)}
	}
	return Field{name: "retry", value: strconv.FormatInt(d.Milliseconds(), 10)}
}

// CheckEventName returns nil if name can name an event, and otherwise an
// error wrapping [ErrEventName] that says why (without repeating the name).
//
// A name has 1 to [MaxEventNameLen] bytes of ASCII letters, digits, '-',
// '_', '.' and ':', and is neither "open" nor "error". The character set is
// what htmx can address: sse-swap and sse-close split lists at commas and
// trim spaces, and hx-trigger="sse:NAME" reads spaces, commas, brackets,
// quotes and slashes as syntax. An EventSource dispatches events named open
// and error to its own connection handlers, which the extension uses to
// reconnect. Line breaks would end the event field early.
func CheckEventName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: empty", ErrEventName)
	}
	if len(name) > MaxEventNameLen {
		return fmt.Errorf("%w: longer than %d bytes", ErrEventName, MaxEventNameLen)
	}
	for i := 0; i < len(name); i++ {
		if !eventNameByte(name[i]) {
			return fmt.Errorf("%w: byte %d is not a letter, digit, '-', '_', '.' or ':'", ErrEventName, i)
		}
	}
	if name == "open" || name == "error" {
		return fmt.Errorf("%w: reserved for connection events", ErrEventName)
	}
	return nil
}

func eventNameByte(c byte) bool {
	switch {
	case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		return true
	case c == '-', c == '_', c == '.', c == ':':
		return true
	}
	return false
}

// maxRetained bounds the capacity of a buffer kept between events: a large
// fragment should not pin its buffer for the lifetime of a stream.
const maxRetained = 64 << 10

// buffer is a byte buffer that clears its storage when it grows and when it
// is reset.
type buffer struct {
	b []byte
}

// WriteString appends v. It never fails.
func (w *buffer) WriteString(v string) (int, error) {
	w.grow(len(v))
	w.b = append(w.b, v...)
	return len(v), nil
}

// grow makes room for n more bytes, clearing the old storage when it
// moves.
func (w *buffer) grow(n int) {
	if n <= cap(w.b)-len(w.b) {
		return
	}
	next := make([]byte, len(w.b), max(2*cap(w.b), len(w.b)+n, 512))
	copy(next, w.b)
	clear(w.b[:cap(w.b)])
	w.b = next
}

// reset clears the storage and empties the buffer, dropping storage larger
// than maxRetained.
func (w *buffer) reset() {
	clear(w.b[:cap(w.b)])
	if cap(w.b) > maxRetained {
		w.b = nil
		return
	}
	w.b = w.b[:0]
}
