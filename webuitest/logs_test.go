package webuitest_test

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/mw"
	"github.com/protolambda/webui/webuitest"
)

func TestLogs(gt *testing.T) {
	t := devtest.ParallelT(gt)
	const secret = "reset-token-4f2a"
	logs := webuitest.NewLogs()
	logs.Logger().Debug("starting", "level", "debug")
	t.Must(be.Substring(logs.String(), "msg=starting"))
	t.Must(logs.NotLogged(secret))

	logs.Logger().Info("reset", "token", secret)
	err := logs.NotLogged("other", secret).Check(t.Context())
	t.Must(be.ErrorIs(err, webuitest.ErrLeak))
	t.Must(be.ErrorContains(err, "the logs leak secrets: secret 2 appears 1 times"))
	t.Must(be.False(strings.Contains(err.Error(), secret)))

	logs.Reset()
	t.Must(be.Equal("", logs.String()))
	quoted := "a \"quoted\"\nvalue"
	logs.Logger().Info("x", "v", quoted)
	t.Must(be.ErrorContains(logs.NotLogged(quoted).Check(t.Context()), "secret 1 appears 1 times"))
	t.Must(be.Panic(func() { logs.NotLogged("") }))
	t.Must(be.Equal("notLogged(1 secrets)", logs.NotLogged("x").String()))
}

func TestLogsRequests(gt *testing.T) {
	t := devtest.ParallelT(gt)
	const secret = "share-link-token-9c1e"
	logs := webuitest.NewLogs()
	h := mw.LogRequests(mw.RequestLog{Logger: logs.Logger()})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mw.Logger(r.Context()).Info("shown share")
	}))
	webuitest.Serve(t, h, webuitest.Get(t, "/share/"+secret+"?t="+secret))
	t.Must(be.Substring(logs.String(), "shown share"))
	t.Must(be.Substring(logs.String(), "http request"))
	t.Must(logs.NotLogged(secret))

	// LogTo attaches a logger without the middleware.
	direct := webuitest.NewLogs()
	webuitest.Serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mw.Logger(r.Context()).Warn("leak", "path", r.URL.Path)
	}), webuitest.Get(t, "/share/"+secret, webuitest.LogTo(direct.Logger())))
	t.Must(be.ErrorIs(direct.NotLogged(secret).Check(t.Context()), webuitest.ErrLeak))
}

func TestLogsPercentEncoded(gt *testing.T) {
	t := devtest.ParallelT(gt)
	const secret = "Quarterly plan M\u00fcller.pdf"
	logs := webuitest.NewLogs()
	h := mw.LogRequests(mw.RequestLog{Logger: logs.Logger(), Path: true})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	webuitest.Serve(t, h, webuitest.Get(t, "/files/"+url.PathEscape(secret)))
	t.Must(be.Substring(logs.String(), "/files/Quarterly%20plan%20M%C3%BCller.pdf"))
	err := logs.NotLogged(secret).Check(t.Context())
	t.Must(be.ErrorIs(err, webuitest.ErrLeak))
	t.Must(be.ErrorContains(err, "secret 1 appears 1 times"))
	t.Must(be.False(strings.Contains(err.Error(), "Quarterly")))

	body := webuitest.NewLogs()
	body.Logger().Info("form", "raw", url.Values{"note": {"my secret note"}}.Encode())
	t.Must(be.ErrorIs(body.NotLogged("my secret note").Check(t.Context()), webuitest.ErrLeak))
}

func TestLogsConcurrent(gt *testing.T) {
	t := devtest.ParallelT(gt)
	logs := webuitest.NewLogs()
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			l := logs.Logger().With("worker", i)
			for range 50 {
				l.Info("tick")
			}
		})
	}
	wg.Wait()
	t.Must(be.Equal(400, strings.Count(logs.String(), "msg=tick")))
	t.Must(be.True(logs.Handler() != nil))
}
