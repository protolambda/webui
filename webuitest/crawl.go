package webuitest

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/protolambda/chord/ct"
	"github.com/protolambda/mustbe/assertion"
)

// ErrCrawl reports the problems that a [Crawl] found.
var ErrCrawl = errors.New("webuitest: crawl found problems")

// RouteChecker reports whether an application routes a request with the
// method and path (with an optional query) to a handler of its own.
type RouteChecker func(method, path string) bool

// MuxRoutes returns the [RouteChecker] of mux: a request is routed when
// the mux matches it to a pattern (on [BaseURL]'s host), the pattern is
// not the catch-all "/" (unless the path is "/"), and the handler is not
// a redirect that the mux generates (to add a trailing slash or clean the
// path), which a page should not link to.
func MuxRoutes(mux *http.ServeMux) RouteChecker {
	redirect := reflect.TypeOf(http.RedirectHandler("/", http.StatusMovedPermanently))
	return func(method, path string) bool {
		u, err := url.Parse(path)
		if err != nil {
			return false
		}
		req := &http.Request{
			Method: method,
			URL:    &url.URL{Path: u.Path, RawPath: u.RawPath, RawQuery: u.RawQuery},
			Host:   defaultBase.Host,
			Header: make(http.Header),
		}
		h, pattern := mux.Handler(req)
		if pattern == "" || reflect.TypeOf(h) == redirect {
			return false
		}
		p := pattern
		if i := strings.IndexByte(p, ' '); i >= 0 {
			p = strings.TrimSpace(p[i+1:])
		}
		if i := strings.IndexByte(p, '/'); i >= 0 {
			p = p[i:]
		}
		return p != "/" || u.Path == "/"
	}
}

// TargetKind classifies the targets that [Crawl] finds.
type TargetKind uint8

const (
	// TargetLink is the href of a link (a, area).
	TargetLink TargetKind = iota + 1
	// TargetForm is the action of a form or the formaction of a button.
	TargetForm
	// TargetHTMX is the URL of an hx-get, hx-post, hx-put, hx-patch or
	// hx-delete attribute.
	TargetHTMX
	// TargetStream is the URL of an sse-connect attribute.
	TargetStream
	// TargetAsset is a stylesheet, icon, preload, script or image.
	TargetAsset
	// TargetRedirect is the Location of a redirect.
	TargetRedirect
)

// String returns the lowercase name of the kind, such as "link", or
// "TargetKind(n)" for other values.
func (k TargetKind) String() string {
	switch k {
	case TargetLink:
		return "link"
	case TargetForm:
		return "form"
	case TargetHTMX:
		return "htmx"
	case TargetStream:
		return "stream"
	case TargetAsset:
		return "asset"
	case TargetRedirect:
		return "redirect"
	}
	return fmt.Sprintf("TargetKind(%d)", uint8(k))
}

// Target is a request that a crawled page can make.
type Target struct {
	Kind   TargetKind
	Method string
	// URL is the path and query of the target, without the fragment.
	URL string
	// Page is the URL of the page with the target.
	Page string
}

// maxRedirects is the longest redirect chain that [Crawl] follows, the
// limit of browsers.
const maxRedirects = 20

// CrawlOptions configure [Crawl].
type CrawlOptions struct {
	// Start are the pages the crawl starts from; empty means "/".
	Start []string
	// Routes, when set, must route every target that the crawl finds.
	Routes RouteChecker
	// Contract checks every crawled page; nil means [PageContract] with
	// [DefaultShell]() and [Rules]().
	Contract Contract
	// ContractFor, when set, picks the contract of each crawled page by
	// the page's URL, for applications whose pages differ in what they
	// load or allow, such as a page with an asset set of its own. A nil
	// result means Contract. The URL is absolute on [BaseURL], with the
	// path and query of the page (after redirects) and no fragment; each
	// call gets a fresh copy, which ContractFor may keep or change.
	ContractFor func(u *url.URL) Contract
	// Skip, when set, excludes targets by path (for example "/logout" or
	// a prefix of downloads) from route checks and from crawling.
	Skip func(path string) bool
	// MaxPages bounds the pages crawled (answers 200); zero means 500. A
	// crawl that reaches the bound reports it. Redirects do not count as
	// pages: a chain of them is bounded by itself, see [Crawl].
	MaxPages int
	// Assets fetches every stylesheet, icon, preload and script once and
	// checks that it is served with status 200, nosniff and the content
	// type of its kind, and that its bytes match the integrity value of
	// the page that loads it.
	Assets bool
	// Fragments fetches the target of every hx-get once as an htmx request
	// ([HTMX]) and checks the fragment with FragmentContract (or
	// [FragmentContract] with [Rules]()).
	Fragments bool
	// FragmentContract checks the fragments of Fragments; nil means
	// [FragmentContract] with [Rules]().
	FragmentContract Contract
	// FragmentContractFor, when set, picks the contract of each fragment
	// of Fragments by the fragment's URL (the hx-get target, absolute on
	// [BaseURL], a fresh copy for each call) as ContractFor does for
	// pages. A nil result means FragmentContract.
	FragmentContractFor func(u *url.URL) Contract
}

