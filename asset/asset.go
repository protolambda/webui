// Package asset serves a fixed set of embedded files, such as stylesheets,
// scripts and fonts, and renders the chord nodes that load them with
// subresource integrity.
//
// A [Set] is built once from an [fs.FS], usually an [embed.FS], and a list of
// [File] entries. Building it reads every file, computes its integrity value
// (SHA-384), checks any pinned integrity value, and derives a version from
// the content of all files. The set never changes afterwards and is safe for
// concurrent use.
//
// # URLs and caching
//
// Files are served under the set's prefix and version: with prefix
// "/assets/app/", the file "app.css" is at "/assets/app/<version>/app.css".
// A new build that changes any file of the set changes the version, and so
// every URL of the set. Responses for the current version are cacheable
// forever ("immutable"). A request for an older or unknown version gets the
// current content with "no-cache": a page loaded before an upgrade still
// gets the files that did not change, and the integrity values of its
// stylesheets and scripts reject those that did. Files that browsers fetch
// without integrity metadata, such as the fonts and images a stylesheet
// refers to and the modules that a module imports, are served at their
// current content. Relative references between files of one set, such as a
// stylesheet that loads a font from "fonts/", keep working because the files
// share the version directory.
//
// Responses carry an exact Content-Type from a fixed table (not the operating
// system's MIME database), "X-Content-Type-Options: nosniff", a
// Content-Length, an ETag that is a digest of the bytes of the representation
// (identity or gzip), and, where it saves space, a gzip encoding computed
// when the set is built. A response without the content, such as the 412 of
// a failed If-Match, carries "Cache-Control: no-store" instead of the file's
// cache policy and ETag.
//
// # Pages
//
// [Set.Stylesheets] and [Set.Scripts] render the <link> and <script> elements
// of the set, with integrity and crossorigin attributes, in the order of the
// files; [File.Unlisted] keeps a file out of them. Classic scripts are
// deferred, and modules ([KindModule]) are deferred by nature, so they run in
// order after the document is parsed. Integrity values make a browser refuse
// a file that differs from what the page was rendered with, also when it
// comes from a cache.
package asset

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/meta"
	"github.com/protolambda/chord/html/script"
)

// ErrInvalid indicates a set definition that cannot be served: a bad prefix
// or file name, an unknown content type, a duplicate file, a file missing
// from the file system (the error also wraps [fs.ErrNotExist]), or a kind
// that does not fit the content type.
var ErrInvalid = errors.New("invalid asset set")

// ErrIntegrity indicates a file whose content does not match its pinned
// integrity value.
var ErrIntegrity = errors.New("asset integrity mismatch")

// Kind is what a file is to a page, which decides how [Set] renders it.
type Kind uint8

const (
	// KindOther is a file that pages do not load directly, such as a source
	// map. It is the zero value: a [File] without a kind gets the kind of its
	// extension, so setting KindOther explicitly changes nothing. To keep a
	// stylesheet or script out of [Set.Stylesheets] and [Set.Scripts], set
	// [File.Unlisted].
	KindOther Kind = iota
	// KindStylesheet is a CSS file, loaded with <link rel="stylesheet">.
	KindStylesheet
	// KindScript is a classic JavaScript file, loaded with a deferred <script>.
	KindScript
	// KindFont is a font, loaded by a stylesheet.
	KindFont
	// KindImage is an image, loaded by a stylesheet or an <img>.
	KindImage
	// KindModule is a JavaScript module, loaded with
	// <script type="module">, which runs deferred, in order with the
	// deferred classic scripts. No extension infers it, not even .mjs (a
	// module may be meant only for other modules to import): set it on the
	// entry modules that pages load. Its integrity value covers the entry
	// file only: browsers fetch the modules it imports without integrity
	// metadata.
	KindModule
)

func (k Kind) String() string {
	switch k {
	case KindOther:
		return "other"
	case KindStylesheet:
		return "stylesheet"
	case KindScript:
		return "script"
	case KindFont:
		return "font"
	case KindImage:
		return "image"
	case KindModule:
		return "module"
	default:
		return fmt.Sprintf("kind(%d)", uint8(k))
	}
}

