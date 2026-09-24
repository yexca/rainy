// Package server assembles the HTTP router (Subsonic API, native API, embedded SPA),
// the cross-cutting middleware and graceful shutdown.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"rainy/internal/api"
	"rainy/internal/app"
	"rainy/internal/subsonic"
	"rainy/web"
)

// streamGrace is how long in-flight requests may continue after shutdown starts before
// their contexts are cancelled (ends SSE streams and long downloads so Shutdown finishes).
const streamGrace = 3 * time.Second

// New returns the configured *http.Server (not yet listening).
func New(a *app.App) *http.Server {
	base, cancel := context.WithCancel(context.Background())
	srv := &http.Server{
		Addr:              a.Cfg.ListenAddr(),
		Handler:           Handler(a),
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
		// No Read/WriteTimeout: audio streams, uploads and SSE are long-lived.
		BaseContext: func(net.Listener) context.Context { return base },
		ErrorLog:    slog.NewLogLogger(slog.Default().Handler(), slog.LevelWarn),
	}
	srv.RegisterOnShutdown(func() { time.AfterFunc(streamGrace, cancel) })
	return srv
}

// Handler builds the root router:
//
//	/rest/*  Subsonic API (CORS: any origin)
//	/api/*   native API (optional dev CORS)
//	/*       embedded single-page app
func Handler(a *app.App) http.Handler {
	r := chi.NewRouter()
	if a.Cfg.TrustProxy {
		r.Use(realIP)
	}
	r.Use(requestLogger, recoverer)

	r.With(restCORS, compress).Mount("/rest", subsonic.New(a).Routes())
	r.With(devCORS(a.Cfg.DevCORS), compress).Mount("/api", api.New(a).Routes())

	spa := newSPA(web.Dist)
	r.With(compress).Handle("/*", spa)
	return r
}

// Serve listens on srv.Addr and serves until ctx is cancelled, then shuts down gracefully
// (waiting at most shutdownTimeout for in-flight requests).
func Serve(ctx context.Context, srv *http.Server, shutdownTimeout time.Duration) error {
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", srv.Addr, err)
	}
	return ServeListener(ctx, srv, ln, shutdownTimeout)
}

// ServeListener is Serve on an existing listener.
func ServeListener(ctx context.Context, srv *http.Server, ln net.Listener, shutdownTimeout time.Duration) error {
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}
	slog.Info("shutting down HTTP server")
	sctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil {
		_ = srv.Close()
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	if err := <-errc; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
