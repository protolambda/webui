package sse_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/text"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe"
	"github.com/protolambda/mustbe/assertion"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/sse"
)

// failingNode fails to render with err.
type failingNode struct{ err error }

func (n failingNode) Eval(context.Context) (elem.Obj, error) { return elem.Obj{}, n.err }

var errRender = errors.New("render failed")

// mustT is what the test helpers need of devtest.T and mustbe.MT.
type mustT interface {
	Helper()
	Must(c assertion.Assertion)
}

func mustParse(t mustT, body string) parsed {
	t.Helper()
	p, err := parseStream(body)
	t.Must(be.NoError(err))
	return p
}

func TestSendFramesLines(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		name string
		html string
		wire string
		data string // what the browser dispatches
	}{
		{"one line", "<p>hi</p>", "data: <p>hi</p>\n", "<p>hi</p>"},
		{"LF", "a\nb", "data: a\ndata: b\n", "a\nb"},
		{"CR", "a\rb", "data: a\ndata: b\n", "a\nb"},
		{"CRLF", "a\r\nb", "data: a\ndata: b\n", "a\nb"},
		{"LF CR is two breaks", "a\n\rb", "data: a\ndata:\ndata: b\n", "a\n\nb"},
		{"empty lines", "a\n\n\nb", "data: a\ndata:\ndata:\ndata: b\n", "a\n\n\nb"},
		{"leading and trailing breaks", "\na\n", "data:\ndata: a\ndata:\n", "\na\n"},
		{"trailing CR", "a\r", "data: a\ndata:\n", "a\n"},
		{"only a break", "\r\n", "data:\ndata:\n", "\n"},
		{"empty", "", "data:\n", ""},
		{"leading space kept", " a\n  b", "data:  a\ndata:   b\n", " a\n  b"},
		{"colons", "data: x", "data: data: x\n", "data: x"},
		{"unicode", "<p>Grüße 👋 \u2028 \u0085 日本</p>", "data: <p>Grüße 👋 \u2028 \u0085 日本</p>\n", "<p>Grüße 👋 \u2028 \u0085 日本</p>"},
	}
	for _, c := range cases {
		w := newFakeWriter()
		s := sse.New(w)
		t.Must(be.NoError(s.Send(t.Context(), "item", elem.Raw(c.html))))
		t.Must(be.Annotate(be.Equal("event: item\n"+c.wire+"\n", w.Body()), "case %s", c.name))
		p := mustParse(t, w.Body())
		t.Must(be.DeepEqual([]event{{Type: "item", Data: c.data}}, p.Events))

		// SendText frames text the same way.
		w = newFakeWriter()
		s = sse.New(w)
		t.Must(be.NoError(s.SendText("item", c.html)))
		t.Must(be.Annotate(be.Equal("event: item\n"+c.wire+"\n", w.Body()), "text case %s", c.name))
	}
}

func TestSendRendersEscapedText(gt *testing.T) {
	t := devtest.ParallelT(gt)
	w := newFakeWriter()
	s := sse.New(w)
	node := div.Div(attr.ID("note"))(text.Text("<script>alert(1)</script>\nevent: forged\n\ndata: x"))
	t.Must(be.NoError(s.Send(t.Context(), "note", node)))
	p := mustParse(t, w.Body())
	t.Must(be.DeepEqual([]event{{
		Type: "note",
		Data: "<div id=\"note\">&lt;script&gt;alert(1)&lt;/script&gt;\nevent: forged\n\ndata: x</div>",
	}}, p.Events))
}

func TestStreamStartsWithHeaders(gt *testing.T) {
	t := devtest.ParallelT(gt)
	w := newFakeWriter()
	w.Header().Set("Cache-Control", "max-age=3600")
	s := sse.New(w)
	t.Must(be.False(s.Started()))
	t.Must(be.NoError(s.Start()))
	t.Must(be.True(s.Started()))
	t.Must(be.Equal(http.StatusOK, w.Status()))
	t.Must(be.Equal(sse.ContentType, w.Header().Get("Content-Type")))
	t.Must(be.Equal("text/event-stream; charset=utf-8", w.Header().Get("Content-Type")))
	t.Must(be.Equal("no-store", w.Header().Get("Cache-Control")))
	t.Must(be.Equal("no", w.Header().Get("X-Accel-Buffering")))
	t.Must(be.Equal("", w.Body()))
	t.Must(be.DeepEqual([]string{"deadline", "header", "flush"}, w.Ops()))

	t.Must(be.NoError(s.Start())) // started already: nothing more
	t.Must(be.NoError(s.Comment("x")))
	t.Must(be.DeepEqual([]string{"deadline", "header", "flush", "deadline", "write", "flush"}, w.Ops()))
}

