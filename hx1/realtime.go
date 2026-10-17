package hx1

import (
	"github.com/protolambda/chord/core/attr"
)

// SSE sets the hx-sse attribute for Server-Sent Events connections.
func SSE(v string) attr.Node { return attr.Name("hx-sse").Value(v) }

// WS sets the hx-ws attribute for WebSocket connections.
func WS(v string) attr.Node { return attr.Name("hx-ws").Value(v) }
