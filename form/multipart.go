package form

import (
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
)

// readMultipart parses a multipart/form-data body.
//
// Parts are read raw ([multipart.Reader.NextRawPart]): a
// Content-Transfer-Encoding header, deprecated for forms by RFC 7578 and
// never sent by browsers, is not decoded. Each part must have a
// Content-Disposition of form-data with a non-empty name and no filename
// parameter. The body must end with the closing delimiter: a truncated body
// is malformed, not a shorter form.
func (rd *reader) readMultipart(src io.Reader, boundary string) error {
	mr := multipart.NewReader(src, boundary)
	for {
		part, err := mr.NextRawPart()
		if err == io.EOF {
			// Only the bare io.EOF marks the closing delimiter; the reader
			// wraps the io.EOF of a body that ends early.
			return nil
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
		rd.values.add(name, rd.own(rd.scratch))
		rd.resetScratch()
	}
}

// partName returns the field name of a part, refusing file parts and parts
// that are not form fields.
func partName(header textproto.MIMEHeader) (string, error) {
	disposition := header.Get("Content-Disposition")
	if disposition == "" {
		return "", fmt.Errorf("%w: part without Content-Disposition", ErrMalformed)
	}
	kind, params, err := mime.ParseMediaType(disposition)
	if err != nil {
		return "", fmt.Errorf("%w: bad Content-Disposition", ErrMalformed)
	}
	if kind != "form-data" {
		return "", fmt.Errorf("%w: part is not form-data", ErrMalformed)
	}
	// The presence of the parameter marks a file part, even when empty: a
	// file input without a selected file sends filename="".
	if _, ok := params["filename"]; ok {
		return "", ErrFile
	}
	name := params["name"]
	if name == "" {
		return "", fmt.Errorf("%w: part without a name", ErrMalformed)
	}
	return name, nil
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
