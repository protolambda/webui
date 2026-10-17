package sse_test

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/mustbe"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/sse"
)

// loopRun is a Loop running on its own goroutine in a synctest bubble.
type loopRun struct {
	w       *fakeWriter
	s       *sse.Stream
	updates *atomic.Int64
	cancel  context.CancelFunc
	done    chan error
}

// counting returns a loop whose updates send a "count" event with the
// number of updates so far, and whose end fragment names the reason.
func counting(timing sse.Timing) (sse.Loop, *atomic.Int64) {
	var n atomic.Int64
	return sse.Loop{
		Timing: timing,
		Update: func(ctx context.Context, s *sse.Stream) error {
			return s.SendText("count", strconv.FormatInt(n.Add(1), 10))
		},
		End: func(reason sse.Reason) elem.Node { return elem.Text("ended: " + reason.String()) },
	}, &n
}

// startLoop runs l in the bubble of gt, and waits until it blocks.
func startLoop(gt *testing.T, l sse.Loop, opts ...sse.Option) *loopRun {
	w := newFakeWriter()
	s := sse.New(w, opts...)
	ctx, cancel := context.WithCancel(gt.Context())
	gt.Cleanup(cancel)
	r := &loopRun{w: w, s: s, cancel: cancel, done: make(chan error, 1)}
	go func() { r.done <- l.Run(ctx, s) }()
	synctest.Wait()
	return r
}

// returned reports whether Run returned, and its error.
func (r *loopRun) returned() (bool, error) {
	synctest.Wait()
	select {
	case err := <-r.done:
		r.done <- err // keep it for later calls
		return true, err
	default:
		return false, nil
	}
}

// gated makes the first update of l wait until the returned gate closes,
// so that the channels a test readies meanwhile are all ready together
// when the loop selects next.
func gated(l sse.Loop) (sse.Loop, chan struct{}) {
	gate := make(chan struct{})
	update, first := l.Update, true
	l.Update = func(ctx context.Context, s *sse.Stream) error {
		if first { // only the goroutine of Run reads and writes it
			first = false
			<-gate
		}
		return update(ctx, s)
	}
	return l, gate
}

// data returns the data of the events so far.
func (r *loopRun) data(t mustT) []string {
	t.Helper()
	var out []string
	for _, e := range mustParse(t, r.w.Body()).Events {
		out = append(out, e.Type+"="+e.Data)
	}
	return out
}

func TestLoopCoalescesChanges(gt *testing.T) {
	gt.Parallel()
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		changes := make(chan struct{})
		l, _ := counting(sse.Timing{Interval: time.Second, Lifetime: time.Hour})
		l.Changes = changes
		r := startLoop(gt, l)
		t.Must(be.DeepEqual([]string{"count=1"}, r.data(t))) // the state at the start

		// A burst right after an update waits for the end of the interval,
		// and becomes one update.
		for range 5 {
			changes <- struct{}{}
		}
		synctest.Wait()
		t.Must(be.DeepEqual([]string{"count=1"}, r.data(t)))
		time.Sleep(time.Second - time.Nanosecond)
		synctest.Wait()
		t.Must(be.DeepEqual([]string{"count=1"}, r.data(t)))
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		t.Must(be.DeepEqual([]string{"count=1", "count=2"}, r.data(t)))

		// A change after a quiet interval updates at once.
		time.Sleep(5 * time.Second)
		changes <- struct{}{}
		synctest.Wait()
		t.Must(be.DeepEqual([]string{"count=1", "count=2", "count=3"}, r.data(t)))

		// Changes during the interval after it wait for its end.
		time.Sleep(time.Second / 4)
		changes <- struct{}{}
		time.Sleep(time.Second / 4)
		changes <- struct{}{}
		synctest.Wait()
		t.Must(be.Len(r.data(t), 3))
		time.Sleep(time.Second / 2)
		synctest.Wait()
		t.Must(be.DeepEqual([]string{"count=1", "count=2", "count=3", "count=4"}, r.data(t)))

		// The client leaves: no end event.
		r.cancel()
		ok, err := r.returned()
		t.Must(be.True(ok))
		t.Must(be.NoError(err))
		t.Must(be.Len(r.data(t), 4))
	})
}

