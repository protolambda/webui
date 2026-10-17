package mw_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/mw"
)

// tracing returns a middleware that records its name around the next handler.
func tracing(trace *[]string, name string) mw.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*trace = append(*trace, name+">")
			next.ServeHTTP(w, r)
			*trace = append(*trace, "<"+name)
		})
	}
}

func TestChainOrder(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var trace []string
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { trace = append(trace, "h") })
	chained := mw.Chain(h, tracing(&trace, "a"), nil, tracing(&trace, "b"))
	chained.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	t.Must(be.Equal("a> b> h <b <a", strings.Join(trace, " ")))
}

func TestChainWithoutMiddleware(gt *testing.T) {
	t := devtest.ParallelT(gt)
	h := http.NotFoundHandler()
	rec := httptest.NewRecorder()
	mw.Chain(h).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	t.Must(be.Equal(http.StatusNotFound, rec.Code))
}

func TestChainPanicsOnMisuse(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Panic(func() { mw.Chain(nil) }))
	nilMiddleware := func(http.Handler) http.Handler { return nil }
	t.Must(be.Panic(func() { mw.Chain(http.NotFoundHandler(), nilMiddleware) }))
}
