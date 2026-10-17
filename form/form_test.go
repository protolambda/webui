package form_test

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/form"
)

const boundary = "b0undary"

const multipartType = "multipart/form-data; boundary=" + boundary

// part returns a raw multipart part with the given header lines.
func part(content string, header ...string) string {
	return "--" + boundary + "\r\n" + strings.Join(header, "\r\n") + "\r\n\r\n" + content + "\r\n"
}

// field returns a raw multipart part of a form field.
func field(name, value string) string {
	return part(value, `Content-Disposition: form-data; name="`+name+`"`)
}

// multipartBody joins parts and appends the closing delimiter.
func multipartBody(parts ...string) string {
	return strings.Join(parts, "") + "--" + boundary + "--\r\n"
}

func request(t devtest.T, contentType string, body io.Reader) *http.Request {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/items", body)
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	return r
}

func readString(t devtest.T, contentType, body string, limits form.Limits) (*form.Values, error) {
	return form.Read(httptest.NewRecorder(), request(t, contentType, strings.NewReader(body)), limits)
}

// dump returns every field of v as name -> values, for comparisons.
func dump(v *form.Values) map[string][]string {
	out := make(map[string][]string)
	for _, name := range v.Names() {
		out[name] = v.Strings(name)
	}
	return out
}

func TestReadURLEncoded(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		body string
		want map[string][]string
	}{
		{"", map[string][]string{}},
		{"a=1&b=2", map[string][]string{"a": {"1"}, "b": {"2"}}},
		{"a=1&a=2&a=", map[string][]string{"a": {"1", "2", ""}}},
		{"q=hello+world%21", map[string][]string{"q": {"hello world!"}}},
		{"%2B=%2b+%20", map[string][]string{"+": {"+  "}}},
		{"flag", map[string][]string{"flag": {""}}},
		{"&a=1&&b=2&", map[string][]string{"a": {"1"}, "b": {"2"}}},
		{"&&&", map[string][]string{}},
		{"a=b=c", map[string][]string{"a": {"b=c"}}},
		{"n%C3%A4me=v%C3%A4lue", map[string][]string{"näme": {"välue"}}},
		{"%41=%4a%4A", map[string][]string{"A": {"JJ"}}},
		{"a=%00%0D%0A", map[string][]string{"a": {"\x00\r\n"}}},
		{"a=\xff\x01", map[string][]string{"a": {"\xff\x01"}}},
		{"a=%26%3D%3B", map[string][]string{"a": {"&=;"}}},
		{"a+b=c", map[string][]string{"a b": {"c"}}},
	}
	for _, c := range cases {
		v, err := readString(t, form.URLEncoded, c.body, form.Limits{})
		t.Mustf(be.NoError(err), "body %q", c.body)
		t.Mustf(be.DeepEqual(c.want, dump(v)), "body %q", c.body)
	}
}

func TestReadURLEncodedMalformed(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, body := range []string{
		"%", "a=%", "a=%4", "a=%4g", "a=%g4", "%zz=1", "a=%%41", "a=1&b=%",
		"=v", "=", "a=1&=2", "a=1&=",
		"a;b=1", "a=1;b=2", ";",
	} {
		v, err := readString(t, form.URLEncoded, body, form.Limits{})
		t.Mustf(be.ErrorIs(err, form.ErrMalformed), "body %q", body)
		t.Mustf(be.NilPtr(v), "body %q", body)
	}
}

func TestReadMultipart(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	t.Must(be.NoError(mw.SetBoundary(boundary)))
	for _, kv := range [][2]string{
		{"title", "Weekly notes"},
		{"tag", "work"},
		{"tag", "urgent"},
		{"body", "line 1\r\nline 2\n--" + boundary + "x"},
		{"empty", ""},
		{"binary", "\x00\xff"},
	} {
		t.Must(be.NoError(mw.WriteField(kv[0], kv[1])))
	}
	t.Must(be.NoError(mw.Close()))

	v, err := form.Read(httptest.NewRecorder(), request(t, mw.FormDataContentType(), &body), form.Limits{})
	t.Must(be.NoError(err))
	t.Must(be.DeepEqual(map[string][]string{
		"title":  {"Weekly notes"},
		"tag":    {"work", "urgent"},
		"body":   {"line 1\r\nline 2\n--" + boundary + "x"},
		"empty":  {""},
		"binary": {"\x00\xff"},
	}, dump(v)))
}

