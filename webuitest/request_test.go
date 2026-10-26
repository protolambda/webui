package webuitest_test

import (
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/mw"
	"github.com/protolambda/webui/webuitest"
)

func TestGet(gt *testing.T) {
	t := devtest.ParallelT(gt)
	req := webuitest.Get(t, "/tasks?page=2#notes")
	t.Must(be.Equal(http.MethodGet, req.Method))
	t.Must(be.Equal("/tasks", req.URL.Path))
	t.Must(be.Equal("page=2", req.URL.RawQuery))
	t.Must(be.Equal("", req.URL.Host))
	t.Must(be.Equal("/tasks?page=2", req.RequestURI))
	t.Must(be.Equal("app.example.test", req.Host))
	t.Must(be.Equal(webuitest.DefaultRemoteAddr, req.RemoteAddr))
	t.Must(be.True(req.TLS != nil && req.TLS.ServerName == "app.example.test"))
	t.Must(be.True(req.Context() == t.Context()))
	h := req.Header
	t.Must(be.Equal("same-origin", h.Get("Sec-Fetch-Site")))
	t.Must(be.Equal("navigate", h.Get("Sec-Fetch-Mode")))
	t.Must(be.Equal("document", h.Get("Sec-Fetch-Dest")))
	t.Must(be.Equal("?1", h.Get("Sec-Fetch-User")))
	t.Must(be.Equal("", h.Get("Origin")))
	t.Must(be.Equal("", h.Get("HX-Request")))
	t.Must(be.Equal(int64(0), req.ContentLength))
}

func TestPostForm(gt *testing.T) {
	t := devtest.ParallelT(gt)
	req := webuitest.Post(t, "/tasks", webuitest.Form(url.Values{"title": {"Write docs"}, "tag": {"a", "b&c"}}))
	t.Must(be.Equal(http.MethodPost, req.Method))
	t.Must(be.Equal(webuitest.BaseURL, req.Header.Get("Origin")))
	t.Must(be.Equal("same-origin", req.Header.Get("Sec-Fetch-Site")))
	t.Must(be.Equal("application/x-www-form-urlencoded", req.Header.Get("Content-Type")))
	body, err := io.ReadAll(req.Body)
	t.Must(be.NoError(err))
	t.Must(be.Equal("tag=a&tag=b%26c&title=Write+docs", string(body)))
	t.Must(be.Equal(int64(len(body)), req.ContentLength))
}

