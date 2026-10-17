package session

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// hostPrefix is the cookie name prefix that binds a cookie to its host: a
// browser accepts it only with Secure, Path=/ and no Domain.
const hostPrefix = "__Host-"

// cookiePolicy holds the attributes of one cookie of the package.
type cookiePolicy struct {
	name     string // full name, with the prefix
	secure   bool
	sameSite http.SameSite
}

// newCookiePolicy validates a cookie name given without prefix (def when
// empty) and returns the policy of the cookie.
func newCookiePolicy(name, def string, insecure, strict bool) (cookiePolicy, error) {
	if name == "" {
		name = def
	}
	if err := (&http.Cookie{Name: name, Value: "v"}).Valid(); err != nil {
		return cookiePolicy{}, fmt.Errorf("session: invalid cookie name %q", name)
	}
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "__host-") || strings.HasPrefix(lower, "__secure-") {
		return cookiePolicy{}, fmt.Errorf("session: cookie name %q has a prefix: give it without, the prefix follows from Insecure", name)
	}
	p := cookiePolicy{name: name, sameSite: http.SameSiteLaxMode}
	if !insecure {
		p.name = hostPrefix + name
		p.secure = true
	}
	if strict {
		p.sameSite = http.SameSiteStrictMode
	}
	return p, nil
}

// cookie returns the cookie with value; maxAge as in [http.Cookie].
func (p cookiePolicy) cookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     p.name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   p.secure,
		HttpOnly: true,
		SameSite: p.sameSite,
	}
}

// value returns the value of the request's cookie, or "" if it has none.
// With several cookies of the name, the first counts.
func (p cookiePolicy) value(r *http.Request) string {
	if r == nil {
		return ""
	}
	c, err := r.Cookie(p.name)
	if err != nil {
		return ""
	}
	return c.Value
}

// lockedReader serializes reads of a source of randomness, so that sources
// that are not safe for concurrent use (deterministic test sources) work.
type lockedReader struct {
	mu sync.Mutex
	r  io.Reader
}

func newLockedReader(r io.Reader) *lockedReader {
	if r == nil {
		r = rand.Reader
	}
	return &lockedReader{r: r}
}

// read fills p, or fails.
func (l *lockedReader) read(p []byte) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, err := io.ReadFull(l.r, p); err != nil {
		clear(p)
		return fmt.Errorf("session: read randomness: %w", err)
	}
	return nil
}

var errNegative = errors.New("session: negative duration or count in options")
