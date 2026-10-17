package mw_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/mw"
)

func TestOriginPolicyCheck(gt *testing.T) {
	t := devtest.ParallelT(gt)
	public := mw.MustParseSite("https://app.example.com")
	local := mw.MustParseSite("http://127.0.0.1:8080")
	type headers map[string]string
	cases := []struct {
		name    string
		site    mw.Site
		method  string
		headers headers
		policy  mw.Headerless
		pass    bool
	}{
		{"GET passes cross-site", public, http.MethodGet, headers{"Sec-Fetch-Site": "cross-site"}, 0, true},
		{"HEAD passes cross-site", public, http.MethodHead, headers{"Origin": "https://evil.example"}, 0, true},
		{"GET passes without site", mw.Site{}, http.MethodGet, nil, 0, true},

		{"fetch same-origin", public, http.MethodPost, headers{"Sec-Fetch-Site": "same-origin"}, 0, true},
		{"fetch none", public, http.MethodPost, headers{"Sec-Fetch-Site": "none"}, 0, true},
		{"fetch same-site", public, http.MethodPost, headers{"Sec-Fetch-Site": "same-site"}, 0, false},
		{"fetch cross-site", public, http.MethodPost, headers{"Sec-Fetch-Site": "cross-site"}, 0, false},
		{"fetch upper case", public, http.MethodPost, headers{"Sec-Fetch-Site": "SAME-ORIGIN"}, 0, false},
		{"fetch empty", public, http.MethodPost, headers{"Sec-Fetch-Site": ""}, 0, false},
		{"fetch wins over matching origin", public, http.MethodPost,
			headers{"Sec-Fetch-Site": "cross-site", "Origin": "https://app.example.com"}, 0, false},
		{"fetch wins over other origin", public, http.MethodPost,
			headers{"Sec-Fetch-Site": "same-origin", "Origin": "https://evil.example"}, 0, true},
		{"fetch cross-site on loopback", local, http.MethodPost, headers{"Sec-Fetch-Site": "cross-site"}, 0, false},

		{"origin equal", public, http.MethodPost, headers{"Origin": "https://app.example.com"}, 0, true},
		{"origin case", public, http.MethodPost, headers{"Origin": "HTTPS://APP.example.com"}, 0, true},
		{"origin http", public, http.MethodPost, headers{"Origin": "http://app.example.com"}, 0, false},
		{"origin other host", public, http.MethodPost, headers{"Origin": "https://evil.example"}, 0, false},
		{"origin null", public, http.MethodPost, headers{"Origin": "null"}, 0, false},
		{"origin explicit port", public, http.MethodPost, headers{"Origin": "https://app.example.com:443"}, 0, false},
		{"origin other port", public, http.MethodPost, headers{"Origin": "https://app.example.com:8443"}, 0, false},
		{"origin suffix", public, http.MethodPost, headers{"Origin": "https://app.example.com.evil.example"}, 0, false},
		{"origin prefix", public, http.MethodPost, headers{"Origin": "https://app.example.co"}, 0, false},
		{"origin with path", public, http.MethodPost, headers{"Origin": "https://app.example.com/"}, 0, false},
		{"origin wins over referer", public, http.MethodPost,
			headers{"Origin": "https://evil.example", "Referer": "https://app.example.com/"}, 0, false},
		{"origin on loopback", local, http.MethodPost, headers{"Origin": "http://127.0.0.1:8080"}, 0, true},
		{"origin other port on loopback", local, http.MethodPost, headers{"Origin": "http://127.0.0.1:9090"}, 0, false},
		{"origin localhost on IP loopback", local, http.MethodPost, headers{"Origin": "http://localhost:8080"}, 0, false},

		{"referer equal", public, http.MethodPost, headers{"Referer": "https://app.example.com/items?page=2"}, 0, true},
		{"referer explicit port", public, http.MethodPost, headers{"Referer": "https://app.example.com:443/x"}, 0, true},
		{"referer case", public, http.MethodPost, headers{"Referer": "HTTPS://App.Example.com/x"}, 0, true},
		{"referer http", public, http.MethodPost, headers{"Referer": "http://app.example.com/x"}, 0, false},
		{"referer suffix", public, http.MethodPost, headers{"Referer": "https://app.example.com.evil.example/x"}, 0, false},
		{"referer path trick", public, http.MethodPost,
			headers{"Referer": "https://evil.example/https://app.example.com/"}, 0, false},
		{"referer user trick", public, http.MethodPost, headers{"Referer": "https://app.example.com@evil.example/"}, 0, false},
		{"referer relative", public, http.MethodPost, headers{"Referer": "/items"}, 0, false},
		{"referer garbage", public, http.MethodPost, headers{"Referer": "::not a url::"}, 0, false},
		{"referer other scheme", public, http.MethodPost, headers{"Referer": "ftp://app.example.com/"}, 0, false},
		{"referer too long", public, http.MethodPost,
			headers{"Referer": "https://app.example.com/" + strings.Repeat("x", 9000)}, 0, false},
		{"referer on loopback", local, http.MethodPost, headers{"Referer": "http://127.0.0.1:8080/"}, 0, true},
		{"referer other on loopback", local, http.MethodPost, headers{"Referer": "http://evil.example/"}, 0, false},

		{"headerless public", public, http.MethodPost, nil, mw.HeaderlessLoopback, false},
		{"headerless loopback", local, http.MethodPost, nil, mw.HeaderlessLoopback, true},
		{"headerless public allowed", public, http.MethodPost, nil, mw.HeaderlessAllow, true},
		{"headerless loopback allowed", local, http.MethodPost, nil, mw.HeaderlessAllow, true},
		{"headerless public denied", public, http.MethodPost, nil, mw.HeaderlessDeny, false},
		{"headerless loopback denied", local, http.MethodPost, nil, mw.HeaderlessDeny, false},
		{"headerless unknown policy", local, http.MethodPost, nil, mw.Headerless(9), false},
		{"allow does not pass cross-site", public, http.MethodPost, headers{"Sec-Fetch-Site": "cross-site"}, mw.HeaderlessAllow, false},

		{"PUT", public, http.MethodPut, headers{"Origin": "https://evil.example"}, 0, false},
		{"PATCH", public, http.MethodPatch, headers{"Origin": "https://evil.example"}, 0, false},
		{"DELETE", public, http.MethodDelete, headers{"Origin": "https://evil.example"}, 0, false},
		{"OPTIONS", public, http.MethodOptions, headers{"Sec-Fetch-Site": "cross-site"}, 0, false},
		{"lower-case get", public, "get", headers{"Sec-Fetch-Site": "cross-site"}, 0, false},
		{"custom method", public, "PROPFIND", headers{"Sec-Fetch-Site": "same-origin"}, 0, true},

		{"zero site", mw.Site{}, http.MethodPost, headers{"Sec-Fetch-Site": "same-origin"}, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			req := httptest.NewRequest(c.method, "https://app.example.com/forms/save", nil)
			for k, v := range c.headers {
				req.Header.Set(k, v)
			}
			err := mw.OriginPolicy{Headerless: c.policy}.Check(req, c.site)
			if c.pass {
				t.Must(be.NoError(err))
				return
			}
			t.Must(be.ErrorIs(err, mw.ErrCrossOrigin))
			for _, v := range c.headers {
				if v != "" {
					t.Mustf(be.False(strings.Contains(err.Error(), v)), "errors do not quote headers")
				}
			}
		})
	}
}

