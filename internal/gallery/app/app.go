// Package app is the webui gallery: a demo application that renders the
// components of webui and runs its packages together, as the bed of
// integration tests and for checks in a real browser.
//
// New returns the application as an http.Handler with the full middleware
// stack, so tests drive it without a network; the gallery command serves it
// with the server package. The pages:
//
//   - / and /components/...: the layout (navbar, sidebar, header, footer)
//     and the Bootstrap components, in a section of pages that the sidebar
//     lists, with toasts appended by out-of-band swaps;
//   - /forms: fields with server-side validation, CSRF tokens,
//     Post/Redirect/Get and notices, for plain and htmx requests;
//   - /dialogs and /disclosure: dialogs, accordions and dropdowns that work
//     without script;
//   - /htmx: fragment swaps, out-of-band swaps, an error answer retargeted
//     into the alerts region, a confirmed delete, a search and a poll;
//   - /live: server-sent event streams that end with an end event;
//   - /login, /logout and /notes: sessions, pre-authentication CSRF tokens
//     and an authentication guard;
//   - /bootstrap-js: the opt-in Bootstrap script with its components, from a
//     second theme asset set.
//
// The demo data (tasks, a project name) lives in memory, bounded, and resets
// when the process restarts. Notes live in the session that wrote them.
package app

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/protolambda/webui/asset"
	"github.com/protolambda/webui/mw"
	"github.com/protolambda/webui/session"
	"github.com/protolambda/webui/sse"
	"github.com/protolambda/webui/theme"
	"github.com/protolambda/webui/web"
)

const (
	// DefaultLiveLifetime is how long a live stream runs before it ends with
	// an end event, unless Options.LiveLifetime says otherwise.
	DefaultLiveLifetime = 2 * time.Minute

	// DemoPassword is the password that the login demo accepts, with any
	// name. The login page shows it: the gallery has no real accounts.
	DemoPassword = "gallery"

	// BootstrapJSPrefix is the URL prefix of the second theme asset set,
	// which adds Bootstrap's script for the opt-in page.
	BootstrapJSPrefix = "/assets/webui-js/"

	// maxBody bounds every request body (mw.MaxBody); forms are bounded
	// further by formLimits.
	maxBody = 256 << 10

	// Live streams per client address group and in total.
	streamsPerClient = 4
	streamsTotal     = 256
)

// ErrOptions reports Options that New refuses.
var ErrOptions = errors.New("gallery: invalid options")

// Options configure New.
type Options struct {
	// Sites are the origins that the gallery is served at, such as
	// http://127.0.0.1:8080; requests naming another host are refused
	// (mw.HostCheck), and state-changing requests must come from one of
	// them (mw.SameOrigin). At least one is required.
	Sites []mw.Site

	// InsecureCookies drops the Secure attribute and the __Host- prefix of
	// the session and pre-authentication cookies, for plain http. It is
	// allowed only when every site is a plain-http site, which mw.ParseSite
	// accepts for loopback hosts only.
	InsecureCookies bool

	// Logger receives the request log and the application's records. Nil
	// means slog.Default at the time of New.
	Logger *slog.Logger

	// Now is the clock of sessions, pre-authentication tokens and the live
	// clock. Nil means time.Now.
	Now func() time.Time

	// LiveLifetime is how long a live stream runs before it ends. Zero means
	// DefaultLiveLifetime.
	LiveLifetime time.Duration
}

// Account is the value of a gallery session: the name the visitor signed in
// with, and the notes written in the session, which end with it.
type Account struct {
	Name  string
	Notes []string
}

// App is the gallery application. Create it with New. It is safe for
// concurrent use.
type App struct {
	handler  http.Handler
	mux      *http.ServeMux
	errs     web.Errors
	logger   *slog.Logger
	now      func() time.Time
	started  time.Time
	lifetime time.Duration

	assets   *asset.Set
	jsAssets *asset.Set

	sessions *session.Store[Account]
	preauth  *session.PreAuth
	streams  *sse.Limiter

	tasks       *taskList
	project     project
	noteChanges notifier
	toasts      atomic.Int64

	stop     chan struct{}
	stopOnce sync.Once
}

