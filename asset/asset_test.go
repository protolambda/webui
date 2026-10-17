package asset_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthttp"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/asset"
)

var (
	appCSS  = []byte(strings.Repeat("body { color: red; }\n", 100))
	appJS   = []byte("console.log('hello');\n")
	fontBin = []byte{0x77, 0x4f, 0x46, 0x32, 0x00, 0x01, 0x02, 0x03}
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"app.css":         {Data: appCSS},
		"app.js":          {Data: appJS},
		"fonts/app.woff2": {Data: fontBin},
		"notes.txt":       {Data: []byte("plain text")},
	}
}

func sri(data []byte) string {
	sum := sha512.Sum384(data)
	return "sha384-" + base64.StdEncoding.EncodeToString(sum[:])
}

func testSet(t devtest.T) *asset.Set {
	s, err := asset.New(asset.Options{
		Prefix: "/assets/app/",
		FS:     testFS(),
		Files: []asset.File{
			{Name: "app.css"},
			{Name: "fonts/app.woff2"},
			{Name: "app.js", Integrity: sri(appJS)},
			{Name: "notes.txt"},
		},
	})
	t.Must(be.NoError(err))
	return s
}

func render(t devtest.T, node elem.Node) string {
	var out strings.Builder
	t.Must(be.NoError(core.Render(t.Context(), node, &out)))
	return out.String()
}

