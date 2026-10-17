package sse

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/protolambda/chord/core/elem"
)

// Defaults of [Timing].
const (
	// DefaultInterval is the shortest time between two updates of a loop.
	DefaultInterval = time.Second
	// DefaultKeepAlive is the interval of keep-alive comments, when it is
	// shorter than two thirds of the write window.
	DefaultKeepAlive = 20 * time.Second
	// DefaultLifetime is how long a loop streams before it ends the stream.
	DefaultLifetime = 30 * time.Minute
)

// ErrConfig reports an invalid [Loop] or [Timing].
var ErrConfig = errors.New("sse: invalid configuration")

// keepAliveText is the text of the comments a loop sends while idle.
const keepAliveText = "keep-alive"

// Timing bounds a [Loop]. The zero value selects the defaults. Applications
// usually hold one Timing in their configuration, check it once with
// [Timing.Validate], and set it as the Timing of every Loop.
type Timing struct {
	// Interval is the shortest time between two updates: changes that
	// arrive sooner after an update are coalesced into one update at the
	// end of the interval. A change after a quiet interval updates at once.
	// Zero selects DefaultInterval.
	Interval time.Duration
	// KeepAlive is the interval of keep-alive comments. It must be shorter
	// than the write window of the stream: on HTTP/2 a write deadline ends
	// the stream when it passes, also while the handler waits. Zero selects
	// DefaultKeepAlive, or two thirds of the write window when that is
	// shorter.
	KeepAlive time.Duration
	// Lifetime caps how long the loop streams; then it ends the stream with
	// ReasonLifetime, and the end fragment can offer a reload. The cap
	// bounds how long a stream holds a connection and whatever state it
	// renders. Zero selects DefaultLifetime.
	Lifetime time.Duration
}

// Validate returns an error wrapping [ErrConfig] when a duration is
// negative or the keep-alive interval is not shorter than writeWindow.
// A writeWindow of zero or less means [DefaultWriteWindow], as in
// [WithWriteWindow].
func (t Timing) Validate(writeWindow time.Duration) error {
	_, err := t.resolve(writeWindow)
	return err
}

// resolve applies the defaults and validates the result.
func (t Timing) resolve(writeWindow time.Duration) (Timing, error) {
	if writeWindow <= 0 {
		writeWindow = DefaultWriteWindow
	}
	if t.Interval < 0 || t.KeepAlive < 0 || t.Lifetime < 0 {
		return t, fmt.Errorf("%w: negative duration", ErrConfig)
	}
	if t.Interval == 0 {
		t.Interval = DefaultInterval
	}
	if t.Lifetime == 0 {
		t.Lifetime = DefaultLifetime
	}
	if t.KeepAlive == 0 {
		t.KeepAlive = DefaultKeepAlive
		if writeWindow < DefaultKeepAlive*3/2 {
			// Two thirds of the window are shorter than the default. Only
			// short windows get here, so the product cannot overflow.
			t.KeepAlive = writeWindow * 2 / 3
		}
	}
	if t.KeepAlive <= 0 || t.KeepAlive >= writeWindow {
		return t, fmt.Errorf("%w: keep-alive interval %v is not shorter than the write window %v", ErrConfig, t.KeepAlive, writeWindow)
	}
	return t, nil
}

// Reason says why a [Loop] ended its stream with an end event.
type Reason uint8

const (
	// ReasonDone means the Done channel closed: the session ended.
	ReasonDone Reason = iota + 1
	// ReasonLifetime means the lifetime cap passed.
	ReasonLifetime
	// ReasonClosed means the Changes channel closed: the feed ended.
	ReasonClosed
	// ReasonFailed means an update failed; Run also returns the error.
	ReasonFailed
)

// String returns a lowercase name of the reason.
func (r Reason) String() string {
	switch r {
	case ReasonDone:
		return "done"
	case ReasonLifetime:
		return "lifetime"
	case ReasonClosed:
		return "closed"
	case ReasonFailed:
		return "failed"
	default:
		return fmt.Sprintf("reason(%d)", uint8(r))
	}
}

