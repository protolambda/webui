package webuitest

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"github.com/protolambda/mustbe/assertion"
)

// Logs captures log records in memory, as text, for tests that check what
// an application logs, such as that secrets never reach the log. It is
// safe for concurrent use. The zero value is not usable: use [NewLogs].
type Logs struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	handler slog.Handler
}

// NewLogs returns an empty capture whose handler records every level.
func NewLogs() *Logs {
	l := &Logs{}
	l.handler = slog.NewTextHandler(lockedWriter{l}, &slog.HandlerOptions{Level: slog.Level(-1 << 10)})
	return l
}

type lockedWriter struct{ l *Logs }

func (w lockedWriter) Write(p []byte) (int, error) {
	w.l.mu.Lock()
	defer w.l.mu.Unlock()
	return w.l.buf.Write(p)
}

// Handler returns the slog handler that records into l.
func (l *Logs) Handler() slog.Handler { return l.handler }

// Logger returns a logger that records into l, for mw.RequestLog.Logger,
// [LogTo] or an application's own logging.
func (l *Logs) Logger() *slog.Logger { return slog.New(l.handler) }

// String returns the text of the records so far.
func (l *Logs) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// Reset drops the records so far.
func (l *Logs) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf.Reset()
}

// NotLogged asserts, when it is checked, that no record so far contains
// any of secrets, also in the quoted and escaped form in which the text
// handler writes values with spaces, quotes or control characters. A
// failure names a secret by its position in secrets and counts its
// occurrences, never showing it. Empty secrets are a programming error:
// NotLogged panics on them.
func (l *Logs) NotLogged(secrets ...string) assertion.Assertion {
	checkValues("Logs.NotLogged", secrets)
	return notLogged{l: l, secrets: append([]string(nil), secrets...)}
}

type notLogged struct {
	l       *Logs
	secrets []string
}

func (a notLogged) String() string { return fmt.Sprintf("notLogged(%d secrets)", len(a.secrets)) }

func (a notLogged) Check(context.Context) error {
	text := a.l.String()
	var found []string
	for i, s := range a.secrets {
		n := strings.Count(text, s)
		if q := strconv.Quote(s); q[1:len(q)-1] != s {
			n += strings.Count(text, q[1:len(q)-1])
		}
		if n > 0 {
			found = append(found, fmt.Sprintf("secret %d appears %d times", i+1, n))
		}
	}
	if len(found) == 0 {
		return nil
	}
	return fmt.Errorf("%w: the logs leak secrets: %s", ErrLeak, strings.Join(found, "; "))
}