func get(t devtest.T, h http.Handler, method, target string, header http.Header) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(t.Context(), method, target, nil)
	for k, vs := range header {
		req.Header[k] = vs
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestInfo(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := testSet(t)
	t.Must(be.Equal("/assets/app/", s.Prefix()))
	t.Must(be.Equal("GET /assets/app/", s.Pattern()))
	t.Must(be.Equal(16, len(s.Version())))

	var names []string
	for info := range s.Files() {
		names = append(names, info.Name)
		t.Must(be.Equal("/assets/app/"+s.Version()+"/"+info.Name, info.URL))
	}
	t.Must(be.DeepEqual([]string{"app.css", "fonts/app.woff2", "app.js", "notes.txt"}, names))

	css, ok := s.Lookup("app.css")
	t.Must(be.True(ok))
	t.Must(be.Equal(asset.KindStylesheet, css.Kind))
	t.Must(be.Equal("text/css; charset=utf-8", css.ContentType))
	t.Must(be.Equal(sri(appCSS), css.Integrity))
	t.Must(be.Equal(len(appCSS), css.Size))

	font, _ := s.Lookup("fonts/app.woff2")
	t.Must(be.Equal(asset.KindFont, font.Kind))
	t.Must(be.Equal("font/woff2", font.ContentType))
	notes, _ := s.Lookup("notes.txt")
	t.Must(be.Equal(asset.KindOther, notes.Kind))

	_, ok = s.Lookup("missing.css")
	t.Must(be.False(ok))
	t.Must(be.Panic(func() { s.URL("missing.css") }))
	t.Must(be.Panic(func() { s.Integrity("missing.css") }))
	t.Must(be.Equal(sri(appJS), s.Integrity("app.js")))
}

func TestVersionFollowsContent(gt *testing.T) {
	t := devtest.ParallelT(gt)
	build := func(fsys fstest.MapFS) string {
		s, err := asset.New(asset.Options{Prefix: "/a/", FS: fsys, Files: []asset.File{{Name: "app.css"}, {Name: "app.js"}}})
		t.Must(be.NoError(err))
		return s.Version()
	}
	base := build(testFS())
	t.Must(be.Equal(base, build(testFS())))

	changed := testFS()
	changed["app.js"] = &fstest.MapFile{Data: []byte("console.log('changed');\n")}
	t.Must(be.NotEqual(base, build(changed)))
}

func TestNewErrors(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		name string
		opts asset.Options
		err  error
	}{
		{"no prefix", asset.Options{FS: testFS(), Files: []asset.File{{Name: "app.css"}}}, asset.ErrInvalid},
		{"relative prefix", asset.Options{Prefix: "assets/", FS: testFS(), Files: []asset.File{{Name: "app.css"}}}, asset.ErrInvalid},
		{"prefix without slash", asset.Options{Prefix: "/assets", FS: testFS(), Files: []asset.File{{Name: "app.css"}}}, asset.ErrInvalid},
		{"prefix with dot segment", asset.Options{Prefix: "/a/../b/", FS: testFS(), Files: []asset.File{{Name: "app.css"}}}, asset.ErrInvalid},
		{"prefix with wildcard", asset.Options{Prefix: "/{x}/", FS: testFS(), Files: []asset.File{{Name: "app.css"}}}, asset.ErrInvalid},
		{"no fs", asset.Options{Prefix: "/a/", Files: []asset.File{{Name: "app.css"}}}, asset.ErrInvalid},
		{"no files", asset.Options{Prefix: "/a/", FS: testFS()}, asset.ErrInvalid},
		{"duplicate", asset.Options{Prefix: "/a/", FS: testFS(), Files: []asset.File{{Name: "app.css"}, {Name: "app.css"}}}, asset.ErrInvalid},
		{"escaping name", asset.Options{Prefix: "/a/", FS: testFS(), Files: []asset.File{{Name: "../app.css"}}}, asset.ErrInvalid},
		{"absolute name", asset.Options{Prefix: "/a/", FS: testFS(), Files: []asset.File{{Name: "/app.css"}}}, asset.ErrInvalid},
		{"name needing escapes", asset.Options{Prefix: "/a/", FS: fstest.MapFS{"a b.css": {}}, Files: []asset.File{{Name: "a b.css"}}}, asset.ErrInvalid},
		{"unknown extension", asset.Options{Prefix: "/a/", FS: fstest.MapFS{"x.bin": {}}, Files: []asset.File{{Name: "x.bin"}}}, asset.ErrInvalid},
		{"stylesheet of wrong type", asset.Options{Prefix: "/a/", FS: testFS(), Files: []asset.File{{Name: "notes.txt", Kind: asset.KindStylesheet}}}, asset.ErrInvalid},
		{"script of wrong type", asset.Options{Prefix: "/a/", FS: testFS(), Files: []asset.File{{Name: "app.css", Kind: asset.KindScript}}}, asset.ErrInvalid},
		{"unknown kind", asset.Options{Prefix: "/a/", FS: testFS(), Files: []asset.File{{Name: "app.css", Kind: asset.Kind(99)}}}, asset.ErrInvalid},
		{"integrity mismatch", asset.Options{Prefix: "/a/", FS: testFS(), Files: []asset.File{{Name: "app.js", Integrity: sri(appCSS)}}}, asset.ErrIntegrity},
		{"malformed integrity", asset.Options{Prefix: "/a/", FS: testFS(), Files: []asset.File{{Name: "app.js", Integrity: "nonsense"}}}, asset.ErrInvalid},
		{"unsupported integrity", asset.Options{Prefix: "/a/", FS: testFS(), Files: []asset.File{{Name: "app.js", Integrity: "md5-abc"}}}, asset.ErrInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t devtest.T) {
			s, err := asset.New(c.opts)
			t.Must(be.ErrorIs(err, c.err))
			t.Must(be.Nil(s))
		})
	}
	t.Run("missing file", func(t devtest.T) {
		_, err := asset.New(asset.Options{Prefix: "/a/", FS: testFS(), Files: []asset.File{{Name: "gone.css"}}})
		t.Must(be.ErrorIs(err, asset.ErrInvalid))
		t.Must(be.ErrorIs(err, fs.ErrNotExist))
		t.Must(be.ErrorContains(err, "gone.css"))
	})
	t.Run("must panics", func(t devtest.T) {
		t.Must(be.Panic(func() { asset.Must(asset.New(asset.Options{})) }))
	})
}

