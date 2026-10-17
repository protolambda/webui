package app

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// The demo data is bounded: every list and text that a visitor can grow has
// a fixed maximum, checked before anything is stored.
const (
	maxTasks       = 50
	maxTitle       = 80
	maxDescription = 1000
	maxNotes       = 20
	maxNote        = 280
	maxProjectName = 60
	maxName        = 40
	maxQuery       = 100
)

// defaultProject is the project name of the dialogs page before a rename.
const defaultProject = "Website relaunch"

var errListFull = errors.New("gallery: the task list is full")

// priority is the priority of a task.
type priority string

const (
	priorityLow    priority = "low"
	priorityNormal priority = "normal"
	priorityHigh   priority = "high"
)

var priorities = []priority{priorityLow, priorityNormal, priorityHigh}

// labels are the labels a task may carry.
var labels = []string{"bug", "docs", "chore"}

// filter selects tasks by state.
type filter string

const (
	filterAll  filter = "all"
	filterOpen filter = "open"
	filterDone filter = "done"
)

var filters = []filter{filterAll, filterOpen, filterDone}

// parseFilter returns the filter named v, or filterAll for anything else.
func parseFilter(v string) filter {
	if f := filter(v); slices.Contains(filters, f) {
		return f
	}
	return filterAll
}

// task is an item of the demo task list.
type task struct {
	ID       int
	Title    string
	Priority priority
	Labels   []string
	Done     bool
}

// taskList is the in-memory task list that the components and htmx pages
// show. It is safe for concurrent use; its methods return copies.
type taskList struct {
	mu    sync.Mutex
	next  int
	items []task
}

func newTaskList() *taskList {
	l := &taskList{}
	for _, t := range []task{
		{Title: "Write the release notes", Priority: priorityHigh, Labels: []string{"docs"}},
		{Title: "Review the settings page", Priority: priorityNormal},
		{Title: "Fix the login redirect", Priority: priorityHigh, Labels: []string{"bug"}, Done: true},
		{Title: "Tidy the task board", Priority: priorityLow, Labels: []string{"chore"}},
		{Title: "Plan the next iteration", Priority: priorityNormal},
		{Title: "Update the screenshots", Priority: priorityLow, Labels: []string{"docs"}, Done: true},
		{Title: "Answer the open questions", Priority: priorityNormal},
	} {
		_, _ = l.add(t)
	}
	return l
}

// list returns the tasks that f selects, in order.
func (l *taskList) list(f filter) []task {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]task, 0, len(l.items))
	for _, t := range l.items {
		if f == filterAll || (f == filterDone) == t.Done {
			t.Labels = slices.Clone(t.Labels)
			out = append(out, t)
		}
	}
	return out
}

// search returns the tasks whose title contains q, ignoring case.
func (l *taskList) search(q string) []task {
	q = strings.ToLower(q)
	var out []task
	for _, t := range l.list(filterAll) {
		if strings.Contains(strings.ToLower(t.Title), q) {
			out = append(out, t)
		}
	}
	return out
}

// add appends t with a new ID, or fails with errListFull.
func (l *taskList) add(t task) (task, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.items) >= maxTasks {
		return task{}, errListFull
	}
	l.next++
	t.ID = l.next
	t.Labels = slices.Clone(t.Labels)
	l.items = append(l.items, t)
	return t, nil
}

// remove deletes the task with the ID and reports whether it existed.
func (l *taskList) remove(id int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	i := slices.IndexFunc(l.items, func(t task) bool { return t.ID == id })
	if i < 0 {
		return false
	}
	l.items = slices.Delete(l.items, i, i+1)
	return true
}

// counts returns the number of open tasks and of all tasks.
func (l *taskList) counts() (open, all int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, t := range l.items {
		if !t.Done {
			open++
		}
	}
	return open, len(l.items)
}

// project holds the project name of the dialogs page.
type project struct {
	mu   sync.Mutex
	name string
}

func (p *project) get() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.name
}

func (p *project) set(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.name = name
}

// notifier signals changes to the event streams that subscribed, without
// blocking: every subscriber has a channel with a buffer of one, so a burst
// of changes leaves one pending signal. It is safe for concurrent use.
type notifier struct {
	mu   sync.Mutex
	subs map[chan struct{}]struct{}
}

// subscribe returns a channel that receives change signals, and the
// function that ends the subscription.
func (n *notifier) subscribe() (<-chan struct{}, func()) {
	c := make(chan struct{}, 1)
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.subs == nil {
		n.subs = make(map[chan struct{}]struct{})
	}
	n.subs[c] = struct{}{}
	return c, func() {
		n.mu.Lock()
		defer n.mu.Unlock()
		delete(n.subs, c)
	}
}

// notify signals every subscriber.
func (n *notifier) notify() {
	n.mu.Lock()
	defer n.mu.Unlock()
	for c := range n.subs {
		select {
		case c <- struct{}{}:
		default:
		}
	}
}

// cleanText returns v without surrounding space, and whether it is valid
// text of 1 to maxRunes characters: valid UTF-8 without control characters.
// Callers show a fixed message when it is not; v is never echoed in errors.
func cleanText(v string, maxRunes int) (string, bool) {
	v = strings.TrimSpace(v)
	if v == "" || !utf8.ValidString(v) || utf8.RuneCountInString(v) > maxRunes {
		return v, false
	}
	for _, r := range v {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return v, false
		}
	}
	return v, true
}

// cleanLine is cleanText for single-line values: line breaks and tabs are
// refused too.
func cleanLine(v string, maxRunes int) (string, bool) {
	v, ok := cleanText(v, maxRunes)
	if ok && strings.ContainsAny(v, "\r\n\t") {
		return v, false
	}
	return v, ok
}
