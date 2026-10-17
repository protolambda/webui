// Package webuitest holds test helpers for applications built on webui,
// layered on chord's ct testing packages (ct, ct/cthtml, ct/cthttp) and
// mustbe assertions.
//
// The helpers do not wrap ct: they return [ct.Rule], [ct.Query],
// [*ct.Subject] and [assertion.Assertion] values and take
// [*cthttp.Result], so they mix freely with ct's own vocabulary and with
// any test runner that checks mustbe assertions (mustbe.WrapT, devtest).
//
// # Rules and contracts
//
// [Rules] bundles the document rules that every webui page satisfies: ct's
// structure rules, the content security policy (no inline script or style,
// no htmx scripting, safe and same-origin URLs, subresource integrity),
// CSRF fields and token leaks, htmx targets and SSE wiring, the no-JS
// dialog markup, and accessibility (icons hidden, controls named, no
// nested controls, rectangular tables, safe external links). Each rule is
// also available alone. Rules are pure values: they never call the test,
// and their messages name locations and attribute names, never secret
// values.
//
// [Shell] describes what every page of an application has in common: the
// document head that webui's page.Document renders (charset, viewport,
// title, color scheme, icon, the htmx-config meta element, stylesheets
// and deferred scripts with integrity) and a main landmark.
// [DefaultShell] builds it from asset sets. [ShellContract],
// [PageContract] and [FragmentContract] apply shell and rules to a
// subject.
//
// # Requests, clients and responses
//
// [Get], [Post] and [Request] build browser-realistic requests on
// [BaseURL], bound to the test's context: fetch metadata on every request,
// Origin on state-changing ones, and options for htmx requests and for
// the negative cases ([FetchSite], [NoFetchMetadata], [Host], ...). Bodies
// are [Form], [Multipart] and [Parts]. [Serve] runs a handler on a request.
//
// [Client] is the handler-level harness: it serves requests through a
// handler, prepares them (for example to attach a session), adds the
// CSRF field to forms, checks pages against a contract ([Client.Page]),
// and submits the page's own forms as a browser would ([FormData],
// [Client.Submit]). [Browser] does the same over a real test server with
// TLS and a cookie jar, for full-stack tests.
//
// [Location], [HXRedirect], [HXRetarget], [Cookie], [NoCookies] and
// [SecurityHeaders] assert responses; cookie values are never printed.
// [CrossSiteProbes], [RejectsCrossSite], [OpenRedirectVectors] and
// [Hostile] supply attack inputs. [Crawl] follows an application's links
// and checks every page and target it finds. [Logs] captures log records,
// and [Logs.NotLogged] checks that no secret reached them.
//
// [Descendants], [Elements], [AttrIs], [HasClass] and [Closest] walk ct
// nodes, for custom rules ([ct.CustomRule]).
//
// # Event streams
//
// [ParseEvents] parses a complete text/event-stream as browsers do, and
// [Event.Fragment] parses an event's data as HTML. [ServeStream] runs a
// streaming handler over an in-memory connection, usable inside
// testing/synctest bubbles, and returns a [Stream] that reads events as
// they arrive.
//
// # Runners
//
// Drivers (request builders, clients, streams, the crawler) take a [TB],
// which *testing.T and devtest.T satisfy; they fail the test only on
// infrastructure errors. [PrintT] prints failures instead, for Example
// tests. [All] combines assertions and reports every failure. Never use
// be.Not on a selection: it also passes when the subject fails to load;
// use [ct.Selection.None].
package webuitest