func TestSendRejectsBeforeWriting(gt *testing.T) {
	t := devtest.ParallelT(gt)
	check := func(err error, target error) {
		t.Helper()
		t.Must(be.ErrorIs(err, target))
	}
	w := newFakeWriter()
	s := sse.New(w)
	check(s.Send(t.Context(), "", elem.Raw("x")), sse.ErrInvalidEventName)
	check(s.Send(t.Context(), "a\nb", elem.Raw("x")), sse.ErrInvalidEventName)
	check(s.SendText("a b", "x"), sse.ErrInvalidEventName)
	check(s.Send(t.Context(), "ok", failingNode{errRender}), errRender)
	check(s.Send(t.Context(), "ok", nil), elem.ErrNilNode)
	check(s.Send(t.Context(), "ok", elem.Raw("x"), sse.ID("a\rb")), sse.ErrInvalidField)
	check(s.SendText("ok", "x", sse.ID("a\x00")), sse.ErrInvalidField)
	check(s.SendText("ok", "x", sse.Retry(-time.Millisecond)), sse.ErrInvalidField)
	check(s.Comment("a\nb"), sse.ErrInvalidField)
	check(s.Comment("a\rb"), sse.ErrInvalidField)
	t.Must(be.False(s.Started()))
	t.Must(be.Empty(w.Ops()))
	t.Must(be.Equal(0, w.Status()))
	t.Must(be.Equal("", w.Header().Get("Content-Type")))
}

func TestRenderErrorKeepsStreamUsable(gt *testing.T) {
	t := devtest.ParallelT(gt)
	w := newFakeWriter()
	s := sse.New(w)
	t.Must(be.NoError(s.Send(t.Context(), "a", elem.Raw("1"))))
	// A failure late in the fragment must not leak its first part.
	err := s.Send(t.Context(), "a", elem.Bundle{elem.Raw("secret part"), failingNode{errRender}})
	t.Must(be.ErrorIs(err, errRender))
	t.Must(be.NoError(s.Send(t.Context(), "a", elem.Raw("2"))))
	t.Must(be.Equal("event: a\ndata: 1\n\nevent: a\ndata: 2\n\n", w.Body()))
}

func TestFields(gt *testing.T) {
	t := devtest.ParallelT(gt)
	w := newFakeWriter()
	s := sse.New(w)
	t.Must(be.NoError(s.Send(t.Context(), "a", elem.Raw("x"), sse.ID("42"), sse.Retry(2500*time.Millisecond))))
	t.Must(be.NoError(s.SendText("b", "y", sse.Field{}, sse.ID(""), sse.Retry(0))))
	t.Must(be.NoError(s.SendText("c", "z", sse.Retry(1500*time.Microsecond))))
	t.Must(be.Equal("event: a\nid: 42\nretry: 2500\ndata: x\n\n"+
		"event: b\nid:\nretry: 0\ndata: y\n\n"+
		"event: c\nretry: 1\ndata: z\n\n", w.Body()))
	p := mustParse(t, w.Body())
	t.Must(be.DeepEqual([]event{
		{Type: "a", Data: "x", ID: "42"},
		{Type: "b", Data: "y", ID: ""},
		{Type: "c", Data: "z", ID: ""},
	}, p.Events))
	t.Must(be.Equal(int64(1), p.Retry))
}

func TestComment(gt *testing.T) {
	t := devtest.ParallelT(gt)
	w := newFakeWriter()
	s := sse.New(w)
	t.Must(be.NoError(s.Comment("keep-alive")))
	t.Must(be.NoError(s.Comment("")))
	t.Must(be.NoError(s.Comment(" spaced: ok ")))
	t.Must(be.Equal(": keep-alive\n\n:\n\n:  spaced: ok \n\n", w.Body()))
	p := mustParse(t, w.Body())
	t.Must(be.Empty(p.Events))
	t.Must(be.DeepEqual([]string{"keep-alive", "", " spaced: ok "}, p.Comments))
}