// Loop drives a [Stream] from change notifications. Build one per stream
// request and call [Loop.Run].
//
// Run sends the current state with Update at the start and after changes,
// at most once per Timing.Interval; sends a keep-alive comment every
// Timing.KeepAlive; and finishes in one of two ways (see the package
// documentation on ending a stream):
//
//   - with an end event, whose fragment End renders: when Done closes
//     (ReasonDone), after Timing.Lifetime (ReasonLifetime), when Changes
//     closes (ReasonClosed), and when an update fails (ReasonFailed);
//   - without one, so the browser reconnects: when Stop closes, also when
//     it closed together with Done or Changes (a server that stops often
//     ends its sessions and feeds too), and when ctx ends (the client
//     left; nobody is listening).
//
// Done takes precedence over updates: once it has closed, no update
// starts, including the first one, also when a change is ready at the same
// time; only the end event follows. An update that is already running when
// Done closes finishes first, so an Update that must not render after a
// revocation checks for it itself.
//
// Updates re-render whole fragments, so signals need not be counted: a
// notifier that sends without blocking on a channel with a buffer of one
// loses nothing, because a send that finds the buffer full leaves a signal
// pending already. On an unbuffered channel such a send is lost while Run
// is busy in Update or a keep-alive write. Nil channels never fire.
//
// Loop embeds Timing for its fields; [Loop.Validate] checks the whole
// loop, not only the timing.
type Loop struct {
	// Timing bounds the loop; the zero value selects the defaults.
	Timing

	// Update sends the current state, usually with one Send per sse-swap
	// target. It runs at the start and after changes, on the goroutine of
	// Run, so keep-alive comments wait for it: it must finish well within
	// the write window. It may call s.End to finish the stream (for example
	// when the item it shows was deleted); Run then returns. Required.
	Update func(ctx context.Context, s *Stream) error

	// End renders the fragment of the end event for a reason: typically a
	// short message and a link onwards (log in again, reload). It may
	// return nil for an empty fragment. Required.
	End func(reason Reason) elem.Node

	// Changes signals that the state changed. A notifier that sends
	// without blocking must use a channel with a buffer of at least one,
	// such as make(chan struct{}, 1): a change while Run is busy then
	// leaves one pending signal, and a burst becomes one update. Closing it
	// ends the stream with ReasonClosed.
	Changes <-chan struct{}

	// Done closes when the stream must end for good, typically the
	// session's Done channel: the stream ends with ReasonDone, and no
	// update starts after it closed.
	Done <-chan struct{}

	// Stop closes when the server stops, for example from a function
	// registered with [http.Server.RegisterOnShutdown] (Shutdown does not
	// cancel request contexts, and waits for handlers to return): the loop
	// returns without an end event, so the page reconnects once the server
	// is back.
	Stop <-chan struct{}
}

// Validate returns an error wrapping [ErrConfig] when Update or End is
// missing, or when the Timing is invalid for writeWindow (see
// [Timing.Validate]; a writeWindow of zero or less means
// [DefaultWriteWindow]). [Loop.Run] makes the same check before it writes
// anything, with the write window of its stream.
func (l Loop) Validate(writeWindow time.Duration) error {
	_, err := l.check(writeWindow)
	return err
}

// check validates the loop and returns its resolved timing.
func (l Loop) check(writeWindow time.Duration) (Timing, error) {
	if l.Update == nil || l.End == nil {
		return Timing{}, fmt.Errorf("%w: Loop needs Update and End", ErrConfig)
	}
	return l.Timing.resolve(writeWindow)
}

