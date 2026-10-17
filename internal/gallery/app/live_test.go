package app_test

import (
	"net/http"
	"net/url"
	"testing"
	"testing/synctest"
	"time"

	"github.com/protolambda/chord/ct"
	"github.com/protolambda/mustbe"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/internal/gallery/app"
	"github.com/protolambda/webui/mw"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/sse"
	"github.com/protolambda/webui/webuitest"
)

// The live tests run in testing/synctest bubbles, where the streams'
// timers run in fake time. devtest.T cannot be used there (it reads the
// test's deadline), so they use mustbe.WrapT for assertions and the
// *testing.T for the drivers.

// newLiveGallery returns a gallery whose live streams end after lifetime.
func newLiveGallery(t mustbe.MT, lifetime time.Duration) *app.App {
	a, err := app.New(app.Options{
		Sites:        []mw.Site{mw.MustParseSite(webuitest.BaseURL)},
		Logger:       webuitest.NewLogs().Logger(),
		LiveLifetime: lifetime,
	})
	t.Must(be.NoError(err))
	return a
}

// endText returns the text of the fragment of an end event.
func endText(gt *testing.T, ev webuitest.Event) string {
	gt.Helper()
	text, err := ev.Fragment("div").Find(ct.Role("status")).Text(gt.Context())
	if err != nil {
		gt.Fatalf("end fragment: %v", err)
	}
	return text
}

func TestLiveClock(gt *testing.T) {
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		a := newLiveGallery(t, 3*time.Second)
		start := time.Now()
		st := webuitest.ServeStream(gt, a, webuitest.Get(gt, "/live/clock", webuitest.EventSource()))
		t.Must(st.Status(http.StatusOK))
		t.Must(st.Header("Content-Type", ct.Exact(sse.ContentType)))
		t.Must(st.Header("Content-Security-Policy", ct.Contains("default-src 'none'")))

		for i, want := range []string{"1", "2", "3"} {
			clock := st.Expect(gt, "clock")
			t.Must(be.Equal(start.Add(time.Duration(i)*time.Second).UTC().Format(time.TimeOnly)+" UTC", clock.Data))
			t.Must(be.Equal(want, st.Expect(gt, "count").Data))
		}
		// The lifetime ends the stream with the end event, whose fragment
		// replaces the region and closes the connection for good.
		end := st.Expect(gt, sse.EventEnd)
		t.Must(be.Equal("The clock stopped after 3s. Start it again", endText(gt, end)))
		t.Must(end.Fragment("div").Find(ct.Role("link", ct.Named("Start it again"))).Matches(ct.Attr("href", "/live")))
		st.ExpectEnd(gt)
		t.Must(be.Equal(3*time.Second, time.Since(start)))
	})
}

// TestLiveShutdown checks that Shutdown ends the streams without an end
// event, so that pages reconnect once the server is back.
func TestLiveShutdown(gt *testing.T) {
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		a := newLiveGallery(t, 0)
		st := webuitest.ServeStream(gt, a, webuitest.Get(gt, "/live/clock", webuitest.EventSource()))
		st.Expect(gt, "clock")
		st.Expect(gt, "count")
		a.Shutdown()
		a.Shutdown() // idempotent
		st.ExpectEnd(gt)
		st.Returned(gt)

		// Streams that start after the shutdown return at once.
		late := webuitest.ServeStream(gt, a, webuitest.Get(gt, "/live/clock", webuitest.EventSource()))
		late.ExpectEnd(gt)
	})
}

func TestLiveLimit(gt *testing.T) {
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		a := newLiveGallery(t, 0)
		open := func() *webuitest.Stream {
			return webuitest.ServeStream(gt, a, webuitest.Get(gt, "/live/clock", webuitest.EventSource()))
		}
		var streams []*webuitest.Stream
		for range 4 {
			st := open()
			st.Expect(gt, "clock")
			streams = append(streams, st)
		}
		// The fifth stream of the same client is refused with an end event,
		// which stops the page from retrying.
		refused := open()
		t.Must(be.Equal("Too many live views are open. Close one and reload. Reload", endText(gt, refused.Expect(gt, sse.EventEnd))))
		refused.ExpectEnd(gt)

		// Another client is not affected, and a closed stream frees its
		// place.
		other := webuitest.ServeStream(gt, a, webuitest.Get(gt, "/live/clock", webuitest.EventSource(), webuitest.RemoteAddr("198.51.100.7:4321")))
		other.Expect(gt, "clock")
		streams[0].Close()
		streams[0].Returned(gt)
		open().Expect(gt, "clock")
	})
}

func TestNotesLive(gt *testing.T) {
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		a := newLiveGallery(t, 0)

		// Signed out: one end event that says why, at status 200, so that
		// the EventSource does not retry.
		st := webuitest.ServeStream(gt, a, webuitest.Get(gt, "/notes/live", webuitest.EventSource()))
		t.Must(st.Status(http.StatusOK))
		end := st.Expect(gt, sse.EventEnd)
		t.Must(be.Equal("Sign in to see the count of your notes. Log in", endText(gt, end)))
		t.Must(end.Fragment("div").Find(ct.Role("link")).Matches(ct.Attr("href", "/login?next=%2Fnotes")))
		st.ExpectEnd(gt)

		// Signed in: the count follows the notes of the session, until the
		// session ends.
		s := session.NewDetached(app.Account{Name: "Ada"}, testCSRF)
		req := webuitest.Get(gt, "/notes/live", webuitest.EventSource())
		st = webuitest.ServeStream(gt, a, req.WithContext(session.WithSession(req.Context(), s)))
		t.Must(be.Equal("0 notes", st.Expect(gt, "notes").Data))

		c := &webuitest.Client{Handler: a, CSRF: testCSRF, Prepare: func(r *http.Request) *http.Request {
			return r.WithContext(session.WithSession(r.Context(), s))
		}}
		res := c.Post(gt, "/notes", webuitest.Form(url.Values{"note": {"Call the plumber"}}))
		t.Must(res.Status(http.StatusSeeOther))
		t.Must(be.Equal("1 note", st.Expect(gt, "notes").Data))

		s.End()
		t.Must(be.Equal("Your session ended. Log in again", endText(gt, st.Expect(gt, sse.EventEnd))))
		st.ExpectEnd(gt)
	})
}

func TestLiveHEAD(gt *testing.T) {
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		a := newLiveGallery(t, 0)
		for _, target := range []string{"/live/clock", "/notes/live"} {
			res := webuitest.Serve(gt, a, webuitest.Request(gt, http.MethodHead, target, webuitest.Body{}))
			t.Must(res.Status(http.StatusOK))
			t.Must(res.Header("Content-Type", ct.Exact(sse.ContentType)))
			t.Must(res.Body(ct.Exact("")))
		}
	})
}