func TestCheckEventName(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, name := range []string{
		"a", "end", "message", "tasks", "Task-List_2", "item.updated", "item:42", "0",
		strings.Repeat("x", sse.MaxEventNameLen), "Open", "errors",
	} {
		t.Must(be.Annotate(be.NoError(sse.CheckEventName(name)), "name %q", name))
	}
	for _, name := range []string{
		"", "a b", " a", "a ", "a,b", "a\nb", "a\rb", "a\tb", "a\x00", "open", "error",
		"a[1]", "a\"b", "a'b", "a/b", "a\\b", "a(b)", "a{b}", "a;b", "a=b", "<a>", "é", "日本",
		strings.Repeat("x", sse.MaxEventNameLen+1),
	} {
		t.Must(be.Annotate(be.ErrorIs(sse.CheckEventName(name), sse.ErrInvalidEventName), "name %q", name))
	}
	// The message explains without repeating the name.
	for _, name := range []string{"item 7f3a9c", "item\n7f3a9c", "7f3a9c" + strings.Repeat("x", sse.MaxEventNameLen)} {
		err := sse.CheckEventName(name)
		t.Must(be.ErrorIs(err, sse.ErrInvalidEventName))
		t.Must(be.False(strings.Contains(err.Error(), "7f3a9c")))
	}
}

func TestEnd(gt *testing.T) {
	t := devtest.ParallelT(gt)
	w := newFakeWriter()
	s := sse.New(w)
	t.Must(be.NoError(s.Send(t.Context(), "a", elem.Raw("1"))))
	t.Must(be.False(s.Ended()))
	t.Must(be.NoError(s.End(t.Context(), elem.Raw("<p>Bye</p>"))))
	t.Must(be.True(s.Ended()))
	t.Must(be.ErrorIs(s.Send(t.Context(), "a", elem.Raw("2")), sse.ErrEnded))
	t.Must(be.ErrorIs(s.SendText("a", "2"), sse.ErrEnded))
	t.Must(be.ErrorIs(s.Comment("x"), sse.ErrEnded))
	t.Must(be.ErrorIs(s.End(t.Context(), elem.Raw("again")), sse.ErrEnded))
	t.Must(be.Equal("event: a\ndata: 1\n\nevent: end\ndata: <p>Bye</p>\n\n", w.Body()))
	p := mustParse(t, w.Body())
	t.Must(be.DeepEqual([]event{{Type: "a", Data: "1"}, {Type: sse.EventEnd, Data: "<p>Bye</p>"}}, p.Events))
}

// A refusal is a whole response: status 200, the stream header, one end
// event.
func TestEndRefusesWithOneEvent(gt *testing.T) {
	t := devtest.ParallelT(gt)
	rec := httptest.NewRecorder()
	s := sse.New(rec)
	t.Must(be.NoError(s.End(t.Context(), text.P()(text.Text("Your session ended.")))))
	t.Must(be.Equal(http.StatusOK, rec.Code))
	t.Must(be.Equal(sse.ContentType, rec.Header().Get("Content-Type")))
	t.Must(be.True(rec.Flushed))
	p := mustParse(t, rec.Body.String())
	t.Must(be.DeepEqual([]event{{Type: "end", Data: "<p>Your session ended.</p>"}}, p.Events))
}

// An end fragment that fails to render still ends the stream, so the
// browser stops reconnecting, and nothing of the fragment leaks.
func TestEndRenderFailureStillEnds(gt *testing.T) {
	t := devtest.ParallelT(gt)
	w := newFakeWriter()
	s := sse.New(w)
	err := s.End(t.Context(), elem.Bundle{elem.Raw("partial"), failingNode{errRender}})
	t.Must(be.ErrorIs(err, errRender))
	t.Must(be.True(s.Ended()))
	t.Must(be.Equal("event: end\ndata:\n\n", w.Body()))
	p := mustParse(t, w.Body())
	t.Must(be.DeepEqual([]event{{Type: "end", Data: ""}}, p.Events))

	w = newFakeWriter()
	s = sse.New(w)
	t.Must(be.ErrorIs(s.End(t.Context(), nil), elem.ErrNilNode))
	t.Must(be.Equal("event: end\ndata:\n\n", w.Body()))
}

