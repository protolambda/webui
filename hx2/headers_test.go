package hx2_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/protolambda/webui/hx2"
)

func TestParseRequestHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("HX-Request", "true")
	h.Set("HX-Boosted", "true")
	h.Set("HX-History-Restore-Request", "true")
	h.Set("HX-Current-URL", "https://example.com/items?page=2")
	h.Set("HX-Prompt", "yes")
	h.Set("HX-Target", "list")
	h.Set("HX-Trigger", "load-more")
	h.Set("HX-Trigger-Name", "more")
	want := hx2.RequestHeaders{
		Request:               true,
		Boosted:               true,
		HistoryRestoreRequest: true,
		CurrentURL:            "https://example.com/items?page=2",
		Prompt:                "yes",
		Target:                "list",
		Trigger:               "load-more",
		TriggerName:           "more",
	}
	if got := hx2.ParseRequestHeaders(h); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	if got := hx2.ParseRequestHeaders(http.Header{}); got != (hx2.RequestHeaders{}) {
		t.Fatalf("absent headers: got %+v", got)
	}
	h = http.Header{}
	h.Set("HX-Request", "false")
	h.Set("HX-Boosted", "1")
	if got := hx2.ParseRequestHeaders(h); got.Request || got.Boosted {
		t.Fatalf("only \"true\" is true: got %+v", got)
	}
}

// htmx sets header values with XMLHttpRequest, which sends each character up
// to U+00FF as one byte (Latin-1). Values with other characters, or with a
// line break, fail there; htmx then sends them encoded with
// encodeURIComponent, plus the header name-URI-AutoEncoded: true.
func TestParseRequestHeadersDecodesBrowserEncodings(t *testing.T) {
	h := http.Header{}
	// "日本 + ü/?" encoded by encodeURIComponent: '+' is %2B, a space %20.
	h.Set("HX-Prompt", "%E6%97%A5%E6%9C%AC%20%2B%20%C3%BC%2F%3F")
	h.Set("HX-Prompt-URI-AutoEncoded", "true")
	h.Set("HX-Target", "caf\xe9")            // id="café", as Latin-1
	h.Set("HX-Trigger", "na\xefve-\xa7\xff") // id="naïve-§ÿ"
	h.Set("HX-Trigger-Name", "a%0Ab")        // a line break, encoded
	h.Set("HX-Trigger-Name-URI-AutoEncoded", "true")
	h.Set("HX-Current-URL", "https://example.com/caf%C3%A9?q=a+b")
	want := hx2.RequestHeaders{
		CurrentURL:  "https://example.com/caf%C3%A9?q=a+b",
		Prompt:      "日本 + ü/?",
		Target:      "café",
		Trigger:     "naïve-§ÿ",
		TriggerName: "a\nb",
	}
	if got := hx2.ParseRequestHeaders(h); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	// Without the marker, a value is not percent-decoded; UTF-8 from other
	// clients stays as it is; a malformed encoded value is kept as sent.
	h = http.Header{}
	h.Set("HX-Prompt", "100%25")
	h.Set("HX-Target", "日本")
	h.Set("HX-Trigger", "50%")
	h.Set("HX-Trigger-URI-AutoEncoded", "true")
	h.Set("HX-Trigger-Name", "a+b")
	h.Set("HX-Trigger-Name-URI-AutoEncoded", "true")
	want = hx2.RequestHeaders{Prompt: "100%25", Target: "日本", Trigger: "50%", TriggerName: "a+b"}
	if got := hx2.ParseRequestHeaders(h); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func expectHeader(t *testing.T, h http.Header, name, want string) {
	t.Helper()
	if got := h.Values(name); len(got) != 1 || got[0] != want {
		t.Fatalf("%s: got %q, want %q", name, got, want)
	}
}

func TestSetPlainResponseHeaders(t *testing.T) {
	h := http.Header{}
	hx2.SetRedirect(h, "/login")
	hx2.SetRefresh(h)
	hx2.SetPushURL(h, "/items?page=2")
	hx2.SetReplaceURL(h, "false")
	hx2.SetReswap(h, "outerHTML show:top")
	hx2.SetRetarget(h, "#errors")
	hx2.SetReselect(h, "#content")
	expectHeader(t, h, "HX-Redirect", "/login")
	expectHeader(t, h, "HX-Refresh", "true")
	expectHeader(t, h, "HX-Push-Url", "/items?page=2")
	expectHeader(t, h, "HX-Replace-Url", "false")
	expectHeader(t, h, "HX-Reswap", "outerHTML show:top")
	expectHeader(t, h, "HX-Retarget", "#errors")
	expectHeader(t, h, "HX-Reselect", "#content")

	hx2.SetRedirect(h, "/other")
	expectHeader(t, h, "HX-Redirect", "/other")
}

func TestSetLocation(t *testing.T) {
	tests := map[string]struct {
		loc  hx2.Location
		want string
	}{
		"path only":       {hx2.Location{Path: "/items"}, "/items"},
		"path like json":  {hx2.Location{Path: "{x}"}, `{"path":"{x}"}`},
		"target and swap": {hx2.Location{Path: "/items", Target: "#main", Swap: "innerHTML"}, `{"path":"/items","target":"#main","swap":"innerHTML"}`},
		"values": {
			hx2.Location{Path: "/search", Values: map[string]any{"q": "<b>", "n": 2}, Headers: map[string]string{"X-A": "b"}, Push: "false"},
			`{"path":"/search","values":{"n":2,"q":"\u003cb\u003e"},"headers":{"X-A":"b"},"push":"false"}`,
		},
		"non-ascii": {hx2.Location{Path: "/caf%C3%A9", Select: "#résumé 😀"}, `{"path":"/caf%C3%A9","select":"#r\u00e9sum\u00e9 \ud83d\ude00"}`},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			h := http.Header{}
			if err := hx2.SetLocation(h, tc.loc); err != nil {
				t.Fatalf("set: %v", err)
			}
			expectHeader(t, h, "HX-Location", tc.want)
		})
	}

	for name, loc := range map[string]hx2.Location{
		"no path":   {Target: "#main"},
		"bad value": {Path: "/x", Values: map[string]any{"f": func() {}}},
		"empty":     {},
	} {
		t.Run("invalid "+name, func(t *testing.T) {
			h := http.Header{}
			h.Set("HX-Location", "/before")
			if err := hx2.SetLocation(h, loc); !errors.Is(err, hx2.ErrInvalidHeader) {
				t.Fatalf("expected ErrInvalidHeader, got %v", err)
			}
			expectHeader(t, h, "HX-Location", "/before")
		})
	}
}