func TestRequestOptions(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Run("htmx", func(t devtest.T) {
		req := webuitest.Post(t, "/tasks", webuitest.Body{}, webuitest.HXTarget("list"), webuitest.HXCurrentURL("/tasks?x=1"))
		h := req.Header
		t.Must(be.Equal("true", h.Get("HX-Request")))
		t.Must(be.Equal("list", h.Get("HX-Target")))
		t.Must(be.Equal(webuitest.BaseURL+"/tasks?x=1", h.Get("HX-Current-URL")))
		t.Must(be.Equal("cors", h.Get("Sec-Fetch-Mode")))
		t.Must(be.Equal("empty", h.Get("Sec-Fetch-Dest")))
		t.Must(be.Equal("", h.Get("Sec-Fetch-User")))
		t.Must(be.Equal(webuitest.BaseURL, h.Get("Origin")))
		t.Must(be.Equal("", h.Get("Content-Type")))
		t.Must(be.True(req.Body == http.NoBody))
	})
	t.Run("htmx current URL from the page URL", func(t devtest.T) {
		h := webuitest.Get(t, "list", webuitest.HTMX(), webuitest.PageURL("/tasks/?page=2")).Header
		t.Must(be.Equal(webuitest.BaseURL+"/tasks/?page=2", h.Get("HX-Current-URL")))
		h = webuitest.Get(t, "list", webuitest.HTMX(), webuitest.PageURL("/tasks/"), webuitest.HXCurrentURL("/other")).Header
		t.Must(be.Equal(webuitest.BaseURL+"/other", h.Get("HX-Current-URL")))
		// A navigation from the page sends none.
		h = webuitest.Get(t, "list", webuitest.PageURL("/tasks/")).Header
		t.Must(be.Equal("", h.Get("HX-Current-URL")))
	})
	t.Run("history restore", func(t devtest.T) {
		h := webuitest.Get(t, "/", webuitest.HXHistoryRestore()).Header
		t.Must(be.Equal("true", h.Get("HX-History-Restore-Request")))
		t.Must(be.Equal("", h.Get("HX-Request")))
		t.Must(be.Equal("cors", h.Get("Sec-Fetch-Mode")))
	})
	t.Run("event source", func(t devtest.T) {
		h := webuitest.Get(t, "/live", webuitest.EventSource()).Header
		t.Must(be.Equal("text/event-stream", h.Get("Accept")))
		t.Must(be.Equal("no-cache", h.Get("Cache-Control")))
		t.Must(be.Equal("empty", h.Get("Sec-Fetch-Dest")))
	})
	t.Run("fetch metadata", func(t devtest.T) {
		h := webuitest.Post(t, "/x", webuitest.Body{}, webuitest.FetchSite("cross-site"), webuitest.Origin("https://evil.example"), webuitest.Referer("https://evil.example/p")).Header
		t.Must(be.Equal("cross-site", h.Get("Sec-Fetch-Site")))
		t.Must(be.Equal("https://evil.example", h.Get("Origin")))
		t.Must(be.Equal("https://evil.example/p", h.Get("Referer")))
		h = webuitest.Post(t, "/x", webuitest.Body{}, webuitest.Origin(""), webuitest.FetchSite("")).Header
		t.Must(be.Equal("", h.Get("Origin")))
		t.Must(be.Equal("", h.Get("Sec-Fetch-Site")))
		t.Must(be.Equal("navigate", h.Get("Sec-Fetch-Mode")))
		h = webuitest.Post(t, "/x", webuitest.Body{}, webuitest.Referer("/a"), webuitest.NoFetchMetadata(), webuitest.Header("Origin", "null")).Header
		for _, name := range []string{"Referer", "Sec-Fetch-Site", "Sec-Fetch-Mode", "Sec-Fetch-Dest", "Sec-Fetch-User"} {
			t.Must(be.Equal("", h.Get(name)))
		}
		t.Must(be.Equal("null", h.Get("Origin")))
		h = webuitest.Get(t, "/x", webuitest.Header("Accept-Language", "nl"), webuitest.Header("Sec-Fetch-Dest", "")).Header
		t.Must(be.Equal("nl", h.Get("Accept-Language")))
		t.Must(be.Equal("", h.Get("Sec-Fetch-Dest")))
	})
	t.Run("connection", func(t devtest.T) {
		logger := slog.New(slog.DiscardHandler)
		req := webuitest.Get(t, "/x", webuitest.Host("evil.example"), webuitest.RemoteAddr("198.51.100.7:4000"),
			webuitest.SendCookie("a", "1"), webuitest.SendCookie("b", "2"), webuitest.LogTo(logger))
		t.Must(be.Equal("evil.example", req.Host))
		t.Must(be.Equal("198.51.100.7:4000", req.RemoteAddr))
		t.Must(be.Equal("a=1; b=2", req.Header.Get("Cookie")))
		t.Must(be.True(mw.Logger(req.Context()) == logger))
		t.Must(be.True(mw.Logger(webuitest.Get(t, "/x").Context()) == slog.Default()))
	})
	t.Run("absolute target", func(t devtest.T) {
		req := webuitest.Get(t, "http://other.example:8080/a")
		t.Must(be.Equal("other.example:8080", req.Host))
		t.Must(be.True(req.TLS == nil))
	})
	t.Run("invalid", func(t devtest.T) {
		r := newRecordT(t.Context())
		t.Must(be.True(webuitest.Get(r, "/%zz") == nil))
		t.Must(be.True(r.failed))
		t.Must(be.Substring(r.output(), "invalid request target"))
		r = newRecordT(t.Context())
		t.Must(be.True(webuitest.Request(r, "BAD METHOD", "/", webuitest.Body{}) == nil))
		t.Must(be.True(r.failed))
	})
}

