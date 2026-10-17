package webuitest

import (
	"bytes"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthtml"
)

// ErrUnterminated reports an event stream that ends inside a line or an
// event, before the blank line that dispatches it. A browser discards
// such an event; a server that frames events correctly never sends one.
var ErrUnterminated = errors.New("webuitest: event stream ends inside an event")

// ErrEventTooLarge reports a line or an event of an event stream longer
// than [MaxEventSize].
var ErrEventTooLarge = errors.New("webuitest: event stream event too large")

// MaxEventSize bounds the size of one line and of the data of one event
// that [ParseEvents] and [Stream] accept: 8 MiB, like the body limit of
// cthttp.
const MaxEventSize = 8 << 20

// Event is a server-sent event as a browser's EventSource dispatches it.
type Event struct {
	// Name is the event type: the last event field of the event, or
	// "message" when it has none.
	Name string
	// Data is the event's data: its data fields joined with LF.
	Data string
	// ID is the last event ID when the event was dispatched: the value of
	// the last id field so far in the stream (ids with NUL are ignored).
	ID string
	// Retry is the reconnection time that a retry field of the event set,
	// zero when the event has none.
	Retry time.Duration
}

// Fragment returns a subject for the event's data parsed as HTML in the
// context of an element with the tag contextTag (e.g. "div" or "tbody"),
// as the htmx SSE extension swaps it into the page.
func (e Event) Fragment(contextTag string, opts ...ct.Option) *ct.Subject {
	return cthtml.FragmentString(contextTag, e.Data, opts...)
}

// ParseEvents parses a complete event stream, as the WHATWG HTML
// "text/event-stream" format defines it: a leading byte order mark is
// skipped; lines end at CR, LF or CRLF; a line starting with a colon is a
// comment, counted but not returned; a field's value starts after the
// colon and one optional space; data fields join with LF; an event
// without data fields is not dispatched; the event type defaults to
// "message"; an id containing NUL is ignored; a retry value must be ASCII
// digits; unknown fields are ignored; invalid UTF-8 becomes U+FFFD.
//
// A stream that ends inside a line or an event fails with
// [ErrUnterminated], and a line or event over [MaxEventSize] with
// [ErrEventTooLarge]. In both cases events holds the complete events
// before the failure, for diagnostics.
func ParseEvents(body []byte) (events []Event, comments int, err error) {
	p := eventParser{max: MaxEventSize}
	err = p.feed(body, func(e Event) bool {
		events = append(events, e)
		return true
	})
	if err == nil {
		err = p.end()
	}
	return events, p.comments, err
}

// errStopped ends parsing when the consumer of events stops.
var errStopped = errors.New("webuitest: event consumer stopped")

// eventParser is an incremental parser of the event stream format.
type eventParser struct {
	max      int
	started  bool // the first line was seen (and its byte order mark dropped)
	line     []byte
	lastCR   bool
	data     []byte
	typ      string
	lastID   string
	retry    time.Duration
	pending  bool // a field line was seen since the last blank line
	comments int
}

// feed parses b, calling emit for every dispatched event; emit returning
// false stops parsing with errStopped.
func (p *eventParser) feed(b []byte, emit func(Event) bool) error {
	for len(b) > 0 {
		if p.lastCR && b[0] == '\n' {
			// The LF of a CRLF whose CR ended the previous line.
			b = b[1:]
		}
		p.lastCR = false
		i := bytes.IndexAny(b, "\r\n")
		if i < 0 {
			if len(p.line)+len(b) > p.max {
				return ErrEventTooLarge
			}
			p.line = append(p.line, b...)
			return nil
		}
		if len(p.line)+i > p.max {
			return ErrEventTooLarge
		}
		p.line = append(p.line, b[:i]...)
		p.lastCR = b[i] == '\r'
		b = b[i+1:]
		if err := p.endLine(emit); err != nil {
			return err
		}
	}
	return nil
}

// end reports whether the stream ended at an event boundary.
func (p *eventParser) end() error {
	if len(p.line) > 0 || p.pending {
		return ErrUnterminated
	}
	return nil
}

func (p *eventParser) endLine(emit func(Event) bool) error {
	line := p.line
	p.line = p.line[:0]
	if !p.started {
		p.started = true
		line = bytes.TrimPrefix(line, []byte("\xEF\xBB\xBF"))
	}
	switch {
	case len(line) == 0:
		return p.dispatch(emit)
	case line[0] == ':':
		p.comments++
		return nil
	}
	p.pending = true
	field, value := line, []byte(nil)
	if j := bytes.IndexByte(line, ':'); j >= 0 {
		field, value = line[:j], line[j+1:]
		value = bytes.TrimPrefix(value, []byte(" "))
	}
	switch string(field) {
	case "event":
		p.typ = text(value)
	case "data":
		if len(p.data)+len(value)+1 > p.max {
			return ErrEventTooLarge
		}
		p.data = append(p.data, value...)
		p.data = append(p.data, '\n')
	case "id":
		if bytes.IndexByte(value, 0) < 0 {
			p.lastID = text(value)
		}
	case "retry":
		if len(value) > 0 && strings.Trim(string(value), "0123456789") == "" {
			if n, err := strconv.ParseInt(string(value), 10, 64); err == nil && n <= math.MaxInt64/int64(time.Millisecond) {
				p.retry = time.Duration(n) * time.Millisecond
			}
		}
	}
	return nil
}

func (p *eventParser) dispatch(emit func(Event) bool) error {
	p.pending = false
	data, typ, retry := p.data, p.typ, p.retry
	p.data, p.typ, p.retry = p.data[:0], "", 0
	if len(data) == 0 {
		return nil
	}
	if typ == "" {
		typ = "message"
	}
	e := Event{Name: typ, Data: text(data[:len(data)-1]), ID: p.lastID, Retry: retry}
	if !emit(e) {
		return errStopped
	}
	return nil
}

// text converts bytes of the stream to a string, replacing invalid UTF-8.
func text(b []byte) string {
	return strings.ToValidUTF8(string(b), "\uFFFD")
}