func TestLoopKeepAlive(gt *testing.T) {
	gt.Parallel()
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		start := time.Now()
		l, _ := counting(sse.Timing{KeepAlive: 10 * time.Second, Lifetime: time.Hour})
		r := startLoop(gt, l)
		time.Sleep(35 * time.Second)
		synctest.Wait()
		p := mustParse(t, r.w.Body())
		t.Must(be.DeepEqual([]string{"keep-alive", "keep-alive", "keep-alive"}, p.Comments))
		t.Must(be.Len(p.Events, 1))
		// Every write pushed the deadline: start, update, three comments.
		deadlines := r.w.Deadlines()
		t.Must(be.Len(deadlines, 5))
		t.Must(be.Equal(start.Add(30*time.Second+30*time.Second), deadlines[4]))
		ok, _ := r.returned()
		t.Must(be.False(ok))
	})
}

// A huge write window keeps the default keep-alive interval: deriving it
// from the window must not overflow.
func TestLoopHugeWriteWindow(gt *testing.T) {
	gt.Parallel()
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		l, _ := counting(sse.Timing{})
		r := startLoop(gt, l, sse.WithWriteWindow(math.MaxInt64))
		ok, err := r.returned()
		t.Must(be.False(ok))
		t.Must(be.NoError(err))
		time.Sleep(sse.DefaultKeepAlive)
		synctest.Wait()
		p := mustParse(t, r.w.Body())
		t.Must(be.DeepEqual([]string{"keep-alive"}, p.Comments))
		t.Must(be.Len(p.Events, 1))
	})
}

// A zero keep-alive interval stays below a short write window.
func TestLoopKeepAliveDefaultFollowsWriteWindow(gt *testing.T) {
	gt.Parallel()
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		l, _ := counting(sse.Timing{})
		r := startLoop(gt, l, sse.WithWriteWindow(3*time.Second))
		time.Sleep(5 * time.Second)
		synctest.Wait()
		t.Must(be.Len(mustParse(t, r.w.Body()).Comments, 2)) // every 2s
	})
}

func TestLoopDefaults(gt *testing.T) {
	gt.Parallel()
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		l, _ := counting(sse.Timing{})
		r := startLoop(gt, l)
		time.Sleep(sse.DefaultLifetime - time.Nanosecond)
		synctest.Wait()
		p := mustParse(t, r.w.Body())
		t.Must(be.Len(p.Comments, int(sse.DefaultLifetime/sse.DefaultKeepAlive)-1))
		ok, _ := r.returned()
		t.Must(be.False(ok))
		time.Sleep(time.Nanosecond)
		ok, err := r.returned()
		t.Must(be.True(ok))
		t.Must(be.NoError(err))
		t.Must(be.DeepEqual([]string{"count=1", "end=ended: lifetime"}, r.data(t)))
	})
}

func TestLoopLifetimeEnds(gt *testing.T) {
	gt.Parallel()
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		l, _ := counting(sse.Timing{Lifetime: time.Minute})
		r := startLoop(gt, l)
		time.Sleep(time.Minute)
		ok, err := r.returned()
		t.Must(be.True(ok))
		t.Must(be.NoError(err))
		t.Must(be.DeepEqual([]string{"count=1", "end=ended: lifetime"}, r.data(t)))
		t.Must(be.True(r.s.Ended()))
		ops := len(r.w.Ops())
		time.Sleep(time.Hour)
		synctest.Wait()
		t.Must(be.Equal(ops, len(r.w.Ops()))) // nothing after the end
	})
}

func TestLoopDoneEnds(gt *testing.T) {
	gt.Parallel()
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		done := make(chan struct{})
		l, _ := counting(sse.Timing{})
		l.Done = done
		r := startLoop(gt, l)
		time.Sleep(time.Minute)
		close(done)
		ok, err := r.returned()
		t.Must(be.True(ok))
		t.Must(be.NoError(err))
		t.Must(be.DeepEqual([]string{"count=1", "end=ended: done"}, r.data(t)))
	})
}

