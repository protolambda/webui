// Package server provides an [http.Server] with hardened defaults and the
// lifecycle around it: [New] builds the server, [Run] serves a listener until
// a context ends and then shuts down gracefully, [LimitConns] bounds the
// connections per client address, and [CheckLoopback] refuses to serve a
// loopback site on a listener that the network reaches.
//
// The package depends on the standard library and
// [github.com/protolambda/webui/mw] only.
//
// # Timeouts
//
// New sets ReadHeaderTimeout (10 s), IdleTimeout (2 min) and MaxHeaderBytes
// (64 KiB), and deliberately leaves ReadTimeout and WriteTimeout at zero. Those
// count from the start of the request and bound the whole exchange, so they
// would end every long-lived response, such as a stream of server-sent events
// or a large download. Bound bodies and responses per request instead, with
// the middleware of package mw:
//
//   - [mw.BodyDeadline] bounds the time to receive a request body, and lets
//     the response outlast it;
//   - [mw.WriteDeadline] bounds the time to send a response from its start,
//     and lets a stream take the deadline over with one deadline per event;
//   - [mw.MaxBody] bounds the size of request bodies.
//
// Together with ReadHeaderTimeout and IdleTimeout, every phase of a
// connection is then bounded, and streams stay possible.
package server
