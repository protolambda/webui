package mw_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/mw"
)

// mountedMuxes returns an outer mux that serves health checks itself and
// mounts the items mux at its catch-all "/" without path stripping.
func mountedMuxes() (outer, inner *http.ServeMux) {
	inner = itemsMux()
	inner.HandleFunc("/notes/", func(http.ResponseWriter, *http.Request) {})
	outer = http.NewServeMux()
	outer.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	})
	outer.Handle("/", inner)
	return outer, inner
}

func TestMounted(gt *testing.T) {
	t := devtest.ParallelT(gt)
	outer, inner := mountedMuxes()
	routes := mw.Mounted(outer, "/", inner)
	pattern := func(method, target string) string {
		_, p := routes.Handler(httptest.NewRequest(method, target, nil))
		return p
	}
	t.Must(be.Equal("GET /items/{id}", pattern(http.MethodGet, "/items/7")))
	t.Must(be.Equal("POST /items", pattern(http.MethodPost, "/items")))
	t.Must(be.Equal("GET /healthz", pattern(http.MethodGet, "/healthz")))
	t.Must(be.Equal("", pattern(http.MethodGet, "/missing")))
	// The inner mux's own redirect, to add the trailing slash.
	h, p := routes.Handler(httptest.NewRequest(http.MethodGet, "/notes", nil))
	ih, ip := inner.Handler(httptest.NewRequest(http.MethodGet, "/notes", nil))
	t.Must(be.Equal(ip, p))
	rec, irec := httptest.NewRecorder(), httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/notes", nil))
	ih.ServeHTTP(irec, httptest.NewRequest(http.MethodGet, "/notes", nil))
	t.Must(be.Equal(irec.Code, rec.Code))
	t.Must(be.Equal("/notes/", rec.Header().Get("Location")))
	// Another mount pattern leaves the catch-all to the outer mux.
	_, p = mw.Mounted(outer, "/items/", inner).Handler(httptest.NewRequest(http.MethodGet, "/items/7", nil))
	t.Must(be.Equal("/", p))

	t.Must(be.Panic(func() { mw.Mounted(nil, "/", inner) }))
	t.Must(be.Panic(func() { mw.Mounted(outer, "/", nil) }))
}

// TestMountedRequestLog logs the requests of a mounted mux behind
// HostCheck: only Mounted keeps the inner patterns.
func TestMountedRequestLog(gt *testing.T) {
	t := devtest.ParallelT(gt)
	outer, inner := mountedMuxes()
	site := mw.MustParseSite("https://app.example.com")
	for _, c := range []struct {
		name   string
		routes mw.Router
		want   []string
	}{
		{"none", nil, []string{"", ""}},
		{"outer", outer, []string{"/", "GET /healthz"}},
		{"mounted", mw.Mounted(outer, "/", inner), []string{"GET /items/{id}", "GET /healthz"}},
	} {
		t.Run(c.name, func(t devtest.T) {
			logger, sink := newCapture(slog.LevelInfo)
			h := mw.Chain(outer, mw.LogRequests(mw.RequestLog{Logger: logger, Routes: c.routes}), mw.HostCheck(site))
			for _, target := range []string{"/items/1", "/healthz"} {
				h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "https://app.example.com"+target, nil))
			}
			var got []string
			for _, r := range sink.Records() {
				if r.Message == "http request" {
					got = append(got, attrs(r)["pattern"])
				}
			}
			t.Must(be.DeepEqual(c.want, got))
		})
	}
}