// pick returns the contract of a page key: the one that pickFor returns
// for its URL, when pickFor is set and returns one, or def.
func pick(key string, def Contract, pickFor func(u *url.URL) Contract) Contract {
	if pickFor == nil {
		return def
	}
	if c := pickFor(mustResolve(key)); c != nil {
		return c
	}
	return def
}

// CrawlReport is the result of a [Crawl]: the pages crawled, the targets
// found, and every problem. It is an assertion that the crawl found no
// problem.
type CrawlReport struct {
	// Start are the start pages.
	Start []string
	// Pages are the URLs of the pages crawled, in crawl order.
	Pages []string
	// Targets are the targets found on the pages, in order.
	Targets []Target
	// Problems describe every problem found.
	Problems []string
}

var _ assertion.Assertion = (*CrawlReport)(nil)

// String describes the assertion.
func (r *CrawlReport) String() string {
	return fmt.Sprintf("crawl(%s)", strings.Join(r.Start, ", "))
}

// Check reports every problem of the crawl at once, with [ErrCrawl].
func (r *CrawlReport) Check(context.Context) error {
	if len(r.Problems) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %d problems in %d pages:\n  %s", ErrCrawl, len(r.Problems), len(r.Pages),
		strings.Join(r.Problems, "\n  "))
}

// Covers asserts that the crawl reached each of the pages (paths with an
// optional query).
func (r *CrawlReport) Covers(pages ...string) assertion.Assertion {
	return constantCheck(fmt.Sprintf("covers(%s)", strings.Join(pages, ", ")), func() error {
		var missing []string
		for _, p := range pages {
			if !contains(r.Pages, p) {
				missing = append(missing, fmt.Sprintf("%q", p))
			}
		}
		if len(missing) > 0 {
			return fmt.Errorf("%w: the crawl did not reach %s", ct.ErrMismatch, strings.Join(missing, ", "))
		}
		return nil
	})
}

// Uses asserts that crawled pages have targets with the method for each
// of urls (paths with an optional query), such as the POST actions an
// application must offer.
func (r *CrawlReport) Uses(method string, urls ...string) assertion.Assertion {
	return constantCheck(fmt.Sprintf("uses(%s %s)", method, strings.Join(urls, ", ")), func() error {
		var missing []string
		for _, u := range urls {
			found := false
			for _, t := range r.Targets {
				if t.Method == method && t.URL == u {
					found = true
					break
				}
			}
			if !found {
				missing = append(missing, fmt.Sprintf("%q", u))
			}
		}
		if len(missing) > 0 {
			return fmt.Errorf("%w: no crawled page has a %s target for %s", ct.ErrMismatch, method, strings.Join(missing, ", "))
		}
		return nil
	})
}

func constantCheck(desc string, check func() error) assertion.Assertion {
	return streamCheck{desc: desc, check: check}
}

// fragmentRef is a link to a fragment of another page.
type fragmentRef struct {
	page, id, from string
}

// assetRef is an asset with the integrity value of the first page that
// loads it.
type assetRef struct {
	kind      string // "stylesheet", "script", "icon" or "preload"
	integrity string
	page      string
}