func TestSendCookies(gt *testing.T) {
	t := devtest.ParallelT(gt)
	res := webuitest.Serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "a", Value: "1", Path: "/", Secure: true, HttpOnly: true})
		http.SetCookie(w, &http.Cookie{Name: "b", Value: "2", Domain: "other.example.test"})
		http.SetCookie(w, &http.Cookie{Name: "a", Value: "3"})
		http.SetCookie(w, &http.Cookie{Name: "gone", Value: "x", MaxAge: -1})
		http.SetCookie(w, &http.Cookie{Name: "expired", Value: "x", Expires: time.Unix(1, 0)})
		http.SetCookie(w, &http.Cookie{Name: "kept", Value: "x", MaxAge: 60, Expires: time.Unix(1, 0)})
		http.SetCookie(w, &http.Cookie{Name: "b", Value: "4", MaxAge: -1})
		http.SetCookie(w, &http.Cookie{Name: "c", Value: "5"})
	}), webuitest.Get(t, "/"))
	req := webuitest.Get(t, "/", webuitest.SendCookie("x", "y"), webuitest.SendCookies(res))
	t.Must(be.Equal("x=y; a=3; kept=x; c=5", req.Header.Get("Cookie")))

	t.Run("failures", func(t devtest.T) {
		const secret = "cookie-value-never-printed"
		broken := webuitest.Serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Set-Cookie", `s="`+secret+`"x`)
		}), webuitest.Get(t, "/x"))
		r := newRecordT(t.Context())
		t.Must(be.True(webuitest.Get(r, "/", webuitest.SendCookies(broken)) == nil))
		t.Must(be.True(r.failed))
		t.Must(be.Substring(r.output(), "webuitest: cookies of GET /x: 1 Set-Cookie headers do not parse"))
		t.Must(be.False(strings.Contains(r.output(), secret)))
		r = newRecordT(t.Context())
		t.Must(be.True(webuitest.Get(r, "/", webuitest.SendCookies(nil)) == nil))
		t.Must(be.Substring(r.output(), "no response to take cookies from"))
	})
}

// readParts parses a multipart body.
func readParts(t devtest.T, req *http.Request) []*multipart.Part {
	mt, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	t.Must(be.NoError(err))
	t.Must(be.Equal("multipart/form-data", mt))
	mr := multipart.NewReader(req.Body, params["boundary"])
	var out []*multipart.Part
	for {
		p, err := mr.NextRawPart()
		if err == io.EOF {
			return out
		}
		t.Must(be.NoError(err))
		data, err := io.ReadAll(p)
		t.Must(be.NoError(err))
		p.Header.Set("X-Content", string(data))
		out = append(out, p)
	}
}