func TestSetTrigger(t *testing.T) {
	type setter func(http.Header, ...hx2.Event) error
	setters := map[string]setter{
		"HX-Trigger":              hx2.SetTrigger,
		"HX-Trigger-After-Swap":   hx2.SetTriggerAfterSwap,
		"HX-Trigger-After-Settle": hx2.SetTriggerAfterSettle,
	}
	tests := map[string]struct {
		events []hx2.Event
		want   string
	}{
		"one":        {[]hx2.Event{{Name: "itemAdded"}}, "itemAdded"},
		"list":       {[]hx2.Event{{Name: "itemAdded"}, {Name: "htmx:abort"}}, "itemAdded, htmx:abort"},
		"detail":     {[]hx2.Event{{Name: "b"}, {Name: "a", Detail: "Saved"}}, `{"b":null,"a":"Saved"}`},
		"object":     {[]hx2.Event{{Name: "notify", Detail: map[string]string{"target": "#bell", "level": "info"}}}, `{"notify":{"level":"info","target":"#bell"}}`},
		"comma name": {[]hx2.Event{{Name: "a,b"}}, `{"a,b":null}`},
		"brace name": {[]hx2.Event{{Name: "{a}"}}, `{"{a}":null}`},
		"space name": {[]hx2.Event{{Name: " a"}}, `{" a":null}`},
		"non-ascii":  {[]hx2.Event{{Name: "é"}, {Name: "msg", Detail: "ok ✓"}}, `{"\u00e9":null,"msg":"ok \u2713"}`},
	}
	for header, set := range setters {
		for name, tc := range tests {
			t.Run(header+" "+name, func(t *testing.T) {
				h := http.Header{}
				if err := set(h, tc.events...); err != nil {
					t.Fatalf("set: %v", err)
				}
				expectHeader(t, h, header, tc.want)
			})
		}
		t.Run(header+" none", func(t *testing.T) {
			h := http.Header{}
			h.Set(header, "old")
			if err := set(h); err != nil {
				t.Fatalf("set: %v", err)
			}
			if got := h.Values(header); len(got) != 0 {
				t.Fatalf("expected no header, got %q", got)
			}
		})
		for name, events := range map[string][]hx2.Event{
			"empty name": {{Name: "a"}, {Name: ""}},
			"repeated":   {{Name: "a"}, {Name: "a", Detail: 1}},
			"bad detail": {{Name: "a", Detail: make(chan int)}},
		} {
			t.Run(header+" invalid "+name, func(t *testing.T) {
				h := http.Header{}
				h.Set(header, "old")
				if err := set(h, events...); !errors.Is(err, hx2.ErrInvalidHeader) {
					t.Fatalf("expected ErrInvalidHeader, got %v", err)
				}
				expectHeader(t, h, header, "old")
			})
		}
	}
}

func ExampleParseRequestHeaders() {
	r := httptest.NewRequest(http.MethodGet, "/items", nil)
	r.Header.Set("HX-Request", "true")
	r.Header.Set("HX-Target", "item-list")

	hx := hx2.ParseRequestHeaders(r.Header)
	fmt.Println(hx.Request, hx.Boosted, hx.Target)
	// Output: true false item-list
}

func ExampleSetTrigger() {
	h := http.Header{}
	_ = hx2.SetTrigger(h, hx2.Event{Name: "itemAdded"}, hx2.Event{Name: "cartChanged"})
	fmt.Println(h.Get("HX-Trigger"))

	_ = hx2.SetTrigger(h, hx2.Event{Name: "showMessage", Detail: map[string]string{"level": "info", "text": "Saved"}})
	fmt.Println(h.Get("HX-Trigger"))
	// Output:
	// itemAdded, cartChanged
	// {"showMessage":{"level":"info","text":"Saved"}}
}

func ExampleSetLocation() {
	h := http.Header{}
	_ = hx2.SetLocation(h, hx2.Location{Path: "/items/42"})
	fmt.Println(h.Get("HX-Location"))

	_ = hx2.SetLocation(h, hx2.Location{Path: "/items/42", Target: "#detail", Swap: "outerHTML"})
	fmt.Println(h.Get("HX-Location"))
	// Output:
	// /items/42
	// {"path":"/items/42","target":"#detail","swap":"outerHTML"}
}
