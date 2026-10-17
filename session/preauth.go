package session

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Defaults of [PreAuthOptions].
const (
	// DefaultPreAuthCookieName is the pre-authentication cookie's name
	// without its prefix; with the default Secure policy the cookie is
	// "__Host-preauth".
	DefaultPreAuthCookieName = "preauth"
	// DefaultPreAuthTTL is how long a pre-authentication form stays valid
	// after its cookie was issued.
	DefaultPreAuthTTL = 10 * time.Minute
	// DefaultPreAuthSkew is how far in the future a cookie's issue time may
	// lie, for a wall clock that stepped back.
	DefaultPreAuthSkew = time.Minute
)

const (
	preAuthKeyBytes   = 32
	preAuthNonceBytes = 32
	// preAuthBodyBytes is the authenticated part of the cookie: the nonce
	// and the issue time in Unix seconds, big-endian.
	preAuthBodyBytes   = preAuthNonceBytes + 8
	preAuthCookieBytes = preAuthBodyBytes + sha256.Size
	// preAuthCookieLen is the length of the encoded cookie value.
	preAuthCookieLen = 96
	// preAuthTokenLen is the length of an encoded form token.
	preAuthTokenLen = 43

	// The labels separate the two MACs under the one key, and this
	// construction from any other use of a key.
	preAuthCookieLabel = "webui/session pre-auth cookie v1\x00"
	preAuthFormLabel   = "webui/session pre-auth form v1\x00"
)

// PreAuthOptions configures a [PreAuth]. The zero value gives the secure
// defaults.
type PreAuthOptions struct {
	// CookieName is the name of the cookie, without prefix; "" means
	// [DefaultPreAuthCookieName]. The rules of [Options.CookieName] apply.
	CookieName string
	// Insecure drops the Secure attribute and the __Host- prefix of the
	// cookie. Use it only to develop over plain http on localhost.
	Insecure bool
	// StrictSameSite marks the cookie SameSite=Strict instead of
	// SameSite=Lax.
	StrictSameSite bool
	// TTL is how long a form stays valid after its cookie was issued; zero
	// means [DefaultPreAuthTTL]. It must be at least a second: issue times
	// have a resolution of one second.
	TTL time.Duration
	// Skew is how far in the future an issue time may lie; zero means
	// [DefaultPreAuthSkew].
	Skew time.Duration
	// Now returns the current time; nil means [time.Now].
	Now func() time.Time
	// Rand is the source of the key and of the nonces; nil means
	// crypto/rand. Reads are serialized. Replace it only in tests.
	Rand io.Reader
}

// PreAuth protects forms that are shown before a session exists, such as a
// login or sign-up form, against cross-site request forgery (a page of
// another site signing the visitor in to an account of the attacker's),
// without keeping any state per visitor. Create it with [NewPreAuth]. It is
// safe for concurrent use. Formatting it with any verb prints only its
// cookie name, never its key.
//
// [PreAuth.Issue] sets a cookie that carries a random nonce, the time of
// issue and an HMAC-SHA256 tag of both, and returns the form token: a second
// HMAC-SHA256 of the same nonce and time under a separate label. Render the
// token with [CSRFInput]. [PreAuth.Verify] accepts a submission only with an
// authentic cookie issued less than the TTL ago (and no more than the skew
// in the future) together with that cookie's form token. Another site can
// read neither the visitor's cookie (HttpOnly) nor the visitor's token (the
// same-origin policy hides the page), and cannot make the visitor's browser
// send a cookie of its choosing (the __Host- prefix keeps sibling hosts from
// planting one), so it cannot get a matching pair submitted. A pair the
// attacker obtained for itself is useless without its cookie in the
// visitor's browser.
//
// The key is drawn at construction and kept in memory only: a restart
// invalidates outstanding forms, and processes behind one load balancer do
// not accept each other's forms.
//
// The trade-off of keeping no state: there is nothing to evict, so no
// number of anonymous visits can expire the form of another visitor or grow
// the server's memory, but a form is not one-time. A cookie and token pair
// verifies any number of times until it expires. That does not weaken the
// CSRF defence, since the pair only ever exists in the visitor's own
// browser, but it means PreAuth is neither a replay defence nor a rate
// limit: throttle login attempts separately, and start a session with
// [Store.New] (which rotates the session ID) once a login succeeds.
type PreAuth struct {
	cookie cookiePolicy
	ttl    time.Duration
	skew   time.Duration
	now    func() time.Time
	rand   *lockedReader
	key    [preAuthKeyBytes]byte
}

// NewPreAuth returns a PreAuth with a fresh random key. It fails on invalid
// options (a bad cookie name, a negative duration, a TTL under a second) or
// when the source of randomness fails.
func NewPreAuth(opts PreAuthOptions) (*PreAuth, error) {
	cookie, err := newCookiePolicy(opts.CookieName, DefaultPreAuthCookieName, opts.Insecure, opts.StrictSameSite)
	if err != nil {
		return nil, err
	}
	if opts.TTL < 0 || opts.Skew < 0 {
		return nil, errNegative
	}
	p := &PreAuth{
		cookie: cookie,
		ttl:    opts.TTL,
		skew:   opts.Skew,
		now:    opts.Now,
		rand:   newLockedReader(opts.Rand),
	}
	if p.ttl == 0 {
		p.ttl = DefaultPreAuthTTL
	}
	if p.ttl < time.Second {
		return nil, errors.New("session: pre-auth TTL under a second")
	}
	if p.skew == 0 {
		p.skew = DefaultPreAuthSkew
	}
	if p.now == nil {
		p.now = time.Now
	}
	if err := p.rand.read(p.key[:]); err != nil {
		return nil, err
	}
	return p, nil
}

