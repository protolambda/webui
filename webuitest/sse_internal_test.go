package webuitest

import (
	"testing"

	"github.com/protolambda/mustbe"
	"github.com/protolambda/mustbe/be"
)

// parseChunked feeds body to a parser in chunks of n bytes, as a stream
// reader receives it.
func parseChunked(body []byte, n int) ([]Event, int, error) {
	p := eventParser{max: MaxEventSize}
	var events []Event
	emit := func(e Event) bool {
		events = append(events, e)
		return true
	}
	for len(body) > 0 {
		k := min(n, len(body))
		if err := p.feed(body[:k], emit); err != nil {
			return events, p.comments, err
		}
		body = body[k:]
	}
	return events, p.comments, p.end()
}

func checkChunking(t mustbe.MT, body []byte) {
	want, wantComments, wantErr := ParseEvents(body)
	for _, n := range []int{1, 2, 3, 7, 64} {
		got, comments, err := parseChunked(body, n)
		t.Must(be.DeepEqual(want, got))
		t.Must(be.Equal(wantComments, comments))
		t.Must(be.Equal(wantErr, err))
	}
}

func TestEventParserChunks(gt *testing.T) {
	t := mustbe.WrapT(gt)
	for _, body := range []string{
		"\xEF\xBB\xBFdata: a\r\n\r\nevent: b\rdata: c\r\r: x\n\nid: 1\ndata: d\n\n",
		"data: a\r",
		"data: a\r\n",
		"\r\n\r\n\r\n",
		"data:\ndata\n\n",
	} {
		checkChunking(t, []byte(body))
	}
}

func FuzzEventParser(f *testing.F) {
	f.Add([]byte("event: a\r\ndata: b\r\n\r\n"))
	f.Add([]byte("\xEF\xBB\xBF: c\rretry: 10\rid: x\x00\r\r"))
	f.Fuzz(func(gt *testing.T, body []byte) {
		checkChunking(mustbe.WrapT(gt), body)
	})
}