func TestReadMultipartEmptyForm(gt *testing.T) {
	t := devtest.ParallelT(gt)
	v, err := readString(t, multipartType, multipartBody(), form.Limits{})
	t.Must(be.NoError(err))
	t.Must(be.Equal(0, v.Len()))
}

func TestReadMultipartRefusals(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cases := []struct {
		name string
		body string
		want error
	}{
		{"file", multipartBody(field("a", "1"), part("data", `Content-Disposition: form-data; name="f"; filename="notes.txt"`)), form.ErrFile},
		{"empty file input", multipartBody(part("", `Content-Disposition: form-data; name="f"; filename=""`, "Content-Type: application/octet-stream")), form.ErrFile},
		{"extended filename", multipartBody(part("x", `Content-Disposition: form-data; name="f"; filename*=UTF-8''notes.txt`)), form.ErrFile},
		{"no name", multipartBody(part("x", `Content-Disposition: form-data`)), form.ErrMalformed},
		{"empty name", multipartBody(part("x", `Content-Disposition: form-data; name=""`)), form.ErrMalformed},
		{"not form-data", multipartBody(part("x", `Content-Disposition: attachment; name="a"`)), form.ErrMalformed},
		{"no disposition", multipartBody(part("x", "Content-Type: text/plain")), form.ErrMalformed},
		{"bad disposition", multipartBody(part("x", `Content-Disposition: form-data; name="a"; name="b"`)), form.ErrMalformed},
		{"bad header line", multipartBody(part("x", "no colon here")), form.ErrMalformed},
		{"truncated", field("a", "1"), form.ErrMalformed},
		{"truncated in part", "--" + boundary + "\r\nContent-Disposition: form-data; name=\"a\"\r\n\r\nvalue", form.ErrMalformed},
		{"empty body", "", form.ErrMalformed},
		{"garbage", "this is not multipart", form.ErrMalformed},
	}
	for _, c := range cases {
		v, err := readString(t, multipartType, c.body, form.Limits{})
		t.Mustf(be.ErrorIs(err, c.want), "case %s", c.name)
		t.Mustf(be.NilPtr(v), "case %s", c.name)
	}
}

func TestReadMultipartBoundary(gt *testing.T) {
	t := devtest.ParallelT(gt)
	long := strings.Repeat("b", 70)
	body := "--" + long + "\r\n" + `Content-Disposition: form-data; name="a"` + "\r\n\r\n1\r\n--" + long + "--\r\n"
	v, err := readString(t, "multipart/form-data; boundary="+long, body, form.Limits{})
	t.Must(be.NoError(err))
	t.Must(be.Equal("1", v.String("a")))

	tooLong := long + "b"
	body = strings.ReplaceAll(body, long, tooLong)
	_, err = readString(t, "multipart/form-data; boundary="+tooLong, body, form.Limits{})
	t.Must(be.ErrorIs(err, form.ErrMalformed))

	_, err = readString(t, "multipart/form-data", multipartBody(field("a", "1")), form.Limits{})
	t.Must(be.ErrorIs(err, form.ErrMalformed))
	// An empty parameter value makes the whole header unparsable.
	_, err = readString(t, "multipart/form-data; boundary=", multipartBody(field("a", "1")), form.Limits{})
	t.Must(be.ErrorIs(err, form.ErrUnsupportedType))
}

