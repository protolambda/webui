package asset

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"
)

// TestGzipETagFollowsGzipBytes covers two builds of one version whose gzip
// bytes differ, as with another Go version or compression level: the URL and
// the identity ETag are the same, so the gzip ETag must tell them apart, or
// a range request could combine parts of both.
func TestGzipETagFollowsGzipBytes(gt *testing.T) {
	t := devtest.ParallelT(gt)
	data := []byte(strings.Repeat("main { padding: 1rem; }\n", 200))
	gzipAt := func(level int) []byte {
		var buf bytes.Buffer
		zw, err := gzip.NewWriterLevel(&buf, level)
		t.Must(be.NoError(err))
		_, err = zw.Write(data)
		t.Must(be.NoError(err))
		t.Must(be.NoError(zw.Close()))
		return buf.Bytes()
	}
	fast, best := gzipAt(gzip.HuffmanOnly), gzipAt(gzip.BestCompression)
	t.Must(be.False(bytes.Equal(fast, best)))

	serve := func(gz []byte) (plain, encoded string) {
		info := Info{Name: "app.css", Kind: KindStylesheet, ContentType: "text/css; charset=utf-8", Size: len(data)}
		e := newEntry(info, data, gz)
		s := &Set{prefix: "/a/", version: "v1", files: []*entry{e}, byName: map[string]*entry{info.Name: e}}
		etag := func(header http.Header) string {
			rec := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/a/v1/app.css", nil)
			req.Header = header
			s.ServeHTTP(rec, req)
			t.Must(be.Equal(http.StatusOK, rec.Code))
			return rec.Header().Get("ETag")
		}
		return etag(http.Header{}), etag(http.Header{"Accept-Encoding": {"gzip"}})
	}
	plainA, gzA := serve(fast)
	plainB, gzB := serve(best)
	t.Must(be.Equal(plainA, plainB))
	t.Must(be.NotEqual(gzA, gzB))
	t.Must(be.NotEqual(plainA, gzA))
}