func TestLoopChangesClosedEnds(gt *testing.T) {
	gt.Parallel()
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		changes := make(chan struct{})
		l, _ := counting(sse.Timing{})
		l.Changes = changes
		r := startLoop(gt, l)
		close(changes)
		ok, err := r.returned()
		t.Must(be.True(ok))
		t.Must(be.NoError(err))
		t.Must(be.DeepEqual([]string{"count=1", "end=ended: closed"}, r.data(t)))
	})
}

// When the server stops, the stream ends without an end event, so the
// page reconnects. The loop is idle here, so Stop is the only ready case;
// TestLoopStopWinsTogether covers Stop ready together with others.
func TestLoopStopReconnects(gt *testing.T) {
	gt.Parallel()
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		stop := make(chan struct{})
		l, _ := counting(sse.Timing{})
		l.Stop = stop
		r := startLoop(gt, l)
		close(stop)
		ok, err := r.returned()
		t.Must(be.True(ok))
		t.Must(be.NoError(err))
		t.Must(be.DeepEqual([]string{"count=1"}, r.data(t)))
		t.Must(be.False(r.s.Ended()))
	})
}

// Stop wins also when another case is ready at the same select: the
// channels close while the first update runs, so the loop finds them all
// ready together when it selects next.
func TestLoopStopWinsTogether(gt *testing.T) {
	gt.Parallel()
	for _, also := range []string{"done", "closed", "change", "lifetime"} {
		for range 20 { // select picks one of the ready cases at random
			synctest.Test(gt, func(gt *testing.T) {
				t := mustbe.WrapT(gt)
				stop, done, changes := make(chan struct{}), make(chan struct{}), make(chan struct{}, 1)
				l, _ := counting(sse.Timing{Lifetime: time.Minute})
				l.Stop, l.Done, l.Changes = stop, done, changes
				l, gate := gated(l)
				r := startLoop(gt, l) // blocks in the first update
				close(stop)
				switch also {
				case "done":
					close(done)
				case "closed":
					close(changes)
				case "change":
					changes <- struct{}{}
					time.Sleep(2 * time.Second) // past the interval: no coalescing
				case "lifetime":
					time.Sleep(time.Minute) // also past a keep-alive tick
				}
				close(gate)
				ok, err := r.returned()
				t.Must(be.Annotate(be.True(ok), "case %s", also))
				t.Must(be.NoError(err))
				t.Must(be.Annotate(be.DeepEqual([]string{"count=1"}, r.data(t)), "case %s", also))
				t.Must(be.False(r.s.Ended()))
			})
		}
	}
}

// Done takes precedence over updates: no update starts after it closed.
func TestLoopDoneWinsOverUpdates(gt *testing.T) {
	gt.Parallel()
	// Done closed before Run: only the end event, no state.
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		done := make(chan struct{})
		close(done)
		l, n := counting(sse.Timing{})
		l.Done = done
		r := startLoop(gt, l)
		ok, err := r.returned()
		t.Must(be.True(ok))
		t.Must(be.NoError(err))
		t.Must(be.Equal(int64(0), n.Load()))
		t.Must(be.DeepEqual([]string{"end=ended: done"}, r.data(t)))
	})
	// Done closed together with a pending change, while an update runs.
	for range 20 { // select picks one of the ready cases at random
		synctest.Test(gt, func(gt *testing.T) {
			t := mustbe.WrapT(gt)
			changes, done := make(chan struct{}, 1), make(chan struct{})
			l, n := counting(sse.Timing{Interval: time.Second})
			l.Changes, l.Done = changes, done
			l, gate := gated(l)
			r := startLoop(gt, l) // blocks in the first update
			changes <- struct{}{}
			time.Sleep(2 * time.Second) // past the interval: no coalescing
			close(done)
			close(gate)
			ok, err := r.returned()
			t.Must(be.True(ok))
			t.Must(be.NoError(err))
			t.Must(be.Equal(int64(1), n.Load()))
			t.Must(be.DeepEqual([]string{"count=1", "end=ended: done"}, r.data(t)))
		})
	}
	// Stop still wins over Done before Run.
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		stop, done := make(chan struct{}), make(chan struct{})
		close(stop)
		close(done)
		l, n := counting(sse.Timing{})
		l.Stop, l.Done = stop, done
		r := startLoop(gt, l)
		ok, err := r.returned()
		t.Must(be.True(ok))
		t.Must(be.NoError(err))
		t.Must(be.Equal(int64(0), n.Load()))
		t.Must(be.Equal("", r.w.Body()))
	})
}