func TestPinnedIntegrityAlgorithms(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		alg string
		sum func([]byte) []byte
	}{
		{"sha256", func(b []byte) []byte { s := sha256.Sum256(b); return s[:] }},
		{"sha384", func(b []byte) []byte { s := sha512.Sum384(b); return s[:] }},
		{"sha512", func(b []byte) []byte { s := sha512.Sum512(b); return s[:] }},
	}
	for _, c := range cases {
		t.Run(c.alg, func(t devtest.T) {
			// app.js pinned with the digest of content, in the case's algorithm.
			pinned := func(content []byte) error {
				pin := c.alg + "-" + base64.StdEncoding.EncodeToString(c.sum(content))
				_, err := asset.New(asset.Options{Prefix: "/a/", FS: testFS(), Files: []asset.File{{Name: "app.js", Integrity: pin}}})
				return err
			}
			t.Must(be.NoError(pinned(appJS)))
			t.Must(be.ErrorIs(pinned(appCSS), asset.ErrIntegrity))
		})
	}
}

func TestExplicitContentType(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s, err := asset.New(asset.Options{
		Prefix: "/a/",
		FS:     fstest.MapFS{"data.bin": {Data: []byte{1, 2, 3}}},
		Files:  []asset.File{{Name: "data.bin", ContentType: "application/octet-stream"}},
	})
	t.Must(be.NoError(err))
	rec := get(t, s, http.MethodGet, s.URL("data.bin"), nil)
	t.Must(be.Equal(http.StatusOK, rec.Code))
	t.Must(be.Equal("application/octet-stream", rec.Header().Get("Content-Type")))
}

