package form

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"
)

const testBoundary = "b0undary"

func testPart(name, value string, extra ...string) string {
	header := `Content-Disposition: form-data; name="` + name + `"` + strings.Join(extra, "")
	return "--" + testBoundary + "\r\n" + header + "\r\n\r\n" + value + "\r\n"
}

func testMultipart(parts ...string) string {
	return strings.Join(parts, "") + "--" + testBoundary + "--\r\n"
}

// observed runs one read with a reader that records every buffer it
// allocates.
type observed struct {
	values  *Values
	err     error
	buffers [][]byte
	maxCap  int
}

func observeRead(t devtest.T, contentType string, body io.Reader, limits Limits) *observed {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", body)
	r.Header.Set("Content-Type", contentType)
	o := new(observed)
	rd := newReader(limits)
	rd.observe = func(b []byte) {
		o.buffers = append(o.buffers, b)
		o.maxCap = max(o.maxCap, cap(b))
	}
	o.values, o.err = rd.read(httptest.NewRecorder(), r)
	return o
}

func allZero(b []byte) bool {
	return !slices.ContainsFunc(b, func(c byte) bool { return c != 0 })
}

// zeroed reports whether every observed buffer is cleared, but those still
// owned by the values (which keep them until Zero).
func (o *observed) zeroed() bool {
	owned := make(map[*byte]bool)
	if o.values != nil {
		for _, vs := range o.values.m {
			for _, v := range vs {
				if cap(v) > 0 {
					owned[&v[:1][0]] = true
				}
			}
		}
	}
	for _, b := range o.buffers {
		if cap(b) > 0 && owned[&b[:1][0]] {
			continue
		}
		if !allZero(b[:cap(b)]) {
			return false
		}
	}
	return true
}

func TestReadClearsEverythingOnError(gt *testing.T) {
	t := devtest.ParallelT(gt)
	secret := strings.Repeat("S", 40)
	limits := Limits{Body: 4096, Field: 64, Fields: 4}
	cases := []struct {
		name, contentType, body string
		want                    error
	}{
		{"urlencoded bad escape", URLEncoded, "a=" + secret + "&b=" + secret + "%zz", ErrMalformed},
		{"urlencoded truncated escape", URLEncoded, "a=" + secret + "&b=" + secret + "%4", ErrMalformed},
		{"urlencoded semicolon", URLEncoded, "a=" + secret + "&b=" + secret + ";", ErrMalformed},
		{"urlencoded nameless", URLEncoded, "a=" + secret + "&=" + secret, ErrMalformed},
		{"urlencoded field too large", URLEncoded, "a=" + secret + "&b=" + secret + secret, ErrTooLarge},
		{"urlencoded too many fields", URLEncoded, "a=" + secret + "&b=" + secret + "&c=" + secret + "&d=" + secret + "&e=" + secret, ErrTooManyFields},
		{"urlencoded body too large", URLEncoded, "a=" + secret + "&b=" + secret + strings.Repeat("x", 4096), ErrTooLarge},
		{"multipart file", multipartContentType, testMultipart(testPart("a", secret), testPart("b", secret), testPart("f", secret, `; filename="x"`)), ErrFile},
		{"multipart nameless", multipartContentType, testMultipart(testPart("a", secret), testPart("", secret)), ErrMalformed},
		{"multipart field too large", multipartContentType, testMultipart(testPart("a", secret), testPart("b", secret+secret)), ErrTooLarge},
		{"multipart too many fields", multipartContentType, testMultipart(testPart("a", secret), testPart("b", secret), testPart("c", secret), testPart("d", secret), testPart("e", secret)), ErrTooManyFields},
		{"multipart truncated", multipartContentType, testPart("a", secret) + testPart("b", secret), ErrMalformed},
		{"multipart body too large", multipartContentType, testMultipart(testPart("a", secret), testPart("b", secret+strings.Repeat("x", 4096))), ErrTooLarge},
	}
	for _, c := range cases {
		o := observeRead(t, c.contentType, strings.NewReader(c.body), limits)
		t.Mustf(be.ErrorIs(o.err, c.want), "case %s", c.name)
		t.Mustf(be.NilPtr(o.values), "case %s", c.name)
		t.Mustf(be.NonEmpty(o.buffers), "case %s", c.name)
		t.Mustf(be.True(o.zeroed()), "case %s: a buffer is not cleared", c.name)
	}
}

const multipartContentType = "multipart/form-data; boundary=" + testBoundary

