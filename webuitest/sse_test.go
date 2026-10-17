package webuitest_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/html/group/list"
	"github.com/protolambda/chord/html/text"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/sse"
	"github.com/protolambda/webui/webuitest"
)

func TestParseEvents(gt *testing.T) {
	t := devtest.ParallelT(gt)
	events, comments, err := webuitest.ParseEvents([]byte("\uFEFF: hello\r\n" +
		"event: a\rdata:  two spaces\r\ndata:x\n\n" +
		"data\n\n" +
		"id: 7\nretry: 1500\ndata: b\n\n" +
		"id: bad\x00\nretry: 1.5\nevent: c\ndata: c\n\n" +
		"event: empty\nretry: 9\n\n" +
		"unknown: field\ndata: d\xff\n\n" +
		": bye\n"))
	t.Must(be.NoError(err))
	t.Must(be.Equal(2, comments))
	t.Must(be.DeepEqual([]webuitest.Event{
		{Name: "a", Data: " two spaces\nx"},
		{Name: "message", Data: ""},
		{Name: "message", Data: "b", ID: "7", Retry: 1500 * time.Millisecond},
		{Name: "c", Data: "c", ID: "7"},
		{Name: "message", Data: "d\uFFFD", ID: "7"},
	}, events))

	t.Run("invalid utf-8 as browsers decode it", func(t devtest.T) {
		for _, c := range []struct{ in, want string }{
			// One U+FFFD per maximal subpart, as TextDecoder and EventSource.
			{"a\xff\xfeb\xe2\x82c", "a\uFFFD\uFFFDb\uFFFDc"},
			{"\xed\xa0\x80", "\uFFFD\uFFFD\uFFFD"},           // encoded surrogate
			{"\xc0\x80", "\uFFFD\uFFFD"},                     // overlong
			{"\xf4\x90\x80\x80", "\uFFFD\uFFFD\uFFFD\uFFFD"}, // above U+10FFFF
			{"\xf0\x9f\x98", "\uFFFD"},                       // truncated
			{"\xe2\x82\xff", "\uFFFD\uFFFD"},
			{"ok \uFFFD \U0001F600", "ok \uFFFD \U0001F600"}, // valid input is kept
		} {
			events, _, err := webuitest.ParseEvents([]byte("data: " + c.in + "\n\n"))
			t.Must(be.NoError(err))
			t.Mustf(be.Equal(c.want, events[0].Data), "data %q", c.in)
		}
	})

	t.Run("line endings", func(t devtest.T) {
		for _, body := range []string{"data: a\n\n", "data: a\r\r", "data: a\r\n\r\n", "data: a\r\n\n", "data: a\n\r"} {
			events, _, err := webuitest.ParseEvents([]byte(body))
			t.Must(be.NoError(err))
			t.Mustf(be.DeepEqual([]webuitest.Event{{Name: "message", Data: "a"}}, events), "body %q", body)
		}
	})
	t.Run("bom only at the start", func(t devtest.T) {
		events, _, err := webuitest.ParseEvents([]byte("data: \uFEFFa\n\n"))
		t.Must(be.NoError(err))
		t.Must(be.Equal("\uFEFFa", events[0].Data))
		events, _, err = webuitest.ParseEvents([]byte("\uFEFFdata: a\n\n"))
		t.Must(be.NoError(err))
		t.Must(be.Equal("a", events[0].Data))
	})
	t.Run("unterminated", func(t devtest.T) {
		for _, body := range []string{"data: x", "data: x\n", "event: x\ndata: y\r", ": partial comment", "data: a\n\ndata: b"} {
			_, _, err := webuitest.ParseEvents([]byte(body))
			t.Mustf(be.ErrorIs(err, webuitest.ErrUnterminated), "body %q", body)
		}
		events, _, err := webuitest.ParseEvents([]byte("data: a\n\ndata: b\n"))
		t.Must(be.ErrorIs(err, webuitest.ErrUnterminated))
		t.Must(be.Len(events, 1))
		_, n, err := webuitest.ParseEvents([]byte(": comment only\n"))
		t.Must(be.NoError(err))
		t.Must(be.Equal(1, n))
		events, _, err = webuitest.ParseEvents(nil)
		t.Must(be.NoError(err))
		t.Must(be.Empty(events))
	})
	t.Run("too large", func(t devtest.T) {
		big := bytes.Repeat([]byte("a"), webuitest.MaxEventSize)
		_, _, err := webuitest.ParseEvents(append([]byte("data: "), big...))
		t.Must(be.ErrorIs(err, webuitest.ErrEventTooLarge))
		half := big[:webuitest.MaxEventSize/2]
		body := append(append(append(append([]byte("data: "), half...), "\ndata: "...), half...), "\n\n"...)
		_, _, err = webuitest.ParseEvents(body)
		t.Must(be.ErrorIs(err, webuitest.ErrEventTooLarge))
	})
	t.Run("retry", func(t devtest.T) {
		events, _, err := webuitest.ParseEvents([]byte("retry:\nretry: -1\nretry: 99999999999999999999\nretry: 1e3\ndata: x\n\n"))
		t.Must(be.NoError(err))
		t.Must(be.Equal(time.Duration(0), events[0].Retry))
	})
}

