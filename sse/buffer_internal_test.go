package sse

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"
)

// allZero reports whether the whole storage of b, up to its capacity, is
// zero.
func allZero(b []byte) bool {
	for _, c := range b[:cap(b)] {
		if c != 0 {
			return false
		}
	}
	return true
}

func TestBuffersClearedAfterEvents(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := New(httptest.NewRecorder())
	t.Must(be.NoError(s.Send(t.Context(), "note", elem.Text("draft text\nsecond line"))))
	t.Must(be.Positive(cap(s.data.b)))
	t.Must(be.Positive(cap(s.frame.b)))
	t.Must(be.True(allZero(s.data.b)))
	t.Must(be.True(allZero(s.frame.b)))
	t.Must(be.Equal(0, len(s.data.b)))

	t.Must(be.NoError(s.SendText("count", "42")))
	t.Must(be.True(allZero(s.data.b)))
	t.Must(be.True(allZero(s.frame.b)))

	t.Must(be.NoError(s.Comment("keep-alive")))
	t.Must(be.True(allZero(s.frame.b)))

	t.Must(be.NoError(s.End(t.Context(), elem.Text("bye"))))
	t.Must(be.True(allZero(s.data.b)))
	t.Must(be.True(allZero(s.frame.b)))
}

func TestBufferClearsOnGrowth(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var b buffer
	_, _ = b.WriteString("secret")
	old := b.b
	_, _ = b.WriteString(strings.Repeat("x", 4096)) // forces a new allocation
	t.Must(be.True(allZero(old)))
	t.Must(be.Equal("secret"+strings.Repeat("x", 4096), string(b.b)))
}

func TestLargeBuffersAreDropped(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := New(httptest.NewRecorder())
	t.Must(be.NoError(s.SendText("big", strings.Repeat("x", maxRetained+1))))
	t.Must(be.Equal(0, cap(s.data.b)))
	t.Must(be.Equal(0, cap(s.frame.b)))
	t.Must(be.NoError(s.SendText("small", "x")))
	t.Must(be.Positive(cap(s.frame.b)))
}

// An event needs one frame allocation at most.
func TestFrameSizedOnce(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := New(httptest.NewRecorder())
	data := strings.Repeat("a\r\n\n\r", 1000)
	t.Must(be.NoError(s.SendText("x", data, ID("1"), Retry(0))))
	// The frame grew from empty exactly once: its capacity is the 512-byte
	// minimum or the size estimate, not a doubling.
	t.Must(be.LessOrEq(cap(s.frame.b), len("event: x\n\nid: 1\nretry: 0\n")+len(data)+(1+2*1000+2*1000)*len("data: \n")))
}
