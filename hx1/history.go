package hx1

import (
	"github.com/protolambda/chord/core/attr"
)

// PushURL sets the hx-push-url attribute to push a URL to browser history.
func PushURL(v string) attr.Node { return attr.Name("hx-push-url").Value(v) }

// ReplaceURL sets the hx-replace-url attribute to replace browser history URL.
func ReplaceURL(v string) attr.Node { return attr.Name("hx-replace-url").Value(v) }

// HistoryElt sets the hx-history-elt boolean attribute.
func HistoryElt() attr.Node { return attr.Name("hx-history-elt").Bool() }