func TestEventFragment(gt *testing.T) {
	t := devtest.ParallelT(gt)
	e := webuitest.Event{Name: "tasks", Data: "<li>a</li>\n<li>b</li>"}
	t.Must(e.Fragment("ul").Find(ct.Tag("li")).Texts("a", "b"))
	t.Must(webuitest.FragmentContract(e.Fragment("ul")))
}

// liveTasks is a stream handler written with the sse package.
func liveTasks(w http.ResponseWriter, r *http.Request) {
	if sse.HandleHEAD(w, r) {
		return
	}
	s := sse.New(w)
	ctx := r.Context()
	if err := s.Send(ctx, "tasks", list.UL()(list.LI()(text.Text("Write docs")))); err != nil {
		return
	}
	if err := s.Comment("keep-alive"); err != nil {
		return
	}
	_ = s.End(ctx, text.P()(text.Text("Done.")))
}

func TestServeStream(gt *testing.T) {
	t := devtest.ParallelT(gt)
	st := webuitest.ServeStream(t, http.HandlerFunc(liveTasks), webuitest.Get(t, "/tasks/live", webuitest.EventSource()))
	t.Must(st.Status(http.StatusOK))
	t.Must(st.Header("Content-Type", ct.Prefix("text/event-stream")))
	e := st.Expect(t, "tasks")
	t.Must(e.Fragment("div").Find(ct.Role("listitem")).Texts("Write docs"))
	end := st.Expect(t, sse.EventEnd)
	t.Must(end.Fragment("div").Find(ct.Tag("p")).Texts("Done."))
	st.ExpectEnd(t)
	t.Must(be.Equal(1, st.Comments()))
	st.Returned(t)
	t.Must(be.Equal(http.StatusOK, st.Response().StatusCode))
	t.Must(be.Equal("GET", st.Response().Request.Method))

	err := st.Status(http.StatusNoContent).Check(t.Context())
	t.Must(be.ErrorContains(err, "GET /tasks/live has status 200, want 204"))
	err = st.Header("Content-Type", ct.Exact("text/html")).Check(t.Context())
	t.Must(be.ErrorContains(err, `header Content-Type of GET /tasks/live is ["text/event-stream; charset=utf-8"]`))
}

func TestServeStreamHEAD(gt *testing.T) {
	t := devtest.ParallelT(gt)
	st := webuitest.ServeStream(t, http.HandlerFunc(liveTasks), webuitest.Request(t, http.MethodHead, "/tasks/live", webuitest.Body{}))
	t.Must(st.Status(http.StatusOK))
	st.ExpectEnd(t)
}

func TestServeStreamWriter(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Run("sniffed content type and split CRLF", func(t devtest.T) {
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "data: <p>a</p>\r")
			w.(http.Flusher).Flush()
			_, _ = io.WriteString(w, "\n\r\n")
		})
		st := webuitest.ServeStream(t, h, webuitest.Get(t, "/"))
		t.Must(st.Header("Content-Type", ct.Exact("text/plain; charset=utf-8")))
		e := st.Next(t)
		t.Must(be.Equal("<p>a</p>", e.Data))
		st.ExpectEnd(t)
	})
	t.Run("no body", func(t devtest.T) {
		var werr error
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
			_, werr = io.WriteString(w, "x")
		})
		st := webuitest.ServeStream(t, h, webuitest.Get(t, "/"))
		t.Must(st.Status(http.StatusNoContent))
		st.ExpectEnd(t)
		st.Returned(t)
		t.Must(be.ErrorIs(werr, http.ErrBodyNotAllowed))
	})
	t.Run("set cookie values are hidden", func(t devtest.T) {
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.SetCookie(w, &http.Cookie{Name: "s", Value: cookieValue})
		})
		st := webuitest.ServeStream(t, h, webuitest.Get(t, "/"))
		err := st.Header("Set-Cookie", ct.Exact("x")).Check(t.Context())
		t.Must(be.ErrorContains(err, "is 1 values"))
		t.Must(be.False(strings.Contains(err.Error(), cookieValue)))
	})
	t.Run("header is sent with the body", func(t devtest.T) {
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			// As in net/http, changes after WriteHeader are not sent.
			w.Header().Set("X-Late", "1")
			// A body larger than the buffer sends the header without a flush.
			_, _ = io.WriteString(w, "data: "+strings.Repeat("x", 5000)+"\n\n")
			<-r.Context().Done()
		})
		st := webuitest.ServeStream(t, h, webuitest.Get(t, "/"))
		t.Must(st.Status(http.StatusOK))
		t.Must(be.Equal("", st.Response().Header.Get("X-Late")))
		t.Must(be.Equal(5000, len(st.Next(t).Data)))
	})
	t.Run("http.ResponseController", func(t devtest.T) {
		var deadlineErr, flushErr error
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rc := http.NewResponseController(w)
			deadlineErr = rc.SetWriteDeadline(time.Now().Add(time.Hour))
			_ = rc.SetReadDeadline(time.Now().Add(time.Hour))
			_, _ = io.WriteString(w, "data: x\n\n")
			flushErr = rc.Flush()
		})
		st := webuitest.ServeStream(t, h, webuitest.Get(t, "/"))
		st.Expect(t, "message")
		st.ExpectEnd(t)
		st.Returned(t)
		t.Must(be.NoError(deadlineErr))
		t.Must(be.NoError(flushErr))
	})
}

// TestServeStreamSynctest runs a sse.Loop in fake time: keep-alive
// comments every few seconds, then the end event at the lifetime.
func TestServeStreamSynctest(gt *testing.T) {
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		changes := make(chan struct{}, 1)
		loop := sse.Loop{
			Timing: sse.Timing{Interval: time.Second, KeepAlive: 5 * time.Second, Lifetime: 58 * time.Second},
			Update: func(ctx context.Context, s *sse.Stream) error {
				return s.Send(ctx, "clock", text.Text(time.Now().UTC().Format(time.TimeOnly)))
			},
			End:     func(sse.Reason) elem.Node { return text.Text("bye") },
			Changes: changes,
		}
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = loop.Run(r.Context(), sse.New(w))
		})
		st := webuitest.ServeStream(gt, h, webuitest.Get(gt, "/clock", webuitest.EventSource()), webuitest.StreamTimeout(2*time.Minute))
		t.Must(be.Equal("00:00:00", st.Expect(gt, "clock").Data))
		time.Sleep(12 * time.Second)
		changes <- struct{}{}
		t.Must(be.Equal("00:00:12", st.Expect(gt, "clock").Data))
		t.Must(be.Equal(2, st.Comments()))
		t.Must(be.Equal("bye", st.Expect(gt, sse.EventEnd).Data))
		st.ExpectEnd(gt)
		t.Must(be.Equal(11, st.Comments()))
		t.Must(be.Equal(58*time.Second, time.Since(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))))
	})
}

func TestServeStreamClose(gt *testing.T) {
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		var ctxErr error
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s := sse.New(w)
			_ = s.Start()
			<-r.Context().Done()
			ctxErr = r.Context().Err()
		})
		st := webuitest.ServeStream(gt, h, webuitest.Get(gt, "/"))
		st.Close()
		st.Close()
		st.Returned(gt)
		t.Must(be.ErrorIs(ctxErr, context.Canceled))
		r := newRecordT(gt.Context())
		st.Next(r)
		t.Must(be.Substring(r.output(), "want an event: the stream was closed"))
	})
}

func TestServeStreamStalledClient(gt *testing.T) {
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		writeErr := make(chan error, 1)
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rc := http.NewResponseController(w)
			for {
				_ = rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if _, err := io.WriteString(w, "data: "+strings.Repeat("x", 1024)+"\n\n"); err != nil {
					writeErr <- err
					return
				}
				if err := rc.Flush(); err != nil {
					writeErr <- err
					return
				}
			}
		})
		st := webuitest.ServeStream(gt, h, webuitest.Get(gt, "/"))
		st.Next(gt)
		// The test stops reading, as a stalled browser would.
		time.Sleep(time.Minute)
		err := <-writeErr
		t.Must(be.ErrorIs(err, os.ErrDeadlineExceeded))
		st.Returned(gt)
	})
}

func TestServeStreamFailures(gt *testing.T) {
	gt.Parallel()
	cases := []struct {
		name    string
		handler http.HandlerFunc
		check   func(t *testing.T, r *recordT, h http.Handler)
		want    string
	}{
		{
			name:    "no header",
			handler: func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() },
			check: func(t *testing.T, r *recordT, h http.Handler) {
				st := webuitest.ServeStream(r, h, webuitest.Get(t, "/x"), webuitest.StreamTimeout(time.Second))
				mustbe.Must(t, be.True(st == nil))
			},
			want: "GET /x: the handler sent no response header within 1s",
		},
		{
			name: "header without a flush",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				_, _ = io.WriteString(w, ": waiting for the first event\n")
				<-r.Context().Done()
			},
			check: func(t *testing.T, r *recordT, h http.Handler) {
				st := webuitest.ServeStream(r, h, webuitest.Get(t, "/x"), webuitest.StreamTimeout(time.Second))
				mustbe.Must(t, be.True(st == nil))
			},
			want: "GET /x: the handler sent no response header within 1s (WriteHeader alone sends none: flush)",
		},
		{
			name: "abort after the header",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, "event: tick\ndata: 1\n\n")
				w.(http.Flusher).Flush()
				panic(http.ErrAbortHandler)
			},
			check: func(t *testing.T, r *recordT, h http.Handler) {
				st := webuitest.ServeStream(r, h, webuitest.Get(t, "/x"))
				st.Expect(r, "tick")
				mustbe.Must(t, be.False(r.failed))
				st.ExpectEnd(r)
			},
			want: "GET /x: want the end of the stream: the handler aborted the response",
		},
		{
			name: "abort before the header is sent",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = io.WriteString(w, "data: 1\n\n")
				panic(http.ErrAbortHandler)
			},
			check: func(t *testing.T, r *recordT, h http.Handler) {
				mustbe.Must(t, be.True(webuitest.ServeStream(r, h, webuitest.Get(t, "/x")) == nil))
			},
			want: "GET /x: the handler aborted the response",
		},
		{
			name: "no event",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.(http.Flusher).Flush()
				<-r.Context().Done()
			},
			check: func(t *testing.T, r *recordT, h http.Handler) {
				st := webuitest.ServeStream(r, h, webuitest.Get(t, "/x"), webuitest.StreamTimeout(time.Second))
				st.Next(r)
			},
			want: "GET /x: want an event: no event within 1s",
		},
		{
			name: "wrong event",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, "event: a\ndata: x\n\n")
			},
			check: func(t *testing.T, r *recordT, h http.Handler) {
				webuitest.ServeStream(r, h, webuitest.Get(t, "/x")).Expect(r, "b")
			},
			want: `GET /x: got event "a", want "b"`,
		},
		{
			name: "event instead of end",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, "event: a\ndata: x\n\n")
			},
			check: func(t *testing.T, r *recordT, h http.Handler) {
				webuitest.ServeStream(r, h, webuitest.Get(t, "/x")).ExpectEnd(r)
			},
			want: `GET /x: got event "a", want the end of the stream`,
		},
		{
			name: "unterminated end",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, "data: x\n")
			},
			check: func(t *testing.T, r *recordT, h http.Handler) {
				webuitest.ServeStream(r, h, webuitest.Get(t, "/x")).ExpectEnd(r)
			},
			want: "want the end of the stream: the stream ended inside an event",
		},
		{
			name: "end instead of event",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			check: func(t *testing.T, r *recordT, h http.Handler) {
				webuitest.ServeStream(r, h, webuitest.Get(t, "/x")).Expect(r, "a")
			},
			want: `GET /x: want event "a": the stream ended`,
		},
		{
			name:    "panic before the header",
			handler: func(w http.ResponseWriter, r *http.Request) { panic("boom") },
			check: func(t *testing.T, r *recordT, h http.Handler) {
				mustbe.Must(t, be.True(webuitest.ServeStream(r, h, webuitest.Get(t, "/x")) == nil))
			},
			want: "GET /x: the handler panicked: boom",
		},
		{
			name: "panic after the header",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, "data: a\n\n")
				w.(http.Flusher).Flush()
				panic("late")
			},
			check: func(t *testing.T, r *recordT, h http.Handler) {
				st := webuitest.ServeStream(r, h, webuitest.Get(t, "/x"))
				st.Next(r)
				st.ExpectEnd(r)
			},
			want: "GET /x: the handler panicked: late",
		},
		{
			name: "handler ignores the client",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.(http.Flusher).Flush()
				time.Sleep(time.Hour)
			},
			check: func(t *testing.T, r *recordT, h http.Handler) {
				st := webuitest.ServeStream(r, h, webuitest.Get(t, "/x"), webuitest.StreamTimeout(time.Second))
				st.Close()
				st.Returned(r)
				r.cleanup()
				time.Sleep(time.Hour)
			},
			want: "GET /x: the handler did not return within 1s",
		},
	}
	for _, c := range cases {
		gt.Run(c.name, func(gt *testing.T) {
			var out string
			var failed bool
			synctest.Test(gt, func(st *testing.T) {
				r := newRecordT(st.Context())
				c.check(st, r, c.handler)
				r.cleanup()
				out, failed = r.output(), r.failed
			})
			t := mustbe.WrapT(gt)
			t.Must(be.True(failed))
			t.Must(be.Substring(out, c.want))
		})
	}
	gt.Run("abort is no panic", func(gt *testing.T) {
		t := devtest.SerialT(gt)
		r := newRecordT(t.Context())
		st := webuitest.ServeStream(r, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "data: a\n\n")
			w.(http.Flusher).Flush()
			panic(http.ErrAbortHandler)
		}), webuitest.Get(t, "/x"))
		st.Next(r)
		st.Returned(r)
		r.cleanup()
		t.Must(be.False(r.failed))
		t.Must(be.Equal("", r.output()))
	})
	gt.Run("nil", func(gt *testing.T) {
		t := devtest.SerialT(gt)
		r := newRecordT(t.Context())
		t.Must(be.True(webuitest.ServeStream(r, nil, nil) == nil))
		t.Must(be.True(r.failed))
	})
}
