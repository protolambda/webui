package session

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/form"
)

// readValues reads an urlencoded body with form.Read.
func readValues(t devtest.T, body string) (*http.Request, *form.Values) {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", form.URLEncoded)
	v, err := form.Read(httptest.NewRecorder(), r, form.Limits{})
	t.Must(be.NoError(err))
	return r, v
}

func TestCheckFormZeroesTheValuesOnFailure(gt *testing.T) {
	t := devtest.ParallelT(gt)
	forms := Forms[int]{}

	r, v := readValues(t, "csrf=forged&password=hunter2")
	password := v.Bytes("password")
	t.Must(be.ErrorIs(forms.checkForm(r, v), ErrCSRF))
	t.Must(be.Equal(0, v.Len()))
	t.Must(be.Equal(string(make([]byte, len("hunter2"))), string(password)))

	// A form that passes keeps its values.
	s := NewDetached(1, "good")
	r, v = readValues(t, "csrf=good&password=hunter2")
	r = r.WithContext(WithSession(r.Context(), s))
	t.Must(be.NoError(forms.checkForm(r, v)))
	t.Must(be.Equal("hunter2", string(v.Bytes("password"))))
	v.Zero()
}