// A failed update while the server stops is worth logging, but sends no
// end event, so the page reconnects after the restart.
func TestLoopUpdateFailureWhileStopping(gt *testing.T) {
	gt.Parallel()
	errStore := errors.New("store unavailable")
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		stop := make(chan struct{})
		l, _ := counting(sse.Timing{})
		l.Stop = stop
		l.Update = func(ctx context.Context, s *sse.Stream) error {
			close(stop)
			return errStore
		}
		r := startLoop(gt, l)
		ok, err := r.returned()
		t.Must(be.True(ok))
		t.Must(be.ErrorIs(err, errStore))
		t.Must(be.Equal("", r.w.Body()))
		t.Must(be.False(r.s.Ended()))
	})
}

// An update cut short by the request context is a normal end: the client
// left, so nothing is logged and nothing more is written.
func TestLoopUpdateFailureAfterClientLeft(gt *testing.T) {
	gt.Parallel()
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		l, _ := counting(sse.Timing{})
		l.Update = func(ctx context.Context, s *sse.Stream) error {
			<-ctx.Done()
			return ctx.Err()
		}
		r := startLoop(gt, l)
		r.cancel()
		ok, err := r.returned()
		t.Must(be.True(ok))
		t.Must(be.NoError(err))
		t.Must(be.Equal("", r.w.Body()))
		t.Must(be.False(r.s.Ended()))
	})
}

func TestLoopUpdateFailureEnds(gt *testing.T) {
	gt.Parallel()
	errStore := errors.New("store unavailable")
	for _, failAt := range []int64{1, 3} {
		synctest.Test(gt, func(gt *testing.T) {
			t := mustbe.WrapT(gt)
			changes := make(chan struct{})
			l, n := counting(sse.Timing{})
			update := l.Update
			l.Update = func(ctx context.Context, s *sse.Stream) error {
				if n.Load()+1 == failAt {
					n.Add(1)
					return errStore
				}
				return update(ctx, s)
			}
			l.Changes = changes
			r := startLoop(gt, l)
			for range failAt - 1 {
				time.Sleep(time.Minute)
				changes <- struct{}{}
			}
			ok, err := r.returned()
			t.Must(be.True(ok))
			t.Must(be.ErrorIs(err, errStore))
			data := r.data(t)
			t.Must(be.Len(data, int(failAt)))
			t.Must(be.Equal("end=ended: failed", data[len(data)-1]))
		})
	}
}

// A failure that comes from rendering a fragment sends nothing of it.
func TestLoopRenderFailureEnds(gt *testing.T) {
	gt.Parallel()
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		l, _ := counting(sse.Timing{})
		l.Update = func(ctx context.Context, s *sse.Stream) error {
			return s.Send(ctx, "item", elem.Bundle{elem.Raw("partial"), failingNode{errRender}})
		}
		r := startLoop(gt, l)
		ok, err := r.returned()
		t.Must(be.True(ok))
		t.Must(be.ErrorIs(err, errRender))
		t.Must(be.DeepEqual([]string{"end=ended: failed"}, r.data(t)))
	})
}

func TestLoopEndFragmentFailure(gt *testing.T) {
	gt.Parallel()
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		done := make(chan struct{})
		l, _ := counting(sse.Timing{})
		l.Done = done
		l.End = func(sse.Reason) elem.Node { return failingNode{errRender} }
		r := startLoop(gt, l)
		close(done)
		ok, err := r.returned()
		t.Must(be.True(ok))
		t.Must(be.ErrorIs(err, errRender)) // worth logging
		t.Must(be.DeepEqual([]string{"count=1", "end="}, r.data(t)))
	})
}