func TestReadContentTypes(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, ct := range []string{
		"", "text/plain", "application/json", "multipart/mixed; boundary=" + boundary,
		"application/x-www-form-urlencoded; =bad", "not a media type",
	} {
		v, err := readString(t, ct, "a=1", form.Limits{})
		t.Mustf(be.ErrorIs(err, form.ErrUnsupportedType), "content type %q", ct)
		t.Mustf(be.NilPtr(v), "content type %q", ct)
	}
	for _, ct := range []string{
		"application/x-www-form-urlencoded; charset=UTF-8",
		"Application/X-WWW-Form-URLEncoded",
	} {
		v, err := readString(t, ct, "a=1", form.Limits{})
		t.Mustf(be.NoError(err), "content type %q", ct)
		t.Mustf(be.Equal("1", v.String("a")), "content type %q", ct)
	}
}

func TestReadIgnoresQuery(gt *testing.T) {
	t := devtest.ParallelT(gt)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/items?a=query", strings.NewReader("b=body"))
	r.Header.Set("Content-Type", form.URLEncoded)
	v, err := form.Read(httptest.NewRecorder(), r, form.Limits{})
	t.Must(be.NoError(err))
	t.Must(be.False(v.Has("a")))
	t.Must(be.Equal("body", v.String("b")))
}

func TestReadBodyLimit(gt *testing.T) {
	t := devtest.ParallelT(gt)
	encodings := []struct {
		name, contentType, body string
	}{
		{"urlencoded", form.URLEncoded, "a=" + strings.Repeat("x", 98)},
		{"multipart", multipartType, multipartBody(field("a", strings.Repeat("x", 20)))},
	}
	for _, e := range encodings {
		size := int64(len(e.body))
		v, err := readString(t, e.contentType, e.body, form.Limits{Body: size})
		t.Mustf(be.NoError(err), "%s at the limit", e.name)
		t.Mustf(be.Equal(1, v.Len()), "%s at the limit", e.name)

		v, err = readString(t, e.contentType, e.body, form.Limits{Body: size - 1})
		t.Mustf(be.ErrorIs(err, form.ErrTooLarge), "%s over the limit", e.name)
		tooBig, ok := errors.AsType[*http.MaxBytesError](err)
		t.Mustf(be.True(ok), "%s: the MaxBytesError is wrapped", e.name)
		t.Must(be.Equal(size-1, tooBig.Limit))
		t.Must(be.NilPtr(v))
	}
}

func TestReadFieldLimit(gt *testing.T) {
	t := devtest.ParallelT(gt)
	limits := form.Limits{Field: 16}
	at, over := strings.Repeat("v", 16), strings.Repeat("v", 17)
	cases := []struct {
		name, contentType string
		ok, tooLarge      string
	}{
		{"urlencoded value", form.URLEncoded, "a=" + at, "a=" + over},
		{"urlencoded escaped value", form.URLEncoded, "a=" + strings.Repeat("%41", 16), "a=" + strings.Repeat("%41", 17)},
		{"urlencoded name", form.URLEncoded, at + "=1", over + "=1"},
		{"urlencoded name without value", form.URLEncoded, at, over},
		{"multipart value", multipartType, multipartBody(field("a", at)), multipartBody(field("a", over))},
		{"multipart name", multipartType, multipartBody(field(at, "1")), multipartBody(field(over, "1"))},
	}
	for _, c := range cases {
		v, err := readString(t, c.contentType, c.ok, limits)
		t.Mustf(be.NoError(err), "%s at the limit", c.name)
		t.Mustf(be.Equal(1, v.Len()), "%s at the limit", c.name)

		v, err = readString(t, c.contentType, c.tooLarge, limits)
		t.Mustf(be.ErrorIs(err, form.ErrTooLarge), "%s over the limit", c.name)
		_, isBody := errors.AsType[*http.MaxBytesError](err)
		t.Mustf(be.False(isBody), "%s: not a body limit", c.name)
		t.Must(be.NilPtr(v))
	}
}