func TestOriginPolicyRepeatedHeaders(gt *testing.T) {
	t := devtest.ParallelT(gt)
	site := mw.MustParseSite("https://app.example.com")
	for _, name := range []string{"Sec-Fetch-Site", "Origin", "Referer"} {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		value := map[string]string{
			"Sec-Fetch-Site": "same-origin", "Origin": "https://app.example.com", "Referer": "https://app.example.com/",
		}[name]
		req.Header.Add(name, value)
		t.Must(be.NoError(mw.OriginPolicy{}.Check(req, site)))
		req.Header.Add(name, value)
		t.Must(be.ErrorIs(mw.OriginPolicy{}.Check(req, site), mw.ErrCrossOrigin))
	}
}

func TestSameOrigin(gt *testing.T) {
	t := devtest.ParallelT(gt)
	site := mw.MustParseSite("https://app.example.com")
	saved := 0
	app := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		saved++
		w.WriteHeader(http.StatusSeeOther)
	})
	logger, sink := newCapture(slog.LevelDebug)
	h := mw.Chain(app, mw.HostCheck(site), mw.SameOrigin(mw.OriginPolicy{}))

	post := func(headers map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "https://app.example.com/forms/save", strings.NewReader("x=1"))
		req = req.WithContext(mw.WithLogger(context.Background(), logger))
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	rec := post(map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"})
	t.Must(be.Equal(http.StatusForbidden, rec.Code))
	t.Must(be.Equal("cross-origin request refused\n", rec.Body.String()))
	t.Must(be.Equal("text/plain; charset=utf-8", rec.Header().Get("Content-Type")))
	t.Must(be.Equal(0, saved))
	records := sink.Records()
	t.Must(be.Len(records, 1))
	t.Must(be.Equal("refused a cross-origin request", records[0].Message))
	t.Must(be.Equal(slog.LevelDebug, records[0].Level))
	got := attrs(records[0])
	t.Must(be.Equal("POST", got["method"]))
	t.Must(be.Substring(got["reason"], "Sec-Fetch-Site"))
	t.Must(be.False(strings.Contains(got["reason"], "evil")))

	rec = post(nil)
	t.Must(be.Equal(http.StatusForbidden, rec.Code))
	t.Must(be.Equal(0, saved))

	rec = post(map[string]string{"Sec-Fetch-Site": "same-origin"})
	t.Must(be.Equal(http.StatusSeeOther, rec.Code))
	t.Must(be.Equal(1, saved))
}

func TestSameOriginCustomDeny(gt *testing.T) {
	t := devtest.ParallelT(gt)
	site := mw.MustParseSite("https://app.example.com")
	deny := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := mw.Chain(http.NotFoundHandler(), mw.HostCheck(site), mw.SameOrigin(mw.OriginPolicy{Deny: deny}))
	req := httptest.NewRequest(http.MethodDelete, "https://app.example.com/items/1", nil)
	req.Header.Set("Origin", "null")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	t.Must(be.Equal(http.StatusTeapot, rec.Code))
}

func TestSameOriginWithoutHostCheck(gt *testing.T) {
	t := devtest.ParallelT(gt)
	h := mw.SameOrigin(mw.OriginPolicy{Headerless: mw.HeaderlessAllow})(http.NotFoundHandler())
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	t.Must(be.Equal(http.StatusForbidden, rec.Code))

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	t.Must(be.Equal(http.StatusNotFound, rec.Code))
}

func TestHeaderlessString(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Must(be.Equal("HeaderlessLoopback", mw.HeaderlessLoopback.String()))
	t.Must(be.Equal("HeaderlessDeny", mw.HeaderlessDeny.String()))
	t.Must(be.Equal("HeaderlessAllow", mw.HeaderlessAllow.String()))
	t.Must(be.Equal("Headerless(7)", mw.Headerless(7).String()))
}
