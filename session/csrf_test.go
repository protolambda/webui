package session_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/protolambda/chord/core"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/session"
)

func TestCheckCSRF(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		expected, got string
		ok            bool
	}{
		{"token", "token", true},
		{"", "", false},
		{"", "token", false},
		{"token", "", false},
		{"token", "tokeN", false},
		{"token", "toke", false},
		{"token", "tokenn", false},
		{"token", " token", false},
	}
	for _, c := range cases {
		err := session.CheckCSRF(c.expected, c.got)
		if c.ok {
			t.Mustf(be.NoError(err), "%q vs %q", c.expected, c.got)
		} else {
			t.Mustf(be.ErrorIs(err, session.ErrCSRF), "%q vs %q", c.expected, c.got)
		}
	}
}

func TestSessionCSRFToken(gt *testing.T) {
	t := devtest.ParallelT(gt)
	st := newStore(t, session.Options[account]{})
	a, cookie := login(t, st, nil, account{})
	b, _ := login(t, st, nil, account{})

	token := a.CSRF()
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	t.Must(be.NoError(err))
	t.Must(be.Equal(32, len(raw)))
	t.Must(be.NotEqual(token, b.CSRF()))
	t.Must(be.NotEqual(token, cookie.Value))

	t.Must(be.NoError(a.CheckCSRF(token)))
	t.Must(be.ErrorIs(a.CheckCSRF(b.CSRF()), session.ErrCSRF))
	t.Must(be.ErrorIs(a.CheckCSRF(""), session.ErrCSRF))
	t.Must(be.ErrorIs(a.CheckCSRF(token[:42]), session.ErrCSRF))
	t.Must(be.ErrorIs(a.CheckCSRF(token+"A"), session.ErrCSRF))
	near := []byte(token)
	near[10] ^= 1
	t.Must(be.ErrorIs(a.CheckCSRF(string(near)), session.ErrCSRF))
	t.Must(be.ErrorIs(a.CheckCSRF(strings.ToUpper(token)), session.ErrCSRF))
	t.Must(be.ErrorIs(a.CheckCSRF(cookie.Value), session.ErrCSRF))
}

func TestCSRFInput(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var out strings.Builder
	t.Must(be.NoError(core.Render(t.Context(), session.CSRFInput("abc-_123"), &out)))
	t.Must(be.Equal(`<input type="hidden" name="csrf" value="abc-_123"/>`, out.String()))

	out.Reset()
	t.Must(be.NoError(core.Render(t.Context(), session.CSRFInput(`"><b>x</b>`), &out)))
	t.Must(be.False(strings.Contains(out.String(), "<b>")))

	page := ct.View(session.CSRFInput("tok"))
	t.Must(page.Find(ct.Tag("input"), ct.Attr("type", "hidden"), ct.Attr("name", session.CSRFField), ct.Attr("value", "tok")))
}
