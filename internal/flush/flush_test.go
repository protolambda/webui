package flush_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/internal/flush"
)

// forwarder passes flushes on, as the wrappers of web and mw do.
type forwarder struct{ http.ResponseWriter }

func (f forwarder) FlushError() error {
	return http.NewResponseController(f.ResponseWriter).Flush()
}
func (f forwarder) Unwrap() http.ResponseWriter { return f.ResponseWriter }
func (forwarder) ForwardsFlush()                {}

var _ flush.Forwarder = forwarder{}

// passer passes flushes on too, but is not marked: a wrapper of another
// package.
type passer struct{ http.ResponseWriter }

func (p passer) FlushError() error {
	return http.NewResponseController(p.ResponseWriter).Flush()
}
func (p passer) Unwrap() http.ResponseWriter { return p.ResponseWriter }

// unwrapper only unwraps.
type unwrapper struct{ http.ResponseWriter }

func (u unwrapper) Unwrap() http.ResponseWriter { return u.ResponseWriter }

// embedder hides the flush: it has neither a flush method nor Unwrap.
type embedder struct{ http.ResponseWriter }

// cycle is a forwarder that unwraps to itself.
type cycle struct{ forwarder }

func (c *cycle) Unwrap() http.ResponseWriter { return c }

func TestSupported(gt *testing.T) {
	t := devtest.ParallelT(gt)
	rec := func() http.ResponseWriter { return httptest.NewRecorder() }
	cases := map[string]struct {
		w    http.ResponseWriter
		want bool
	}{
		"recorder":                           {rec(), true},
		"forwarder":                          {forwarder{rec()}, true},
		"forwarders and unwrapper":           {forwarder{unwrapper{forwarder{rec()}}}, true},
		"embedder":                           {embedder{rec()}, false},
		"forwarder of embedder":              {forwarder{embedder{rec()}}, false},
		"unwrapper of forwarder of embedder": {unwrapper{forwarder{embedder{rec()}}}, false},
		"forwarder of nil":                   {forwarder{nil}, false},
	}
	for name, c := range cases {
		t.Mustf(be.Equal(c.want, flush.Supported(c.w)), "%s", name)
		// The check agrees with the flush.
		err := http.NewResponseController(c.w).Flush()
		t.Mustf(be.Equal(!c.want, errors.Is(err, http.ErrNotSupported)), "%s: flush error %v", name, err)
	}

	// The check looks at types: a wrapper that is no forwarder decides
	// with its flush method, and only the flush itself fails.
	w := passer{embedder{rec()}}
	t.Must(be.True(flush.Supported(w)))
	t.Must(be.ErrorIs(http.NewResponseController(w).Flush(), http.ErrNotSupported))

	// The controller would loop forever.
	t.Must(be.False(flush.Supported(&cycle{forwarder{rec()}})))
}
