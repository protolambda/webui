package sse_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/aria"
	"github.com/protolambda/chord/html/group/div"
	"github.com/protolambda/chord/html/group/list"
	"github.com/protolambda/chord/html/section"
	"github.com/protolambda/chord/html/text"

	"github.com/protolambda/webui/sse"
)

// liveEnd is a typical end fragment: why the stream ended and where to go.
func liveEnd(message, href, link string) elem.Node {
	return text.P(aria.Role("status"))(
		text.Text(message+" "),
		elem.If(href != "", text.A(text.Href(href))(text.Text(link))),
	)
}

// A stream handler refuses with a 200 stream that carries one end event:
// a redirect to a login page would only make the page retry.
func Example() {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if sse.HandleHEAD(w, r) {
			return
		}
		s := sse.New(w)
		if r.Header.Get("Cookie") == "" { // stands in for a session lookup
			_ = s.End(r.Context(), liveEnd("Your session ended.", "/login", "Log in again"))
			return
		}
		// ... stream with sse.Loop
	}

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/tasks/live", nil))
	fmt.Println(rec.Code, rec.Header().Get("Content-Type"))
	fmt.Print(rec.Body.String())
	// Output:
	// 200 text/event-stream; charset=utf-8
	// event: end
	// data: <p role="status">Your session ended. <a href="/login">Log in again</a></p>
}

// Send renders a fragment and frames each of its lines as a data field.
func ExampleStream_Send() {
	rec := httptest.NewRecorder()
	s := sse.New(rec)
	tasks := list.UL()(
		list.LI()(text.Text("Write the docs")),
		list.LI()(text.Text("Review <notes>")),
	)
	if err := s.Send(context.Background(), "tasks", tasks); err != nil {
		fmt.Println(err)
	}
	note := text.Pre()(text.Text("line one\nline two"))
	if err := s.Send(context.Background(), "note", note); err != nil {
		fmt.Println(err)
	}
	fmt.Println(rec.Header().Get("Cache-Control"), rec.Header().Get("X-Accel-Buffering"))
	fmt.Print(rec.Body.String())
	// Output:
	// no-store no
	// event: tasks
	// data: <ul><li>Write the docs</li><li>Review &lt;notes&gt;</li></ul>
	//
	// event: note
	// data: <pre>line one
	// data: line two</pre>
}

// Invalid event names fail before anything is written.
func ExampleStream_Send_invalidName() {
	rec := httptest.NewRecorder()
	s := sse.New(rec)
	err := s.Send(context.Background(), "tasks,notes", elem.Text("x"))
	fmt.Println(err)
	fmt.Println(s.Started())
	// Output:
	// sse: invalid event name: byte 5 is not a letter, digit, '-', '_', '.' or ':'
	// false
}

// SendText sends data verbatim, with optional id and retry fields.
func ExampleStream_SendText() {
	rec := httptest.NewRecorder()
	s := sse.New(rec)
	_ = s.SendText("count", "3", sse.ID("17"), sse.Retry(5*time.Second))
	fmt.Print(rec.Body.String())
	// Output:
	// event: count
	// id: 17
	// retry: 5000
	// data: 3
}

// Comments keep an idle stream open.
func ExampleStream_Comment() {
	rec := httptest.NewRecorder()
	s := sse.New(rec)
	_ = s.Comment("keep-alive")
	fmt.Printf("%q\n", rec.Body.String())
	// Output:
	// ": keep-alive\n\n"
}

// End sends the end event; the stream accepts nothing after it.
func ExampleStream_End() {
	rec := httptest.NewRecorder()
	s := sse.New(rec)
	_ = s.End(context.Background(), liveEnd("Live updates paused.", "/tasks", "Reload"))
	fmt.Print(rec.Body.String())
	fmt.Println(s.SendText("count", "4"))
	// Output:
	// event: end
	// data: <p role="status">Live updates paused. <a href="/tasks">Reload</a></p>
	//
	// sse: stream ended
}

// A loop sends the state at the start and after changes, and ends with an
// end event when the session ends. Here the session ends after the first
// update, as if the user logged out in another tab.
func ExampleLoop() {
	sessionDone := make(chan struct{})
	tasks := []string{"Write the docs"}

	rec := httptest.NewRecorder()
	err := sse.Loop{
		Timing: sse.Timing{Interval: time.Second, KeepAlive: 15 * time.Second, Lifetime: 10 * time.Minute},
		Update: func(ctx context.Context, s *sse.Stream) error {
			items := make([]elem.Node, 0, len(tasks))
			for _, task := range tasks {
				items = append(items, list.LI()(text.Text(task)))
			}
			err := s.Send(ctx, "tasks", list.UL()(items...))
			close(sessionDone) // stands in for a logout
			return err
		},
		End: func(reason sse.Reason) elem.Node {
			if reason == sse.ReasonDone {
				return liveEnd("Your session ended.", "/login", "Log in again")
			}
			return liveEnd("Live updates paused.", "/tasks", "Reload")
		},
		Done: sessionDone, // no update starts after it closed
		// Changes: a notifier's channel, made with make(chan struct{}, 1);
		// Stop: closed when the server stops.
	}.Run(context.Background(), sse.New(rec))
	fmt.Println(err)
	fmt.Print(rec.Body.String())
	// Output:
	// <nil>
	// event: tasks
	// data: <ul><li>Write the docs</li></ul>
	//
	// event: end
	// data: <p role="status">Your session ended. <a href="/login">Log in again</a></p>
}

