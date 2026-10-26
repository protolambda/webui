// Package webuitest holds test helpers for applications built on webui,
// layered on chord's ct testing packages (ct, ct/cthtml, ct/cthttp) and
// mustbe assertions.
//
// The helpers do not wrap ct: they return [ct.Rule], [ct.Query],
// [*ct.Subject] and [assertion.Assertion] values and take
// [*cthttp.Result], so they mix freely with ct's own vocabulary, and their
// assertions with any runner that checks mustbe assertions (mustbe.WrapT,
// devtest).
//
// # Rules and contracts
//
// [Rules] bundles the document rules that every webui page satisfies: ct's
// structure rules, the content security policy (no inline script or style,
// no htmx scripting, safe and same-origin URLs, subresource integrity),
// CSRF fields and token leaks, htmx targets and SSE wiring, the no-JS
// dialog markup, and accessibility (icons hidden, controls named, no
// nested controls, rectangular tables, safe external links). Each rule is
// also available alone. A fragment is checked as htmx runs it, under the
// htmx configuration of the page around it: webui's, the zero hx2.Config,
// unless [WithHTMXConfig] names another. Rules are pure values: they never
// call the test, and their messages name locations and attribute names,
// never secret values.
//
// [Shell] describes what every page of an application has in common: the
// document head that webui's page.Document renders (charset, viewport,
// title, color scheme, icon, the htmx-config meta element, stylesheets,
// and deferred scripts and modules with integrity) and a main landmark.
// [DefaultShell] builds it from asset sets; the default contracts of
// [Client] and [Crawl] take it as [Client.Shell] and [CrawlOptions.Shell],
// for an application that loads other sets than the default theme's.
// [ShellContract], [PageContract] and [FragmentContract] apply shell and
// rules to a subject; [LayoutRegions] checks the regions of page.Layout,
// whose ids (page.MainID and the others) [DocumentTargets] lists for
// fragments.
//
// # Requests, clients and responses
//
// [Get], [Post] and [Request] build browser-realistic requests on
// [BaseURL], bound to the test's context: fetch metadata on every request,
// Origin on state-changing ones, and options for htmx requests and for
// the negative cases ([FetchSite], [NoFetchMetadata], [Host], ...). Bodies
// are [Form], [Multipart] and [Parts]. [Serve] runs a handler on a request.
// A [Client] and a [Browser] build their requests on [Client.Base] and
// [BrowserOptions.Base] instead, for an application tested at another
// site, such as a plain http loopback site; as [SiteSender]s, they report
// it to [Crawl].
//
// [Client] is the handler-level harness: it serves requests through a
// handler, prepares them (for example to attach a session), adds the
// CSRF field to hand-built posts, checks pages against a contract
// ([Client.Page]), submits the page's own forms as a browser would
// ([FormData], [Client.Submit], with [PageURL] for actions relative to the
// page), and sends the requests that htmx sends for the page's elements
// ([HTMXData], [Client.SubmitHTMX]: method and URL of hx-post, hx-delete
// and the others, hx-vals, hx-include, hx-params, and the HX-* headers). A
// Client keeps no cookies: attach state to its requests (Prepare,
// [SendCookie], or [SendCookies] with the cookies of an earlier response,
// which [CookieValue] reads one by one). [Browser] is the same harness
// over a real test server with TLS and a cookie jar, for full-stack tests
// and for flows that need cookies, such as logins.
//
// [Location], [HXRedirect], [HXRetarget], [Cookie], [NoCookie],
// [NoCookies] and [SecurityHeaders] assert responses; cookie values are
// never printed. [StrictPolicy] and [PolicyFor] are the security headers
// of pages, [AssetPolicy] and [AssetPolicyFor] those of asset files.
// [CrossSiteProbes], [RejectsCrossSite], [OpenRedirectVectors] and
// [Hostile] supply attack inputs. [Crawl] follows an application's links
// and checks every page and target it finds, with a contract per page
// when [CrawlOptions.ContractFor] is set. [Logs] captures log records,
// and [Logs.NotLogged] checks that no secret reached them.
//
// [AttrIs] and [HasClass] read ct nodes, for custom rules
// ([ct.CustomRule]) next to [ct.Elements], [ct.Descendants] and
// [ct.Closest].
//
// # Event streams
//
// [ParseEvents] parses a complete text/event-stream as browsers do, and
// [Event.Fragment] parses an event's data as HTML. [ServeStream] runs a
// streaming handler over an in-memory connection, usable inside
// testing/synctest bubbles, and returns a [Stream] that reads events as
// they arrive; [Client.Stream] does so for a client's handler, with its
// Prepare and Options, and [Browser.Stream] over the test server.
//
// # Runners
//
// Drivers (request builders, clients, streams, the crawler) take a [TB],
// which *testing.T and devtest.T satisfy, but not the MT of mustbe.WrapT
// (see [TB]); they fail the test only on infrastructure errors. [PrintT]
// prints failures instead, for Example tests. [All] combines assertions
// and reports every failure. Never use be.Not on a selection: it also
// passes when the subject fails to load; use [ct.Selection.None].
package webuitest