func TestWriteDeadlines(gt *testing.T) {
	gt.Parallel()
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt) // devtest.SerialT reads T.Deadline, which a bubble forbids
		w := newFakeWriter()
		s := sse.New(w)
		t.Must(be.Equal(sse.DefaultWriteWindow, s.WriteWindow()))
		start := time.Now()
		t.Must(be.NoError(s.SendText("a", "1")))
		time.Sleep(10 * time.Second)
		t.Must(be.NoError(s.Comment("")))
		time.Sleep(time.Minute)
		t.Must(be.NoError(s.End(context.Background(), elem.Raw("bye"))))
		t.Must(be.DeepEqual([]time.Time{
			start.Add(30 * time.Second),
			start.Add(40 * time.Second),
			start.Add(100 * time.Second),
		}, w.Deadlines()))
		// The deadline comes before the header and before every write.
		t.Must(be.DeepEqual([]string{
			"deadline", "header", "write", "flush",
			"deadline", "write", "flush",
			"deadline", "write", "flush",
		}, w.Ops()))
	})
}

func TestWithWriteWindow(gt *testing.T) {
	gt.Parallel()
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt) // devtest.SerialT reads T.Deadline, which a bubble forbids
		w := newFakeWriter()
		s := sse.New(w, sse.WithWriteWindow(5*time.Second))
		t.Must(be.Equal(5*time.Second, s.WriteWindow()))
		t.Must(be.NoError(s.Start()))
		t.Must(be.DeepEqual([]time.Time{time.Now().Add(5 * time.Second)}, w.Deadlines()))

		for _, d := range []time.Duration{0, -time.Second} {
			t.Must(be.Equal(sse.DefaultWriteWindow, sse.New(w, sse.WithWriteWindow(d)).WriteWindow()))
		}
	})
}

// Writers without write deadlines work: the stream skips the deadline.
func TestUnsupportedDeadline(gt *testing.T) {
	t := devtest.ParallelT(gt)
	w := newFakeWriter()
	w.deadlineErr = http.ErrNotSupported
	s := sse.New(w)
	t.Must(be.NoError(s.SendText("a", "1")))
	t.Must(be.NoError(s.Comment("")))
	t.Must(be.Equal("event: a\ndata: 1\n\n:\n\n", w.Body()))
	t.Must(be.DeepEqual([]string{"deadline", "header", "write", "flush", "deadline", "write", "flush"}, w.Ops()))

	// A recorder flushes, but has no deadlines.
	rec := httptest.NewRecorder()
	s = sse.New(rec)
	t.Must(be.NoError(s.SendText("a", "1")))
	t.Must(be.True(rec.Flushed))
}

// embedWriter is the classic wrapper that loses the optional interfaces:
// it embeds the interface, and has neither Flush nor Unwrap.
type embedWriter struct{ http.ResponseWriter }

// unwrapWriter is a wrapper that the controller can see through.
type unwrapWriter struct{ http.ResponseWriter }

func (u unwrapWriter) Unwrap() http.ResponseWriter { return u.ResponseWriter }

// cycleWriter unwraps to itself.
type cycleWriter struct{ http.ResponseWriter }

func (c *cycleWriter) Unwrap() http.ResponseWriter { return c }

// A writer that cannot flush would hold every event back: the stream fails
// before it writes anything, with an error that is not a client failure.
func TestUnflushableWriterFails(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := map[string]func(w *fakeWriter) http.ResponseWriter{
		"plain":           func(*fakeWriter) http.ResponseWriter { return &plainWriter{} },
		"embedding":       func(w *fakeWriter) http.ResponseWriter { return embedWriter{w} },
		"unwrap to plain": func(*fakeWriter) http.ResponseWriter { return unwrapWriter{&plainWriter{}} },
		"unwrap to nil":   func(*fakeWriter) http.ResponseWriter { return unwrapWriter{nil} },
		"unwrap cycle":    func(w *fakeWriter) http.ResponseWriter { return &cycleWriter{w} },
	}
	for name, wrap := range cases {
		inner := newFakeWriter()
		s := sse.New(wrap(inner))
		err := s.SendText("a", "1")
		t.Must(be.Annotate(be.ErrorIs(err, http.ErrNotSupported), "case %s", name))
		t.Must(be.Annotate(be.False(errors.Is(err, sse.ErrWrite)), "case %s", name))
		t.Must(be.Equal("sse: response writer cannot flush: feature not supported", err.Error()))
		t.Must(be.False(s.Started()))
		// The error is sticky, and nothing reached the writer: the handler
		// can still answer with an error status.
		t.Must(be.ErrorIs(s.Start(), http.ErrNotSupported))
		t.Must(be.ErrorIs(s.Comment(""), http.ErrNotSupported))
		t.Must(be.ErrorIs(s.End(t.Context(), elem.Text("bye")), http.ErrNotSupported))
		t.Must(be.Annotate(be.Empty(inner.Ops()), "case %s", name))
	}

	// Through Unwrap, the controller reaches the flusher.
	w := newFakeWriter()
	s := sse.New(unwrapWriter{unwrapWriter{w}})
	t.Must(be.NoError(s.SendText("a", "1")))
	t.Must(be.DeepEqual([]string{"deadline", "header", "write", "flush"}, w.Ops()))
}