// Run streams to s until the stream ends, the client leaves (ctx ends) or
// the server stops. ctx is usually the request context; it is also the
// render context of the updates and the end fragment.
//
// Run validates the loop before it writes anything (see [Loop.Validate]):
// an invalid loop returns an error wrapping [ErrConfig] with the response
// untouched. A stream that already ended returns [ErrEnded]. A response
// writer that cannot flush returns an error wrapping
// [http.ErrNotSupported], also with nothing written, so the handler can
// still answer with an error (see [Stream]).
//
// Otherwise Run returns nil, except for failures worth logging: an update
// that failed (joined with any failure to render the end fragment), and an
// end fragment that failed to render. A client that left or stalled
// (errors wrapping [ErrWrite], or ctx ending) is a normal end. The response
// has started by then, so the handler must not write an error page.
func (l Loop) Run(ctx context.Context, s *Stream) error {
	t, err := l.check(s.WriteWindow())
	if err != nil {
		return err
	}
	if s.Ended() {
		return ErrEnded
	}
	if ctx.Err() != nil {
		return nil
	}
	if err := s.Start(); err != nil {
		return worthLogging(ctx, err) // nil when the client cannot be reached
	}
	if closed(l.Stop) {
		return nil
	}

	lifetime := time.NewTimer(t.Lifetime)
	defer lifetime.Stop()
	keepAlive := time.NewTicker(t.KeepAlive)
	defer keepAlive.Stop()

	last := time.Now()
	if done, err := l.update(ctx, s); done {
		return err
	}
	var due <-chan time.Time // armed while a coalesced update waits
	for {
		select {
		case _, ok := <-l.Changes:
			if !ok {
				return l.end(ctx, s, ReasonClosed)
			}
			if due != nil {
				continue // the pending update covers this change
			}
			if wait := t.Interval - time.Since(last); wait > 0 {
				due = time.After(wait)
				continue
			}
		case <-due:
		case <-keepAlive.C:
			if closed(l.Stop) {
				return nil
			}
			if err := s.Comment(keepAliveText); err != nil {
				return worthLogging(ctx, err) // nil when the client left or stalled
			}
			continue
		case <-l.Done:
			return l.end(ctx, s, ReasonDone)
		case <-l.Stop:
			return nil
		case <-lifetime.C:
			return l.end(ctx, s, ReasonLifetime)
		case <-ctx.Done():
			return nil
		}
		due = nil
		if closed(l.Stop) || ctx.Err() != nil {
			return nil
		}
		last = time.Now()
		if done, err := l.update(ctx, s); done {
			return err
		}
	}
}

// update runs Update, and reports whether the loop is over, with the error
// Run returns. Every update starts here, so this is where Done takes
// precedence: over the first update, and over a change or a due update
// that was ready together with Done.
func (l Loop) update(ctx context.Context, s *Stream) (bool, error) {
	if closed(l.Done) {
		return true, l.end(ctx, s, ReasonDone) // end keeps Stop first
	}
	err := l.Update(ctx, s)
	switch {
	case s.Ended():
		return true, worthLogging(ctx, err)
	case err == nil:
		return false, nil
	case worthLogging(ctx, err) == nil:
		return true, nil
	case closed(l.Stop):
		// Reconnect after the restart rather than show a failure.
		return true, fmt.Errorf("sse: update: %w", err)
	}
	renderErr, _ := s.end(ctx, l.endNode(ReasonFailed))
	return true, errors.Join(fmt.Errorf("sse: update: %w", err), worthLogging(ctx, renderErr))
}

// end ends the stream with an end event for reason, unless the server
// stops. A failed write is a normal end; a failed render is worth logging.
func (l Loop) end(ctx context.Context, s *Stream, reason Reason) error {
	if closed(l.Stop) {
		return nil
	}
	renderErr, _ := s.end(ctx, l.endNode(reason))
	return worthLogging(ctx, renderErr)
}

// endNode returns the end fragment for reason; nil means an empty one.
func (l Loop) endNode(reason Reason) elem.Node {
	if node := l.End(reason); node != nil {
		return node
	}
	return elem.Noop()
}

// worthLogging drops the errors of a client that left or stalled.
func worthLogging(ctx context.Context, err error) error {
	if err == nil || errors.Is(err, ErrWrite) || ctx.Err() != nil {
		return nil
	}
	return err
}

// closed reports whether c is closed (or has a value ready). A nil channel
// is never closed.
func closed(c <-chan struct{}) bool {
	if c == nil {
		return false
	}
	select {
	case <-c:
		return true
	default:
		return false
	}
}