func TestServe(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := testSet(t)

	t.Run("current version", func(t devtest.T) {
		res := cthttp.Serve(s, httptest.NewRequestWithContext(t.Context(), http.MethodGet, s.URL("app.js"), nil))
		t.Must(res.Status(http.StatusOK))
		t.Must(res.Header("Content-Type", ct.Exact("text/javascript; charset=utf-8")))
		t.Must(res.Header("Cache-Control", ct.Exact("public, max-age=31536000, immutable")))
		t.Must(res.Header("X-Content-Type-Options", ct.Exact("nosniff")))
		t.Must(res.Header("ETag", ct.Prefix(`"`)))
		t.Must(be.DeepEqual(appJS, res.Bytes()))
	})
	t.Run("nested name", func(t devtest.T) {
		rec := get(t, s, http.MethodGet, s.URL("fonts/app.woff2"), nil)
		t.Must(be.Equal(http.StatusOK, rec.Code))
		t.Must(be.Equal("font/woff2", rec.Header().Get("Content-Type")))
		t.Must(be.DeepEqual(fontBin, rec.Body.Bytes()))
	})
	t.Run("other version", func(t devtest.T) {
		rec := get(t, s, http.MethodGet, "/assets/app/0123456789abcdef/app.js", nil)
		t.Must(be.Equal(http.StatusOK, rec.Code))
		t.Must(be.Equal("no-cache", rec.Header().Get("Cache-Control")))
		t.Must(be.DeepEqual(appJS, rec.Body.Bytes()))
	})
	t.Run("head", func(t devtest.T) {
		rec := get(t, s, http.MethodHead, s.URL("app.js"), nil)
		t.Must(be.Equal(http.StatusOK, rec.Code))
		t.Must(be.Equal(0, rec.Body.Len()))
		t.Must(be.Equal("text/javascript; charset=utf-8", rec.Header().Get("Content-Type")))
	})
	t.Run("not modified", func(t devtest.T) {
		etag := get(t, s, http.MethodGet, s.URL("app.js"), nil).Header().Get("ETag")
		for _, inm := range []string{etag, "W/" + etag, `"other", ` + etag, "*"} {
			rec := get(t, s, http.MethodGet, s.URL("app.js"), http.Header{"If-None-Match": {inm}})
			t.Must(be.Equal(http.StatusNotModified, rec.Code))
			t.Must(be.Equal(0, rec.Body.Len()))
		}
		rec := get(t, s, http.MethodGet, s.URL("app.js"), http.Header{"If-None-Match": {`"other"`}})
		t.Must(be.Equal(http.StatusOK, rec.Code))
	})
	t.Run("precondition failed", func(t devtest.T) {
		// The empty 412 must not look like a cacheable copy of the file.
		for _, accept := range []string{"", "gzip"} {
			rec := get(t, s, http.MethodGet, s.URL("app.css"), http.Header{"If-Match": {`"other"`}, "Accept-Encoding": {accept}})
			t.Must(be.Equal(http.StatusPreconditionFailed, rec.Code))
			t.Must(be.Equal("no-store", rec.Header().Get("Cache-Control")))
			t.Must(be.Equal("", rec.Header().Get("ETag")))
			t.Must(be.Equal("", rec.Header().Get("Content-Encoding")))
			t.Must(be.Equal("", rec.Header().Get("Content-Length")))
			t.Must(be.Equal(0, rec.Body.Len()))
		}
		for _, im := range []string{"*", get(t, s, http.MethodGet, s.URL("app.css"), nil).Header().Get("ETag")} {
			rec := get(t, s, http.MethodGet, s.URL("app.css"), http.Header{"If-Match": {im}})
			t.Must(be.Equal(http.StatusOK, rec.Code))
			t.Must(be.Equal("public, max-age=31536000, immutable", rec.Header().Get("Cache-Control")))
		}
	})
	t.Run("range not satisfiable", func(t devtest.T) {
		rec := get(t, s, http.MethodGet, s.URL("app.css"), http.Header{"Range": {"bytes=100000-"}, "Accept-Encoding": {"gzip"}})
		t.Must(be.Equal(http.StatusRequestedRangeNotSatisfiable, rec.Code))
		t.Must(be.Equal("no-store", rec.Header().Get("Cache-Control")))
		t.Must(be.Equal("", rec.Header().Get("ETag")))
		t.Must(be.Equal("", rec.Header().Get("Content-Encoding")))
	})
	t.Run("not found", func(t devtest.T) {
		for _, target := range []string{
			"/assets/app/" + s.Version() + "/missing.css",
			"/assets/app/" + s.Version(),
			"/assets/app/" + s.Version() + "/",
			"/assets/app/app.css",
			"/assets/app//app.css",
			"/elsewhere/" + s.Version() + "/app.css",
		} {
			rec := get(t, s, http.MethodGet, target, nil)
			t.Must(be.Equal(http.StatusNotFound, rec.Code))
			t.Must(be.Equal("nosniff", rec.Header().Get("X-Content-Type-Options")))
		}
	})
	t.Run("method not allowed", func(t devtest.T) {
		rec := get(t, s, http.MethodPost, s.URL("app.js"), nil)
		t.Must(be.Equal(http.StatusMethodNotAllowed, rec.Code))
		t.Must(be.Equal("GET, HEAD", rec.Header().Get("Allow")))
	})
	t.Run("mux pattern", func(t devtest.T) {
		mux := http.NewServeMux()
		mux.Handle(s.Pattern(), s)
		rec := get(t, mux, http.MethodGet, s.URL("app.css"), nil)
		t.Must(be.Equal(http.StatusOK, rec.Code))
		rec = get(t, mux, http.MethodHead, s.URL("app.css"), nil)
		t.Must(be.Equal(http.StatusOK, rec.Code))
		rec = get(t, mux, http.MethodPost, s.URL("app.css"), nil)
		t.Must(be.Equal(http.StatusMethodNotAllowed, rec.Code))
	})
}