// A writer whose flush reports http.ErrNotSupported fails the same way,
// after the header.
func TestFlushNotSupportedIsSticky(gt *testing.T) {
	t := devtest.ParallelT(gt)
	w := newFakeWriter()
	w.flushErr = http.ErrNotSupported
	s := sse.New(w)
	err := s.SendText("a", "1")
	t.Must(be.ErrorIs(err, http.ErrNotSupported))
	t.Must(be.False(errors.Is(err, sse.ErrWrite)))
	t.Must(be.True(s.Started()))
	ops := len(w.Ops())
	w.flushErr = nil
	t.Must(be.ErrorIs(s.SendText("a", "2"), http.ErrNotSupported))
	t.Must(be.ErrorIs(s.Comment(""), http.ErrNotSupported))
	t.Must(be.Equal(ops, len(w.Ops())))
}

// Behind a real server, a wrapper without Flush or Unwrap would deliver
// nothing until the handler returns. The stream reports it before it
// starts, so the handler answers with an error.
func TestUnflushableWrapperOverHTTP(gt *testing.T) {
	gt.Parallel()
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		sendErr := make(chan error, 1)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s := sse.New(embedWriter{w})
			err := s.SendText("a", "1")
			sendErr <- err
			if err != nil && !s.Started() {
				http.Error(w, "stream unavailable", http.StatusInternalServerError)
			}
		})
		body, err := serveOnce(gt, handler, time.Minute)
		t.Must(be.NoError(err))
		t.Must(be.ErrorIs(<-sendErr, http.ErrNotSupported))
		t.Must(be.Equal("stream unavailable\n", body))
	})
}

func TestWriteFailuresAreSticky(gt *testing.T) {
	t := devtest.ParallelT(gt)
	errConn := errors.New("connection reset")
	cases := map[string]func(w *fakeWriter){
		"deadline": func(w *fakeWriter) { w.deadlineErr = errConn },
		"write":    func(w *fakeWriter) { w.writeErr = errConn },
		"flush":    func(w *fakeWriter) { w.flushErr = errConn },
	}
	for name, breakIt := range cases {
		w := newFakeWriter()
		s := sse.New(w)
		t.Must(be.NoError(s.SendText("a", "1")))
		breakIt(w)
		err := s.SendText("a", "2")
		t.Must(be.Annotate(be.ErrorIs(err, sse.ErrWrite), "case %s", name))
		t.Must(be.Annotate(be.ErrorIs(err, errConn), "case %s", name))

		// The stream stays broken, even when the connection would accept
		// writes again: a later event could follow half an event.
		w.deadlineErr, w.writeErr, w.flushErr = nil, nil, nil
		ops := len(w.Ops())
		t.Must(be.ErrorIs(s.SendText("a", "3"), sse.ErrWrite))
		t.Must(be.ErrorIs(s.Comment(""), sse.ErrWrite))
		t.Must(be.ErrorIs(s.Start(), sse.ErrWrite))
		t.Must(be.ErrorIs(s.End(t.Context(), elem.Raw("x")), sse.ErrWrite))
		t.Must(be.Equal(ops, len(w.Ops())))
	}
}

// A deadline failure before the stream starts writes nothing, so the
// handler can still answer otherwise.
func TestDeadlineFailureBeforeStart(gt *testing.T) {
	t := devtest.ParallelT(gt)
	w := newFakeWriter()
	w.deadlineErr = errors.New("closed")
	s := sse.New(w)
	t.Must(be.ErrorIs(s.SendText("a", "1"), sse.ErrWrite))
	t.Must(be.False(s.Started()))
	t.Must(be.Equal(0, w.Status()))
}

