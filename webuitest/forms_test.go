package webuitest_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/ct"
	"github.com/protolambda/chord/ct/cthtml"
	"github.com/protolambda/chord/html/form"
	"github.com/protolambda/chord/html/form/textarea"
	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/webuitest"
)

const formPage = `<form id="f" action="/tasks" method="post">
<input type="hidden" name="csrf" value="tok">
<input name="title" value="Write&#10;docs">
<input type="email" name="mail" value=" a@example.test ">
<input name="disabled" value="x" disabled>
<fieldset disabled><legend><input name="inlegend" value="y"></legend><input name="fs" value="x"></fieldset>
<input type="checkbox" name="done" checked>
<input type="checkbox" name="flag" value="1">
<input type="radio" name="prio" value="low"><input type="radio" name="prio" value="high" checked>
<select name="tag"><option>none</option><option value="w" selected>work</option></select>
<select name="first"><option disabled>a</option><option>  b  c </option></select>
<select name="last"><option selected>a</option><option selected>b</option></select>
<select name="multi" multiple><option selected>x</option><option selected disabled>y</option><optgroup disabled><option selected>g</option></optgroup><option selected value="z">Z</option></select>
<select name="none" multiple><option>q</option></select>
<select name="sized" size="3"><option>q</option></select>
<textarea name="body">
line1
line2</textarea>
<input type="file" name="upload">
<input type="range" name="vol" min="0" max="10">
<input type="color" name="c" value="#ABCDEF">
<input type="hidden" name="_charset_">
<input name="q" dirname="q.dir" value="x">
<input name="" value="ignored">
<datalist><option value="d"></option><input name="inlist" value="no"></datalist>
<output name="out">o</output>
<button name="action" value="save">Save</button>
<button id="del" name="action" value="delete" formaction="/tasks/delete" formenctype="multipart/form-data">Delete</button>
<input type="image" name="img" src="/x.png" alt="go">
<button id="plain" type="button" name="b" value="x">no</button>
<button id="off" name="off" disabled>off</button>
<button type="reset" name="r">r</button>
</form>
<input form="f" name="outside" value="o">
<form id="g" action="/search?x=1#top"><input name="q" value="a b"><button id="find">Find</button></form>
<form id="d" method="dialog"><button>Close</button></form>
<form id="e" method="post" enctype="text/plain"><input name="a" value="1"><textarea name="b">x
y</textarea></form>
<form id="n"></form>`

func formFields(parts []webuitest.Part) [][2]string {
	var out [][2]string
	for _, p := range parts {
		v := p.Value
		if p.File {
			v = "file:" + p.Filename + ":" + p.ContentType
		}
		out = append(out, [2]string{p.Name, v})
	}
	return out
}