// File describes one file of a [Set].
type File struct {
	// Name is the slash-separated path of the file in the FS, and in URLs
	// below the set's version directory, e.g. "app.css" or "fonts/x.woff2".
	// It may use ASCII letters, digits, '.', '-', '_' and '/' only.
	Name string
	// Kind is what the file is to a page. The zero value takes the kind of
	// the extension: .css is a stylesheet, .js is a classic script, font and
	// image extensions are fonts and images, and others, including .mjs, are
	// KindOther. Set [KindModule] on a JavaScript module that pages load.
	Kind Kind
	// ContentType overrides the content type of the extension. It is
	// required for an extension that the fixed table does not know.
	ContentType string
	// Integrity optionally pins the content: a subresource-integrity value
	// such as "sha384-...". [New] fails with [ErrIntegrity] when the content
	// differs. Pin third-party files with the value their project publishes.
	Integrity string
	// Unlisted keeps a stylesheet, script or module out of [Set.Stylesheets]
	// and [Set.Scripts], and so out of every page that loads the whole set:
	// for example a worker script, a script that only some pages load, or a
	// stylesheet that another one imports. The file is still served, and
	// [Set.URL], [Set.Integrity], [Set.Stylesheet], [Set.Script] and
	// [Set.Preload] work as for any other file.
	Unlisted bool
}

// Options configure a [Set].
type Options struct {
	// Prefix is the URL path under which the set is served. It must start
	// and end with "/", e.g. "/assets/app/".
	Prefix string
	// FS holds the files.
	FS fs.FS
	// Files lists the files of the set, in the order pages load them.
	Files []File
}

// Info describes a file of a built [Set].
type Info struct {
	Name        string
	Kind        Kind
	ContentType string
	// URL is the absolute path of the current version of the file.
	URL string
	// Integrity is the SHA-384 subresource-integrity value of the content.
	Integrity string
	// Size is the length of the content in bytes.
	Size int
	// Unlisted is [File.Unlisted]: the file is not rendered by
	// [Set.Stylesheets] or [Set.Scripts].
	Unlisted bool
}

type entry struct {
	info Info
	data []byte
	// gz is the gzip encoding of data, nil when it would not save space.
	gz []byte
	// etag and gzETag are the strong validators of data and gz, each a
	// digest of the bytes it labels. The gzip bytes depend on the compressor
	// (its Go version and level), not only on data, so they cannot share a
	// tag derived from data: two builds of one version could otherwise
	// serve different bytes under one tag, and a range request could splice
	// them.
	etag, gzETag string
}

// Set is an immutable set of files with their URLs, integrity values and
// HTTP handler. Build one with [New]; the zero value is not usable.
type Set struct {
	prefix  string
	version string
	files   []*entry
	byName  map[string]*entry
}

var _ http.Handler = (*Set)(nil)

// New reads and checks the files of opts, and returns the set that serves
// them. Errors wrap [ErrInvalid] (a file missing from opts.FS also wraps
// [fs.ErrNotExist]) or [ErrIntegrity], or the error of reading a file.
func New(opts Options) (*Set, error) {
	if err := checkPrefix(opts.Prefix); err != nil {
		return nil, err
	}
	if opts.FS == nil {
		return nil, fmt.Errorf("%w: no file system", ErrInvalid)
	}
	if len(opts.Files) == 0 {
		return nil, fmt.Errorf("%w: no files", ErrInvalid)
	}
	s := &Set{
		prefix: opts.Prefix,
		files:  make([]*entry, 0, len(opts.Files)),
		byName: make(map[string]*entry, len(opts.Files)),
	}
	version := sha256.New()
	for _, f := range opts.Files {
		e, err := load(opts.FS, f)
		if err != nil {
			return nil, err
		}
		if _, ok := s.byName[f.Name]; ok {
			return nil, fmt.Errorf("%w: duplicate file %q", ErrInvalid, f.Name)
		}
		s.byName[f.Name] = e
		s.files = append(s.files, e)
		// Name, type and digest of every file, length-prefixed so that no
		// two different sets hash the same input.
		for _, part := range []string{e.info.Name, e.info.ContentType, e.info.Integrity} {
			fmt.Fprintf(version, "%d:%s;", len(part), part)
		}
	}
	s.version = hex.EncodeToString(version.Sum(nil)[:8])
	for _, e := range s.files {
		e.info.URL = s.prefix + s.version + "/" + e.info.Name
	}
	return s, nil
}

// Must returns s, and panics if err is not nil. It suits sets of embedded
// files, whose errors are programming errors: Must(New(...)).
func Must(s *Set, err error) *Set {
	if err != nil {
		panic(err)
	}
	return s
}