func TestReadFieldCount(gt *testing.T) {
	t := devtest.ParallelT(gt)
	limits := form.Limits{Fields: 3}
	cases := []struct {
		name, contentType string
		ok, tooMany       string
	}{
		{"urlencoded", form.URLEncoded, "a=1&a=2&b=", "a=1&a=2&b=&c"},
		{"urlencoded with empty fields", form.URLEncoded, "&a=1&&a=2&&b=&&", "&a=1&&a=2&&b=&&c&"},
		{"multipart", multipartType,
			multipartBody(field("a", "1"), field("a", "2"), field("b", "")),
			multipartBody(field("a", "1"), field("a", "2"), field("b", ""), field("c", ""))},
	}
	for _, c := range cases {
		v, err := readString(t, c.contentType, c.ok, limits)
		t.Mustf(be.NoError(err), "%s at the limit", c.name)
		t.Mustf(be.Equal(3, v.Len()), "%s at the limit", c.name)

		v, err = readString(t, c.contentType, c.tooMany, limits)
		t.Mustf(be.ErrorIs(err, form.ErrTooManyFields), "%s over the limit", c.name)
		t.Must(be.NilPtr(v))
	}
}

func TestReadDefaultLimits(gt *testing.T) {
	t := devtest.ParallelT(gt)
	v, err := readString(t, form.URLEncoded, "a="+strings.Repeat("x", form.DefaultField), form.Limits{})
	t.Must(be.NoError(err))
	t.Must(be.Equal(form.DefaultField, len(v.Bytes("a"))))

	_, err = readString(t, form.URLEncoded, "a="+strings.Repeat("x", form.DefaultField+1), form.Limits{})
	t.Must(be.ErrorIs(err, form.ErrTooLarge))

	_, err = readString(t, form.URLEncoded, strings.Repeat("a&", form.DefaultFields), form.Limits{})
	t.Must(be.NoError(err))
	_, err = readString(t, form.URLEncoded, strings.Repeat("a&", form.DefaultFields+1), form.Limits{})
	t.Must(be.ErrorIs(err, form.ErrTooManyFields))

	_, err = readString(t, form.URLEncoded, "a="+strings.Repeat("&", form.DefaultBody), form.Limits{})
	t.Must(be.ErrorIs(err, form.ErrTooLarge))
}

func TestReadNegativeLimitsPanic(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, limits := range []form.Limits{{Body: -1}, {Field: -1}, {Fields: -1}} {
		t.Must(be.Panic(func() {
			_, _ = readString(t, form.URLEncoded, "a=1", limits)
		}))
	}
}

type failingReader struct{ err error }

func (f failingReader) Read([]byte) (int, error) { return 0, f.err }

func TestReadBodyError(gt *testing.T) {
	t := devtest.ParallelT(gt)
	cause := errors.New("connection reset")
	for _, ct := range []string{form.URLEncoded, multipartType} {
		body := io.MultiReader(strings.NewReader("a=1&"), failingReader{cause})
		v, err := form.Read(httptest.NewRecorder(), request(t, ct, body), form.Limits{})
		t.Mustf(be.ErrorIs(err, form.ErrMalformed), "content type %q", ct)
		t.Mustf(be.ErrorIs(err, cause), "content type %q", ct)
		t.Must(be.NilPtr(v))
	}
}

func TestReadNilWriterAndBody(gt *testing.T) {
	t := devtest.ParallelT(gt)
	r := request(t, form.URLEncoded, strings.NewReader("a=12345"))
	_, err := form.Read(nil, r, form.Limits{Body: 4})
	_, ok := errors.AsType[*http.MaxBytesError](err)
	t.Must(be.True(ok))

	r = request(t, form.URLEncoded, nil)
	r.Body = nil
	v, err := form.Read(nil, r, form.Limits{})
	t.Must(be.NoError(err))
	t.Must(be.Equal(0, v.Len()))
}

