package urlpath_test

import (
	"strings"
	"testing"

	"github.com/protolambda/devtest"
	"github.com/protolambda/mustbe/be"

	"github.com/protolambda/webui/internal/urlpath"
)

func TestLocal(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, s := range []string{
		"/", "/live", "/a/b?c=d&e=%20#f", "/%E6%97%A5", "/tasks?next=%2F%2Fother.example",
		"/a:b", "/?", "/#", "/" + strings.Repeat("a", 4096),
	} {
		t.Must(be.Annotate(be.True(urlpath.Local(s)), "%q", s))
		t.Must(be.Annotate(be.True(urlpath.Printable(s)), "%q", s))
	}
	for _, s := range []string{
		"", "live", "./live", "?q", "#f",
		"//other.example", "///other.example", "/\\other.example", "\\/other.example",
		"/\t/other.example", "/\n/other.example", "/\r/other.example", " /x", "/x ", "/a b",
		"/a\x00", "/a\x7f", "/日本", "/\xff", "/x?y=日",
		"/a%zz", "/a%2",
		"https://other.example/x", "javascript:alert(1)", "data:,x", "mailto:a@b.example",
	} {
		t.Must(be.Annotate(be.False(urlpath.Local(s)), "%q", s))
	}
}

func TestPrintable(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for _, s := range []string{"x", "https://other.example/a?b#c", "data:,", "~!@#$%^&*()_+{}|:\"<>?`-=[];',./"} {
		t.Must(be.Annotate(be.True(urlpath.Printable(s)), "%q", s))
	}
	for _, s := range []string{"", " ", "a b", "a\\b", "a\tb", "a\nb", "a\x00", "a\x7f", "a\x80", "é"} {
		t.Must(be.Annotate(be.False(urlpath.Printable(s)), "%q", s))
	}
}

// Every byte value outside printable ASCII, and the backslash, makes a
// path not local, wherever it is.
func TestLocalRejectsEveryBadByte(gt *testing.T) {
	t := devtest.ParallelT(gt)
	for b := 0; b < 256; b++ {
		c := byte(b)
		bad := c <= ' ' || c >= 0x7f || c == '\\'
		for _, s := range []string{"/" + string(c), "/a" + string(c) + "b", "/a?" + string(c), "/a#" + string(c)} {
			if bad {
				t.Must(be.Annotate(be.False(urlpath.Local(s)), "byte %#x in %q", c, s))
			}
		}
	}
}