// Crawl follows the links of an application from the start pages, with
// GET requests sent by s (a [Client] or a [Browser]), and checks every
// page and every target it finds, collecting all problems in the report
// instead of stopping at the first:
//
//   - every page answers 200 with HTML that satisfies its contract, from
//     ContractFor or Contract (redirects are followed when they stay on
//     the site, up to 20 in a row as browsers follow them: a longer
//     chain, or one that comes back to a URL of the chain, is a problem;
//     other statuses are problems);
//   - every link, form action, formaction, htmx request URL, sse-connect
//     URL and asset on the site is routed (with Routes);
//   - every link to a fragment of another crawled page names an element
//     of that page;
//   - with Assets and Fragments, the assets and hx-get fragments, see
//     [CrawlOptions].
//
// Only same-site links (paths, or absolute URLs on [BaseURL]'s host) are
// followed; forms are route-checked, never submitted. Infrastructure
// failures of s fail the test.
func Crawl(t TB, s Sender, o CrawlOptions) *CrawlReport {
	t.Helper()
	start := o.Start
	if len(start) == 0 {
		start = []string{"/"}
	}
	maxPages := o.MaxPages
	if maxPages <= 0 {
		maxPages = 500
	}
	contract := o.Contract
	if contract == nil {
		contract = func(p *ct.Subject) assertion.Assertion { return PageContract(p, DefaultShell()) }
	}
	fragContract := o.FragmentContract
	if fragContract == nil {
		fragContract = func(f *ct.Subject) assertion.Assertion { return FragmentContract(f) }
	}
	r := &CrawlReport{Start: append([]string(nil), start...)}
	problem := func(format string, args ...any) { r.Problems = append(r.Problems, fmt.Sprintf(format, args...)) }

	type queued struct {
		url, from string
		// chain holds the URLs of the redirects that led here, in order.
		chain []string
	}
	var queue []queued
	seen := make(map[string]bool)
	enqueue := func(u, from string, chain []string) {
		if !seen[u] {
			seen[u] = true
			queue = append(queue, queued{u, from, chain})
		}
	}
	for _, p := range start {
		if key, _, ok := siteURL(defaultBase, p); ok {
			enqueue(key, "the start", nil)
		} else {
			problem("start page %q is not a path of the site", p)
		}
	}
	pageIDs := make(map[string]map[string]bool)
	var fragRefs []fragmentRef
	assets := make(map[string]assetRef)
	var assetOrder []string
	fragments := make(map[string]string)
	var fragmentOrder []string

	for len(queue) > 0 {
		q := queue[0]
		queue = queue[1:]
		if len(r.Pages) == maxPages {
			problem("stopped after %d pages; %d more were found", maxPages, len(queue)+1)
			break
		}
		res := s.Send(t, http.MethodGet, q.url, Body{})
		if res == nil {
			return r
		}
		if err := res.Captured().Check(t.Context()); err != nil {
			problem("GET %s (from %s): %v", q.url, q.from, err)
			continue
		}
		code := res.StatusCode()
		if code >= 300 && code < 400 {
			location := res.Headers().Get("Location")
			key, _, ok := siteURL(mustResolve(q.url), location)
			if !ok {
				continue
			}
			r.Targets = append(r.Targets, Target{Kind: TargetRedirect, Method: http.MethodGet, URL: key, Page: q.url})
			if o.Skip == nil || !o.Skip(pathOf(key)) {
				if o.Routes != nil && !o.Routes(http.MethodGet, key) {
					problem("redirect of %s to %s has no route", q.url, key)
				}
				chain := append(append([]string(nil), q.chain...), q.url)
				switch {
				case contains(chain, key):
					problem("GET %s (from %s): the redirects loop back to %s", q.url, q.from, key)
				case len(chain) > maxRedirects:
					problem("GET %s (from %s): more than %d redirects in a row", q.url, q.from, maxRedirects)
				default:
					enqueue(key, q.url, chain)
				}
			}
			continue
		}
		if code != http.StatusOK {
			problem("GET %s (from %s) answered %d", q.url, q.from, code)
			continue
		}
		r.Pages = append(r.Pages, q.url)
		if ctype := mediaEssence(res.Headers().Get("Content-Type")); ctype != "text/html" {
			continue
		}
		page := res.HTML()
		if err := pick(q.url, contract, o.ContractFor)(page).Check(t.Context()); err != nil {
			problem("page %s: %v", q.url, err)
		}
		doc, err := page.Load(t.Context())
		if err != nil {
			continue
		}
		root := doc.Root()
		ids := make(map[string]bool)
		for id := range idIndex(root) {
			ids[id] = true
		}
		pageIDs[q.url] = ids
		base := mustResolve(q.url)
		for _, tg := range pageTargets(root) {
			key, frag, ok := siteURL(base, tg.url)
			if !ok {
				continue
			}
			target := Target{Kind: tg.kind, Method: tg.method, URL: key, Page: q.url}
			r.Targets = append(r.Targets, target)
			if o.Skip != nil && o.Skip(pathOf(key)) {
				continue
			}
			if o.Routes != nil && !o.Routes(tg.method, key) {
				problem("%s %s (%s on %s) has no route", tg.method, key, tg.kind, q.url)
			}
			switch tg.kind {
			case TargetLink:
				if frag != "" && key != q.url {
					fragRefs = append(fragRefs, fragmentRef{page: key, id: frag, from: q.url})
				}
				enqueue(key, q.url, nil)
			case TargetAsset:
				if _, dup := assets[key]; !dup {
					assets[key] = assetRef{kind: tg.assetKind, integrity: tg.integrity, page: q.url}
					assetOrder = append(assetOrder, key)
				}
			case TargetHTMX:
				if tg.method == http.MethodGet {
					if _, dup := fragments[key]; !dup {
						fragments[key] = q.url
						fragmentOrder = append(fragmentOrder, key)
					}
				}
			}
		}
	}
	for _, f := range fragRefs {
		ids, crawled := pageIDs[f.page]
		if crawled && !ids[f.id] {
			problem("link on %s to %s#%s: the page has no element with that id", f.from, f.page, f.id)
		}
	}
	if o.Assets {
		for _, key := range assetOrder {
			if msg := checkAsset(t, s, key, assets[key]); msg != "" {
				problem("%s", msg)
			}
		}
	}
	if o.Fragments {
		for _, key := range fragmentOrder {
			res := s.Send(t, http.MethodGet, key, Body{}, HTMX())
			if res == nil {
				return r
			}
			switch code := res.StatusCode(); code {
			case http.StatusOK:
				if err := pick(key, fragContract, o.FragmentContractFor)(res.HTMLFragment("div")).Check(t.Context()); err != nil {
					problem("fragment %s (hx-get on %s): %v", key, fragments[key], err)
				}
			case http.StatusNoContent:
			default:
				problem("fragment %s (hx-get on %s) answered %d", key, fragments[key], code)
			}
		}
	}
	return r
}

