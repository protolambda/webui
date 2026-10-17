package form_test

import (
	"bytes"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/protolambda/webui/form"
)

// A handler reads a urlencoded form (the default of HTML forms and htmx),
// uses its public fields as strings, and zeroes the form when done.
func ExampleRead() {
	r := httptest.NewRequest(http.MethodPost, "/notes", strings.NewReader("title=Weekly+notes&tag=work&tag=urgent&pinned=on"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	v, err := form.Read(httptest.NewRecorder(), r, form.Limits{})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer v.Zero()
	fmt.Println(v.String("title"))
	fmt.Println(v.Strings("tag"))
	fmt.Println(v.Has("pinned"), v.Has("archived"))
	// Output:
	// Weekly notes
	// [work urgent]
	// true false
}

// Multipart forms are read the same way.
func ExampleRead_multipart() {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("name", "Tasks")
	_ = mw.WriteField("description", "Things to do\r\nthis week")
	_ = mw.Close()
	r := httptest.NewRequest(http.MethodPost, "/projects", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())

	v, err := form.Read(httptest.NewRecorder(), r, form.Limits{})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer v.Zero()
	fmt.Printf("%q\n", v.Names())
	fmt.Printf("%q\n", v.String("description"))
	// Output:
	// ["description" "name"]
	// "Things to do\r\nthis week"
}

// A secret field is taken out of the form as an owned byte slice, which the
// caller clears after use, instead of being copied into a string.
func ExampleValues_Take() {
	r := httptest.NewRequest(http.MethodPost, "/account/password", strings.NewReader("account=ada&password=correct+horse"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	v, err := form.Read(httptest.NewRecorder(), r, form.Limits{})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer v.Zero()

	password := v.Take("password")
	defer clear(password)
	fmt.Println(len(password), v.Has("password"))
	// Output:
	// 13 false
}

// Printing or logging a form shows only its number of values, never a name
// or a value, whatever the verb.
func ExampleValues_Format() {
	r := httptest.NewRequest(http.MethodPost, "/account/password", strings.NewReader("account=ada&password=correct+horse"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	v, err := form.Read(httptest.NewRecorder(), r, form.Limits{})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer v.Zero()

	fmt.Println(v)
	fmt.Printf("%#v %s %x\n", v, v, v)
	// Output:
	// form.Values(len=2)
	// form.Values(len=2) form.Values(len=2) form.Values(len=2)
}

// Errors classify the failure: a body over its limit becomes a 413, any
// other failure a 400.
func ExampleRead_limits() {
	status := func(err error) int {
		switch {
		case err == nil:
			return http.StatusOK
		case errors.Is(err, form.ErrTooLarge):
			return http.StatusRequestEntityTooLarge
		case errors.Is(err, form.ErrUnsupportedType):
			return http.StatusUnsupportedMediaType
		default:
			return http.StatusBadRequest
		}
	}
	for _, body := range []string{"note=short", "note=" + strings.Repeat("x", 100), "note=%zz", "a&b&c&d"} {
		r := httptest.NewRequest(http.MethodPost, "/notes", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		v, err := form.Read(httptest.NewRecorder(), r, form.Limits{Body: 64, Field: 32, Fields: 3})
		v.Zero()
		_, bodyLimit := errors.AsType[*http.MaxBytesError](err)
		fmt.Println(status(err), bodyLimit, err)
	}
	// Output:
	// 200 false <nil>
	// 413 true form: form too large: body over 64 bytes: http: request body too large
	// 400 false form: malformed form: bad percent escape
	// 400 false form: too many fields: more than 3
}

// Limits loaded from configuration are checked once at startup: a
// negative limit would otherwise make every form read panic.
func ExampleLimits_Validate() {
	for _, l := range []form.Limits{{}, {Body: 64 << 10, Fields: 50}, {Fields: -1}} {
		fmt.Println(l.Validate())
	}
	// Output:
	// <nil>
	// <nil>
	// form: invalid limits: negative Fields
}