func TestFormData(gt *testing.T) {
	t := devtest.ParallelT(gt)
	page := cthtml.PageString(htmlPage(formPage))
	f := page.Find(ct.ID("f"))
	base := [][2]string{
		{"csrf", "tok"},
		{"title", "Writedocs"},
		{"mail", "a@example.test"},
		{"inlegend", "y"},
		{"done", "on"},
		{"prio", "high"},
		{"tag", "w"},
		{"first", "b c"},
		{"last", "b"},
		{"multi", "x"},
		{"multi", "z"},
		{"body", "line1\r\nline2"},
		{"upload", "file::application/octet-stream"},
		{"vol", "5"},
		{"c", "#abcdef"},
		{"_charset_", "UTF-8"},
		{"q", "x"},
		{"q.dir", "ltr"},
	}
	t.Run("no submitter", func(t devtest.T) {
		s, err := webuitest.FormData(t.Context(), page, f, nil)
		t.Must(be.NoError(err))
		t.Must(be.Equal("POST", s.Method))
		t.Must(be.Equal("/tasks", s.Action))
		t.Must(be.Equal(webuitest.EnctypeURLEncoded, s.Enctype))
		t.Must(be.DeepEqual(append(base, [2]string{"outside", "o"}), formFields(s.Fields)))
		t.Must(be.True(s.HasControl("flag")))
		t.Must(be.True(s.HasControl("disabled")))
		t.Must(be.False(s.HasControl("other")))
		t.Must(be.DeepEqual([]string{"x", "z"}, s.Values()["multi"]))
		t.Must(be.Len(s.Values()["upload"], 0))
	})
	t.Run("submitter", func(t devtest.T) {
		s, err := webuitest.FormData(t.Context(), page, f, page.Find(ct.Tag("button"), ct.Text("Save")))
		t.Must(be.NoError(err))
		want := append(append([][2]string{}, base...), [2]string{"action", "save"}, [2]string{"outside", "o"})
		t.Must(be.DeepEqual(want, formFields(s.Fields)))
	})
	t.Run("submitter overrides", func(t devtest.T) {
		s, err := webuitest.FormData(t.Context(), page, f, page.Find(ct.ID("del")))
		t.Must(be.NoError(err))
		t.Must(be.Equal("/tasks/delete", s.Action))
		t.Must(be.Equal("POST", s.Method))
		t.Must(be.Equal(webuitest.EnctypeMultipart, s.Enctype))
		t.Must(be.Equal("multipart/form-data", s.Body().ContentType()[:19]))
	})
	t.Run("image submitter", func(t devtest.T) {
		s, err := webuitest.FormData(t.Context(), page, f, page.Find(ct.Tag("input"), ct.Attr("type", "image")))
		t.Must(be.NoError(err))
		got := formFields(s.Fields)
		t.Must(be.DeepEqual([][2]string{{"img.x", "0"}, {"img.y", "0"}, {"outside", "o"}}, got[len(got)-3:]))
	})
	t.Run("get", func(t devtest.T) {
		s, err := webuitest.FormData(t.Context(), page, page.Find(ct.ID("g")), page.Find(ct.ID("find")))
		t.Must(be.NoError(err))
		t.Must(be.Equal("GET", s.Method))
		req := s.Request(t)
		t.Must(be.Equal("/search", req.URL.Path))
		t.Must(be.Equal("q=a+b", req.URL.RawQuery))
		t.Must(be.Equal(http.MethodGet, req.Method))
	})
	t.Run("text plain", func(t devtest.T) {
		s, err := webuitest.FormData(t.Context(), page, page.Find(ct.ID("e")), nil)
		t.Must(be.NoError(err))
		s.Action = "/notes"
		req := s.Request(t)
		body, err := io.ReadAll(req.Body)
		t.Must(be.NoError(err))
		t.Must(be.Equal("a=1\r\nb=x\r\ny\r\n", string(body)))
		t.Must(be.Equal("text/plain", req.Header.Get("Content-Type")))
	})
	t.Run("post request", func(t devtest.T) {
		s, err := webuitest.FormData(t.Context(), page, f, nil)
		t.Must(be.NoError(err))
		s.Set("title", "New title")
		s.Set("flag", "1")
		s.Set("multi")
		s.SetFile("upload", "notes.txt", "text/plain", []byte("hello"))
		req := s.Request(t)
		t.Must(be.NoError(req.ParseForm()))
		// An urlencoded body sends the name of a file, not its content.
		t.Must(be.Equal("notes.txt", req.PostFormValue("upload")))
		t.Must(be.Equal("New title", req.PostFormValue("title")))
		t.Must(be.Equal("1", req.PostFormValue("flag")))
		t.Must(be.Len(req.PostForm["multi"], 0))
		t.Must(be.Equal("tok", req.PostFormValue("csrf")))
	})
	t.Run("set keeps the position", func(t devtest.T) {
		s, err := webuitest.FormData(t.Context(), page, page.Find(ct.ID("g")), nil)
		t.Must(be.NoError(err))
		s.Set("x", "1")
		s.Set("q", "b\nc")
		t.Must(be.DeepEqual([][2]string{{"q", "b\r\nc"}, {"x", "1"}}, formFields(s.Fields)))
	})
	t.Run("not sendable", func(t devtest.T) {
		s, err := webuitest.FormData(t.Context(), page, page.Find(ct.ID("d")), nil)
		t.Must(be.NoError(err))
		t.Must(be.Equal("DIALOG", s.Method))
		r := newRecordT(t.Context())
		t.Must(be.True(s.Request(r) == nil))
		t.Must(be.Substring(r.output(), "a dialog form sends no request"))
		s, err = webuitest.FormData(t.Context(), page, page.Find(ct.ID("n")), nil)
		t.Must(be.NoError(err))
		r = newRecordT(t.Context())
		t.Must(be.True(s.Request(r) == nil))
		t.Must(be.Substring(r.output(), "the form has no action; set Submission.Action"))
	})
	t.Run("errors", func(t devtest.T) {
		cases := []struct {
			name      string
			form, sub *ct.Selection
			want      string
		}{
			{"two forms", page.Find(ct.Tag("form"), ct.Attr("method", "post")), nil, `matches 2 elements, want one form`},
			{"no form", page.Find(ct.ID("absent")), nil, "matches 0 elements"},
			{"not a form", page.Find(ct.ID("del")), nil, "is a button, not a form"},
			{"not a submit button", f, page.Find(ct.ID("plain")), "is not a submit button"},
			{"other form", f, page.Find(ct.ID("find")), "does not belong to the form"},
			{"disabled", f, page.Find(ct.ID("off")), "is disabled"},
		}
		for _, c := range cases {
			_, err := webuitest.FormData(t.Context(), page, c.form, c.sub)
			t.Mustf(be.ErrorIs(err, webuitest.ErrFormData), "case %s", c.name)
			t.Must(be.ErrorContains(err, c.want))
		}
		other := cthtml.PageString(htmlPage(`<form id="x"></form>`))
		_, err := webuitest.FormData(t.Context(), page, other.Find(ct.ID("x")), nil)
		t.Must(be.ErrorContains(err, "is not part of the page"))
		broken := ct.View(elem.Fn(func(context.Context) (elem.Node, error) { return nil, errors.New("boom") }))
		_, err = webuitest.FormData(t.Context(), broken, broken.Find(ct.Tag("form")), nil)
		t.Must(be.ErrorIs(err, ct.ErrLoad))
	})
	t.Run("view textarea", func(t devtest.T) {
		view := form.Form(form.Action("/n"), form.Method(form.MethodPost))(
			textarea.Textarea(textarea.Name("text"))(elem.Text("\nhello")),
			textarea.Textarea(textarea.Name("two"))(elem.Text("\n\nworld")),
		)
		page := ct.View(view)
		s, err := webuitest.FormData(t.Context(), page, page.Find(ct.Tag("form")), nil)
		t.Must(be.NoError(err))
		t.Must(be.Equal(url.Values{"text": {"hello"}, "two": {"\r\nworld"}}.Encode(), s.Values().Encode()))
	})
}

