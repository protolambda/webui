package session

import (
	"fmt"
	"io"
)

// hidden keeps secret state behind two pointer levels. A single pointer
// field is insufficient: when a copied public struct is formatted with
// an unsupported verb, fmt's diagnostic dereferences that pointer at the
// top level. It then prints this handle's value pointer at a nested
// level, where it does not dereference it. No secret state is formatted.
type hidden[T any] struct {
	value *T
}

func hide[T any](value T) *hidden[T] {
	return &hidden[T]{value: &value}
}

// formatRedacted writes desc, the redacted description of a value, for the
// formatting verb: fmt consults String only for %v, %s, %q, %x and %X (and
// GoString for %#v), and prints the fields of the value for every other
// verb, secrets included. Those verbs format desc as they would a String
// result; %#v prints desc as GoString did; every other verb writes desc
// as is.
func formatRedacted(f fmt.State, verb rune, desc string) {
	switch verb {
	case 'v':
		if f.Flag('#') {
			_, _ = io.WriteString(f, desc)
			return
		}
		fallthrough
	case 's', 'q', 'x', 'X':
		_, _ = fmt.Fprintf(f, fmt.FormatString(f, verb), desc)
	default:
		_, _ = io.WriteString(f, desc)
	}
}