// Errors may be logged: they must not repeat names, values or headers.
func TestReadErrorsDoNotQuoteInput(gt *testing.T) {
	t := devtest.ParallelT(gt)
	const marker = "s3cr3tmarker"
	cases := []struct{ contentType, body string }{
		{form.URLEncoded, marker + "=" + marker + "%zz"},
		{form.URLEncoded, marker + "=" + marker + ";"},
		{form.URLEncoded, "=" + marker},
		{multipartType, multipartBody(part(marker, `Content-Disposition: form-data; name="`+marker+`"; filename="`+marker+`"`))},
		{multipartType, multipartBody(part(marker, marker+" without a colon"))},
		{multipartType, multipartBody(part(marker, `Content-Disposition: `+marker+`; name="a"`))},
		{"text/" + marker, "a=1"},
	}
	for _, c := range cases {
		_, err := readString(t, c.contentType, c.body, form.Limits{Field: 4})
		t.Must(be.Error(err))
		t.Mustf(be.False(strings.Contains(err.Error(), marker)), "error %q", err)
	}
}

func TestValuesAccessors(gt *testing.T) {
	t := devtest.ParallelT(gt)
	v, err := readString(t, form.URLEncoded, "b=2&a=1&a=&secret=hunter2&secret=again&on", form.Limits{})
	t.Must(be.NoError(err))

	t.Must(be.DeepEqual([]string{"a", "b", "on", "secret"}, v.Names()))
	t.Must(be.Equal(6, v.Len()))

	t.Must(be.DeepEqual([]byte("1"), v.Bytes("a")))
	t.Must(be.Equal("1", v.String("a")))
	t.Must(be.DeepEqual([]string{"1", ""}, v.Strings("a")))
	t.Must(be.True(v.Has("on")))
	t.Must(be.NonNil(v.Bytes("on")))
	t.Must(be.Equal(0, len(v.Bytes("on"))))

	t.Must(be.False(v.Has("missing")))
	t.Must(be.Nil(v.Bytes("missing")))
	t.Must(be.Equal("", v.String("missing")))
	t.Must(be.Nil(v.Strings("missing")))
	t.Must(be.Nil(v.Take("missing")))

	first := v.Take("secret")
	t.Must(be.Equal("hunter2", string(first)))
	t.Must(be.Equal(len(first), cap(first)))
	t.Must(be.Equal("again", v.String("secret")))
	second := v.Take("secret")
	t.Must(be.Equal("again", string(second)))
	t.Must(be.False(v.Has("secret")))
	t.Must(be.Equal(4, v.Len()))

	// Taken values belong to the caller: Zero does not reach them.
	v.Zero()
	t.Must(be.Equal("hunter2", string(first)))
	clear(first)
	clear(second)
}

func TestValuesZero(gt *testing.T) {
	t := devtest.ParallelT(gt)
	v, err := readString(t, multipartType, multipartBody(field("pass", "hunter2"), field("pass", "again"), field("note", "x")), form.Limits{})
	t.Must(be.NoError(err))
	pass := v.Bytes("pass")
	note := v.Bytes("note")
	all := v.Strings("pass") // copies, unaffected

	v.Zero()
	t.Must(be.DeepEqual([]byte{0, 0, 0, 0, 0, 0, 0}, pass))
	t.Must(be.DeepEqual([]byte{0}, note))
	t.Must(be.DeepEqual([]string{"hunter2", "again"}, all))
	t.Must(be.False(v.Has("pass")))
	t.Must(be.Equal(0, v.Len()))
	t.Must(be.Nil(v.Names()))
	v.Zero() // idempotent
}

func TestNilValues(gt *testing.T) {
	t := devtest.ParallelT(gt)
	var v *form.Values
	t.Must(be.Nil(v.Bytes("a")))
	t.Must(be.Equal("", v.String("a")))
	t.Must(be.Nil(v.Strings("a")))
	t.Must(be.False(v.Has("a")))
	t.Must(be.Nil(v.Take("a")))
	t.Must(be.Nil(v.Names()))
	t.Must(be.Equal(0, v.Len()))
	v.Zero()

	var zero form.Values
	t.Must(be.False(zero.Has("a")))
	zero.Zero()
}