func TestHandleHEAD(gt *testing.T) {
	t := devtest.ParallelT(gt)
	rec := httptest.NewRecorder()
	t.Must(be.True(sse.HandleHEAD(rec, httptest.NewRequestWithContext(t.Context(), http.MethodHead, "/live", nil))))
	t.Must(be.Equal(http.StatusOK, rec.Code))
	t.Must(be.Equal(sse.ContentType, rec.Header().Get("Content-Type")))
	t.Must(be.Equal("no-store", rec.Header().Get("Cache-Control")))
	t.Must(be.Equal("", rec.Body.String()))

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		rec = httptest.NewRecorder()
		t.Must(be.False(sse.HandleHEAD(rec, httptest.NewRequestWithContext(t.Context(), method, "/live", nil))))
		t.Must(be.False(rec.Flushed))
		t.Must(be.Equal("", rec.Header().Get("Content-Type")))
	}
}

// A stream through net/http outlives the server's write and read
// timeouts, because each write extends the write deadline (net/http does
// not apply the read timeout once the request is read). The connection is
// a net.Pipe, whose deadlines follow the bubble's fake time.
func TestStreamOutlivesServerTimeouts(gt *testing.T) {
	gt.Parallel()
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		const timeout = time.Second
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s := sse.New(w, sse.WithWriteWindow(timeout))
			for i := range 5 {
				if err := s.SendText("tick", strconv.Itoa(i)); err != nil {
					return
				}
				time.Sleep(timeout * 3 / 4)
				if err := s.Comment(""); err != nil {
					return
				}
				time.Sleep(timeout * 3 / 4)
			}
			_ = s.End(r.Context(), elem.Raw("done"))
		})
		body, err := serveOnce(gt, handler, timeout)
		t.Must(be.NoError(err))
		p := mustParse(t, body)
		t.Must(be.Len(p.Events, 6))
		t.Must(be.Len(p.Comments, 5))
		t.Must(be.Equal(event{Type: "end", Data: "done"}, p.Events[5]))
	})
}

// Without the stream's deadlines, the same pauses hit the server's write
// timeout: the test above would fail if the stream did not extend them.
func TestServerWriteTimeoutCutsPlainResponses(gt *testing.T) {
	gt.Parallel()
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		const timeout = time.Second
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rc := http.NewResponseController(w)
			for range 3 {
				if _, err := w.Write([]byte(": x\n\n")); err != nil {
					return
				}
				if err := rc.Flush(); err != nil {
					return
				}
				time.Sleep(timeout * 3 / 4)
			}
		})
		body, err := serveOnce(gt, handler, timeout)
		t.Must(be.ErrorIs(err, io.ErrUnexpectedEOF))
		t.Must(be.Equal(": x\n\n: x\n\n", body)) // the write after the timeout failed
	})
}

// serveOnce serves one GET request over a net.Pipe with the given server
// timeouts, and returns the response body.
func serveOnce(gt *testing.T, handler http.Handler, timeout time.Duration) (string, error) {
	srvConn, cliConn := net.Pipe()
	l := &oneConnListener{conn: srvConn, closed: make(chan struct{})}
	srv := &http.Server{Handler: handler, WriteTimeout: timeout, ReadTimeout: timeout}
	served := make(chan struct{})
	go func() {
		defer close(served)
		_ = srv.Serve(l)
	}()
	defer func() {
		_ = srv.Close()
		<-served
	}()
	tr := &http.Transport{DialContext: func(context.Context, string, string) (net.Conn, error) { return cliConn, nil }}
	defer tr.CloseIdleConnections()

	req, err := http.NewRequestWithContext(gt.Context(), http.MethodGet, "http://example.com/live", nil)
	if err != nil {
		return "", err
	}
	res, err := tr.RoundTrip(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	var body strings.Builder
	_, err = io.Copy(&body, res.Body)
	return body.String(), err
}

// oneConnListener accepts one connection, then blocks until closed.
type oneConnListener struct {
	mu     sync.Mutex
	conn   net.Conn
	closed chan struct{}
	once   sync.Once
}

func (l *oneConnListener) Accept() (net.Conn, error) {
	l.mu.Lock()
	c := l.conn
	l.conn = nil
	l.mu.Unlock()
	if c != nil {
		return c, nil
	}
	<-l.closed
	return nil, net.ErrClosed
}

func (l *oneConnListener) Close() error {
	l.once.Do(func() { close(l.closed) })
	return nil
}

func (l *oneConnListener) Addr() net.Addr { return pipeAddr{} }

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }
