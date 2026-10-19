package mw_test

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/mw"
)

func TestBodyDeadline(gt *testing.T) {
	synctest.Test(gt, func(st *testing.T) {
		t := mustbe.WrapT(st)
		h := mw.BodyDeadline(5 * time.Second)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		start := time.Now()

		rec := newDeadlineRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("x=1")))
		t.Must(be.DeepEqual([]time.Time{start.Add(5 * time.Second)}, rec.Reads()))
		t.Must(be.Len(rec.Writes(), 0))

		rec = newDeadlineRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		t.Mustf(be.Len(rec.Reads(), 0), "no deadline without a body")

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Body = nil
		rec = newDeadlineRecorder()
		h.ServeHTTP(rec, req)
		t.Must(be.Len(rec.Reads(), 0))

		t.Mustf(be.NoPanic(func() {
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader("x")))
		}), "writers without deadlines are left alone")
	})
}

func TestWriteDeadlineStartsWithResponse(gt *testing.T) {
	const timeout = 30 * time.Second
	type step func(w http.ResponseWriter)
	sleep := func(d time.Duration) step { return func(http.ResponseWriter) { time.Sleep(d) } }
	write := func(w http.ResponseWriter) { _, _ = w.Write([]byte("x")) }
	header := func(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }
	hints := func(w http.ResponseWriter) { w.WriteHeader(http.StatusEarlyHints) }
	flush := func(w http.ResponseWriter) { _ = http.NewResponseController(w).Flush() }
	readFrom := func(w http.ResponseWriter) {
		// A LimitedReader has no WriteTo, so io.Copy uses the writer's ReadFrom.
		_, _ = io.Copy(w, io.LimitReader(strings.NewReader("xyz"), 1))
	}
	copyNothing := func(w http.ResponseWriter) {
		_, _ = io.Copy(w, io.LimitReader(strings.NewReader(""), 1))
	}
	slowCopy := func(d time.Duration) step {
		return func(w http.ResponseWriter) {
			_, _ = io.Copy(w, io.LimitReader(&slowReader{delay: d, r: strings.NewReader("xyz")}, 2))
		}
	}
	takeOver := func(d time.Time) step {
		return func(w http.ResponseWriter) { _ = http.NewResponseController(w).SetWriteDeadline(d) }
	}
	hijack := func(w http.ResponseWriter) {
		c, _, err := http.NewResponseController(w).Hijack()
		if err == nil {
			_ = c.Close()
		}
	}
	t0 := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC) // the start of synctest bubbles
	far := t0.Add(time.Hour)
	cases := []struct {
		name   string
		proto  int
		expect bool // the request expects 100 Continue
		steps  []step
		writes []time.Time
	}{
		{"HTTP/1 100 Continue", 1, true, []step{sleep(10 * time.Second), header, sleep(time.Second), write},
			[]time.Time{t0.Add(timeout), t0.Add(10*time.Second + timeout), t0.Add(11*time.Second + timeout)}},
		{"HTTP/1 header", 1, false, []step{sleep(10 * time.Second), header, sleep(time.Second), write},
			[]time.Time{t0.Add(10*time.Second + timeout), t0.Add(11*time.Second + timeout)}},
		{"HTTP/2 header", 2, true, []step{sleep(10 * time.Second), header},
			[]time.Time{t0.Add(10*time.Second + timeout), t0.Add(10*time.Second + timeout)}},
		{"HTTP/2 write once", 2, false, []step{sleep(10 * time.Second), write, sleep(10 * time.Second), write},
			[]time.Time{t0.Add(10*time.Second + timeout), t0.Add(20*time.Second + timeout)}},
		{"HTTP/2 flush", 2, false, []step{sleep(time.Second), flush, sleep(time.Second)},
			[]time.Time{t0.Add(time.Second + timeout), t0.Add(2*time.Second + timeout)}},
		{"HTTP/2 copy", 2, false, []step{sleep(time.Second), readFrom, sleep(time.Second)},
			[]time.Time{t0.Add(time.Second + timeout), t0.Add(2*time.Second + timeout)}},
		{"HTTP/2 nothing written", 2, false, []step{sleep(10 * time.Second)},
			[]time.Time{t0.Add(10*time.Second + timeout)}},
		{"HTTP/2 empty copy", 2, false, []step{sleep(time.Second), copyNothing, sleep(time.Second)},
			[]time.Time{t0.Add(2*time.Second + timeout)}},
		{"HTTP/2 slow copy", 2, false, []step{slowCopy(10 * time.Second), sleep(time.Second)},
			[]time.Time{t0.Add(10*time.Second + timeout), t0.Add(11*time.Second + timeout)}},
		{"HTTP/1 take over", 1, false, []step{sleep(5 * time.Second), takeOver(far), sleep(time.Minute), write},
			[]time.Time{far, t0.Add(65*time.Second + timeout)}},
		{"HTTP/1 take over without deadline", 1, false, []step{takeOver(time.Time{}), write},
			[]time.Time{{}, t0.Add(timeout)}},
		{"HTTP/2 take over after start", 2, false, []step{write, takeOver(far), write, sleep(time.Second)},
			[]time.Time{t0.Add(timeout), far, t0.Add(time.Second + timeout)}},
		{"HTTP/1 hijack", 1, true, []step{sleep(time.Second), hijack}, []time.Time{t0.Add(timeout)}},
		{"HTTP/2 failed hijack", 2, false, []step{hijack, sleep(time.Second)},
			[]time.Time{t0.Add(time.Second + timeout)}},
		{"HTTP/1 early hints", 1, false, []step{hints, sleep(time.Minute), header, sleep(time.Second)},
			[]time.Time{t0.Add(timeout), t0.Add(time.Minute + timeout), t0.Add(61*time.Second + timeout)}},
		{"HTTP/2 early hints", 2, false, []step{hints, sleep(time.Minute), header, sleep(time.Second)},
			[]time.Time{t0.Add(time.Minute + timeout), t0.Add(61*time.Second + timeout)}},
		{"HTTP/1 early hints after take over", 1, false, []step{takeOver(far), hints, write},
			[]time.Time{far, t0.Add(timeout)}},
		{"HTTP/1 early hints after start", 1, false, []step{write, sleep(time.Second), hints},
			[]time.Time{t0.Add(timeout), t0.Add(time.Second + timeout)}},
	}
	for _, c := range cases {
		gt.Run(c.name, func(gt *testing.T) {
			synctest.Test(gt, func(st *testing.T) {
				t := mustbe.WrapT(st)
				t.Must(be.Equal(t0, time.Now().UTC()))
				h := mw.WriteDeadline(timeout)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					for _, s := range c.steps {
						s(w)
					}
				}))
				req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader("body"))
				req.ProtoMajor = c.proto
				if c.expect {
					req.Header.Set("Expect", "100-Continue")
				}
				rec := newDeadlineRecorder()
				rec.noHijack = c.proto == 2
				h.ServeHTTP(rec, req)
				got := rec.Writes()
				t.Must(be.Equal(len(c.writes), len(got)))
				for i := range got {
					t.Mustf(be.True(c.writes[i].Equal(got[i])), "deadline %d: want %v, got %v", i, c.writes[i], got[i])
				}
			})
		})
	}
}

// slowReader takes delay before it yields its first byte.
type slowReader struct {
	delay   time.Duration
	r       io.Reader
	started bool
}

func (s *slowReader) Read(p []byte) (int, error) {
	if !s.started {
		s.started = true
		time.Sleep(s.delay)
	}
	return s.r.Read(p)
}

// TestWriteDeadlineCoversRecoveredPanics has the handler panic: the
// deadline is set as the panic passes through WriteDeadline, so that the
// error page of a recovering middleware outside it (as web.Errors.Recover
// is placed) is bounded as any response.
func TestWriteDeadlineCoversRecoveredPanics(gt *testing.T) {
	synctest.Test(gt, func(st *testing.T) {
		t := mustbe.WrapT(st)
		const timeout = 30 * time.Second
		t0 := time.Now()
		inner := mw.WriteDeadline(timeout)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			time.Sleep(time.Second)
			panic("handler failed")
		}))
		recovering := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recover() != nil {
					http.Error(w, "failed", http.StatusInternalServerError)
				}
			}()
			inner.ServeHTTP(w, r)
		})
		rec := newDeadlineRecorder()
		recovering.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		t.Must(be.Equal(http.StatusInternalServerError, rec.Code))
		writes := rec.Writes()
		t.Must(be.Len(writes, 1))
		t.Must(be.True(t0.Add(time.Second + timeout).Equal(writes[0])))
	})
}

func TestWriteDeadlineExpectWithoutBody(gt *testing.T) {
	synctest.Test(gt, func(st *testing.T) {
		t := mustbe.WrapT(st)
		h := mw.WriteDeadline(time.Minute)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			time.Sleep(time.Second)
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Expect", "100-continue")
		rec := newDeadlineRecorder()
		h.ServeHTTP(rec, req)
		t.Mustf(be.Len(rec.Writes(), 1), "no deadline at arrival without a body")
	})
}

func TestWriteDeadlineContinue(gt *testing.T) {
	synctest.Test(gt, func(st *testing.T) {
		t := mustbe.WrapT(st)
		h := mw.WriteDeadline(30 * time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				return
			}
			_, _ = fmt.Fprintf(w, "stored %d bytes", len(body))
		}))
		ln, stop := serve(h)
		defer stop()
		c, err := ln.Dial()
		t.Must(be.NoError(err))
		defer c.Close()
		_, err = io.WriteString(c, "PUT /file HTTP/1.1\r\nHost: app.example.com\r\nContent-Length: 5\r\nExpect: 100-continue\r\n\r\n")
		t.Must(be.NoError(err))
		br := bufio.NewReader(c)
		interim, err := http.ReadResponse(br, nil)
		t.Must(be.NoError(err))
		t.Must(be.Equal(http.StatusContinue, interim.StatusCode))
		_, err = io.WriteString(c, "hello")
		t.Must(be.NoError(err))
		res, err := http.ReadResponse(br, nil)
		t.Must(be.NoError(err))
		body, err := io.ReadAll(res.Body)
		t.Must(be.NoError(err))
		t.Must(be.Equal("stored 5 bytes", string(body)))
	})
}

// An informational response, such as 103 Early Hints, does not start the
// response's deadline: the work after it does not count.
func TestWriteDeadlineEarlyHints(gt *testing.T) {
	synctest.Test(gt, func(st *testing.T) {
		t := mustbe.WrapT(st)
		var flushErr error
		h := mw.WriteDeadline(30 * time.Second)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Link", "</assets/app.css>; rel=preload; as=style")
			w.WriteHeader(http.StatusEarlyHints)
			time.Sleep(40 * time.Second) // work after the hint does not count
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "page")
			flushErr = http.NewResponseController(w).Flush()
		}))
		ln, stop := serve(h)
		defer stop()
		c, err := ln.Dial()
		t.Must(be.NoError(err))
		defer c.Close()
		_, err = io.WriteString(c, "GET /page HTTP/1.1\r\nHost: app.example.com\r\n\r\n")
		t.Must(be.NoError(err))
		br := bufio.NewReader(c)
		interim, err := http.ReadResponse(br, nil)
		t.Must(be.NoError(err))
		t.Must(be.Equal(http.StatusEarlyHints, interim.StatusCode))
		res, err := http.ReadResponse(br, nil)
		t.Must(be.NoError(err))
		t.Must(be.Equal(http.StatusOK, res.StatusCode))
		body, err := io.ReadAll(res.Body)
		t.Must(be.NoError(err))
		t.Must(be.Equal("page", string(body)))
		t.Must(be.NoError(flushErr))
	})
}

func TestWritersSupportResponseController(gt *testing.T) {
	t := devtest.ParallelT(gt)
	stacks := map[string][]mw.Middleware{
		"deadline":         {mw.WriteDeadline(time.Minute)},
		"log":              {mw.LogRequests(mw.RequestLog{})},
		"log and deadline": {mw.LogRequests(mw.RequestLog{}), mw.WriteDeadline(time.Minute)},
		"deadline and log": {mw.WriteDeadline(time.Minute), mw.LogRequests(mw.RequestLog{})},
	}
	for name, stack := range stacks {
		t.Run(name, func(t devtest.T) {
			var flusher, hijacker bool
			var errs []error
			h := mw.Chain(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				var f http.Flusher
				f, flusher = w.(http.Flusher)
				_, hijacker = w.(http.Hijacker)
				if flusher {
					f.Flush()
				}
				rc := http.NewResponseController(w)
				errs = append(errs, rc.EnableFullDuplex(), rc.SetReadDeadline(time.Time{}),
					rc.SetWriteDeadline(time.Time{}), rc.Flush())
			}), stack...)
			rec := newDeadlineRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
			t.Must(be.True(flusher))
			t.Must(be.True(hijacker))
			for _, err := range errs {
				t.Must(be.NoError(err))
			}
			t.Must(be.True(rec.Flushed))
			t.Must(be.Len(rec.Reads(), 1))
			t.Mustf(be.True(slices.ContainsFunc(rec.Writes(), time.Time.IsZero)), "the handler's deadline arrives")

			// A writer without the optional methods reports that.
			errs = nil
			h.ServeHTTP(plainWriter{httptest.NewRecorder()}, httptest.NewRequest(http.MethodGet, "/", nil))
			for _, err := range errs {
				t.Must(be.ErrorIs(err, http.ErrNotSupported))
			}
		})
	}
}

// plainWriter is a ResponseWriter without optional methods.
type plainWriter struct{ w http.ResponseWriter }

func (p plainWriter) Header() http.Header         { return p.w.Header() }
func (p plainWriter) Write(b []byte) (int, error) { return p.w.Write(b) }
func (p plainWriter) WriteHeader(code int)        { p.w.WriteHeader(code) }

func TestDeadlinesPanicOnMisuse(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Panic(func() { mw.BodyDeadline(0) }))
	t.Must(be.Panic(func() { mw.BodyDeadline(-time.Second) }))
	t.Must(be.Panic(func() { mw.WriteDeadline(0) }))
	t.Must(be.Panic(func() { mw.WriteDeadline(-time.Second) }))
	t.Must(be.Panic(func() { mw.MaxBody(-1) }))
}

// roundTrip writes a raw request to a new connection to ln and reads the
// response.
func roundTrip(ln *pipeListener, raw string) (*http.Response, string, error) {
	c, err := ln.Dial()
	if err != nil {
		return nil, "", err
	}
	defer c.Close()
	if _, err := io.WriteString(c, raw); err != nil {
		return nil, "", err
	}
	res, err := http.ReadResponse(bufio.NewReader(c), nil)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	return res, string(body), err
}

func TestBodyDeadlineEndsStalledBodies(gt *testing.T) {
	synctest.Test(gt, func(st *testing.T) {
		t := mustbe.WrapT(st)
		var readErr error
		var readFor time.Duration
		h := mw.BodyDeadline(30 * time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			_, readErr = io.ReadAll(r.Body)
			readFor = time.Since(start)
			w.WriteHeader(http.StatusRequestTimeout)
		}))
		ln, stop := serve(h)
		defer stop()
		res, _, err := roundTrip(ln, "POST /upload HTTP/1.1\r\nHost: app.example.com\r\nContent-Length: 100\r\n\r\npartial")
		t.Must(be.NoError(err))
		t.Must(be.Equal(http.StatusRequestTimeout, res.StatusCode))
		t.Must(be.ErrorIs(readErr, os.ErrDeadlineExceeded))
		t.Must(be.Equal(30*time.Second, readFor))
	})
}

func TestBodyDeadlineSparesLongResponses(gt *testing.T) {
	synctest.Test(gt, func(st *testing.T) {
		t := mustbe.WrapT(st)
		h := mw.BodyDeadline(30 * time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "read failed", http.StatusBadRequest)
				return
			}
			time.Sleep(2 * time.Minute)
			_, _ = fmt.Fprintf(w, "got %q", body)
		}))
		ln, stop := serve(h)
		defer stop()
		res, body, err := roundTrip(ln, "POST /save HTTP/1.1\r\nHost: app.example.com\r\nContent-Length: 5\r\n\r\nhello")
		t.Must(be.NoError(err))
		t.Must(be.Equal(http.StatusOK, res.StatusCode))
		t.Must(be.Equal(`got "hello"`, body))

		res, body, err = roundTrip(ln, "GET /slow HTTP/1.1\r\nHost: app.example.com\r\n\r\n")
		t.Must(be.NoError(err))
		t.Must(be.Equal(http.StatusOK, res.StatusCode))
		t.Must(be.Equal(`got ""`, body))
	})
}

func TestWriteDeadlineEndsStalledClients(gt *testing.T) {
	synctest.Test(gt, func(st *testing.T) {
		t := mustbe.WrapT(st)
		var writeErr error
		var wroteFor time.Duration
		done := make(chan struct{})
		h := mw.WriteDeadline(30 * time.Second)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			defer close(done)
			time.Sleep(10 * time.Second) // work before the response does not count
			start := time.Now()
			chunk := make([]byte, 64<<10)
			for writeErr == nil {
				_, writeErr = w.Write(chunk)
			}
			wroteFor = time.Since(start)
		}))
		ln, stop := serve(h)
		defer stop()
		c, err := ln.Dial()
		t.Must(be.NoError(err))
		defer c.Close()
		_, err = io.WriteString(c, "GET /export HTTP/1.1\r\nHost: app.example.com\r\n\r\n")
		t.Must(be.NoError(err))
		<-done // the client never reads
		t.Must(be.ErrorIs(writeErr, os.ErrDeadlineExceeded))
		t.Must(be.Equal(30*time.Second, wroteFor))
	})
}

func TestWriteDeadlineSparesStreamsThatTakeOver(gt *testing.T) {
	synctest.Test(gt, func(st *testing.T) {
		t := mustbe.WrapT(st)
		stream := func(takeOver bool) http.Handler {
			return mw.WriteDeadline(30 * time.Second)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				rc := http.NewResponseController(w)
				for i := range 3 {
					if takeOver {
						_ = rc.SetWriteDeadline(time.Now().Add(5 * time.Second))
					}
					if _, err := fmt.Fprintf(w, "event %d\n", i); err != nil {
						return
					}
					if err := rc.Flush(); err != nil {
						return
					}
					time.Sleep(time.Minute)
				}
			}))
		}
		ln, stop := serve(stream(true))
		defer stop()
		res, body, err := roundTrip(ln, "GET /events HTTP/1.1\r\nHost: app.example.com\r\n\r\n")
		t.Must(be.NoError(err))
		t.Must(be.Equal(http.StatusOK, res.StatusCode))
		t.Must(be.Equal("event 0\nevent 1\nevent 2\n", body))

		ln2, stop2 := serve(stream(false))
		defer stop2()
		_, body, err = roundTrip(ln2, "GET /events HTTP/1.1\r\nHost: app.example.com\r\n\r\n")
		t.Mustf(be.Error(err), "a stream that does not take over is cut at the deadline")
		t.Must(be.Equal("event 0\n", body))
	})
}

func TestMaxBody(gt *testing.T) {
	t := devtest.ParallelT(gt)
	read := func(limit int64, body string) error {
		var err error
		h := mw.MaxBody(limit)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			_, err = io.ReadAll(r.Body)
		}))
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
		return err
	}
	t.Must(be.NoError(read(5, "hello")))
	t.Must(be.NoError(read(0, "")))
	var tooLarge *http.MaxBytesError
	err := read(4, "hello")
	t.Must(be.True(errors.As(err, &tooLarge)))
	t.Must(be.Equal(int64(4), tooLarge.Limit))
	t.Must(be.True(errors.As(read(0, "x"), &tooLarge)))
}

func TestMaxBodyKeepsMissingBodies(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var bodies []io.ReadCloser
	h := mw.MaxBody(10)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		bodies = append(bodies, r.Body)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Body = nil
	h.ServeHTTP(httptest.NewRecorder(), req)
	t.Must(be.Len(bodies, 2))
	t.Mustf(be.True(bodies[0] == http.NoBody), "a request without a body keeps http.NoBody")
	t.Must(be.True(bodies[1] == nil))
}

// Behind the writers of the recommended stack, MaxBody still makes an HTTP/1
// server close the connection after a body over the bound, instead of
// reading the rest of the body and serving the next request.
func TestMaxBodyClosesConnectionBehindWrappers(gt *testing.T) {
	stacks := map[string][]mw.Middleware{
		"alone": {mw.MaxBody(10)},
		"recommended": {
			mw.LogRequests(mw.RequestLog{Logger: slog.New(slog.DiscardHandler)}),
			mw.BodyDeadline(mw.DefaultBodyTimeout),
			mw.WriteDeadline(mw.DefaultWriteTimeout),
			mw.SecurityHeaders(mw.Headers{}),
			mw.MaxBody(10),
		},
	}
	for name, stack := range stacks {
		gt.Run(name, func(gt *testing.T) {
			synctest.Test(gt, func(st *testing.T) {
				t := mustbe.WrapT(st)
				h := mw.Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if _, err := io.ReadAll(r.Body); err != nil {
						var tooLarge *http.MaxBytesError
						if errors.As(err, &tooLarge) {
							http.Error(w, "too large", http.StatusRequestEntityTooLarge)
							return
						}
					}
					_, _ = io.WriteString(w, "ok")
				}), stack...)
				ln, stop := serve(h)
				defer stop()
				c, err := ln.Dial()
				t.Must(be.NoError(err))
				defer c.Close()
				body := strings.Repeat("x", 1000)
				go func() {
					// Pipelined: the second request follows the oversized body.
					_, _ = io.WriteString(c, "POST /notes HTTP/1.1\r\nHost: app.example.com\r\nContent-Length: 1000\r\n\r\n"+
						body+"GET / HTTP/1.1\r\nHost: app.example.com\r\n\r\n")
				}()
				br := bufio.NewReader(c)
				res, err := http.ReadResponse(br, nil)
				t.Must(be.NoError(err))
				t.Must(be.Equal(http.StatusRequestEntityTooLarge, res.StatusCode))
				t.Mustf(be.True(res.Close), "the response announces Connection: close")
				_, err = io.Copy(io.Discard, res.Body)
				t.Must(be.NoError(err))
				_, err = http.ReadResponse(br, nil)
				t.Mustf(be.Error(err), "the connection serves no further request")
			})
		})
	}
}
