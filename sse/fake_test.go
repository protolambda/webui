package sse_test

import (
	"bytes"
	"net/http"
	"sync"
	"time"
)

// fakeWriter is a ResponseWriter that supports write deadlines and
// flushing, records every operation in order, and fails on demand. It is
// safe for concurrent use, so a test can read it while a loop writes.
type fakeWriter struct {
	mu        sync.Mutex
	header    http.Header
	status    int
	body      bytes.Buffer
	ops       []string
	deadlines []time.Time

	deadlineErr error // returned by SetWriteDeadline
	writeErr    error // returned by Write
	flushErr    error // returned by FlushError
}

func newFakeWriter() *fakeWriter {
	return &fakeWriter{header: make(http.Header)}
}

func (f *fakeWriter) Header() http.Header { return f.header }

func (f *fakeWriter) WriteHeader(code int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ops = append(f.ops, "header")
	if f.status == 0 {
		f.status = code
	}
}

func (f *fakeWriter) Write(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ops = append(f.ops, "write")
	if f.status == 0 {
		f.status = http.StatusOK
	}
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return f.body.Write(b)
}

// SetWriteDeadline is what http.ResponseController looks for.
func (f *fakeWriter) SetWriteDeadline(t time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ops = append(f.ops, "deadline")
	if f.deadlineErr != nil {
		return f.deadlineErr
	}
	f.deadlines = append(f.deadlines, t)
	return nil
}

// FlushError is what http.ResponseController looks for.
func (f *fakeWriter) FlushError() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ops = append(f.ops, "flush")
	return f.flushErr
}

func (f *fakeWriter) Body() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.body.String()
}

func (f *fakeWriter) Ops() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.ops...)
}

func (f *fakeWriter) Deadlines() []time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]time.Time(nil), f.deadlines...)
}

func (f *fakeWriter) Status() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.status
}

func (f *fakeWriter) FailWrites(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writeErr = err
}

// plainWriter supports neither deadlines nor flushing.
type plainWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (p *plainWriter) Header() http.Header {
	if p.header == nil {
		p.header = make(http.Header)
	}
	return p.header
}

func (p *plainWriter) WriteHeader(code int) {
	if p.status == 0 {
		p.status = code
	}
}

func (p *plainWriter) Write(b []byte) (int, error) {
	if p.status == 0 {
		p.status = http.StatusOK
	}
	return p.body.Write(b)
}