// pageTarget is a target found on a page.
type pageTarget struct {
	kind      TargetKind
	method    string
	url       string
	assetKind string
	integrity string
}

// pageTargets returns the targets of a page, in tree order.
func pageTargets(root ct.Node) []pageTarget {
	idx := indexForms(root)
	var out []pageTarget
	for e := range ct.Elements(root) {
		switch e.Tag() {
		case "a", "area":
			if href, ok := attrOf(e, "href"); ok && !strings.HasPrefix(strings.TrimSpace(href), "#") {
				out = append(out, pageTarget{kind: TargetLink, method: http.MethodGet, url: href})
			}
		case "form":
			if m := formMethod(e); m != "dialog" {
				action, _ := attrOf(e, "action")
				out = append(out, pageTarget{kind: TargetForm, method: strings.ToUpper(m), url: action})
			}
		case "button", "input":
			if !submitButton(e) {
				break
			}
			fa, hasAction := attrOf(e, "formaction")
			fm, hasMethod := attrOf(e, "formmethod")
			f := idx.owner[e]
			if f == nil || (!hasAction && !hasMethod) {
				break
			}
			m := formMethod(f)
			if hasMethod {
				m = normMethod(fm)
			}
			if !hasAction {
				fa, _ = attrOf(f, "action")
			}
			if m != "dialog" {
				out = append(out, pageTarget{kind: TargetForm, method: strings.ToUpper(m), url: fa})
			}
		case "link":
			rel, _ := attrOf(e, "rel")
			kind := ""
			switch {
			case hasFieldFold(rel, "stylesheet"):
				kind = "stylesheet"
			case hasFieldFold(rel, "icon"):
				kind = "icon"
			case hasFieldFold(rel, "preload"), hasFieldFold(rel, "modulepreload"):
				kind = "preload"
			}
			if href, ok := attrOf(e, "href"); ok && kind != "" && parseRef(href).scheme == "" {
				integrity, _ := attrOf(e, "integrity")
				out = append(out, pageTarget{kind: TargetAsset, method: http.MethodGet, url: href, assetKind: kind, integrity: integrity})
			}
		case "script":
			if src, ok := attrOf(e, "src"); ok && src != "" {
				integrity, _ := attrOf(e, "integrity")
				out = append(out, pageTarget{kind: TargetAsset, method: http.MethodGet, url: src, assetKind: "script", integrity: integrity})
			}
		case "img":
			if src, ok := attrOf(e, "src"); ok && parseRef(src).scheme == "" {
				out = append(out, pageTarget{kind: TargetAsset, method: http.MethodGet, url: src, assetKind: "image"})
			}
		}
		for _, name := range htmxRequestAttrs {
			if v, _, ok := hxAttrOf(e, name); ok {
				out = append(out, pageTarget{kind: TargetHTMX, method: strings.ToUpper(strings.TrimPrefix(name, "hx-")), url: v})
			}
		}
		if v, _, ok := hxAttrOf(e, "sse-connect"); ok {
			out = append(out, pageTarget{kind: TargetStream, method: http.MethodGet, url: v})
		}
	}
	return out
}