func checkPrefix(prefix string) error {
	if !strings.HasPrefix(prefix, "/") || !strings.HasSuffix(prefix, "/") {
		return fmt.Errorf("%w: prefix %q must start and end with \"/\"", ErrInvalid, prefix)
	}
	if prefix != "/" && !validPath(strings.Trim(prefix, "/")) {
		return fmt.Errorf("%w: prefix %q is not a clean path of letters, digits, '.', '-', '_'", ErrInvalid, prefix)
	}
	return nil
}

// validPath reports whether name is a clean relative slash path whose
// characters need no escaping in a URL path or a ServeMux pattern.
func validPath(name string) bool {
	if !fs.ValidPath(name) || name == "." {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		case c == '.', c == '-', c == '_', c == '/':
		default:
			return false
		}
	}
	return true
}

func load(fsys fs.FS, f File) (*entry, error) {
	if !validPath(f.Name) {
		return nil, fmt.Errorf("%w: file name %q is not a clean path of letters, digits, '.', '-', '_'", ErrInvalid, f.Name)
	}
	ext := strings.ToLower(path.Ext(f.Name))
	contentType := f.ContentType
	if contentType == "" {
		contentType = contentTypes[ext]
		if contentType == "" {
			return nil, fmt.Errorf("%w: file %q: no content type for extension %q", ErrInvalid, f.Name, ext)
		}
	}
	kind := f.Kind
	if kind == KindOther {
		kind = kinds[ext]
	}
	if err := checkKind(kind, contentType); err != nil {
		return nil, fmt.Errorf("%w: file %q: %v", ErrInvalid, f.Name, err)
	}
	data, err := fs.ReadFile(fsys, f.Name)
	if errors.Is(err, fs.ErrNotExist) {
		// Most likely a typo in the name or a file missing from an embed
		// pattern: a definition error like the others.
		return nil, fmt.Errorf("%w: file %q: %w", ErrInvalid, f.Name, err)
	}
	if err != nil {
		return nil, fmt.Errorf("read asset %q: %w", f.Name, err)
	}
	sum := sha512.Sum384(data)
	integrity := "sha384-" + base64.StdEncoding.EncodeToString(sum[:])
	if f.Integrity != "" {
		if err := checkIntegrity(f.Integrity, data); err != nil {
			return nil, fmt.Errorf("file %q: %w", f.Name, err)
		}
	}
	gz, err := compress(data)
	if err != nil {
		return nil, fmt.Errorf("compress asset %q: %w", f.Name, err)
	}
	// Only worth a second representation when it saves at least an eighth.
	if len(gz) >= len(data)-len(data)/8 {
		gz = nil
	}
	info := Info{
		Name:        f.Name,
		Kind:        kind,
		ContentType: contentType,
		Integrity:   integrity,
		Size:        len(data),
		Unlisted:    f.Unlisted,
	}
	return newEntry(info, data, gz), nil
}

// newEntry returns the entry that serves data and gz, its gzip encoding (nil
// for none), with an ETag for each that is a digest of its own bytes.
func newEntry(info Info, data, gz []byte) *entry {
	e := &entry{info: info, data: data, etag: digestTag(data)}
	if gz != nil {
		e.gz, e.gzETag = gz, digestTag(gz)+".gz"
	}
	return e
}

// digestTag returns an opaque ETag value (without quotes) for b: 96 bits of
// its SHA-256 digest in hex.
func digestTag(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:12])
}

func checkKind(kind Kind, contentType string) error {
	mediaType, _, _ := strings.Cut(contentType, ";")
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	switch kind {
	case KindStylesheet:
		// Browsers refuse a stylesheet of any other type under nosniff.
		if mediaType != "text/css" {
			return fmt.Errorf("a stylesheet must be text/css, not %q", contentType)
		}
	case KindScript, KindModule:
		// And a script of a non-JavaScript type.
		if mediaType != "text/javascript" && mediaType != "application/javascript" {
			return fmt.Errorf("a %v must be text/javascript, not %q", kind, contentType)
		}
	case KindOther, KindFont, KindImage:
	default:
		return fmt.Errorf("unknown kind %v", kind)
	}
	return nil
}

