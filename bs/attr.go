package bs

import (
	"context"
	"fmt"

	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
)

// rawClass returns a class attribute that is written verbatim. Call it only
// with compile-time literals, digits, or values of a validated closed set
// (never with caller input): the value is not escaped.
func rawClass(value string) attr.Node {
	return attr.Name("class").Raw(value)
}

// fallback is an attribute that a component renders only when the caller's
// attributes do not set an attribute of the same name, so that callers can
// override it without a duplicate attribute error. See [compose]. Rendered
// on its own, it is the plain attribute.
type fallback attr.Obj

func (f fallback) Eval(context.Context) (attr.Obj, error) {
	return attr.Obj(f), nil
}

// fallbackRaw is a fallback attribute with a verbatim value: only for
// literals and digits.
func fallbackRaw(name, value string) attr.Node {
	return fallback{Kind: attr.KindRawValue, Key: name, Val: value}
}

// fallbackValue is a fallback attribute with an escaped value.
func fallbackValue(name, value string) attr.Node {
	return fallback{Kind: attr.KindValue, Key: name, Val: value}
}

// fallbackBool is a fallback boolean attribute.
func fallbackBool(name string) attr.Node {
	return fallback{Kind: attr.KindBool, Key: name}
}

// compose returns the attributes of a component element: own, in order,
// followed by the caller's attributes user. An own attribute made with a
// fallback constructor is left out when user sets an attribute of the same
// name (ASCII case-insensitively).
//
// When own has a fallback, user is evaluated when the element renders, with
// the render's context, and each user node is evaluated exactly once: lazy
// nodes ([attr.Fn], [attr.Seq], single-use sequences) are inspected as well
// as evaluated objects and bundles.
func compose(user []attr.Node, own ...attr.Node) attr.Node {
	withFallback := false
	for _, a := range own {
		if _, ok := a.(fallback); ok {
			withFallback = true
			break
		}
	}
	if !withFallback {
		out := make(attr.Bundle, 0, len(own)+len(user))
		out = append(out, own...)
		return append(out, user...)
	}
	return attr.Fn(func(ctx context.Context) (attr.Node, error) {
		objs, err := flatten(ctx, user, nil)
		if err != nil {
			return nil, err
		}
		out := make(attr.Bundle, 0, len(own)+len(objs))
		for _, a := range own {
			if f, ok := a.(fallback); ok && sets(objs, f.Key) {
				continue
			}
			out = append(out, a)
		}
		for _, o := range objs {
			out = append(out, o)
		}
		return out, nil
	})
}

// userAttrs returns the single caller attribute of a struct component as a
// list: empty when it is nil.
func userAttrs(a attr.Node) []attr.Node {
	if a == nil {
		return nil
	}
	return []attr.Node{a}
}

// flatten evaluates attribute nodes into their attribute objects, in order,
// descending into bundles and dropping no-ops, as the renderer would.
func flatten(ctx context.Context, nodes []attr.Node, out []attr.Obj) ([]attr.Obj, error) {
	for _, n := range nodes {
		var err error
		if out, err = flattenNode(ctx, n, out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func flattenNode(ctx context.Context, n attr.Node, out []attr.Obj) ([]attr.Obj, error) {
	if n == nil {
		return nil, attr.ErrNilNode
	}
	obj, err := n.Eval(ctx)
	if err != nil {
		return nil, err
	}
	kind, err := obj.Validate()
	if err != nil {
		return nil, err
	}
	switch kind {
	case attr.KindNoop:
		return out, nil
	case attr.KindBundle:
		if obj.Sub == nil {
			return out, nil
		}
		for sub := range obj.Sub {
			if out, err = flattenNode(ctx, sub, out); err != nil {
				return nil, err
			}
		}
		return out, nil
	}
	obj.Kind = kind
	return append(out, obj), nil
}

// sets reports whether objs contain an attribute with the given lowercase
// name, ignoring ASCII case as HTML does.
func sets(objs []attr.Obj, name string) bool {
	for _, o := range objs {
		if asciiEqualFold(o.Key, name) {
			return true
		}
	}
	return false
}

// asciiEqualFold compares a with the lowercase ASCII string lower, ignoring
// ASCII case only.
func asciiEqualFold(a, lower string) bool {
	if len(a) != len(lower) {
		return false
	}
	for i := 0; i < len(a); i++ {
		c := a[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != lower[i] {
			return false
		}
	}
	return true
}

// errAttr is an attribute that fails the render with its error.
type errAttr struct{ err error }

func (e errAttr) Eval(context.Context) (attr.Obj, error) {
	return attr.Obj{}, e.err
}

// idTarget returns the attribute name="#id" (a CSS selector of the id), or
// an attribute that fails the render when id is not a valid id.
func idTarget(component, name, id string) attr.Node {
	if !ValidID(id) {
		return errAttr{fmt.Errorf("%s: %w", component, ErrInvalidID)}
	}
	return attr.Name(name).Value("#" + id)
}

// idValue returns the attribute name=id, or an attribute that fails the
// render when id is not a valid id.
func idValue(component, name, id string) attr.Node {
	if !ValidID(id) {
		return errAttr{fmt.Errorf("%s: %w", component, ErrInvalidID)}
	}
	return attr.Name(name).Value(id)
}

// checkID returns an error wrapping [ErrInvalidID] when id is not valid.
func checkID(component, id string) error {
	if !ValidID(id) {
		return fmt.Errorf("%s: %w", component, ErrInvalidID)
	}
	return nil
}

// optional returns an optional component field as a child list: empty when
// the field is nil.
func optional(n elem.Node) []elem.Node {
	if n == nil {
		return nil
	}
	return []elem.Node{n}
}

// visuallyHidden wraps children in a span.visually-hidden: text for
// assistive technology only.
func visuallyHidden(children ...elem.Node) elem.Node {
	return elem.Name("span").New(rawClass("visually-hidden"))(children...)
}