func TestLoopNilEndFragment(gt *testing.T) {
	gt.Parallel()
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		done := make(chan struct{})
		l, _ := counting(sse.Timing{})
		l.Done = done
		l.End = func(sse.Reason) elem.Node { return nil }
		r := startLoop(gt, l)
		close(done)
		ok, err := r.returned()
		t.Must(be.True(ok))
		t.Must(be.NoError(err))
		t.Must(be.DeepEqual([]string{"count=1", "end="}, r.data(t)))
	})
}

// Update may end the stream itself.
func TestLoopUpdateEnds(gt *testing.T) {
	gt.Parallel()
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		changes := make(chan struct{})
		l, n := counting(sse.Timing{})
		update := l.Update
		l.Update = func(ctx context.Context, s *sse.Stream) error {
			if n.Load() == 1 {
				return s.End(ctx, elem.Text("the item was deleted"))
			}
			return update(ctx, s)
		}
		l.Changes = changes
		r := startLoop(gt, l)
		time.Sleep(time.Minute)
		changes <- struct{}{}
		ok, err := r.returned()
		t.Must(be.True(ok))
		t.Must(be.NoError(err))
		t.Must(be.DeepEqual([]string{"count=1", "end=the item was deleted"}, r.data(t)))
	})
}

// A client that left or stalled is a normal end: Run returns nil.
func TestLoopClientGone(gt *testing.T) {
	gt.Parallel()
	errConn := errors.New("broken pipe")
	for _, when := range []string{"keep-alive", "update", "end"} {
		synctest.Test(gt, func(gt *testing.T) {
			t := mustbe.WrapT(gt)
			changes, done := make(chan struct{}), make(chan struct{})
			l, _ := counting(sse.Timing{})
			l.Changes, l.Done = changes, done
			r := startLoop(gt, l)
			r.w.FailWrites(errConn)
			switch when {
			case "keep-alive":
				time.Sleep(sse.DefaultKeepAlive)
			case "update":
				changes <- struct{}{}
				time.Sleep(time.Second) // before the first keep-alive
			case "end":
				close(done)
			}
			ok, err := r.returned()
			t.Must(be.Annotate(be.True(ok), "case %s", when))
			t.Must(be.NoError(err))
		})
	}
}

func TestLoopStartConditions(gt *testing.T) {
	gt.Parallel()
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		l, n := counting(sse.Timing{})

		// The client left already: nothing is written.
		w := newFakeWriter()
		ctx, cancel := context.WithCancel(gt.Context())
		cancel()
		t.Must(be.NoError(l.Run(ctx, sse.New(w))))
		t.Must(be.Empty(w.Ops()))

		// The server stops already: an empty stream, so the browser
		// reconnects.
		stop := make(chan struct{})
		close(stop)
		stopping := l
		stopping.Stop = stop
		w = newFakeWriter()
		t.Must(be.NoError(stopping.Run(gt.Context(), sse.New(w))))
		t.Must(be.Equal("", w.Body()))
		t.Must(be.Equal(sse.ContentType, w.Header().Get("Content-Type")))

		// An ended stream.
		w = newFakeWriter()
		s := sse.New(w)
		t.Must(be.NoError(s.End(gt.Context(), elem.Text("bye"))))
		t.Must(be.ErrorIs(l.Run(gt.Context(), s), sse.ErrEnded))

		// A client that cannot be reached at the start.
		w = newFakeWriter()
		w.deadlineErr = errors.New("closed")
		t.Must(be.NoError(l.Run(gt.Context(), sse.New(w))))

		// A writer that cannot flush is a configuration bug: worth
		// logging, and nothing is written, so the handler can answer.
		p := &plainWriter{}
		err := l.Run(gt.Context(), sse.New(p))
		t.Must(be.ErrorIs(err, http.ErrNotSupported))
		t.Must(be.False(errors.Is(err, sse.ErrWrite)))
		t.Must(be.Equal(0, p.status))
		t.Must(be.Equal("", p.body.String()))

		t.Must(be.Equal(int64(0), n.Load()))
	})
}

