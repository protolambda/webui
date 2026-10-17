package sse_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"
)

// event is one dispatched event, as an EventSource sees it.
type event struct {
	Type string
	Data string
	ID   string
}

// parsed is what a browser reads from a whole event stream.
type parsed struct {
	Events   []event
	Comments []string
	Retry    int64 // the last valid retry value; -1 when none
}

// errUnterminated reports a stream that stops inside a line or an event.
var errUnterminated = errors.New("stream ends inside an event")

// parseStream interprets body as the WHATWG HTML "text/event-stream"
// format: a leading BOM is skipped, lines end at CR, LF or CRLF, a line
// starting with a colon is a comment, one space after a field's colon is
// dropped, data lines join with LF, an event with no data is not
// dispatched, the type defaults to "message", an id with NUL is ignored,
// and retry needs ASCII digits. It fails when body stops inside a line or
// an event, which a server that frames events correctly never does.
func parseStream(body string) (parsed, error) {
	out := parsed{Retry: -1}
	body = strings.TrimPrefix(body, "\uFEFF")
	var (
		eventType, lastID string
		data              strings.Builder
		pending           bool // a field was read since the last dispatch
	)
	for body != "" {
		i := strings.IndexAny(body, "\r\n")
		if i < 0 {
			return out, errUnterminated
		}
		line := body[:i]
		if body[i] == '\r' && i+1 < len(body) && body[i+1] == '\n' {
			i++
		}
		body = body[i+1:]

		if line == "" {
			pending = false
			if data.Len() == 0 {
				eventType = ""
				continue
			}
			typ := eventType
			if typ == "" {
				typ = "message"
			}
			out.Events = append(out.Events, event{Type: typ, Data: strings.TrimSuffix(data.String(), "\n"), ID: lastID})
			data.Reset()
			eventType = ""
			continue
		}
		if line[0] == ':' {
			out.Comments = append(out.Comments, strings.TrimPrefix(line[1:], " "))
			continue
		}
		pending = true
		field, value := line, ""
		if j := strings.IndexByte(line, ':'); j >= 0 {
			field, value = line[:j], strings.TrimPrefix(line[j+1:], " ")
		}
		switch field {
		case "event":
			eventType = value
		case "data":
			data.WriteString(value)
			data.WriteByte('\n')
		case "id":
			if !strings.ContainsRune(value, 0) {
				lastID = value
			}
		case "retry":
			if n, err := strconv.ParseInt(value, 10, 64); err == nil && value != "" && strings.Trim(value, "0123456789") == "" {
				out.Retry = n
			}
		}
	}
	if pending {
		return out, errUnterminated
	}
	return out, nil
}

func TestParseStream(gt *testing.T) {
	t := devtest.ParallelT(gt)
	got, err := parseStream("\uFEFF: hello\r\n" +
		"event: a\rdata:  two spaces\r\ndata:x\n\n" +
		"data\n\n" +
		"id: 7\nretry: 1500\ndata: b\n\n" +
		"id: bad\x00\nretry: 1.5\nevent: c\ndata: c\n\n" +
		"event: empty\n\n" +
		"unknown: field\ndata: d\n\n")
	t.Must(be.NoError(err))
	t.Must(be.DeepEqual([]event{
		{Type: "a", Data: " two spaces\nx"},
		{Type: "message", Data: ""},
		{Type: "message", Data: "b", ID: "7"},
		{Type: "c", Data: "c", ID: "7"},
		{Type: "message", Data: "d", ID: "7"},
	}, got.Events))
	t.Must(be.DeepEqual([]string{"hello"}, got.Comments))
	t.Must(be.Equal(int64(1500), got.Retry))
}

func TestParseStreamUnterminated(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, body := range []string{"data: x", "data: x\n", "event: x\ndata: y\r"} {
		_, err := parseStream(body)
		t.Must(be.ErrorIs(err, errUnterminated))
	}
	_, err := parseStream(": comment only\n")
	t.Must(be.NoError(err))
}