func TestGzip(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := testSet(t)
	plain := get(t, s, http.MethodGet, s.URL("app.css"), nil)
	t.Must(be.Equal("", plain.Header().Get("Content-Encoding")))
	t.Must(be.Equal("Accept-Encoding", plain.Header().Get("Vary")))
	t.Must(be.DeepEqual(appCSS, plain.Body.Bytes()))

	for _, accept := range []string{"gzip", "br, gzip;q=0.5", "GZIP", "*", "deflate, x-gzip"} {
		rec := get(t, s, http.MethodGet, s.URL("app.css"), http.Header{"Accept-Encoding": {accept}})
		t.Must(be.Equal("gzip", rec.Header().Get("Content-Encoding")))
		t.Must(be.NotEqual(plain.Header().Get("ETag"), rec.Header().Get("ETag")))
		zr, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
		t.Must(be.NoError(err))
		data, err := io.ReadAll(zr)
		t.Must(be.NoError(err))
		t.Must(be.DeepEqual(appCSS, data))
	}
	for _, accept := range []string{"gzip;q=0", "gzip; q=0.000", "*;q=0", "br", "identity", "*, gzip;q=0"} {
		rec := get(t, s, http.MethodGet, s.URL("app.css"), http.Header{"Accept-Encoding": {accept}})
		t.Must(be.Equal("", rec.Header().Get("Content-Encoding")))
	}

	// Small or incompressible files have no second representation.
	rec := get(t, s, http.MethodGet, s.URL("fonts/app.woff2"), http.Header{"Accept-Encoding": {"gzip"}})
	t.Must(be.Equal("", rec.Header().Get("Content-Encoding")))
	t.Must(be.Equal("", rec.Header().Get("Vary")))
}

func TestGzipRepresentation(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := testSet(t)
	gz := http.Header{"Accept-Encoding": {"gzip"}}
	full := get(t, s, http.MethodGet, s.URL("app.css"), gz)
	t.Must(be.Equal(http.StatusOK, full.Code))
	t.Must(be.Equal("gzip", full.Header().Get("Content-Encoding")))
	body := full.Body.Bytes()
	gzETag := full.Header().Get("ETag")
	plainETag := get(t, s, http.MethodGet, s.URL("app.css"), nil).Header().Get("ETag")

	t.Run("content length", func(t devtest.T) {
		// The gzip bytes are precomputed, so their length is known.
		t.Must(be.Equal(strconv.Itoa(len(body)), full.Header().Get("Content-Length")))
		head := get(t, s, http.MethodHead, s.URL("app.css"), gz)
		t.Must(be.Equal(http.StatusOK, head.Code))
		t.Must(be.Equal("gzip", head.Header().Get("Content-Encoding")))
		t.Must(be.Equal(strconv.Itoa(len(body)), head.Header().Get("Content-Length")))
		t.Must(be.Equal(0, head.Body.Len()))
	})
	t.Run("not modified", func(t devtest.T) {
		rec := get(t, s, http.MethodGet, s.URL("app.css"), http.Header{"Accept-Encoding": {"gzip"}, "If-None-Match": {gzETag}})
		t.Must(be.Equal(http.StatusNotModified, rec.Code))
		t.Must(be.Equal("Accept-Encoding", rec.Header().Get("Vary")))
		t.Must(be.Equal("", rec.Header().Get("Content-Encoding")))
		t.Must(be.Equal("public, max-age=31536000, immutable", rec.Header().Get("Cache-Control")))
		t.Must(be.Equal(0, rec.Body.Len()))
		// The tag of the other representation does not match.
		rec = get(t, s, http.MethodGet, s.URL("app.css"), http.Header{"Accept-Encoding": {"gzip"}, "If-None-Match": {plainETag}})
		t.Must(be.Equal(http.StatusOK, rec.Code))
		t.Must(be.Equal("gzip", rec.Header().Get("Content-Encoding")))
	})
	t.Run("range", func(t devtest.T) {
		rec := get(t, s, http.MethodGet, s.URL("app.css"), http.Header{"Accept-Encoding": {"gzip"}, "Range": {"bytes=0-9"}})
		t.Must(be.Equal(http.StatusPartialContent, rec.Code))
		t.Must(be.Equal("gzip", rec.Header().Get("Content-Encoding")))
		t.Must(be.Equal("bytes 0-9/"+strconv.Itoa(len(body)), rec.Header().Get("Content-Range")))
		t.Must(be.Equal("10", rec.Header().Get("Content-Length")))
		t.Must(be.DeepEqual(body[:10], rec.Body.Bytes()))

		rec = get(t, s, http.MethodGet, s.URL("app.css"), http.Header{"Accept-Encoding": {"gzip"}, "Range": {"bytes=0-9"}, "If-Range": {gzETag}})
		t.Must(be.Equal(http.StatusPartialContent, rec.Code))
		// A range validated by the other representation's tag gets the whole file.
		rec = get(t, s, http.MethodGet, s.URL("app.css"), http.Header{"Accept-Encoding": {"gzip"}, "Range": {"bytes=0-9"}, "If-Range": {plainETag}})
		t.Must(be.Equal(http.StatusOK, rec.Code))
		t.Must(be.DeepEqual(body, rec.Body.Bytes()))
	})
}

