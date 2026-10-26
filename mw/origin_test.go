package mw_test

import (
	"context"
	"encoding/json"
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
	shop := mw.MustParseSite("https://shop.example")
	info := mw.MustParseSite("https://info.example.com")
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

		{"origin long s", shop, http.MethodPost, headers{"Origin": "https://\u017Fhop.example"}, 0, false},
		{"origin ASCII case", shop, http.MethodPost, headers{"Origin": "https://SHOP.example"}, 0, true},
		{"referer dotted I", info, http.MethodPost, headers{"Referer": "https://\u0130nfo.example.com/"}, 0, false},
		{"referer ASCII case", info, http.MethodPost, headers{"Referer": "https://INFO.example.com/"}, 0, true},

		{"websocket same-origin", public, http.MethodGet,
			headers{"Upgrade": "websocket", "Sec-Fetch-Site": "same-origin", "Origin": "https://app.example.com"}, 0, true},
		{"websocket origin only", public, http.MethodGet, headers{"Upgrade": "websocket", "Origin": "https://app.example.com"}, 0, true},
		{"websocket same-site", public, http.MethodGet,
			headers{"Upgrade": "websocket", "Sec-Fetch-Site": "same-site", "Origin": "https://other.example.com"}, 0, false},
		{"websocket cross-site", public, http.MethodGet,
			headers{"Upgrade": "websocket", "Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"}, 0, false},
		{"websocket other origin", public, http.MethodGet, headers{"Upgrade": "websocket", "Origin": "https://evil.example"}, 0, false},
		{"websocket other port on loopback", local, http.MethodGet,
			headers{"Upgrade": "websocket", "Origin": "http://127.0.0.1:9090"}, 0, false},
		{"websocket headerless public", public, http.MethodGet, headers{"Upgrade": "websocket"}, 0, false},
		{"websocket headerless loopback", local, http.MethodGet, headers{"Upgrade": "websocket"}, 0, true},
		{"websocket headerless allowed", public, http.MethodGet, headers{"Upgrade": "websocket"}, mw.HeaderlessAllow, true},
		{"websocket zero site", mw.Site{}, http.MethodGet, headers{"Upgrade": "websocket", "Sec-Fetch-Site": "same-origin"}, 0, false},
		{"other upgrade", public, http.MethodGet, headers{"Upgrade": "chat", "Origin": "https://evil.example"}, 0, false},
		{"HEAD upgrade", public, http.MethodHead, headers{"Upgrade": "websocket", "Origin": "https://evil.example"}, 0, false},
		{"navigation upgrading to https", public, http.MethodGet,
			headers{"Upgrade-Insecure-Requests": "1", "Sec-Fetch-Site": "cross-site"}, 0, true},
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
	t.Must(be.Equal("nosniff", rec.Header().Get("X-Content-Type-Options")))
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

// Behind LogRequests, the request-scoped logger carries the method, so a
// refusal does not repeat it; a logger attached inside LogRequests gets
// the method from the refusal.
func TestSameOriginLogsMethodOnce(gt *testing.T) {
	t := devtest.ParallelT(gt)
	site := mw.MustParseSite("https://app.example.com")
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tasks", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusSeeOther)
	})
	var out strings.Builder
	logger := slog.New(slog.NewJSONHandler(&out, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey || a.Key == "duration" {
				return slog.Attr{} // for stable records
			}
			return a
		},
	}))
	refuse := func(mws ...mw.Middleware) string {
		out.Reset()
		h := mw.Chain(mux, append([]mw.Middleware{mw.LogRequests(mw.RequestLog{Logger: logger, Routes: mux})}, mws...)...)
		req := httptest.NewRequest(http.MethodPost, "https://app.example.com/tasks", strings.NewReader("title=x"))
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		t.Must(be.Equal(http.StatusForbidden, rec.Code))
		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		t.Must(be.Len(lines, 2)) // the refusal, then the request
		return lines[0]
	}

	line := refuse(mw.HostCheck(site), mw.SameOrigin(mw.OriginPolicy{}))
	t.Must(be.DeepEqual([]string{"level", "msg", "method", "pattern", "reason"}, jsonKeys(t, line)))
	t.Must(be.Substring(line, `"msg":"refused a cross-origin request","method":"POST","pattern":"POST /tasks","reason":`))

	inner := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(mw.WithLogger(r.Context(), logger.With("app", "tasks"))))
		})
	}
	line = refuse(mw.HostCheck(site), inner, mw.SameOrigin(mw.OriginPolicy{}))
	t.Must(be.DeepEqual([]string{"level", "msg", "app", "method", "reason"}, jsonKeys(t, line)))
	t.Must(be.Substring(line, `"app":"tasks","method":"POST","reason":`))
}

// jsonKeys returns the keys of the JSON object in line in their order,
// repeated keys included, which a decoded map would merge.
func jsonKeys(t devtest.T, line string) []string {
	dec := json.NewDecoder(strings.NewReader(line))
	tok, err := dec.Token()
	t.Must(be.NoError(err))
	t.Must(be.Equal[json.Token](json.Delim('{'), tok))
	var keys []string
	for dec.More() {
		tok, err := dec.Token()
		t.Must(be.NoError(err))
		key, _ := tok.(string)
		keys = append(keys, key)
		var value json.RawMessage
		t.Must(be.NoError(dec.Decode(&value)))
	}
	return keys
}

// WebSocket handshakes are GET requests whose responses pages of other
// origins can read and write, so SameOrigin checks them.
func TestSameOriginWebSocketHandshake(gt *testing.T) {
	t := devtest.ParallelT(gt)
	site := mw.MustParseSite("https://app.example.com")
	opened := 0
	h := mw.Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { opened++ }),
		mw.HostCheck(site), mw.SameOrigin(mw.OriginPolicy{}))
	handshake := func(fetchSite, origin string) int {
		req := httptest.NewRequest(http.MethodGet, "https://app.example.com/live", nil)
		req.Header.Set("Connection", "Upgrade")
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Sec-WebSocket-Version", "13")
		req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
		req.Header.Set("Sec-Fetch-Mode", "websocket")
		req.Header.Set("Sec-Fetch-Site", fetchSite)
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	t.Must(be.Equal(http.StatusForbidden, handshake("same-site", "https://other.example.com")))
	t.Must(be.Equal(http.StatusForbidden, handshake("cross-site", "https://evil.example")))
	t.Must(be.Equal(0, opened))
	t.Must(be.Equal(http.StatusOK, handshake("same-origin", "https://app.example.com")))
	t.Must(be.Equal(1, opened))
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