func TestReadClearsScratchOnSuccess(gt *testing.T) {
	t := devtest.ParallelT(gt)
	secret := strings.Repeat("S", 3000) // grows the scratch buffer several times
	for _, c := range []struct{ contentType, body string }{
		{URLEncoded, "a=" + secret + "&b=" + url.QueryEscape(secret) + "&c"},
		{multipartContentType, testMultipart(testPart("a", secret), testPart("b", secret), testPart("c", ""))},
	} {
		o := observeRead(t, c.contentType, strings.NewReader(c.body), Limits{})
		t.Must(be.NoError(o.err))
		t.Must(be.Equal(secret, o.values.String("b")))
		t.Mustf(be.True(o.zeroed()), "%s: scratch not cleared", c.contentType)

		o.values.Zero()
		for _, b := range o.buffers {
			t.Must(be.True(allZero(b[:cap(b)])))
		}
	}
}

// The scratch buffer never grows past the field limit plus one byte, nor
// past the body limit plus one byte.
func TestReadScratchIsBounded(gt *testing.T) {
	t := devtest.ParallelT(gt)
	huge := strings.Repeat("x", 100_000)
	cases := []struct {
		contentType, body string
		limits            Limits
		maxCap            int
	}{
		{URLEncoded, "a=" + huge, Limits{Field: 1000}, readBufferSize},
		{multipartContentType, testMultipart(testPart("a", huge)), Limits{Field: 1000}, 1001},
		{URLEncoded, "a=" + huge, Limits{Body: 600, Field: 1 << 30}, readBufferSize},
		{multipartContentType, testMultipart(testPart("a", huge)), Limits{Body: 600, Field: 1 << 30}, 601},
	}
	for _, c := range cases {
		o := observeRead(t, c.contentType, strings.NewReader(c.body), c.limits)
		t.Must(be.ErrorIs(o.err, ErrTooLarge))
		t.Mustf(be.LessOrEq(o.maxCap, c.maxCap), "%s %+v", c.contentType, c.limits)
		t.Must(be.True(o.zeroed()))
	}
}

// FuzzReadURLEncoded checks the urlencoded parser against net/url, and that
// it clears every buffer whatever the input. The seed corpus runs as part of
// the normal tests.
func FuzzReadURLEncoded(f *testing.F) {
	for _, seed := range []string{
		"", "a=1&b=2", "a=1&a=2&a=", "q=hello+world%21", "%2B=%2b", "flag", "&a=1&&b=2&",
		"a=b=c", "n%C3%A4me=v%C3%A4lue", "a=%00", "%", "a=%4", "a=%zz", "=v", "a;b=1",
		"a=" + strings.Repeat("x", 40), strings.Repeat("a&", 12), "%41%42=%43%44",
	} {
		f.Add(seed)
	}
	const field, fields = 32, 8
	f.Fuzz(func(gt *testing.T, body string) {
		t := devtest.SerialT(gt)
		o := observeRead(t, URLEncoded, strings.NewReader(body), Limits{Body: 1 << 20, Field: field, Fields: fields})
		t.Must(be.LessOrEq(o.maxCap, max(readBufferSize, field+1)))
		t.Must(be.True(o.zeroed()))
		if o.err != nil {
			t.Must(be.NilPtr(o.values))
			ok := errors.Is(o.err, ErrMalformed) || errors.Is(o.err, ErrTooLarge) || errors.Is(o.err, ErrTooManyFields)
			t.Mustf(be.True(ok), "unclassified error %v", o.err)
		}

		want, qerr := url.ParseQuery(body)
		if qerr != nil {
			// net/url refuses bad escapes and semicolons, and so must Read.
			t.Mustf(be.Error(o.err), "net/url refused %q: %v", body, qerr)
			return
		}
		fits := true
		total := 0
		for name, vs := range want {
			total += len(vs)
			if name == "" || len(name) > field {
				fits = false
			}
			for _, v := range vs {
				if len(v) > field {
					fits = false
				}
			}
		}
		if total > fields {
			fits = false
		}
		if fits {
			t.Mustf(be.NoError(o.err), "body %q", body)
		}
		if o.err != nil {
			return
		}
		got := make(url.Values)
		for name, vs := range o.values.m {
			for _, v := range vs {
				got[name] = append(got[name], string(v))
			}
		}
		t.Mustf(be.DeepEqual(want, got), "body %q", body)
		o.values.Zero()
	})
}
