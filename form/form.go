package form

import (
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"slices"
)

// Default limits, used for the zero fields of [Limits].
const (
	// DefaultBody bounds the whole request body: 1 MiB.
	DefaultBody = 1 << 20
	// DefaultField bounds each field name and each decoded value: 64 KiB.
	DefaultField = 64 << 10
	// DefaultFields bounds the number of fields (name and value pairs, or
	// multipart parts): 1000.
	DefaultFields = 1000
)

// Media types accepted by [Read].
const (
	Multipart  = "multipart/form-data"
	URLEncoded = "application/x-www-form-urlencoded"
)

// maxBoundary is the longest multipart boundary RFC 2046 allows.
const maxBoundary = 70

// Sentinel errors returned by [Read], possibly wrapped with more detail.
// Callers match them with [errors.Is] and must treat them as immutable.
var (
	// ErrMalformed reports a body that is not a well-formed form of its
	// declared type, a field without a name, or a body that could not be
	// read to its end.
	ErrMalformed = errors.New("form: malformed form")
	// ErrTooLarge reports a body, a field name or a value over its limit.
	// When the body limit was hit, the error also wraps the
	// [*http.MaxBytesError], so [errors.As] finds it.
	ErrTooLarge = errors.New("form: form too large")
	// ErrTooManyFields reports a form with more fields than allowed.
	ErrTooManyFields = errors.New("form: too many fields")
	// ErrUnsupportedType reports a request whose Content-Type is missing or
	// is neither [Multipart] nor [URLEncoded].
	ErrUnsupportedType = errors.New("form: unsupported content type")
	// ErrFile reports a multipart part that carries a file (any part with a
	// filename parameter, including the empty filename of a file input
	// without a selected file).
	ErrFile = errors.New("form: file parts are not accepted")
)

// Limits bound a form before anything is allocated for it. A zero field
// means its default ([DefaultBody], [DefaultField], [DefaultFields]); a
// negative field is a programming error and makes [Read] panic.
type Limits struct {
	// Body bounds the whole request body in bytes, including multipart
	// boundaries and part headers, or the percent-encoded text.
	Body int64
	// Field bounds each field name and each value in bytes, after decoding.
	Field int64
	// Fields bounds the number of fields. A repeated name counts once per
	// value.
	Fields int
}

// withDefaults returns l with zero fields replaced by the defaults. It
// panics on negative fields.
func (l Limits) withDefaults() Limits {
	if l.Body < 0 || l.Field < 0 || l.Fields < 0 {
		panic("form: negative limit")
	}
	if l.Body == 0 {
		l.Body = DefaultBody
	}
	if l.Field == 0 {
		l.Field = DefaultField
	}
	if l.Fields == 0 {
		l.Fields = DefaultFields
	}
	return l
}

// Values holds the fields of a form as owned byte slices. A name may repeat;
// its values keep the order of the request.
//
// A Values is not safe for concurrent use. Slices returned by [Values.Bytes]
// are borrowed: they stay valid until the value is taken or the form is
// zeroed, and the caller must not keep them longer. The zero Values is an
// empty form, and a nil *Values behaves as one.
type Values struct {
	m map[string][][]byte
}

// Bytes returns the first value of the named field, borrowed from v: nil if
// the field is absent, and a non-nil empty slice if it is present but empty.
// The slice is cleared by [Values.Zero]; do not retain it past that call.
func (v *Values) Bytes(name string) []byte {
	if v == nil {
		return nil
	}
	if vs := v.m[name]; len(vs) > 0 {
		return vs[0]
	}
	return nil
}

// String returns a copy of the first value of the named field as a string,
// or "" if the field is absent. Use it for public fields only: the string
// cannot be cleared. Use [Values.Bytes] or [Values.Take] for secrets.
func (v *Values) String(name string) string {
	return string(v.Bytes(name))
}

// Strings returns copies of every value of the named field, in request
// order, or nil if the field is absent. Like [Values.String], it is meant
// for public fields.
func (v *Values) Strings(name string) []string {
	if v == nil {
		return nil
	}
	vs := v.m[name]
	if len(vs) == 0 {
		return nil
	}
	out := make([]string, len(vs))
	for i, b := range vs {
		out[i] = string(b)
	}
	return out
}

// Has reports whether the named field is present, even with an empty value
// (an empty text input, or a checked checkbox, which browsers send as "on"
// when it has no value attribute).
func (v *Values) Has(name string) bool {
	return v != nil && len(v.m[name]) > 0
}

// Take removes the first value of the named field from v and transfers it
// to the caller, or returns nil if the field is absent. The caller owns the
// slice and should clear it after use; its length equals its capacity, so
// clear(b) clears all of it. [Values.Zero] no longer reaches a taken value.
func (v *Values) Take(name string) []byte {
	if v == nil {
		return nil
	}
	vs := v.m[name]
	if len(vs) == 0 {
		return nil
	}
	b := vs[0]
	vs[0] = nil
	if len(vs) == 1 {
		delete(v.m, name)
	} else {
		v.m[name] = vs[1:]
	}
	return b
}