// CookieName returns the full name of the cookie, prefix included.
func (p *PreAuth) CookieName() string {
	return p.cookie.name
}

// Issue returns the form token for a pre-authentication form, to render
// with [CSRFInput], and makes sure the browser holds its cookie. If r
// already carries a valid cookie with at least half of its lifetime left,
// Issue returns that cookie's token and sets nothing, so that tabs of the
// form opened within that first half share one cookie and stay valid
// together. Otherwise it sets a new cookie on w, which expires with the
// form; the browser then replaces the old cookie, and forms rendered from
// the old one are refused (the visitor reloads them). r may be nil. Issue
// fails only if the source of randomness fails, without touching w.
func (p *PreAuth) Issue(w http.ResponseWriter, r *http.Request) (string, error) {
	now := p.now()
	if body, issued, err := p.parse(p.cookie.value(r), now); err == nil && now.Sub(issued) < p.ttl/2 {
		return p.formToken(&body), nil
	}
	var raw [preAuthCookieBytes]byte
	if err := p.rand.read(raw[:preAuthNonceBytes]); err != nil {
		return "", err
	}
	binary.BigEndian.PutUint64(raw[preAuthNonceBytes:preAuthBodyBytes], uint64(now.Unix()))
	body := [preAuthBodyBytes]byte(raw[:preAuthBodyBytes])
	copy(raw[preAuthBodyBytes:], p.mac(preAuthCookieLabel, &body))
	http.SetCookie(w, p.cookie.cookie(base64.RawURLEncoding.EncodeToString(raw[:]), int(p.ttl/time.Second)))
	return p.formToken(&body), nil
}

// Verify checks a submitted form token against r's cookie: the cookie must
// be authentic and unexpired, and token must be its form token. It returns
// nil or an error wrapping [ErrCSRF], whose detail (missing, malformed,
// forged, expired, mismatch) is meant for logs. Verify is stateless: a
// valid pair verifies again until it expires.
func (p *PreAuth) Verify(r *http.Request, token string) error {
	body, _, err := p.parse(p.cookie.value(r), p.now())
	if err != nil {
		return err
	}
	if len(token) != preAuthTokenLen ||
		subtle.ConstantTimeCompare([]byte(token), []byte(p.formToken(&body))) != 1 {
		return fmt.Errorf("%w: pre-auth token does not match its cookie", ErrCSRF)
	}
	return nil
}

// ClearCookie sets a cookie on w that removes the pre-authentication cookie
// from the browser, for example after a successful login.
func (p *PreAuth) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, p.cookie.cookie("", -1))
}

// String describes p without revealing its key.
func (p *PreAuth) String() string {
	return "session.PreAuth(" + p.cookie.name + ")"
}

// GoString is like String, so that %#v does not print the key either.
func (p *PreAuth) GoString() string {
	return p.String()
}

// Format prints String for every verb (fmt would print the key for verbs
// such as %d), formatted as a string for %v, %s, %q, %x and %X.
func (p *PreAuth) Format(f fmt.State, verb rune) {
	formatRedacted(f, verb, p.String())
}

// parse checks a cookie value at now and returns its authenticated body and
// issue time.
func (p *PreAuth) parse(value string, now time.Time) (body [preAuthBodyBytes]byte, issued time.Time, err error) {
	if value == "" {
		return body, issued, fmt.Errorf("%w: no pre-auth cookie", ErrCSRF)
	}
	// The length is checked first: a long value is refused undecoded.
	if len(value) != preAuthCookieLen {
		return body, issued, fmt.Errorf("%w: malformed pre-auth cookie", ErrCSRF)
	}
	var raw [preAuthCookieBytes]byte
	if n, err := base64.RawURLEncoding.Strict().Decode(raw[:], []byte(value)); err != nil || n != preAuthCookieBytes {
		return body, issued, fmt.Errorf("%w: malformed pre-auth cookie", ErrCSRF)
	}
	body = [preAuthBodyBytes]byte(raw[:preAuthBodyBytes])
	if !hmac.Equal(raw[preAuthBodyBytes:], p.mac(preAuthCookieLabel, &body)) {
		return body, issued, fmt.Errorf("%w: forged pre-auth cookie", ErrCSRF)
	}
	issued = time.Unix(int64(binary.BigEndian.Uint64(body[preAuthNonceBytes:])), 0)
	age := now.Sub(issued)
	if age >= p.ttl {
		return body, issued, fmt.Errorf("%w: expired pre-auth cookie", ErrCSRF)
	}
	if age < -p.skew {
		return body, issued, fmt.Errorf("%w: pre-auth cookie issued in the future", ErrCSRF)
	}
	return body, issued, nil
}

// mac returns the HMAC-SHA256 of body under p's key, with label.
func (p *PreAuth) mac(label string, body *[preAuthBodyBytes]byte) []byte {
	m := hmac.New(sha256.New, p.key[:])
	m.Write([]byte(label))
	m.Write(body[:])
	return m.Sum(nil)
}

// formToken returns the encoded form token of a cookie body.
func (p *PreAuth) formToken(body *[preAuthBodyBytes]byte) string {
	return base64.RawURLEncoding.EncodeToString(p.mac(preAuthFormLabel, body))
}
