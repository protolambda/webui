package form

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"slices"
)

// Default limits, used for the zero fields of [Limits]. Note the units:
// DefaultBody and DefaultField are sizes in bytes, DefaultFields is a count.
const (
	// DefaultBody bounds the whole request body: 1 MiB.
	DefaultBody = 1 << 20
	// DefaultField bounds the size of each field name and each decoded
	// value: 64 KiB.
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
	// declared type, a field without a name, a multipart part whose name is
	// ambiguous (two Content-Disposition headers, or both a name and an
	// RFC 2231 name* parameter), or a body that could not be read to its
	// end.
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
	// ErrFile reports a multipart part that carries a file: any part with a
	// filename parameter in any form (filename, filename*, filename*0, ...),
	// whether or not its value can be decoded, including the empty filename
	// of a file input without a selected file.
	ErrFile = errors.New("form: file parts are not accepted")
)

// Limits bound a form as it is read; see the package documentation for
// what each bound guarantees. A zero field means its default
// ([DefaultBody], [DefaultField], [DefaultFields]); a negative field is a
// programming error and makes [Read] panic.
//
// Mind the units: Body and Field are sizes in bytes, Fields is a count.
// Both kinds accept untyped constants, so Limits{Fields: 64 << 10} compiles
// but allows 65536 fields and keeps the default field size.
type Limits struct {
	// Body bounds the size of the whole request body in bytes: multipart
	// boundaries, part headers and anything after the closing delimiter
	// included, or the percent-encoded text. A request that declares a
	// larger Content-Length is refused before its body is read.
	Body int64
	// Field bounds the size in bytes of each field name and each value,
	// after decoding. It also bounds the header of each multipart part, to
	// about 3*Field + 4 KiB before the header is parsed.
	Field int64
	// Fields bounds the number of fields (a count, not a size). A repeated
	// name counts once per value.
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
//
// Formatting or logging a Values never shows its names or values: see
// [Values.Format] and [Values.LogValue].
type Values struct {
	m map[string][][]byte
}

// Format implements [fmt.Formatter]. For every verb and flag (%v, %+v, %#v,
// %s, %q, %x, ...) it writes only the number of values, as in
// "form.Values(len=2)", never a field name or value, so printing a form or
// an error or struct that contains one cannot leak a secret field. The
// receiver is a value so that a dereferenced Values and a Values stored in
// an exported struct field are covered too; fmt prints a nil *Values as
// <nil>. fmt cannot call methods through an unexported struct field, so do
// not format structs that keep a Values in one.
func (v Values) Format(f fmt.State, _ rune) {
	_, _ = io.WriteString(f, v.summary())
}

// LogValue implements [slog.LogValuer] with the same redacted text as
// [Values.Format], so every slog handler (JSON included) logs only the
// number of values. It is safe on a nil *Values.
func (v *Values) LogValue() slog.Value {
	return slog.StringValue(v.summary())
}

// summary describes v without its names or values.
func (v *Values) summary() string {
	return fmt.Sprintf("form.Values(len=%d)", v.Len())
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
// A request whose declared Content-Length exceeds the body limit is refused
// before its body is read. Otherwise the body is wrapped in
// [http.MaxBytesReader] with w, which on an oversized body tells the server
// to close the connection after the response; w may be nil. r.Body itself
// is not replaced: on success it has been read to its end (for multipart,
// past the closing delimiter, so that trailing bytes count against the body
// limit too), and on error it is left partly read or unread.
//
// On success the caller owns the returned *Values and should call
// [Values.Zero] when done with it. On error Read returns nil, after clearing
// everything it read; the error matches one of the package's sentinel errors
// and its message never contains request data.
func Read(w http.ResponseWriter, r *http.Request, limits Limits) (*Values, error) {
	return newReader(limits).read(w, r)
}

// reader holds the state of one [Read].
type reader struct {
	limits Limits
	// fieldMax is the effective field limit: limits.Field, but no more
	// than the body limit (a field cannot be larger than its body), and
	// less than math.MaxInt so that fieldMax+1 is an int on every platform.
	fieldMax int
	// scratch accumulates the field being read. It grows up to fieldMax+1
	// bytes; every buffer it outgrows is cleared, and the
	// last one is cleared when the read ends.
	scratch []byte
	values  *Values
	fields  int
	// observe, when set (by tests only), is called with every buffer the
	// reader allocates, so tests can check that all of them end up cleared.
	// chunk marks the fixed-size read buffer of a urlencoded body, which
	// the field limit does not bound.
	observe func(b []byte, chunk bool)
}

func newReader(limits Limits) *reader {
	limits = limits.withDefaults()
	fieldMax := min(limits.Field, limits.Body, math.MaxInt-1)
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
	boundary := params["boundary"]
	switch mediaType {
	case URLEncoded:
	case Multipart:
		if boundary == "" || len(boundary) > maxBoundary {
			return nil, fmt.Errorf("%w: bad multipart boundary", ErrMalformed)
		}
	default:
		return nil, fmt.Errorf("%w: not a form", ErrUnsupportedType)
	}
	// The server enforces a declared length, so a body that declares more
	// than the limit cannot fit: refuse it before reading any of it (with
	// "Expect: 100-continue", before the client even sends it).
	if r.ContentLength > rd.limits.Body {
		return nil, errBodyTooLarge(&http.MaxBytesError{Limit: rd.limits.Body})
	}
	var src io.Reader = http.NoBody
	if r.Body != nil {
		src = r.Body
	}
	body := &bodyReader{r: http.MaxBytesReader(w, io.NopCloser(src), rd.limits.Body)}
	if mediaType == URLEncoded {
		err = rd.readURLEncoded(body)
	} else {
		err = rd.readMultipart(body, boundary)
	}
	if err == nil && body.err != nil {
		// A parser that succeeded despite a failed read (it should not):
		// the failure still decides the result.
		err = body.err
	}
	if err != nil {
		return nil, body.classify(err)
	}
	return rd.values, nil
}

// bodyReader records the first error of the underlying body other than
// io.EOF, so that a failure of the transport or of the body limit can be
// told apart from a parse error, whatever the parser made of it.
//
// While the multipart parser reads part headers, a budget also caps the
// bytes it may pull ([bodyReader.limit]), so that an oversized header is
// refused before the parser has buffered all of it.
type bodyReader struct {
	r   io.Reader
	err error
	// limited reports that a budget applies; left is what remains of it.
	limited bool
	left    int64
}

// errPartHeaderTooLarge reports a multipart part header over its budget.
var errPartHeaderTooLarge = fmt.Errorf("%w: multipart part header", ErrTooLarge)

// limit caps the bytes that later reads may return to n in total, until
// [bodyReader.unlimit].
func (b *bodyReader) limit(n int64) { b.limited, b.left = true, n }

// unlimit lifts the cap set by [bodyReader.limit]; the body limit stays.
func (b *bodyReader) unlimit() { b.limited = false }

func (b *bodyReader) Read(p []byte) (int, error) {
	if b.limited {
		if b.left <= 0 {
			if b.err == nil {
				b.err = errPartHeaderTooLarge
			}
			return 0, errPartHeaderTooLarge
		}
		if int64(len(p)) > b.left {
			p = p[:b.left]
		}
	}
	n, err := b.r.Read(p)
	if b.limited {
		b.left -= int64(n)
	}
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
		if b.err == errPartHeaderTooLarge {
			return b.err
		}
		if tooBig, ok := errors.AsType[*http.MaxBytesError](b.err); ok {
			return errBodyTooLarge(tooBig)
		}
		return fmt.Errorf("%w: %w", ErrMalformed, readError{b.err})
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

// readError reports a failure to read the body. Its text is fixed because
// the text of the cause may quote the request (net/http quotes a malformed
// line of a chunked trailer, and a caller's body wrapper may do anything);
// the cause stays reachable with [errors.Is] and [errors.As].
type readError struct{ cause error }

func (readError) Error() string { return "read body failed" }

func (e readError) Unwrap() error { return e.cause }

// errBodyTooLarge reports a body over the body limit. The text of a
// [*http.MaxBytesError] is fixed, so it may be repeated.
func errBodyTooLarge(tooBig *http.MaxBytesError) error {
	return fmt.Errorf("%w: body over %d bytes: %w", ErrTooLarge, tooBig.Limit, tooBig)
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
// buffer is cleared. Callers only grow a scratch buffer whose capacity is
// at most fieldMax.
func (rd *reader) grow() {
	old := rd.scratch
	next := make([]byte, len(old), growSize(cap(old), rd.fieldMax+1))
	if rd.observe != nil {
		rd.observe(next[:cap(next)], false)
	}
	copy(next, old)
	clear(old[:cap(old)])
	rd.scratch = next
}

// growSize returns the next capacity of a scratch buffer of capacity c < limit:
// double c, but at least 512 bytes and at most limit. It cannot overflow an
// int, whatever its size on the platform: 2*c is only computed when it is
// less than limit.
func growSize(c, limit int) int {
	if c >= limit/2 {
		return limit
	}
	return max(2*c, min(512, limit))
}

// own returns an exactly sized copy of b, which the caller then owns.
func (rd *reader) own(b []byte) []byte {
	out := make([]byte, len(b))
	if rd.observe != nil {
		rd.observe(out, false)
	}
	copy(out, b)
	return out
}

func errFieldTooLarge(limit int64) error {
	return fmt.Errorf("%w: field over %d bytes", ErrTooLarge, limit)
}