func TestFormDataValues(gt *testing.T) {
	t := devtest.ParallelT(gt)
	page := cthtml.PageString(htmlPage(`<form id="f">
<input type="range" name="r1" min="10" max="20" value="15.5">
<input type="range" name="r2" min="0" max="10" value="99">
<input type="range" name="r3" min="5" max="1">
<input type="range" name="r4" min="x">
<input type="color" name="c1" value="red">
<input type="url" name="u" value=" https://example.test/ ">
<input type="password" name="p" value="a&#13;b">
<input type="number" name="n" value=" 7 ">
<input type="checkbox" name="cb" value="" checked>
<input type="hidden" name="h" value="a&#10;b">
<input type="radio" checked value="nameless">
<input type="file">
<select><option selected>nameless</option></select>
<textarea>nameless</textarea>
<button name="">x</button>
<input type="image" src="/i.png" alt="i" id="img">
<input type="submit" name="s" value="Go" id="s">
</form>`))
	s, err := webuitest.FormData(t.Context(), page, page.Find(ct.ID("f")), page.Find(ct.ID("img")))
	t.Must(be.NoError(err))
	t.Must(be.DeepEqual([][2]string{
		{"r1", "15.5"}, {"r2", "10"}, {"r3", "5"}, {"r4", "50"},
		{"c1", "#000000"}, {"u", "https://example.test/"}, {"p", "ab"}, {"n", " 7 "},
		{"cb", ""}, {"h", "a\r\nb"}, {"x", "0"}, {"y", "0"},
	}, formFields(s.Fields)))
	s, err = webuitest.FormData(t.Context(), page, page.Find(ct.ID("f")), page.Find(ct.ID("s")))
	t.Must(be.NoError(err))
	t.Must(be.DeepEqual([]string{"Go"}, s.Values()["s"]))
	t.Must(be.Len(s.Values()["x"], 0))
}
