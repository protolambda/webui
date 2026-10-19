package form_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/form"
)

// wrappedWriter stands for the writers of middleware: it wraps the server's
// writer and reaches it through Unwrap.
type wrappedWriter struct{ http.ResponseWriter }

func (w wrappedWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// TestReadClosesConnectionBehindWrappers posts a chunked body over the
// limit to a handler whose writer wraps the server's. The signal of
// http.MaxBytesReader to close the connection after the response reaches
// the server's writer through Unwrap, so the response announces
// Connection: close instead of the server reading the rest of the body for
// the next request. The rest is small here: without the signal, the server
// would read it and keep the connection.
func TestReadClosesConnectionBehindWrappers(gt *testing.T) {
	t := devtest.ParallelT(gt)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := form.Read(wrappedWriter{w}, r, form.Limits{Body: 1024})
		switch {
		case errors.Is(err, form.ErrTooLarge):
			http.Error(w, "too large", http.StatusRequestEntityTooLarge)
		case err != nil:
			http.Error(w, "unexpected", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)
	// A body type that NewRequest knows no length of is sent chunked, so
	// the declared length does not refuse it first.
	body := io.NopCloser(strings.NewReader("a=" + strings.Repeat("x", 4000)))
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL, body)
	t.Must(be.NoError(err))
	req.Header.Set("Content-Type", form.URLEncoded)
	res, err := srv.Client().Do(req)
	t.Must(be.NoError(err))
	defer res.Body.Close()
	t.Must(be.Equal(http.StatusRequestEntityTooLarge, res.StatusCode))
	t.Mustf(be.True(res.Close), "the response announces Connection: close")
}
