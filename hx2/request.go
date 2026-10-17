package hx2

import (
	"github.com/protolambda/chord/core/attr"
)

// Include sets the hx-include attribute to include additional elements in request.
func Include(v string) attr.Node { return attr.Name("hx-include").Value(v) }

// Params sets the hx-params attribute to filter request parameters.
func Params(v string) attr.Node { return attr.Name("hx-params").Value(v) }

// Vals sets the hx-vals attribute to add values to the request.
func Vals(v string) attr.Node { return attr.Name("hx-vals").Value(v) }

// Headers sets the hx-headers attribute to add headers to the request.
func Headers(v string) attr.Node { return attr.Name("hx-headers").Value(v) }

// Request sets the hx-request attribute to configure request behavior.
func Request(v string) attr.Node { return attr.Name("hx-request").Value(v) }

// Sync sets the hx-sync attribute to synchronize AJAX requests.
func Sync(v string) attr.Node { return attr.Name("hx-sync").Value(v) }