func TestMultipart(gt *testing.T) {
	t := devtest.ParallelT(gt)
	t.Run("fields", func(t devtest.T) {
		req := webuitest.Post(t, "/up", webuitest.Multipart(map[string][]string{"b": {"2"}, "a": {"1", "x\r\ny"}}))
		parts := readParts(t, req)
		t.Must(be.Len(parts, 3))
		t.Must(be.Equal(`form-data; name="a"`, parts[0].Header.Get("Content-Disposition")))
		t.Must(be.Equal("x\r\ny", parts[1].Header.Get("X-Content")))
		t.Must(be.Equal("", parts[0].Header.Get("Content-Type")))
		t.Must(be.Equal("b", parts[2].FormName()))
	})
	t.Run("files", func(t devtest.T) {
		body := webuitest.Parts(
			webuitest.Part{Name: `n"a` + "\n" + `me`, Value: "v"},
			webuitest.Part{Name: "file", File: true, Filename: "my \"report\"\r\n.txt", Value: "content"},
			webuitest.Part{Name: "img", File: true, Filename: "a.png", ContentType: "image/png", Value: "PNG"},
			webuitest.Part{Name: "none", File: true},
		)
		parts := readParts(t, webuitest.Post(t, "/up", body))
		t.Must(be.Len(parts, 4))
		t.Must(be.Equal(`form-data; name="n%22a%0Ame"`, parts[0].Header.Get("Content-Disposition")))
		t.Must(be.Equal(`form-data; name="file"; filename="my %22report%22%0D%0A.txt"`, parts[1].Header.Get("Content-Disposition")))
		t.Must(be.Equal("application/octet-stream", parts[1].Header.Get("Content-Type")))
		t.Must(be.Equal("content", parts[1].Header.Get("X-Content")))
		t.Must(be.Equal("image/png", parts[2].Header.Get("Content-Type")))
		t.Must(be.Equal(`form-data; name="none"; filename=""`, parts[3].Header.Get("Content-Disposition")))
		// The standard library reads it as a file upload.
		req := webuitest.Post(t, "/up", body)
		t.Must(be.NoError(req.ParseMultipartForm(1 << 20)))
		f, fh, err := req.FormFile("img")
		t.Must(be.NoError(err))
		t.Must(be.NoError(f.Close()))
		t.Must(be.Equal("a.png", fh.Filename))
	})
	t.Run("boundary clash", func(t devtest.T) {
		ct := webuitest.Parts(webuitest.Part{Name: "a", Value: "x"}).ContentType()
		_, params, err := mime.ParseMediaType(ct)
		t.Must(be.NoError(err))
		clash := "--" + params["boundary"] + "\r\n"
		parts := readParts(t, webuitest.Post(t, "/up", webuitest.Parts(webuitest.Part{Name: "a", Value: clash})))
		t.Must(be.Len(parts, 1))
		t.Must(be.Equal(clash, parts[0].Header.Get("X-Content")))
	})
	t.Run("deterministic", func(t devtest.T) {
		b := webuitest.Multipart(map[string][]string{"a": {"1"}})
		t.Must(be.Equal(string(b.Bytes()), string(webuitest.Multipart(map[string][]string{"a": {"1"}}).Bytes())))
	})
}

func TestBody(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var zero webuitest.Body
	t.Must(be.Equal("", zero.ContentType()))
	t.Must(be.Len(zero.Bytes(), 0))
	with := zero.With("csrf", "t")
	t.Must(be.Equal("application/x-www-form-urlencoded", with.ContentType()))
	t.Must(be.Equal("csrf=t", string(with.Bytes())))
	t.Must(be.False(zero.Has("csrf")))
	t.Must(be.True(with.Has("csrf")))

	form := webuitest.Form(url.Values{"a": {"1"}})
	more := form.With("b", "2")
	t.Must(be.Equal("a=1", string(form.Bytes())))
	t.Must(be.Equal("a=1&b=2", string(more.Bytes())))
	t.Must(be.Len(more.Fields(), 2))

	data := []byte(`{"a":1}`)
	raw := webuitest.Raw("application/json", data)
	data[0] = 'x'
	t.Must(be.Equal(`{"a":1}`, string(raw.Bytes())))
	t.Must(be.Equal("application/json", raw.ContentType()))
	t.Must(be.Equal(`{"a":1}`, string(raw.With("b", "2").Bytes())))
	t.Must(be.True(strings.HasPrefix(webuitest.Parts().ContentType(), "multipart/form-data; boundary=")))
}

func TestServe(gt *testing.T) {
	t := devtest.ParallelT(gt)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.PathValue("id")+" "+r.FormValue("title")+" "+r.Header.Get("Origin"))
	})
	res := webuitest.Serve(t, mux, webuitest.Post(t, "/tasks/7", webuitest.Form(url.Values{"title": {"x"}})))
	t.Must(res.Status(http.StatusOK))
	t.Must(be.Equal("7 x "+webuitest.BaseURL, res.BodyString()))

	r := newRecordT(t.Context())
	t.Must(be.True(webuitest.Serve(r, nil, webuitest.Get(t, "/")) == nil))
	t.Must(be.True(r.failed))
}
