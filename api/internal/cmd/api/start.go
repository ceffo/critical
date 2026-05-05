// Package api is the boot wiring for the api binary. cmd/api/main.go is a
// one-liner that calls Start(); everything else — env parsing, logger,
// composition root, signal handling — happens here.
package api

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/rest/router"
	"github.com/ceffo/critical/api/internal/rest/server"
)

// Config is the binary-level config — derived from environment variables in
// Start. Each value has a sensible default so the app boots out of the box.
type Config struct {
	DSN      string
	SeedsDir string
	Addr     string
}

// loadConfig reads env with defaults.
func loadConfig() Config {
	cfg := Config{DSN: "critical.db", SeedsDir: "seeds", Addr: ":8080"}
	if v := os.Getenv("CRITICAL_DSN"); v != "" {
		cfg.DSN = v
	}
	if v := os.Getenv("CRITICAL_SEEDS_DIR"); v != "" {
		cfg.SeedsDir = v
	}
	if v := os.Getenv("CRITICAL_ADDR"); v != "" {
		cfg.Addr = v
	}
	return cfg
}

// Start boots the api: load env, build composition root, build router, run
// HTTP server, and shut everything down on SIGINT/SIGTERM. Returns the first
// non-shutdown error.
func Start() error {
	cfg := loadConfig()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	bs, err := composition.New(ctx, composition.Config{DSN: cfg.DSN, SeedsDir: cfg.SeedsDir})
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if cerr := bs.Close(shutdownCtx); cerr != nil {
			log.Warn("composition close", "error", cerr)
		}
	}()

	srvCfg := server.DefaultConfig()
	srvCfg.Addr = cfg.Addr
	srv := server.New(srvCfg, router.New(log, bs))

	errCh := make(chan error, 1)
	go func() {
		log.Info("api listening", "addr", cfg.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown requested")
	case err := <-errCh:
		if err != nil {
			return err
		}
	}

	if err := srv.Shutdown(context.Background()); err != nil && !errors.Is(err, context.Canceled) {
		log.Warn("server shutdown", "error", err)
	}
	return nil
}
