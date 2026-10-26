package form

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"net/textproto"
	"strings"
	"unicode"
)

// headerSlack is the room a multipart part header gets beyond three times
// the field limit: the delimiter line, the header names and the syntax of
// the Content-Disposition, and a short Content-Type line.
const headerSlack = 4096

// readMultipart parses a multipart/form-data body, whose boundary the
// caller checked: not empty, at most 70 bytes, and without a colon (see
// [endReader]).
//
// Parts are read raw ([multipart.Reader.NextRawPart]): a
// Content-Transfer-Encoding header, deprecated for forms by RFC 7578 and
// never sent by browsers, is not decoded. Each part must have a single
// Content-Disposition of form-data with a non-empty name and no filename
// parameter. The body must end with the closing delimiter: a truncated body
// is malformed ([errTruncated]), not a shorter form. What follows the
// closing delimiter is read and dropped, so that it counts against the body
// limit.
//
// While it parses the header of a part, the multipart reader may pull no
// more than [reader.headerBudget] bytes from the body (plus what its
// 4 KiB read-ahead already holds), so that a header far larger than any
// field it can name is refused before it is buffered and parsed. This bounds
// the memory of each part's header; the allocations of header parsing over
// a whole body remain proportional to the body.
func (rd *reader) readMultipart(src *bodyReader, boundary string) error {
	mr := multipart.NewReader(&endReader{r: src, closing: "--" + boundary + "--"}, boundary)
	budget := rd.headerBudget()
	for {
		src.limit(budget)
		part, err := mr.NextRawPart()
		src.unlimit()
		if err == io.EOF {
			// The end of the body is hidden from the multipart reader
			// (endReader), so a bare io.EOF marks the closing delimiter.
			// The epilogue carries no fields but still counts against the
			// body limit.
			_, err := io.Copy(io.Discard, src)
			return err
		}
		if err != nil {
			return err
		}
		if err := rd.countField(); err != nil {
			return err
		}
		name, err := partName(part.Header)
		if err != nil {
			return err
		}
		if len(name) > rd.fieldMax {
			return errFieldTooLarge(rd.limits.Field)
		}
		if err := rd.readPart(part); err != nil {
			return err
		}
		rd.values.Add(name, rd.own(rd.scratch))
		rd.resetScratch()
	}
}

// errTruncated reports a multipart body that ends before its closing
// delimiter.
var errTruncated = fmt.Errorf("%w: multipart body ends before the closing delimiter", ErrMalformed)

// endReader passes a multipart body on to the [multipart.Reader] but hides
// its end: where the body ends, a read returns [errTruncated], unless the
// last line of the body (what follows the last '\n') is the closing
// delimiter, "--boundary--" and optional spaces or tabs; only then does it
// return io.EOF.
//
// NextRawPart returns a bare io.EOF in three cases. Two end the form: a
// closing delimiter line with a line end, which involves no io.EOF of the
// body, and one at the very end of the body, which the multipart reader
// tells from a cut-off line by the io.EOF after it. The third is a
// truncated body: where the body ends at the start of a header line of the
// next part, net/textproto passes the io.EOF up unchanged, and the form
// would read as complete without that part. With the end of the body
// hidden, that and every other early end fails with errTruncated.
//
// The io.EOF passed on does not reopen the third case: a body that ends on
// a closing delimiter line does not end at the start of a line, and where
// the multipart reader reads that line as a header line, it fails for want
// of a colon. That needs a boundary without a colon, which [Read] refuses
// although RFC 2046 allows it: net/textproto would parse a closing
// delimiter such as "--x:y--" as a header line and pass the io.EOF after
// it up unchanged, so a body cut off after a delimiter line, or after a
// header line, would read as complete. Watching the bytes for the closing
// delimiter instead would not do: only the multipart reader knows where
// its lines start (in a part of a body with CRLF line ends, a bare LF does
// not end a line).
type endReader struct {
	r io.Reader
	// closing is the closing delimiter, "--" + boundary + "--".
	closing string
	// matched is how much of closing the last line read so far matches:
	// len(closing) once it is all of it, possibly followed by spaces or
	// tabs, and -1 once the line cannot match.
	matched int
}

func (e *endReader) Read(p []byte) (int, error) {
	n, err := e.r.Read(p)
	e.scan(p[:n])
	if err != io.EOF {
		return n, err
	}
	if e.matched == len(e.closing) {
		return n, io.EOF
	}
	return n, errTruncated
}

// scan follows the last line of the body through the bytes b that come
// next.
func (e *endReader) scan(b []byte) {
	if i := bytes.LastIndexByte(b, '\n'); i >= 0 {
		e.matched = 0
		b = b[i+1:]
	}
	for _, c := range b {
		switch {
		case e.matched < 0:
			return
		case e.matched < len(e.closing):
			if c == e.closing[e.matched] {
				e.matched++
			} else {
				e.matched = -1
			}
		case c != ' ' && c != '\t':
			e.matched = -1
		}
	}
}

// headerBudget returns the bytes the multipart reader may pull from the
// body while it parses one part header: three times the field limit, as a
// name at the limit takes up to three bytes per byte when percent-encoded
// (RFC 2231), plus [headerSlack].
func (rd *reader) headerBudget() int64 {
	field := int64(rd.fieldMax)
	if field > (math.MaxInt64-headerSlack)/3 {
		return math.MaxInt64
	}
	return 3*field + headerSlack
}

