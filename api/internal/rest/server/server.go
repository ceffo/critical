// Package server is a tiny http.Server wrapper. No application concerns —
// just lifecycle (ListenAndServe, Shutdown).
package server

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Config drives addr/timeouts.
type Config struct {
	Addr            string
	ReadHeader      time.Duration
	ShutdownTimeout time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{Addr: ":8080", ReadHeader: 5 * time.Second, ShutdownTimeout: 10 * time.Second}
}

// Server wraps *http.Server with named lifecycle methods.
type Server struct {
	cfg Config
	srv *http.Server
}

// New constructs a Server bound to handler.
func New(cfg Config, handler http.Handler) *Server {
	return &Server{
		cfg: cfg,
		srv: &http.Server{
			Addr:              cfg.Addr,
			Handler:           handler,
			ReadHeaderTimeout: cfg.ReadHeader,
		},
	}
}

// ListenAndServe blocks until ListenAndServe returns. ErrServerClosed is
// translated to nil so callers can `return s.ListenAndServe()` cleanly.
func (s *Server) ListenAndServe() error {
	if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown stops accepting new connections and waits for active ones up to
// cfg.ShutdownTimeout.
func (s *Server) Shutdown(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.ShutdownTimeout)
	defer cancel()
	return s.srv.Shutdown(ctx)
}