// checkIntegrity checks data against a pinned subresource-integrity value.
func checkIntegrity(pinned string, data []byte) error {
	alg, want, ok := strings.Cut(pinned, "-")
	if !ok {
		return fmt.Errorf("%w: malformed pinned integrity %q", ErrInvalid, pinned)
	}
	var got []byte
	switch alg {
	case "sha256":
		sum := sha256.Sum256(data)
		got = sum[:]
	case "sha384":
		sum := sha512.Sum384(data)
		got = sum[:]
	case "sha512":
		sum := sha512.Sum512(data)
		got = sum[:]
	default:
		return fmt.Errorf("%w: unsupported integrity algorithm %q", ErrInvalid, alg)
	}
	if base64.StdEncoding.EncodeToString(got) != want {
		return fmt.Errorf("%w: content is %s-%s, pinned %s", ErrIntegrity, alg, base64.StdEncoding.EncodeToString(got), pinned)
	}
	return nil
}

func compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(data); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// contentTypes maps lowercase extensions to the content types the set
// serves. The table is fixed so that responses do not depend on the MIME
// database of the operating system.
var contentTypes = map[string]string{
	".css":   "text/css; charset=utf-8",
	".js":    "text/javascript; charset=utf-8",
	".mjs":   "text/javascript; charset=utf-8",
	".json":  "application/json",
	".map":   "application/json",
	".txt":   "text/plain; charset=utf-8",
	".woff2": "font/woff2",
	".woff":  "font/woff",
	".ttf":   "font/ttf",
	".otf":   "font/otf",
	".svg":   "image/svg+xml",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".gif":   "image/gif",
	".webp":  "image/webp",
	".avif":  "image/avif",
	".ico":   "image/x-icon",
	".wasm":  "application/wasm",
}

// kinds maps lowercase extensions to the kinds they infer. ".mjs" is not
// here: a module is not a classic script, and only some modules are entry
// points, so modules are tagged [KindModule] explicitly.
var kinds = map[string]Kind{
	".css":   KindStylesheet,
	".js":    KindScript,
	".woff2": KindFont,
	".woff":  KindFont,
	".ttf":   KindFont,
	".otf":   KindFont,
	".svg":   KindImage,
	".png":   KindImage,
	".jpg":   KindImage,
	".jpeg":  KindImage,
	".gif":   KindImage,
	".webp":  KindImage,
	".avif":  KindImage,
	".ico":   KindImage,
}

// Prefix returns the URL path prefix of the set, e.g. "/assets/app/".
func (s *Set) Prefix() string { return s.prefix }

// Version returns the version directory of the set's URLs: a hash of the
// names, types and contents of all its files.
func (s *Set) Version() string { return s.version }

// Pattern returns the [http.ServeMux] pattern of the set, "GET " followed by
// the prefix, which also matches HEAD requests:
//
//	mux.Handle(set.Pattern(), set)
func (s *Set) Pattern() string { return "GET " + s.prefix }

// Files returns the files of the set, in their declared order.
func (s *Set) Files() iter.Seq[Info] {
	return func(yield func(Info) bool) {
		for _, e := range s.files {
			if !yield(e.info) {
				return
			}
		}
	}
}

// Lookup returns the file with the given name.
func (s *Set) Lookup(name string) (Info, bool) {
	e, ok := s.byName[name]
	if !ok {
		return Info{}, false
	}
	return e.info, true
}

func (s *Set) mustEntry(name string) *entry {
	e, ok := s.byName[name]
	if !ok {
		panic(fmt.Sprintf("asset: no file %q in set %s", name, s.prefix))
	}
	return e
}

// URL returns the absolute path of the current version of the named file.
// It panics if the set has no such file: names are fixed at build time, so
// an unknown name is a programming error.
func (s *Set) URL(name string) string { return s.mustEntry(name).info.URL }

// Integrity returns the subresource-integrity value of the named file. It
// panics if the set has no such file.
func (s *Set) Integrity(name string) string { return s.mustEntry(name).info.Integrity }

// Stylesheets returns a <link rel="stylesheet"> element for every stylesheet
// of the set that is not [File.Unlisted], in order.
func (s *Set) Stylesheets() elem.Node {
	var out elem.Bundle
	for _, e := range s.files {
		if e.info.Kind == KindStylesheet && !e.info.Unlisted {
			out = append(out, stylesheet(e.info))
		}
	}
	return out
}