// mustResolve returns the URL of a page key on the base URL: a new value,
// never the shared base.
func mustResolve(key string) *url.URL {
	u, err := url.Parse(key)
	if err != nil {
		base := *defaultBase
		return &base
	}
	return defaultBase.ResolveReference(u)
}

// siteURL resolves ref against base and returns its path and query (the
// key of a page), its fragment, and whether it is an http(s) URL on the
// site's host. An empty ref is the base itself, as for a form without an
// action.
func siteURL(base *url.URL, ref string) (key, fragment string, ok bool) {
	ref = strings.TrimSpace(ref)
	r, err := url.Parse(ref)
	if err != nil {
		return "", "", false
	}
	u := base.ResolveReference(r)
	if (u.Scheme != "http" && u.Scheme != "https") || !strings.EqualFold(u.Host, defaultBase.Host) {
		return "", "", false
	}
	key = u.EscapedPath()
	if key == "" {
		key = "/"
	}
	if u.RawQuery != "" {
		key += "?" + u.RawQuery
	}
	return key, u.Fragment, true
}

func pathOf(key string) string {
	p, _, _ := strings.Cut(key, "?")
	return p
}

// checkAsset fetches an asset and describes what is wrong with it.
func checkAsset(t TB, s Sender, key string, a assetRef) string {
	t.Helper()
	res := s.Send(t, http.MethodGet, key, Body{}, Header("Sec-Fetch-Mode", "no-cors"), Header("Sec-Fetch-Dest", assetDest(a.kind)), Header("Sec-Fetch-User", ""))
	if res == nil {
		return ""
	}
	if err := res.Captured().Check(t.Context()); err != nil {
		return fmt.Sprintf("asset %s (on %s): %v", key, a.page, err)
	}
	if code := res.StatusCode(); code != http.StatusOK {
		return fmt.Sprintf("asset %s (on %s) answered %d", key, a.page, code)
	}
	var problems []string
	h := res.Headers()
	if !strings.EqualFold(h.Get("X-Content-Type-Options"), "nosniff") {
		problems = append(problems, "no X-Content-Type-Options: nosniff")
	}
	ctype := mediaEssence(h.Get("Content-Type"))
	switch a.kind {
	case "stylesheet":
		if ctype != "text/css" {
			problems = append(problems, fmt.Sprintf("content type %q, want text/css", ctype))
		}
	case "script":
		if ctype != "text/javascript" && ctype != "application/javascript" {
			problems = append(problems, fmt.Sprintf("content type %q, want text/javascript", ctype))
		}
	}
	if a.integrity != "" && !integrityMatches(a.integrity, res.Bytes()) {
		problems = append(problems, "the served bytes do not match the integrity value of the page")
	}
	if len(problems) == 0 {
		return ""
	}
	return fmt.Sprintf("asset %s (on %s): %s", key, a.page, strings.Join(problems, "; "))
}

func assetDest(kind string) string {
	switch kind {
	case "stylesheet":
		return "style"
	case "script":
		return "script"
	case "image", "icon":
		return "image"
	}
	return "empty"
}

// integrityMatches reports whether data matches one of the well-formed
// digests of an integrity value of the strongest algorithm it lists, as
// browsers check it.
func integrityMatches(integrity string, data []byte) bool {
	strength := map[string]int{"sha256": 1, "sha384": 2, "sha512": 3}
	best := 0
	type digest struct {
		alg string
		sum []byte
	}
	var digests []digest
	for tok := range strings.FieldsSeq(integrity) {
		alg, sum, ok := parseIntegrity(tok)
		if !ok {
			continue
		}
		digests = append(digests, digest{alg, sum})
		best = max(best, strength[alg])
	}
	for _, d := range digests {
		if strength[d.alg] != best {
			continue
		}
		var got []byte
		switch d.alg {
		case "sha256":
			s := sha256.Sum256(data)
			got = s[:]
		case "sha384":
			s := sha512.Sum384(data)
			got = s[:]
		case "sha512":
			s := sha512.Sum512(data)
			got = s[:]
		}
		if string(got) == string(d.sum) {
			return true
		}
	}
	return false
}