// Names returns the names of the fields present in v, sorted.
func (v *Values) Names() []string {
	if v == nil || len(v.m) == 0 {
		return nil
	}
	names := make([]string, 0, len(v.m))
	for name := range v.m {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// Len returns the number of values in v, counting each value of a repeated
// name.
func (v *Values) Len() int {
	if v == nil {
		return 0
	}
	n := 0
	for _, vs := range v.m {
		n += len(vs)
	}
	return n
}

// Zero clears every value still held by v and empties it. Slices borrowed
// through [Values.Bytes] are cleared too; taken values are not. Zero is
// idempotent, and safe on a nil *Values. Call it (typically deferred) once
// the handler is done with the form.
func (v *Values) Zero() {
	if v == nil {
		return
	}
	for name, vs := range v.m {
		for _, b := range vs {
			clear(b)
		}
		clear(vs)
		delete(v.m, name)
	}
}

// add stores an owned value under name.
func (v *Values) add(name string, value []byte) {
	if v.m == nil {
		v.m = make(map[string][][]byte)
	}
	v.m[name] = append(v.m[name], value)
}

// Read reads the form in the body of r, which must be [Multipart] or
// [URLEncoded], within limits (see [Limits] for the defaults of its zero
// fields). The URL query string is ignored.
//
// The body is wrapped in [http.MaxBytesReader] with w, which on an oversized
// body tells the server to close the connection after the response; w may be
// nil. r.Body itself is not replaced, and is left partly read on error.
//
// On success the caller owns the returned *Values and should call
// [Values.Zero] when done with it. On error Read returns nil, after clearing
// everything it read; the error matches one of the package's sentinel errors
// and never contains request data.
func Read(w http.ResponseWriter, r *http.Request, limits Limits) (*Values, error) {
	return newReader(limits).read(w, r)
}

// reader holds the state of one [Read].
type reader struct {
	limits Limits
	// fieldMax is the effective field limit: limits.Field, but no more
	// than the body limit (a field cannot be larger than its body).
	fieldMax int
	// scratch accumulates the field being read. It grows up to fieldMax+1
	// bytes; every buffer it outgrows is cleared, and the
	// last one is cleared when the read ends.
	scratch []byte
	values  *Values
	fields  int
	// observe, when set (by tests only), is called with every buffer the
	// reader allocates, so tests can check that all of them end up cleared.
	observe func([]byte)
}

func newReader(limits Limits) *reader {
	limits = limits.withDefaults()
	fieldMax := min(limits.Field, limits.Body, math.MaxInt32)
	return &reader{limits: limits, fieldMax: int(fieldMax), values: new(Values)}
}

func (rd *reader) read(w http.ResponseWriter, r *http.Request) (_ *Values, err error) {
	defer func() {
		clear(rd.scratch[:cap(rd.scratch)])
		rd.scratch = nil
		if err != nil {
			rd.values.Zero()
		}
	}()
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		return nil, fmt.Errorf("%w: missing", ErrUnsupportedType)
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, fmt.Errorf("%w: unparsable", ErrUnsupportedType)
	}
	var src io.Reader = http.NoBody
	if r.Body != nil {
		src = r.Body
	}
	body := &bodyReader{r: http.MaxBytesReader(w, io.NopCloser(src), rd.limits.Body)}
	switch mediaType {
	case URLEncoded:
		err = rd.readURLEncoded(body)
	case Multipart:
		boundary := params["boundary"]
		if boundary == "" || len(boundary) > maxBoundary {
			return nil, fmt.Errorf("%w: bad multipart boundary", ErrMalformed)
		}
		err = rd.readMultipart(body, boundary)
	default:
		return nil, fmt.Errorf("%w: not a form", ErrUnsupportedType)
	}
	if err != nil {
		return nil, body.classify(err)
	}
	return rd.values, nil
}

// bodyReader records the first error of the underlying body other than
// io.EOF, so that a failure of the transport or of the body limit can be
// told apart from a parse error, whatever the parser made of it.
type bodyReader struct {
	r   io.Reader
	err error
}

func (b *bodyReader) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	if err != nil && err != io.EOF && b.err == nil {
		b.err = err
	}
	return n, err
}

// classify turns err, returned by a parser, into the error of [Read]. Errors
// of the body take precedence; parser errors that are not already
// classified become [ErrMalformed] without their text, which may quote the
// request.
func (b *bodyReader) classify(err error) error {
	if b.err != nil {
		if tooBig, ok := errors.AsType[*http.MaxBytesError](b.err); ok {
			return fmt.Errorf("%w: body over %d bytes: %w", ErrTooLarge, tooBig.Limit, tooBig)
		}
		return fmt.Errorf("%w: read body: %w", ErrMalformed, b.err)
	}
	for _, sentinel := range []error{ErrMalformed, ErrTooLarge, ErrTooManyFields, ErrFile} {
		if errors.Is(err, sentinel) {
			return err
		}
	}
	if errors.Is(err, multipart.ErrMessageTooLarge) {
		return fmt.Errorf("%w: multipart headers", ErrTooLarge)
	}
	return fmt.Errorf("%w: bad syntax", ErrMalformed)
}

// countField counts one more field, failing past the limit.
func (rd *reader) countField() error {
	if rd.fields >= rd.limits.Fields {
		return fmt.Errorf("%w: more than %d", ErrTooManyFields, rd.limits.Fields)
	}
	rd.fields++
	return nil
}

// grow makes room for at least one more byte in rd.scratch, keeping its
// contents, without exceeding fieldMax+1 bytes of capacity. The outgrown
// buffer is cleared.
func (rd *reader) grow() {
	limit := rd.fieldMax + 1
	old := rd.scratch
	size := max(2*cap(old), 512)
	size = min(size, limit)
	next := make([]byte, len(old), size)
	if rd.observe != nil {
		rd.observe(next[:cap(next)])
	}
	copy(next, old)
	clear(old[:cap(old)])
	rd.scratch = next
}

// own returns an exactly sized copy of b, which the caller then owns.
func (rd *reader) own(b []byte) []byte {
	out := make([]byte, len(b))
	if rd.observe != nil {
		rd.observe(out)
	}
	copy(out, b)
	return out
}

func errFieldTooLarge(limit int64) error {
	return fmt.Errorf("%w: field over %d bytes", ErrTooLarge, limit)
}
