package bi

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"
	"strings"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
)

// ErrEmptyLabel indicates an empty or blank label for a meaningful icon; see
// [Icon.Label].
var ErrEmptyLabel = errors.New("empty icon label")

// Icon is the name of a Bootstrap Icons icon, without the "bi-" class prefix,
// e.g. "alarm". The constants of this package are all the icons of release
// [Version]; [Lookup] checks a run-time name against them.
//
// An Icon is an [elem.Node]: it renders as a decorative
// <i class="bi bi-NAME" aria-hidden="true"></i>. A name that is not
// lower-case ASCII letters, digits and hyphens is HTML-escaped (see the
// package documentation). The rendered icon needs the icons stylesheet and
// font.
type Icon string

var _ elem.Node = Icon("")

// Eval renders the icon as a decorative element, hidden from assistive
// technology. It is the same as i.With().
func (i Icon) Eval(ctx context.Context) (elem.Obj, error) {
	return i.With().Eval(ctx)
}

// With returns the decorative icon with extra attributes, rendered after the
// icon's own: <i class="bi bi-NAME ..." aria-hidden="true" ...></i>.
//
// Extra classes are merged into the single class attribute of the icon.
// Other attributes follow the usual chord rule that an attribute may appear
// once: passing aria-hidden fails the render. For an icon with meaning, use
// [Icon.Label] rather than adding role or aria-label here, because
// aria-hidden would hide that label. A nil attribute fails the render.
func (i Icon) With(attrs ...attr.Node) elem.Node {
	return i.element(ariaHidden, attrs)
}

// Label returns the icon as a meaningful image named label:
// <i class="bi bi-NAME ..." role="img" aria-label="LABEL" ...></i>. Use it
// for an icon that conveys information that is not also given as text, such
// as a status icon in a table cell. The label is HTML-escaped. Extra
// attributes behave as in [Icon.With].
//
// An empty or blank label would leave the image without an accessible name:
// the node then fails evaluation with an error wrapping [ErrEmptyLabel].
func (i Icon) Label(label string, attrs ...attr.Node) elem.Node {
	if strings.TrimSpace(label) == "" {
		return elem.Fn(func(context.Context) (elem.Node, error) {
			return nil, errEmptyLabel
		})
	}
	return i.element(attr.Cons(roleImg, attr.Name("aria-label").Value(label)), attrs)
}

var (
	ariaHidden = attr.Name("aria-hidden").Raw("true")
	roleImg    = attr.Name("role").Raw("img")
	// errEmptyLabel is shared: the error carries no per-call detail.
	errEmptyLabel = fmt.Errorf("bi: %w", ErrEmptyLabel)
)

// element renders the icon's <i> element: the class, the accessibility
// attributes a11y, then the extra attributes.
func (i Icon) element(a11y attr.Node, extra []attr.Node) elem.Node {
	attrs := make([]attr.Node, 0, 2+len(extra))
	attrs = append(attrs, i.class(), a11y)
	attrs = append(attrs, extra...)
	return elem.Name("i").New(attrs...)()
}

// class returns the class attribute of the icon. Only a name made of
// lower-case ASCII letters, digits and hyphens is written raw; any other name
// is escaped, so a forged Icon cannot break out of the attribute.
func (i Icon) class() attr.Node {
	value := "bi bi-" + string(i)
	if validName(string(i)) {
		return attr.Name("class").Raw(value)
	}
	return attr.Name("class").Value(value)
}

// validName reports whether name is non-empty and consists of lower-case
// ASCII letters, digits and hyphens only.
func validName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

// All returns an iterator over every icon constant of this package, in the
// order of the Bootstrap Icons stylesheet. The iterator is restartable and
// safe for concurrent use.
func All() iter.Seq[Icon] {
	return func(yield func(Icon) bool) {
		for _, ic := range all {
			if !yield(ic) {
				return
			}
		}
	}
}

// Lookup returns the icon with the given name, as used after the "bi-"
// class prefix (e.g. "alarm-fill"), and whether it is one of the icons of
// release [Version]. Names are case-sensitive. Lookup is safe for concurrent
// use and does not allocate.
func Lookup(name string) (Icon, bool) {
	k, found := slices.BinarySearchFunc(byName[:], name, func(idx uint16, target string) int {
		return strings.Compare(string(all[idx]), target)
	})
	if !found {
		return "", false
	}
	return all[byName[k]], true
}
