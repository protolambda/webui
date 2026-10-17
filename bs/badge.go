package bs

import (
	"github.com/protolambda/chord/core/attr"
	"github.com/protolambda/chord/core/elem"
	"github.com/protolambda/chord/html/text"
)

// Badge creates a basic Bootstrap badge (span.badge). A badge is plain
// text for assistive technology: put it after a real space or a
// visually-hidden separator so that the surrounding accessible name reads
// well, e.g. "Inbox 3" instead of "Inbox3".
func Badge(attrs ...attr.Node) elem.Scope {
	return text.Span(attr.Cons(rawClass("badge"), attrs...))
}

// BadgeOf creates a Bootstrap badge of the variant
// (span.badge.text-bg-{v}). An unknown variant fails the render with
// [ErrInvalidOption].
func BadgeOf(v Variant, attrs ...attr.Node) elem.Scope {
	return text.Span(compose(attrs, rawClass("badge"), variantClass("bs.BadgeOf", "text-bg-", v)))
}

// BadgePrimary creates a primary Bootstrap badge.
func BadgePrimary(attrs ...attr.Node) elem.Scope {
	return text.Span(attr.Cons(rawClass("badge text-bg-primary"), attrs...))
}

// BadgeSecondary creates a secondary Bootstrap badge.
func BadgeSecondary(attrs ...attr.Node) elem.Scope {
	return text.Span(attr.Cons(rawClass("badge text-bg-secondary"), attrs...))
}

// BadgeSuccess creates a success Bootstrap badge.
func BadgeSuccess(attrs ...attr.Node) elem.Scope {
	return text.Span(attr.Cons(rawClass("badge text-bg-success"), attrs...))
}

// BadgeDanger creates a danger Bootstrap badge.
func BadgeDanger(attrs ...attr.Node) elem.Scope {
	return text.Span(attr.Cons(rawClass("badge text-bg-danger"), attrs...))
}

// BadgeWarning creates a warning Bootstrap badge.
func BadgeWarning(attrs ...attr.Node) elem.Scope {
	return text.Span(attr.Cons(rawClass("badge text-bg-warning"), attrs...))
}

// BadgeInfo creates an info Bootstrap badge.
func BadgeInfo(attrs ...attr.Node) elem.Scope {
	return text.Span(attr.Cons(rawClass("badge text-bg-info"), attrs...))
}

// BadgeLight creates a light Bootstrap badge.
func BadgeLight(attrs ...attr.Node) elem.Scope {
	return text.Span(attr.Cons(rawClass("badge text-bg-light"), attrs...))
}

// BadgeDark creates a dark Bootstrap badge.
func BadgeDark(attrs ...attr.Node) elem.Scope {
	return text.Span(attr.Cons(rawClass("badge text-bg-dark"), attrs...))
}