// TestGzipOverHTTP checks the framing of gzip responses on a real
// connection: a length instead of chunks, and a readable bodiless 412.
func TestGzipOverHTTP(gt *testing.T) {
	t := devtest.ParallelT(gt)
	big := []byte(strings.Repeat("main { padding: 1rem; margin: 0 auto; }\n", 2000))
	s, err := asset.New(asset.Options{Prefix: "/a/", FS: fstest.MapFS{"big.css": {Data: big}}, Files: []asset.File{{Name: "big.css"}}})
	t.Must(be.NoError(err))
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	// Without transparent decompression, to see the encoded response.
	client := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	t.Cleanup(client.CloseIdleConnections)
	do := func(method string, header http.Header) (*http.Response, []byte) {
		req, err := http.NewRequestWithContext(t.Context(), method, srv.URL+s.URL("big.css"), nil)
		t.Must(be.NoError(err))
		req.Header = header
		res, err := client.Do(req)
		t.Must(be.NoError(err))
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		t.Must(be.NoError(err))
		return res, body
	}

	res, body := do(http.MethodGet, http.Header{"Accept-Encoding": {"gzip"}})
	t.Must(be.Equal(http.StatusOK, res.StatusCode))
	t.Must(be.Equal("gzip", res.Header.Get("Content-Encoding")))
	t.Must(be.Equal(0, len(res.TransferEncoding)))
	t.Must(be.Equal(int64(len(body)), res.ContentLength))
	t.Must(be.True(len(body) < len(big)))

	res, _ = do(http.MethodHead, http.Header{"Accept-Encoding": {"gzip"}})
	t.Must(be.Equal(int64(len(body)), res.ContentLength))

	res, empty := do(http.MethodGet, http.Header{"Accept-Encoding": {"gzip"}, "If-Match": {`"other"`}})
	t.Must(be.Equal(http.StatusPreconditionFailed, res.StatusCode))
	t.Must(be.Equal("", res.Header.Get("Content-Encoding")))
	t.Must(be.Equal(0, len(empty)))
}

func TestTags(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := testSet(t)
	v := s.Version()
	t.Must(be.Equal(
		`<link rel="stylesheet" href="/assets/app/`+v+`/app.css" integrity="`+sri(appCSS)+`" crossorigin="anonymous"/>`,
		render(t, s.Stylesheets())))
	t.Must(be.Equal(
		`<script src="/assets/app/`+v+`/app.js" defer integrity="`+sri(appJS)+`" crossorigin="anonymous"></script>`,
		render(t, s.Scripts())))
	t.Must(be.Equal(render(t, s.Stylesheets()), render(t, s.Stylesheet("app.css"))))
	t.Must(be.Equal(render(t, s.Scripts()), render(t, s.Script("app.js"))))

	var out strings.Builder
	err := core.Render(context.Background(), s.Script("app.css"), &out)
	t.Must(be.ErrorIs(err, asset.ErrInvalid))
	err = core.Render(context.Background(), s.Stylesheet("app.js"), &out)
	t.Must(be.ErrorIs(err, asset.ErrInvalid))
	err = core.Render(context.Background(), s.Preload("notes.txt"), &out)
	t.Must(be.ErrorIs(err, asset.ErrInvalid))
	t.Must(be.Panic(func() { s.Stylesheet("missing.css") }))

	t.Must(be.Equal(
		`<link rel="preload" href="/assets/app/`+v+`/fonts/app.woff2" as="font" type="font/woff2" integrity="`+sri(fontBin)+`" crossorigin="anonymous"/>`,
		render(t, s.Preload("fonts/app.woff2"))))
}

