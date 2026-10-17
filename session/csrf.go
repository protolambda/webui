package session

import (
	"crypto/subtle"
	"errors"

	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/form/input"
)

// CSRFField is the name of the hidden form field that carries a CSRF token.
const CSRFField = "csrf"

// ErrCSRF reports a missing, wrong or expired CSRF token. Respond with a
// generic client error (400 or 403) without saying more: the detail wrapped
// with it is for server logs.
var ErrCSRF = errors.New("session: invalid CSRF token")

// CSRFInput returns the hidden input that carries token in a form:
// <input type="hidden" name="csrf" value="...">. Make it part of every
// state-changing (POST) form, and of no GET form: a token in a URL leaks
// through history, logs and the Referer header. The value is escaped by the
// renderer like any attribute value.
func CSRFInput(token string) elem.Node {
	return input.Input(input.Type(input.Hidden), input.Name(CSRFField), input.Value(token))
}

// CheckCSRF compares a submitted token got with the expected token in
// constant time for tokens of equal length (the length of a token is not
// secret). It returns [ErrCSRF] if they differ, and always when expected is
// empty, so that a missing token on the server side never matches a missing
// field in the form.
func CheckCSRF(expected, got string) error {
	if expected == "" || subtle.ConstantTimeCompare([]byte(expected), []byte(got)) != 1 {
		return ErrCSRF
	}
	return nil
}
