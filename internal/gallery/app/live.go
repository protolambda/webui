package app

import (
	"context"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/aria"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/script"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/bi"
	"github.com/protolambda/webui/server"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/sse"
	"github.com/protolambda/webui/web"
)

// Event names of the live streams.
const (
	eventClock = "clock"
	eventCount = "count"
	eventNotes = "notes"
)

// clockEvery is the period of the live clock's updates; liveTiming's
// Interval must not exceed it.
const clockEvery = time.Second

// liveTiming is the timing of the gallery's live streams.
func liveTiming(lifetime time.Duration) sse.Timing {
	return sse.Timing{Interval: time.Second, KeepAlive: 15 * time.Second, Lifetime: lifetime}
}

func (a *App) live(w http.ResponseWriter, r *http.Request) error {
	return web.Render(w, r, http.StatusOK, a.page(r, view{
		Title: "Live",
		Icon:  bi.Broadcast,
		Main: elem.Bundle{
			para("Live regions are server-sent event streams that htmx swaps into the page. The server renders the first state with the page, so it shows without script. A stream ends with an end event, whose fragment replaces the region; returning without one (as at a server shutdown) makes the page reconnect."),
			demo("clock", "Clock",
				para("The clock ticks every second, until the stream reaches its lifetime of "+a.lifetime.String()+"."),
				div.Div(sse.Region("/live/clock"), attr.Class("card card-body mb-2"))(
					text.P(attr.Class("mb-0 font-monospace"))(
						text.Text("Server time: "),
						text.Span(sse.Swap(eventClock))(text.Text(a.clock())),
						text.Text(" · updates: "),
						text.Span(sse.Swap(eventCount))(text.Text("0")),
					),
				),
				script.Noscript()(para("Without script the clock stands still: reload the page to update it.", attr.Class("small text-body-secondary"))),
			),
			demo("private-count", "Private notes counter",
				para("A protected stream: it counts the notes of your session, and ends with an end event when you are not signed in, or when the session ends."),
				a.notesRegion(r),
			),
		},
	}))
}

// notesRegion renders the live count of the notes of r's session. Without
// a session, the stream ends at once with the same message.
func (a *App) notesRegion(r *http.Request) elem.Node {
	content := text.P(attr.Class("mb-0"))(text.Text("Sign in to see the count of your notes."))
	if s, ok := session.FromContext[Account](r.Context()); ok {
		content = text.P(attr.Class("mb-0"))(
			bi.Broadcast, text.Text(" Live count: "),
			text.Span(sse.Swap(eventNotes), aria.Live("polite"))(notesCount(s)),
		)
	}
	return div.Div(sse.Region("/notes/live"), attr.Class("card card-body mb-3"))(content)
}

// notesCount renders the number of notes of a session.
func notesCount(s *session.Session[Account]) elem.Node {
	n := len(notesOf(s))
	if n == 1 {
		return text.Text("1 note")
	}
	return text.Text(strconv.Itoa(n) + " notes")
}

// liveEnd renders the fragment of an end event: why the stream ended, and
// a link onwards.
func liveEnd(message, href, label string) elem.Node {
	return text.P(aria.Role("status"), attr.Class("mb-0"))(
		bi.InfoCircle, text.Text(" "+message+" "),
		elem.If(href != "", link(href, label)),
	)
}

// streamKey returns the key of r's client for the stream limiter: its
// address group (server.AddrPrefix).
func streamKey(r *http.Request) string {
	ap, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return "unknown"
	}
	return server.AddrPrefix(ap.Addr()).String()
}

// liveClock streams the clock and the number of updates, to anyone.
func (a *App) liveClock(w http.ResponseWriter, r *http.Request) error {
	if sse.HandleHEAD(w, r) {
		return nil
	}
	s := sse.New(w)
	release, ok := a.streams.Acquire(streamKey(r))
	if !ok {
		return s.End(r.Context(), liveEnd("Too many live views are open. Close one and reload.", "/live", "Reload"))
	}
	defer release()
	updates := 0
	return sse.Loop{
		Timing: liveTiming(a.lifetime),
		Update: func(ctx context.Context, s *sse.Stream) error {
			updates++
			if err := s.Send(ctx, eventClock, text.Text(a.clock())); err != nil {
				return err
			}
			return s.Send(ctx, eventCount, text.Text(strconv.Itoa(updates)))
		},
		End: func(reason sse.Reason) elem.Node {
			if reason == sse.ReasonLifetime {
				return liveEnd("The clock stopped after "+a.lifetime.String()+".", "/live", "Start it again")
			}
			return liveEnd("Live updates stopped.", "/live", "Reload")
		},
		Every: clockEvery,
		Stop:  a.stop,
	}.Run(r.Context(), s)
}

// notesLive streams the count of the notes of the request's session. It is
// a protected stream outside the authentication guard: without a session
// it refuses with an end event, which stops the browser from reconnecting.
func (a *App) notesLive(w http.ResponseWriter, r *http.Request) error {
	if sse.HandleHEAD(w, r) {
		return nil
	}
	s := sse.New(w)
	sess, ok := session.FromContext[Account](r.Context())
	if !ok {
		return s.End(r.Context(), liveEnd("Sign in to see the count of your notes.", "/login?next=%2Fnotes", "Log in"))
	}
	release, ok := a.streams.Acquire(sess.ID())
	if !ok {
		return s.End(r.Context(), liveEnd("Too many live views are open. Close one and reload.", "/notes", "Reload"))
	}
	defer release()
	changes, unsubscribe := a.noteChanges.subscribe()
	defer unsubscribe()
	return sse.Loop{
		Timing: liveTiming(a.lifetime),
		Update: func(ctx context.Context, s *sse.Stream) error {
			return s.Send(ctx, eventNotes, notesCount(sess))
		},
		End: func(reason sse.Reason) elem.Node {
			switch reason {
			case sse.ReasonDone:
				return liveEnd("Your session ended.", "/login?next=%2Fnotes", "Log in again")
			case sse.ReasonLifetime:
				return liveEnd("Live updates paused.", "/notes", "Reload")
			}
			return liveEnd("Live updates stopped.", "/notes", "Reload")
		},
		Changes: changes,
		Done:    sess.Done(),
		Stop:    a.stop,
	}.Run(r.Context(), s)
}