// New returns the gallery for opts. It fails with an error wrapping
// ErrOptions for invalid options.
func New(opts Options) (*App, error) {
	if len(opts.Sites) == 0 {
		return nil, fmt.Errorf("%w: no site", ErrOptions)
	}
	for _, s := range opts.Sites {
		if s.IsZero() {
			return nil, fmt.Errorf("%w: zero site", ErrOptions)
		}
		if opts.InsecureCookies && s.HTTPS() {
			return nil, fmt.Errorf("%w: insecure cookies need plain-http loopback sites", ErrOptions)
		}
	}
	if opts.LiveLifetime < 0 {
		return nil, fmt.Errorf("%w: negative live lifetime", ErrOptions)
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	lifetime := opts.LiveLifetime
	if lifetime == 0 {
		lifetime = DefaultLiveLifetime
	}
	if err := liveTiming(lifetime).Validate(sse.DefaultWriteWindow); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOptions, err)
	}

	assets, err := theme.Assets(theme.Options{})
	if err != nil {
		return nil, fmt.Errorf("gallery: theme assets: %w", err)
	}
	jsAssets, err := theme.Assets(theme.Options{Prefix: BootstrapJSPrefix, BootstrapJS: true})
	if err != nil {
		return nil, fmt.Errorf("gallery: theme assets with Bootstrap JS: %w", err)
	}
	sessions, err := session.NewStore(session.Options[Account]{
		Insecure:    opts.InsecureCookies,
		MaxSessions: 256,
		Passive:     passive,
		OnEndReason: func(id string, reason session.EndReason, _ *Account) {
			logger.Info("session ended", "session", id, "reason", reason)
		},
		Now: opts.Now,
	})
	if err != nil {
		return nil, fmt.Errorf("gallery: sessions: %w", err)
	}
	preauth, err := session.NewPreAuth(session.PreAuthOptions{Insecure: opts.InsecureCookies, Now: opts.Now})
	if err != nil {
		return nil, fmt.Errorf("gallery: pre-authentication: %w", err)
	}

	a := &App{
		logger:   logger,
		now:      now,
		started:  now(),
		lifetime: lifetime,
		assets:   assets,
		jsAssets: jsAssets,
		sessions: sessions,
		preauth:  preauth,
		streams:  sse.NewLimiter(streamsPerClient, streamsTotal),
		tasks:    newTaskList(),
		stop:     make(chan struct{}),
	}
	a.project.set(defaultProject)
	a.errs = web.Errors{Document: a.errorPage, Home: "/"}
	a.mux = a.routes()
	a.handler = mw.Chain(a.mux,
		mw.LogRequests(mw.RequestLog{Logger: logger, Routes: a.mux}),
		a.errs.Recover,
		mw.BodyDeadline(mw.DefaultBodyTimeout),
		mw.WriteDeadline(mw.DefaultWriteTimeout),
		mw.SecurityHeaders(mw.Headers{}),
		mw.HostCheck(opts.Sites...),
		mw.SameOrigin(mw.OriginPolicy{Deny: a.errs.Handle(refuseCrossOrigin)}),
		mw.MaxBody(maxBody),
		a.sessions.Middleware,
	)
	return a, nil
}

// ServeHTTP serves the gallery.
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.handler.ServeHTTP(w, r)
}

// Shutdown ends the live streams for a server shutdown: they return without
// an end event, so pages reconnect once the server is back, and streams
// that start later return at once. Register it with server.OnShutdown. It is
// idempotent.
func (a *App) Shutdown() {
	a.stopOnce.Do(func() { close(a.stop) })
}

// Sweep ends expired sessions and returns how many it ended. Call it
// periodically from a goroutine the caller owns, so that the live streams
// of expired sessions end on time.
func (a *App) Sweep() int {
	return a.sessions.Sweep()
}

// Mux returns the router of the gallery, for route checks in tests
// (webuitest.MuxRoutes). Register nothing on it.
func (a *App) Mux() *http.ServeMux {
	return a.mux
}

// routes registers the pages of the gallery.
func (a *App) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle(a.assets.Pattern(), a.assets)
	mux.Handle(a.jsAssets.Pattern(), a.jsAssets)
	handle := func(pattern string, fn web.HandlerFunc) {
		mux.Handle(pattern, a.errs.Handle(fn))
	}

	handle("GET /{$}", a.home)
	handle("GET /components", a.components)
	handle("GET /components/controls", a.componentsControls)
	handle("GET /components/content", a.componentsContent)
	handle("GET /components/feedback", a.componentsFeedback)
	handle("GET /components/toast", a.toast)
	handle("GET /components/toast/dismiss", a.dismissToast)

	handle("GET /forms", a.forms)
	handle("POST /forms", a.submitTask)

	handle("GET /dialogs", a.dialogs)
	handle("POST /dialogs/rename", a.renameProject)
	handle("POST /dialogs/reset", a.resetProject)

	handle("GET /disclosure", a.disclosure)
	handle("GET /disclosure/time", a.disclosureTime)

	handle("GET /htmx", a.htmx)
	handle("GET /htmx/tasks", a.htmxTasks)
	handle("POST /htmx/tasks", a.htmxAddTask)
	handle("POST /htmx/tasks/{id}/delete", a.htmxDeleteTask)
	handle("POST /htmx/fail", a.htmxFail)
	handle("GET /htmx/search", a.htmxSearch)
	handle("GET /htmx/poll", a.htmxPoll)

	handle("GET /live", a.live)
	// Event streams answer a missing session with an end event, never with
	// the login redirect of the guard below: an EventSource that is
	// redirected to a page fails and reconnects forever.
	handle("GET /live/clock", a.liveClock)
	handle("GET /notes/live", a.notesLive)

	guard := a.errs.RequireAuth("/login", a.signedIn)
	mux.Handle("GET /notes", guard(a.errs.Handle(a.notes)))
	mux.Handle("POST /notes", guard(a.errs.Handle(a.addNote)))
	handle("GET /login", a.loginForm)
	handle("POST /login", a.login)
	handle("POST /logout", a.logout)

	handle("GET /bootstrap-js", a.bootstrapJS)

	mux.Handle("/", a.errs.NotFound())
	return mux
}

// passive reports the requests that do not count as use of a session:
// polls and event streams, which a page left open would otherwise make to
// keep its session alive forever.
func passive(r *http.Request) bool {
	switch r.URL.Path {
	case "/live/clock", "/notes/live", "/htmx/poll":
		return true
	}
	return false
}

// refuseCrossOrigin answers requests that mw.SameOrigin refuses, with the
// error page of the gallery.
func refuseCrossOrigin(http.ResponseWriter, *http.Request) error {
	return mw.ErrCrossOrigin
}