func TestPreload(gt *testing.T) {
	t := devtest.ParallelT(gt)
	logo := []byte("<svg xmlns=\"http://www.w3.org/2000/svg\"/>")
	fsys := testFS()
	fsys["logo.svg"] = &fstest.MapFile{Data: logo}
	fsys["main.mjs"] = &fstest.MapFile{Data: []byte("export const x = 1;\n")}
	s, err := asset.New(asset.Options{Prefix: "/a/", FS: fsys, Files: []asset.File{
		{Name: "app.css"}, {Name: "app.js"}, {Name: "fonts/app.woff2"}, {Name: "logo.svg"}, {Name: "main.mjs", Kind: asset.KindModule},
	}})
	t.Must(be.NoError(err))
	v := s.Version()
	cases := []struct{ name, want string }{
		{"app.css", `<link rel="preload" href="/a/` + v + `/app.css" as="style" type="text/css" integrity="` + sri(appCSS) + `" crossorigin="anonymous"/>`},
		{"app.js", `<link rel="preload" href="/a/` + v + `/app.js" as="script" type="text/javascript" integrity="` + sri(appJS) + `" crossorigin="anonymous"/>`},
		{"fonts/app.woff2", `<link rel="preload" href="/a/` + v + `/fonts/app.woff2" as="font" type="font/woff2" integrity="` + sri(fontBin) + `" crossorigin="anonymous"/>`},
		// CSS url() and <img> fetch images in no-cors mode without integrity;
		// a crossorigin preload would never be used.
		{"logo.svg", `<link rel="preload" href="/a/` + v + `/logo.svg" as="image" type="image/svg+xml"/>`},
		{"main.mjs", `<link rel="modulepreload" href="/a/` + v + `/main.mjs" integrity="` + s.Integrity("main.mjs") + `" crossorigin="anonymous"/>`},
	}
	for _, c := range cases {
		t.Must(be.Equal(c.want, render(t, s.Preload(c.name))))
	}
}

func TestModules(gt *testing.T) {
	t := devtest.ParallelT(gt)
	mod := []byte("import { x } from \"./lib.mjs\";\nconsole.log(x);\n")
	fsys := testFS()
	fsys["main.mjs"] = &fstest.MapFile{Data: mod}
	fsys["lib.mjs"] = &fstest.MapFile{Data: []byte("export const x = 1;\n")}
	s, err := asset.New(asset.Options{Prefix: "/a/", FS: fsys, Files: []asset.File{
		{Name: "lib.mjs"},
		{Name: "app.js"},
		{Name: "main.mjs", Kind: asset.KindModule},
	}})
	t.Must(be.NoError(err))
	v := s.Version()

	// .mjs is not a classic script: without a kind it is only served.
	lib, _ := s.Lookup("lib.mjs")
	t.Must(be.Equal(asset.KindOther, lib.Kind))
	t.Must(be.Equal("text/javascript; charset=utf-8", lib.ContentType))
	rec := get(t, s, http.MethodGet, s.URL("lib.mjs"), nil)
	t.Must(be.Equal(http.StatusOK, rec.Code))
	t.Must(be.Equal("text/javascript; charset=utf-8", rec.Header().Get("Content-Type")))

	// Modules and classic scripts render in declaration order.
	module := `<script type="module" src="/a/` + v + `/main.mjs" integrity="` + sri(mod) + `" crossorigin="anonymous"></script>`
	t.Must(be.Equal(
		`<script src="/a/`+v+`/app.js" defer integrity="`+sri(appJS)+`" crossorigin="anonymous"></script>`+module,
		render(t, s.Scripts())))
	t.Must(be.Equal(module, render(t, s.Script("main.mjs"))))
	var out strings.Builder
	t.Must(be.ErrorIs(core.Render(t.Context(), s.Script("lib.mjs"), &out), asset.ErrInvalid))

	_, err = asset.New(asset.Options{Prefix: "/a/", FS: fsys, Files: []asset.File{{Name: "app.css", Kind: asset.KindModule}}})
	t.Must(be.ErrorIs(err, asset.ErrInvalid))
}