// Scripts returns a <script> element for every script and module of the set
// that is not [File.Unlisted], in order: deferred for a classic script,
// type="module" for a module. Both kinds run in document order once the
// document is parsed, so they may sit in the head.
func (s *Set) Scripts() elem.Node {
	var out elem.Bundle
	for _, e := range s.files {
		if (e.info.Kind == KindScript || e.info.Kind == KindModule) && !e.info.Unlisted {
			out = append(out, scriptElem(e.info))
		}
	}
	return out
}

// Stylesheet returns the <link rel="stylesheet"> element of the named file,
// which must be a stylesheet of the set. A file of another kind is a render
// error; an unknown name panics, like [Set.URL].
func (s *Set) Stylesheet(name string) elem.Node {
	info := s.mustEntry(name).info
	if info.Kind != KindStylesheet {
		return kindError(info, KindStylesheet)
	}
	return stylesheet(info)
}

// Script returns the <script> element of the named file, which must be a
// script or module of the set, also when it is [File.Unlisted]: deferred for
// a classic script, type="module" for a module. A file of another kind is a
// render error; an unknown name panics, like [Set.URL].
func (s *Set) Script(name string) elem.Node {
	info := s.mustEntry(name).info
	if info.Kind != KindScript && info.Kind != KindModule {
		return kindError(info, KindScript)
	}
	return scriptElem(info)
}

// Preload returns a <link rel="preload"> element for the named file, which
// lets the browser fetch it before the page or stylesheet that uses it asks
// for it, such as a font that a stylesheet loads. A module gets
// <link rel="modulepreload">. A file of KindOther is a render error; an
// unknown name panics, like [Set.URL].
//
// A browser uses a preload only for a later fetch in the same CORS mode.
// Stylesheet, script, module and font preloads carry integrity and
// crossorigin="anonymous", like the elements this package renders and like
// the font fetches of browsers. An image preload carries neither: it matches
// CSS url() images and <img> elements without a crossorigin attribute, which
// are no-cors fetches without integrity metadata.
func (s *Set) Preload(name string) elem.Node {
	info := s.mustEntry(name).info
	mediaType, _, _ := strings.Cut(info.ContentType, ";")
	mediaType = strings.TrimSpace(mediaType)
	var as string
	switch info.Kind {
	case KindStylesheet:
		as = "style"
	case KindScript:
		as = "script"
	case KindFont:
		as = "font"
	case KindModule:
		return meta.Link(
			meta.Rel("modulepreload"),
			meta.Href(info.URL),
			meta.Integrity(info.Integrity),
			meta.Crossorigin("anonymous"),
		)
	case KindImage:
		return meta.Link(
			meta.Rel("preload"),
			meta.Href(info.URL),
			attr.Name("as").Value("image"),
			meta.Type(mediaType),
		)
	default:
		return elem.Fn(func(context.Context) (elem.Node, error) {
			return nil, fmt.Errorf("%w: file %q of kind %v cannot be preloaded", ErrInvalid, info.Name, info.Kind)
		})
	}
	return meta.Link(
		meta.Rel("preload"),
		meta.Href(info.URL),
		attr.Name("as").Value(as),
		meta.Type(mediaType),
		meta.Integrity(info.Integrity),
		meta.Crossorigin("anonymous"),
	)
}

func kindError(info Info, want Kind) elem.Node {
	return elem.Fn(func(context.Context) (elem.Node, error) {
		return nil, fmt.Errorf("%w: file %q is a %v, not a %v", ErrInvalid, info.Name, info.Kind, want)
	})
}

func stylesheet(info Info) elem.Node {
	return meta.Link(
		meta.Rel("stylesheet"),
		meta.Href(info.URL),
		meta.Integrity(info.Integrity),
		meta.Crossorigin("anonymous"),
	)
}

func scriptElem(info Info) elem.Node {
	if info.Kind == KindModule {
		// Module scripts are deferred by default; defer has no effect.
		return script.Script(
			script.Type("module"),
			script.Src(info.URL),
			script.Integrity(info.Integrity),
			script.Crossorigin("anonymous"),
		)()
	}
	return script.Script(
		script.Src(info.URL),
		script.Defer(),
		script.Integrity(info.Integrity),
		script.Crossorigin("anonymous"),
	)()
}

// immutable is the cache policy of the current version: a year, the
// longest that caches honor, without revalidation.
const immutable = "public, max-age=31536000, immutable"

