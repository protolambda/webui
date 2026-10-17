package form

import (
	"fmt"
	"io"
)

// readBufferSize is the size of the chunks read from a urlencoded body.
const readBufferSize = 4096

// readURLEncoded parses an application/x-www-form-urlencoded body as a
// stream, without holding more of it than one read chunk and the field
// being decoded.
//
// The grammar is the urlencoded parser of the URL standard, strict where
// browsers never produce the input. Fields are separated by '&'; the name
// of a field ends at its first '=' and later '=' belong to the value; '+'
// decodes to a space and "%XX" to the byte with the hex value XX; any other
// byte stands for itself. Empty fields ("a=1&&b=2&") are skipped, and a
// field without '=' has the empty value, as in [net/url.ParseQuery]. These
// are malformed: a '%' that is not followed by two hex digits; a field with
// an empty name; and a raw ';', which some servers and proxies treat as a
// separator too (browsers always encode it as %3B), so that it could split
// fields differently there than here.
func (rd *reader) readURLEncoded(src io.Reader) error {
	buf := make([]byte, readBufferSize)
	if rd.observe != nil {
		rd.observe(buf, true)
	}
	defer clear(buf)
	p := urlParser{rd: rd}
	for {
		n, err := src.Read(buf)
		if perr := p.feed(buf[:n]); perr != nil {
			return perr
		}
		if err == io.EOF {
			return p.end()
		}
		if err != nil {
			return err
		}
	}
}

// urlParser is the state of the urlencoded parser between chunks. The bytes
// of the current name or value accumulate in rd.scratch.
type urlParser struct {
	rd *reader
	// started reports that the current field has at least one byte.
	started bool
	// inValue reports that the current field's '=' was seen; name then
	// holds its decoded name.
	inValue bool
	name    string
	// esc counts the bytes of a percent escape seen so far: 0 outside an
	// escape, 1 after '%', 2 after '%' and the first hex digit (in hi).
	esc int
	hi  byte
}

var errBadEscape = fmt.Errorf("%w: bad percent escape", ErrMalformed)

// feed parses the next chunk of the body.
func (p *urlParser) feed(chunk []byte) error {
	for _, c := range chunk {
		if !p.started {
			if c == '&' {
				continue // an empty field
			}
			// Counted at its first byte, a field over the limit is refused
			// before any of it is decoded.
			if err := p.rd.countField(); err != nil {
				return err
			}
			p.started = true
		}
		switch p.esc {
		case 1:
			h, ok := unhex(c)
			if !ok {
				return errBadEscape
			}
			p.hi, p.esc = h, 2
			continue
		case 2:
			l, ok := unhex(c)
			if !ok {
				return errBadEscape
			}
			p.esc = 0
			if err := p.rd.appendByte(p.hi<<4 | l); err != nil {
				return err
			}
			continue
		}
		var err error
		switch c {
		case '%':
			p.esc = 1
		case '+':
			err = p.rd.appendByte(' ')
		case '&':
			err = p.finish()
		case '=':
			if p.inValue {
				err = p.rd.appendByte('=')
			} else {
				err = p.endName()
			}
		case ';':
			err = fmt.Errorf("%w: raw semicolon", ErrMalformed)
		default:
			err = p.rd.appendByte(c)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// endName ends the name of the current field at its '='.
func (p *urlParser) endName() error {
	if len(p.rd.scratch) == 0 {
		return fmt.Errorf("%w: field without a name", ErrMalformed)
	}
	p.name = string(p.rd.scratch)
	p.rd.resetScratch()
	p.inValue = true
	return nil
}

// finish stores the current field and resets the state for the next one.
func (p *urlParser) finish() error {
	if !p.inValue {
		// A field without '=': all of it is the name.
		if err := p.endName(); err != nil {
			return err
		}
	}
	p.rd.values.add(p.name, p.rd.own(p.rd.scratch))
	p.rd.resetScratch()
	*p = urlParser{rd: p.rd}
	return nil
}

// end handles the end of the body.
func (p *urlParser) end() error {
	if p.esc != 0 {
		return errBadEscape
	}
	if p.started {
		return p.finish()
	}
	return nil
}

// appendByte appends b to the field being decoded, failing when the field
// would exceed the field limit.
func (rd *reader) appendByte(b byte) error {
	if len(rd.scratch) >= rd.fieldMax {
		return errFieldTooLarge(rd.limits.Field)
	}
	if len(rd.scratch) == cap(rd.scratch) {
		rd.grow()
	}
	rd.scratch = append(rd.scratch, b)
	return nil
}

// resetScratch clears the field just consumed and empties rd.scratch,
// keeping its buffer for the next field.
func (rd *reader) resetScratch() {
	clear(rd.scratch)
	rd.scratch = rd.scratch[:0]
}

// unhex returns the value of the hex digit c.
func unhex(c byte) (byte, bool) {
	switch {
	case '0' <= c && c <= '9':
		return c - '0', true
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10, true
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