// partName returns the field name of a part, refusing file parts and parts
// that are not form fields.
func partName(header textproto.MIMEHeader) (string, error) {
	dispositions := header.Values("Content-Disposition")
	switch {
	case len(dispositions) == 0 || dispositions[0] == "":
		return "", fmt.Errorf("%w: part without Content-Disposition", ErrMalformed)
	case len(dispositions) > 1:
		// Parsers disagree on which one counts: refuse the ambiguity.
		return "", fmt.Errorf("%w: repeated Content-Disposition", ErrMalformed)
	}
	disposition := dispositions[0]
	kind, params, err := mime.ParseMediaType(disposition)
	if err != nil {
		return "", fmt.Errorf("%w: bad Content-Disposition", ErrMalformed)
	}
	if kind != "form-data" {
		return "", fmt.Errorf("%w: part is not form-data", ErrMalformed)
	}
	raw := scanParams(disposition)
	// The presence of the parameter marks a file part, even when empty: a
	// file input without a selected file sends filename="". ParseMediaType
	// drops the RFC 2231 forms it cannot decode (an unsupported charset, a
	// continuation that does not start at 0), so the raw names decide too.
	if _, ok := params["filename"]; ok || raw.filename {
		return "", ErrFile
	}
	if raw.plainName && raw.extendedName {
		// ParseMediaType lets name* override name; other parsers may not.
		return "", fmt.Errorf("%w: name given twice", ErrMalformed)
	}
	name := params["name"]
	if name == "" {
		return "", fmt.Errorf("%w: part without a name", ErrMalformed)
	}
	return name, nil
}

// rawParams describes the parameter names of a header value as written,
// before RFC 2231 decoding.
type rawParams struct {
	// filename reports a filename parameter in any form: filename,
	// filename*, filename*0, filename*0*, ...
	filename bool
	// plainName and extendedName report a name parameter, and one in an
	// RFC 2231 form (name*, name*0, ...).
	plainName, extendedName bool
}

// scanParams scans the parameter names of v, a header value that
// [mime.ParseMediaType] accepted, with the same grammar: the media type up
// to the first ';', then parameters "; name=value" where the value is a
// token or a quoted string, white space around the separators, and an
// optional trailing ';'. Should v not have that shape after all, the scan
// fails closed and reports a filename.
func scanParams(v string) rawParams {
	var out rawParams
	base, _, _ := strings.Cut(v, ";")
	rest := v[len(base):]
	for {
		rest = trimLeftSpace(rest)
		if rest == "" {
			return out
		}
		var ok bool
		if rest, ok = strings.CutPrefix(rest, ";"); !ok {
			return rawParams{filename: true}
		}
		rest = trimLeftSpace(rest)
		if rest == "" {
			return out // a trailing ';'
		}
		var key string
		key, rest = cutToken(rest)
		if key == "" {
			return rawParams{filename: true}
		}
		key = strings.ToLower(key)
		if rest, ok = strings.CutPrefix(trimLeftSpace(rest), "="); !ok {
			return rawParams{filename: true}
		}
		if rest, ok = cutValue(trimLeftSpace(rest)); !ok {
			return rawParams{filename: true}
		}
		switch keyBase, _, extended := strings.Cut(key, "*"); keyBase {
		case "filename":
			out.filename = true
		case "name":
			if extended {
				out.extendedName = true
			} else {
				out.plainName = true
			}
		}
	}
}

// trimLeftSpace trims leading white space as [mime.ParseMediaType] does.
func trimLeftSpace(s string) string {
	return strings.TrimLeftFunc(s, unicode.IsSpace)
}

// cutToken splits a leading token (RFC 2045) off s.
func cutToken(s string) (token, rest string) {
	i := 0
	for i < len(s) && isTokenChar(s[i]) {
		i++
	}
	return s[:i], s[i:]
}

// cutValue splits a leading token or quoted string off s. Within quotes a
// backslash escapes the next byte; [mime.ParseMediaType] only treats it as
// an escape before a special character, but either reading ends the string
// at the same quote, since '"' and '\' are special characters.
func cutValue(s string) (rest string, ok bool) {
	if !strings.HasPrefix(s, `"`) {
		token, rest := cutToken(s)
		return rest, token != ""
	}
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '"':
			return s[i+1:], true
		}
	}
	return "", false
}

// isTokenChar reports whether c may appear in a token (RFC 2045): a
// printable ASCII byte other than a space or a special character.
func isTokenChar(c byte) bool {
	return c > ' ' && c < 0x7f && !strings.ContainsRune(`()<>@,;:\"/[]?=`, rune(c))
}

// readPart reads the content of a part into rd.scratch, failing as soon as
// it exceeds the field limit: the scratch buffer never grows past
// fieldMax+1 bytes.
func (rd *reader) readPart(part io.Reader) error {
	rd.resetScratch()
	for {
		if len(rd.scratch) == cap(rd.scratch) {
			if len(rd.scratch) > rd.fieldMax {
				return errFieldTooLarge(rd.limits.Field)
			}
			rd.grow()
		}
		n, err := part.Read(rd.scratch[len(rd.scratch):cap(rd.scratch)])
		rd.scratch = rd.scratch[:len(rd.scratch)+n]
		if len(rd.scratch) > rd.fieldMax {
			return errFieldTooLarge(rd.limits.Field)
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