func TestLoopConfig(gt *testing.T) {
	gt.Parallel()
	synctest.Test(gt, func(gt *testing.T) {
		t := mustbe.WrapT(gt)
		valid, _ := counting(sse.Timing{})
		cases := map[string]struct {
			loop   func(l *sse.Loop)
			window time.Duration
		}{
			"no update":             {loop: func(l *sse.Loop) { l.Update = nil }},
			"no end":                {loop: func(l *sse.Loop) { l.End = nil }},
			"negative interval":     {loop: func(l *sse.Loop) { l.Interval = -1 }},
			"negative keep-alive":   {loop: func(l *sse.Loop) { l.KeepAlive = -1 }},
			"negative lifetime":     {loop: func(l *sse.Loop) { l.Lifetime = -1 }},
			"keep-alive = window":   {loop: func(l *sse.Loop) { l.KeepAlive = sse.DefaultWriteWindow }},
			"keep-alive > window":   {loop: func(l *sse.Loop) { l.KeepAlive = time.Minute }},
			"keep-alive = custom":   {loop: func(l *sse.Loop) { l.KeepAlive = 5 * time.Second }, window: 5 * time.Second},
			"default > tiny window": {loop: func(l *sse.Loop) {}, window: time.Nanosecond},
		}
		for name, c := range cases {
			l := valid
			c.loop(&l)
			w := newFakeWriter()
			err := l.Run(gt.Context(), sse.New(w, sse.WithWriteWindow(c.window)))
			t.Must(be.Annotate(be.ErrorIs(err, sse.ErrConfig), "case %s", name))
			t.Must(be.Annotate(be.Empty(w.Ops()), "case %s", name))
			// Validate checks the whole loop, as Run does.
			t.Must(be.Annotate(be.ErrorIs(l.Validate(c.window), sse.ErrConfig), "case %s", name))
		}
		t.Must(be.NoError(valid.Validate(0)))
		t.Must(be.NoError(valid.Validate(5 * time.Second)))
	})
}

func TestTimingValidate(gt *testing.T) {
	gt.Parallel()
	t := mustbe.WrapT(gt)
	t.Must(be.NoError(sse.Timing{}.Validate(0)))
	t.Must(be.NoError(sse.Timing{}.Validate(sse.DefaultWriteWindow)))
	t.Must(be.NoError(sse.Timing{}.Validate(9 * time.Second))) // derived: 6s
	t.Must(be.NoError(sse.Timing{KeepAlive: 29 * time.Second}.Validate(0)))
	t.Must(be.NoError(sse.Timing{Interval: time.Hour, Lifetime: time.Minute}.Validate(0)))
	t.Must(be.ErrorIs(sse.Timing{KeepAlive: 30 * time.Second}.Validate(0), sse.ErrConfig))
	t.Must(be.ErrorIs(sse.Timing{KeepAlive: 9 * time.Second}.Validate(9*time.Second), sse.ErrConfig))
	t.Must(be.ErrorIs(sse.Timing{Interval: -time.Second}.Validate(0), sse.ErrConfig))
	t.Must(be.ErrorIs(sse.Timing{}.Validate(time.Nanosecond), sse.ErrConfig))
	// Deriving the keep-alive interval from a huge window does not
	// overflow.
	for _, window := range []time.Duration{math.MaxInt64, math.MaxInt64/2 + 1, 200 * 365 * 24 * time.Hour} {
		t.Must(be.NoError(sse.Timing{}.Validate(window)))
	}
	t.Must(be.NoError(sse.Timing{}.Validate(2 * time.Nanosecond))) // derived: 1ns
	t.Must(be.NoError(sse.Timing{}.Validate(sse.DefaultKeepAlive * 3 / 2)))
}

func TestReasonString(gt *testing.T) {
	gt.Parallel()
	t := mustbe.WrapT(gt)
	got := []string{
		sse.ReasonDone.String(), sse.ReasonLifetime.String(), sse.ReasonClosed.String(),
		sse.ReasonFailed.String(), sse.Reason(0).String(), sse.Reason(200).String(),
	}
	t.Must(be.Equal("done lifetime closed failed reason(0) reason(200)", strings.Join(got, " ")))
}
