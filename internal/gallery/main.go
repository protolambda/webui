// Command gallery serves the webui gallery, a demo application that renders
// every component of webui, for checks in a real browser:
//
//	go run ./internal/gallery
//	go run ./internal/gallery -addr 127.0.0.1:9090
//	go run ./internal/gallery -addr 0.0.0.0:8080 -site https://gallery.example.com
//
// Without -site, a loopback listen address serves the sites http://HOST:PORT
// and http://localhost:PORT, with cookies for plain http. A listener that
// the network reaches needs -site: the origin that browsers use, usually an
// https URL of a reverse proxy, as loopback sites must stay on loopback
// listeners.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/protolambda/webui/internal/gallery/app"
	"github.com/protolambda/webui/mw"
	"github.com/protolambda/webui/server"
)

// sweepInterval is how often expired sessions are ended.
const sweepInterval = time.Minute

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	var sites []string
	flag.Func("site", "origin the gallery is served at, e.g. https://gallery.example.com (repeatable; default: the loopback listen address and localhost)", func(v string) error {
		sites = append(sites, v)
		return nil
	})
	debug := flag.Bool("debug", false, "log at debug level")
	flag.Parse()

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ln, err := net.Listen("tcp", *addr)
	if err == nil {
		err = run(mw.WithLogger(ctx, logger), logger, ln, sites)
	}
	if err != nil {
		logger.Error("gallery failed", "err", err)
		os.Exit(1)
	}
	logger.Info("gallery stopped")
}

// run serves the gallery on ln until ctx ends, and owns ln: it is closed
// when run returns. Without siteURLs, ln must be a loopback listener, whose
// default sites gallerySites derives.
//
// run owns the goroutine that sweeps expired sessions, and registers the
// gallery's Shutdown with the server, so that live streams end when the
// server shuts down instead of holding it up until the shutdown timeout.
func run(ctx context.Context, logger *slog.Logger, ln net.Listener, siteURLs []string) error {
	sites, err := gallerySites(ln.Addr(), siteURLs)
	if err == nil {
		err = server.CheckLoopback(ln.Addr(), sites...)
	}
	// Cookies drop Secure and the __Host- prefix only when every site is
	// plain http, which mw.ParseSite accepts for loopback hosts only, and
	// CheckLoopback keeps on a loopback listener.
	insecure := true
	for _, s := range sites {
		insecure = insecure && !s.HTTPS()
	}
	var gallery *app.App
	if err == nil {
		gallery, err = app.New(app.Options{Sites: sites, InsecureCookies: insecure, Logger: logger})
	}
	if err != nil {
		return errors.Join(err, ln.Close())
	}

	var wg sync.WaitGroup
	defer wg.Wait()
	sweepCtx, endSweep := context.WithCancel(ctx)
	defer endSweep()
	wg.Go(func() { sweep(sweepCtx, gallery) })

	srv := server.New(gallery, server.Options{Logger: logger})
	for _, s := range sites {
		logger.Info("gallery serving", "url", s.Origin()+"/")
	}
	return server.Run(ctx, srv, server.LimitConns(ln, server.ConnLimit{Logger: logger}), server.OnShutdown(gallery.Shutdown))
}

// sweep ends expired sessions every sweepInterval until ctx ends.
func sweep(ctx context.Context, gallery *app.App) {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			gallery.Sweep()
		}
	}
}

// gallerySites parses the configured sites, or derives the default sites
// of a loopback listener: its own address and localhost, over plain http.
func gallerySites(addr net.Addr, siteURLs []string) ([]mw.Site, error) {
	if len(siteURLs) > 0 {
		sites := make([]mw.Site, 0, len(siteURLs))
		for _, raw := range siteURLs {
			s, err := mw.ParseSite(raw)
			if err != nil {
				return nil, fmt.Errorf("-site: %w", err)
			}
			sites = append(sites, s)
		}
		return sites, nil
	}
	tcp, ok := addr.(*net.TCPAddr)
	if !ok || !server.IsLoopbackAddr(addr) {
		return nil, errors.New("the listener is not on a loopback address: set -site to the origin that browsers use")
	}
	port := strconv.Itoa(tcp.Port)
	sites := make([]mw.Site, 0, 2)
	for _, host := range []string{tcp.IP.String(), "localhost"} {
		s, err := mw.ParseSite("http://" + net.JoinHostPort(host, port))
		if err != nil {
			return nil, fmt.Errorf("default site: %w", err)
		}
		sites = append(sites, s)
	}
	return sites, nil
}