// ServeHTTP serves the files of the set for GET and HEAD requests below its
// prefix, and answers 404 for unknown files and 405 for other methods.
//
// The current version is served as immutable. Any other version directory
// gets the current content with "Cache-Control: no-cache", see the package
// documentation. Conditional requests (If-None-Match, If-Match) and ranges
// are handled by [http.ServeContent]; ranges apply to the representation
// sent, gzip or not, each with its own ETag. A response without the content
// other than 304, such as a 412 or 416, has "Cache-Control: no-store" and no
// ETag or Content-Encoding, so that no cache stores it as the file.
func (s *Set) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		h.Set("Allow", "GET, HEAD")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	rest, ok := strings.CutPrefix(r.URL.Path, s.prefix)
	if !ok {
		http.NotFound(w, r)
		return
	}
	version, name, ok := strings.Cut(rest, "/")
	if !ok || version == "" {
		http.NotFound(w, r)
		return
	}
	e, ok := s.byName[name]
	if !ok {
		http.NotFound(w, r)
		return
	}
	if version == s.version {
		h.Set("Cache-Control", immutable)
	} else {
		h.Set("Cache-Control", "no-cache")
	}
	h.Set("Content-Type", e.info.ContentType)
	cw := &contentWriter{ResponseWriter: w}
	data, etag := e.data, e.etag
	if e.gz != nil {
		h.Add("Vary", "Accept-Encoding")
		if acceptsGzip(r.Header.Values("Accept-Encoding")) {
			data, etag, cw.encoding = e.gz, e.gzETag, "gzip"
		}
	}
	h.Set("ETag", `"`+etag+`"`)
	http.ServeContent(cw, r, "", time.Time{}, bytes.NewReader(data))
}

// contentWriter fixes the headers of a response of [http.ServeContent] when
// it writes the status, for statuses ServeContent does not clean up itself:
//
//   - Content-Encoding is set only on 200 and 206, the responses that carry
//     the content. Set earlier, it makes ServeContent omit Content-Length (a
//     rule meant for writers that compress on the fly, not for precomputed
//     gzip bytes of known length), and it would label a bodiless 412.
//   - Any other status but 304 drops the ETag and replaces the cache
//     policy with "no-store": the 412 of a failed If-Match would otherwise
//     keep the immutable policy, and a shared cache could store the empty
//     response as the file for a year.
type contentWriter struct {
	http.ResponseWriter
	encoding    string // "gzip" or "" (identity)
	wroteHeader bool
}

func (w *contentWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.wroteHeader = true
		h := w.Header()
		switch code {
		case http.StatusOK, http.StatusPartialContent:
			if w.encoding != "" {
				h.Set("Content-Encoding", w.encoding)
			}
		case http.StatusNotModified:
		default:
			h.Del("Content-Encoding")
			h.Del("Etag")
			h.Set("Cache-Control", "no-store")
		}
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *contentWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}

// ReadFrom keeps the copy path of the underlying writer, which for a
// net/http response uses pooled buffers, as without the wrapper.
func (w *contentWriter) ReadFrom(r io.Reader) (int64, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if rf, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(r)
	}
	return io.Copy(w.ResponseWriter, r)
}

// Unwrap lets [http.ResponseController] reach the underlying writer.
func (w *contentWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// acceptsGzip reports whether Accept-Encoding header values accept gzip:
// listed (or covered by "*") without a zero quality.
func acceptsGzip(values []string) bool {
	gzip, star := 0, 0 // 0: not listed, 1: accepted, -1: refused (q=0)
	for _, v := range values {
		for item := range strings.SplitSeq(v, ",") {
			coding, params, _ := strings.Cut(item, ";")
			coding = strings.ToLower(strings.TrimSpace(coding))
			state := 1
			for p := range strings.SplitSeq(params, ";") {
				k, val, ok := strings.Cut(p, "=")
				if ok && strings.EqualFold(strings.TrimSpace(k), "q") && zeroQuality(strings.TrimSpace(val)) {
					state = -1
				}
			}
			switch coding {
			case "gzip", "x-gzip":
				gzip = state
			case "*":
				star = state
			}
		}
	}
	if gzip != 0 {
		return gzip > 0
	}
	return star > 0
}

// zeroQuality reports whether a qvalue is zero: "0", "0.", "0.0", "0.00" or
// "0.000".
func zeroQuality(q string) bool {
	if q == "" || q[0] != '0' {
		return false
	}
	return strings.Trim(q[1:], ".0") == "" && strings.Count(q, ".") <= 1 && len(q) <= 5
}