// Validate checks a whole loop: its functions and its timing.
func ExampleLoop_Validate() {
	fmt.Println(sse.Loop{}.Validate(sse.DefaultWriteWindow))
	loop := sse.Loop{
		Update: func(ctx context.Context, s *sse.Stream) error { return s.SendText("count", "1") },
		End:    func(sse.Reason) elem.Node { return nil },
	}
	fmt.Println(loop.Validate(sse.DefaultWriteWindow))
	// Output:
	// sse: invalid configuration: Loop needs Update and End
	// <nil>
}

// Validate configured timing at startup.
func ExampleTiming_Validate() {
	fmt.Println(sse.Timing{}.Validate(sse.DefaultWriteWindow))
	fmt.Println(sse.Timing{KeepAlive: time.Minute}.Validate(sse.DefaultWriteWindow))
	// Output:
	// <nil>
	// sse: invalid configuration: keep-alive interval 1m0s is not shorter than the write window 30s
}

// A limiter bounds streams per key and in total.
func ExampleLimiter() {
	streams := sse.NewLimiter(2, 256)
	first, _ := streams.Acquire("account-7")
	_, ok := streams.Acquire("account-7")
	fmt.Println(ok)
	_, ok = streams.Acquire("account-7")
	fmt.Println(ok)
	first()
	first() // releasing is idempotent
	_, ok = streams.Acquire("account-7")
	fmt.Println(ok, streams.Active())
	// Output:
	// true
	// false
	// true 2
}

// A live region renders its first state on the server; children update on
// their events, and the end event replaces the region's content.
func ExampleRegion() {
	core.Dump(section.Section(attr.ID("tasks-live"), aria.Label("Tasks"), sse.Region("/tasks/live"))(
		div.Div(attr.ID("tasks"), sse.Swap("tasks"), aria.Live("polite"))(
			list.UL()(list.LI()(text.Text("Write the docs"))),
		),
		div.Div(attr.ID("task-count"), sse.Swap("count"))(text.Text("1 task")),
	))
	// Output: <section id="tasks-live" aria-label="Tasks" hx-ext="sse" sse-connect="/tasks/live" sse-swap="end" sse-close="end"><div id="tasks" sse-swap="tasks" aria-live="polite"><ul><li>Write the docs</li></ul></div><div id="task-count" sse-swap="count">1 task</div></section>
}

// Connect opens a stream without swapping the end event into the element.
func ExampleConnect() {
	core.Dump(div.Div(sse.Connect("/notes/live"))(
		div.Div(sse.Swap("notes"))(),
		text.P(sse.Swap(sse.EventEnd), aria.Role("status"))(),
	))
	// Output: <div hx-ext="sse" sse-connect="/notes/live" sse-close="end"><div sse-swap="notes"></div><p sse-swap="end" role="status"></p></div>
}

// Swap can list several events.
func ExampleSwap() {
	core.Dump(div.Div(sse.Swap("tasks", "notes"))())
	// Output: <div sse-swap="tasks,notes"></div>
}

// Region accepts same-origin paths only.
func ExampleRegion_invalidURL() {
	err := core.Render(context.Background(), div.Div(sse.Region("//other.example/live"))(), new(discard))
	fmt.Println(errors.Is(err, sse.ErrURL))
	fmt.Println(err)
	// Output:
	// true
	// render: at div[0]: attribute: sse: invalid stream URL: starts with '//', which names another host
}

// discard is an io.StringWriter that drops its input.
type discard struct{}

func (discard) WriteString(s string) (int, error) { return len(s), nil }

func ExampleCheckEventName() {
	fmt.Println(sse.CheckEventName("task-list"))
	fmt.Println(sse.CheckEventName("open"))
	fmt.Println(sse.CheckEventName("task list"))
	// Output:
	// <nil>
	// sse: invalid event name: reserved for connection events
	// sse: invalid event name: byte 4 is not a letter, digit, '-', '_', '.' or ':'
}

// HEAD requests get the stream header only.
func ExampleHandleHEAD() {
	rec := httptest.NewRecorder()
	fmt.Println(sse.HandleHEAD(rec, httptest.NewRequest(http.MethodHead, "/tasks/live", nil)))
	fmt.Println(rec.Code, rec.Header().Get("Content-Type"), rec.Body.Len())
	// Output:
	// true
	// 200 text/event-stream; charset=utf-8 0
}
