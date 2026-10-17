package hx2

import (
	"github.com/protolambda/chord/core/attr"
)

// SSE extension attributes
//
// In htmx 2, server-sent events moved to the sse extension. Enable it with
// [Ext]("sse") on the element or an ancestor. webui's theme assets include
// the extension script ([github.com/protolambda/webui/theme.SSEJS]) unless
// they are built without it.
//
// Reference: https://htmx.org/extensions/sse/

// SSEConnect sets the sse-connect attribute to establish an SSE connection
// to url, which the page's connect-src must allow.
// Use with hx-ext="sse" on the element or a parent.
func SSEConnect(url string) attr.Node { return attr.Name("sse-connect").Value(url) }

// SSESwap sets the sse-swap attribute: the element swaps in the data of the
// events named messageName (a comma-separated list for several), with the
// swap style of hx-swap. Unnamed events are named "message".
// Use with hx-ext="sse" on the element or a parent.
func SSESwap(messageName string) attr.Node { return attr.Name("sse-swap").Value(messageName) }

// SSEClose sets the sse-close attribute to close the connection on a specific message.
// Use with hx-ext="sse" on the element or a parent.
func SSEClose(messageName string) attr.Node { return attr.Name("sse-close").Value(messageName) }

// WebSocket extension attributes
//
// In htmx 2, WebSocket support moved to the ws extension. Enable it with
// [Ext]("ws") on the element or an ancestor. The extension script is opt-in
// in webui: build the theme assets with the WebSocket option, which adds
// [github.com/protolambda/webui/theme.WSJS]. Without the script, these
// attributes do nothing. The page's connect-src must allow the WebSocket URL.
//
// Reference: https://htmx.org/extensions/ws/

// WSConnect sets the ws-connect attribute to establish a WebSocket connection.
// Use with hx-ext="ws" on the element or a parent, and the opt-in ws
// extension script.
func WSConnect(url string) attr.Node { return attr.Name("ws-connect").Value(url) }

// WSSend sets the ws-send attribute to mark an element that sends data over WebSocket.
// Use with hx-ext="ws" on the element or a parent. Typically used on forms.
// The ws extension sends the values of the form as a JSON message, with the
// htmx request headers in its HEADERS member: like every value from the
// client, they are untrusted.
func WSSend() attr.Node { return attr.Name("ws-send").Bool() }
