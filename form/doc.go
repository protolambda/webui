// Package form reads HTML form submissions from request bodies into owned,
// zeroable byte slices, bounded before anything is allocated.
//
// [Read] accepts the two encodings a browser uses for forms without files:
// multipart/form-data and application/x-www-form-urlencoded (the default of
// both plain HTML forms and htmx). It never looks at the URL query string,
// and it refuses file parts: uploads need a streaming reader of their own.
//
// # Bounds
//
// Every read is bounded by [Limits]: the whole body (through
// [http.MaxBytesReader]), each field name and each decoded value, and the
// number of fields. A value is read into a scratch buffer that never grows
// past the field limit plus one byte, so an oversized field is detected
// without allocating more than that; the zero [Limits] gives the documented
// defaults. Violations return [ErrTooLarge] or [ErrTooManyFields]; a body
// over the body limit also matches [*http.MaxBytesError] with [errors.As].
//
// # Owned values
//
// Values are copied into exactly sized byte slices that the [Values] owns,
// rather than strings: a password or another secret field can then be used
// and cleared ([Values.Take], [Values.Zero]) instead of lingering as an
// immutable string until the garbage collector reuses its memory. Read
// clears its scratch buffers before it returns, and on every error path it
// also clears the values read so far.
//
// Zeroing is best effort within Go's runtime model: the network stack, TLS
// and the multipart reader keep parts of the body in buffers that this
// package cannot reach, and copies the caller makes are the caller's to
// clear. What the package guarantees is that it keeps no copy of a value of
// its own once the value is zeroed or taken: grown scratch buffers are
// cleared before they are dropped, and so is every buffer on return.
//
// # Errors
//
// Errors are classified by the sentinels [ErrMalformed], [ErrTooLarge],
// [ErrTooManyFields], [ErrUnsupportedType] and [ErrFile]. Error messages
// never contain field names, values or other parts of the request, so they
// are safe to log. A failure to read the body itself (a dropped connection,
// a read deadline) is reported as [ErrMalformed] wrapping the read error.
//
// # Cross-site request forgery
//
// Neither encoding protects against cross-site request forgery: a page of
// another site can submit a form with either of them, multipart/form-data
// included, because both are CORS-safelisted content types that need no
// preflight. The defences are an origin check on every state-changing
// request (for example the same-origin policy middleware of this module) and
// a CSRF token in every form (package session). Read only parses.
package form
