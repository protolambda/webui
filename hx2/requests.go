// Package hx2 provides HTMX v2 attributes.
// This is a standalone package with no dependency on the hx (v1) package.
package hx2

import (
	"github.com/protolambda/chord/core/attr"
)

// Get sets the hx-get attribute for HTTP GET requests.
func Get(v string) attr.Node { return attr.Name("hx-get").Value(v) }

// Post sets the hx-post attribute for HTTP POST requests.
func Post(v string) attr.Node { return attr.Name("hx-post").Value(v) }

// Put sets the hx-put attribute for HTTP PUT requests.
func Put(v string) attr.Node { return attr.Name("hx-put").Value(v) }

// Patch sets the hx-patch attribute for HTTP PATCH requests.
func Patch(v string) attr.Node { return attr.Name("hx-patch").Value(v) }

// Delete sets the hx-delete attribute for HTTP DELETE requests.
func Delete(v string) attr.Node { return attr.Name("hx-delete").Value(v) }