func TestUnlisted(gt *testing.T) {
	t := devtest.ParallelT(gt)
	worker := []byte("self.onmessage = () => {};\n")
	extra := []byte("aside { margin: 0; }\n")
	fsys := testFS()
	fsys["worker.js"] = &fstest.MapFile{Data: worker}
	fsys["extra.css"] = &fstest.MapFile{Data: extra}
	s, err := asset.New(asset.Options{Prefix: "/a/", FS: fsys, Files: []asset.File{
		{Name: "app.js"},
		{Name: "worker.js", Unlisted: true},
		{Name: "extra.css", Unlisted: true},
	}})
	t.Must(be.NoError(err))
	v := s.Version()

	page := ct.View(elem.Bundle{s.Stylesheets(), s.Scripts()})
	t.Must(page.Find(ct.Tag("link")).None())
	t.Must(page.Find(ct.Tag("script")).AttrValues("src", "/a/"+v+"/app.js"))

	// Still served, and rendered on request.
	info, ok := s.Lookup("worker.js")
	t.Must(be.True(ok))
	t.Must(be.True(info.Unlisted))
	t.Must(be.Equal(asset.KindScript, info.Kind))
	t.Must(be.Equal(
		`<script src="/a/`+v+`/worker.js" defer integrity="`+sri(worker)+`" crossorigin="anonymous"></script>`,
		render(t, s.Script("worker.js"))))
	t.Must(be.Equal(
		`<link rel="stylesheet" href="/a/`+v+`/extra.css" integrity="`+sri(extra)+`" crossorigin="anonymous"/>`,
		render(t, s.Stylesheet("extra.css"))))
	rec := get(t, s, http.MethodGet, s.URL("worker.js"), nil)
	t.Must(be.Equal(http.StatusOK, rec.Code))
	t.Must(be.DeepEqual(worker, rec.Body.Bytes()))
	app, _ := s.Lookup("app.js")
	t.Must(be.False(app.Unlisted))
}

func TestTagsMatchServedBytes(gt *testing.T) {
	t := devtest.ParallelT(gt)
	s := testSet(t)
	page := ct.View(elem.Bundle{s.Stylesheets(), s.Scripts()})
	for _, sel := range []*ct.Selection{page.Find(ct.Tag("link")), page.Find(ct.Tag("script"))} {
		href, ok, err := sel.Attr(t.Context(), "href")
		t.Must(be.NoError(err))
		if !ok {
			href, ok, err = sel.Attr(t.Context(), "src")
			t.Must(be.NoError(err))
		}
		t.Must(be.True(ok))
		integrity, ok, err := sel.Attr(t.Context(), "integrity")
		t.Must(be.NoError(err))
		t.Must(be.True(ok))
		rec := get(t, s, http.MethodGet, href, nil)
		t.Must(be.Equal(http.StatusOK, rec.Code))
		t.Must(be.Equal(integrity, sri(rec.Body.Bytes())))
	}
}

func TestKindString(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var got []string
	for _, k := range []asset.Kind{asset.KindOther, asset.KindStylesheet, asset.KindScript, asset.KindFont, asset.KindImage, asset.KindModule, asset.Kind(42)} {
		got = append(got, k.String())
	}
	t.Must(be.True(slices.Equal([]string{"other", "stylesheet", "script", "font", "image", "module", "kind(42)"}, got)))
}
