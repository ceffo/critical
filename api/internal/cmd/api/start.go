// Package api is the boot wiring for the api binary. cmd/api/main.go is a
// one-liner that calls Start(); everything else — env parsing, logger,
// composition root, signal handling — happens here.
package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/caarlos0/env/v11"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/rest/handler/webhdl"
	"github.com/ceffo/critical/api/internal/rest/router"
	"github.com/ceffo/critical/api/internal/rest/server"
)

// Config is the binary-level config — derived from environment variables in
// Start. Each value has a sensible default so the app boots out of the box.
type Config struct {
	// DSN is the PostgreSQL database (ADR 0025 step 1.1); the default is the
	// compose database of `just dev`.
	DSN      string `env:"CRITICAL_DSN" envDefault:"postgres://critical:critical@localhost:5432/critical?sslmode=disable"`
	SeedsDir string `env:"CRITICAL_SEEDS_DIR" envDefault:"seeds"`
	Addr     string `env:"CRITICAL_ADDR" envDefault:":8080"`
	// Catalogue is the spell catalogue artifact (ADR 0021 step 4): `just
	// catalogue` fetches it to .catalogue/catalogue.json. A relative path is
	// resolved against the repo root (resolveFromRepoRoot).
	Catalogue string `env:"CRITICAL_CATALOGUE" envDefault:".catalogue/catalogue.json"`
	// WebDir is the built web app the server serves outside /api (ADR 0024
	// step 1). Unset, as in `just dev` where Vite serves it, it serves
	// nothing there.
	WebDir string `env:"CRITICAL_WEB_DIR"`
	// Storage selects where the data lives: empty, the database at DSN;
	// StorageMemory, the in-memory storage of the e2e runs (ADR 0025 step 8),
	// which opens no database and keeps nothing after the process exits.
	Storage string `env:"CRITICAL_STORAGE"`
	// FlyApp is set by fly.io on every machine it runs (FLY_APP_NAME): the
	// in-memory storage is refused there (ADR 0025 step 8.3).
	FlyApp string `env:"FLY_APP_NAME"`
}

// StorageMemory is the CRITICAL_STORAGE value of the in-memory storage.
const StorageMemory = "memory"

// commit is the commit the binary was built from, reported by /api/health
// (ADR 0024 step 1.4). The image build sets it with
// -ldflags "-X github.com/ceffo/critical/api/internal/cmd/api.commit=<sha>".
var commit = "unknown"

// Validate rejects empty required fields (e.g. CRITICAL_DSN set to "").
func (c Config) Validate() error {
	switch {
	case c.DSN == "":
		return errors.New("CRITICAL_DSN must not be empty")
	case c.SeedsDir == "":
		return errors.New("CRITICAL_SEEDS_DIR must not be empty")
	case c.Addr == "":
		return errors.New("CRITICAL_ADDR must not be empty")
	case c.Catalogue == "":
		return errors.New("CRITICAL_CATALOGUE must not be empty")
	}
	return c.validateStorage()
}

// validateStorage accepts the database (CRITICAL_STORAGE unset) or the
// in-memory storage, and refuses the in-memory storage where real data lives
// (ADR 0025 step 8.3): on a fly.io machine (staging and production run
// nowhere else) and in the production image (the only one serving the built
// web app, CRITICAL_WEB_DIR). Its data would vanish at the next restart.
func (c Config) validateStorage() error {
	switch c.Storage {
	case "":
		return nil
	case StorageMemory:
	default:
		return fmt.Errorf("CRITICAL_STORAGE must be unset or %q, got %q", StorageMemory, c.Storage)
	}
	switch {
	case c.FlyApp != "":
		return fmt.Errorf("CRITICAL_STORAGE=%s is for e2e runs only, refused on fly.io (app %s)", StorageMemory, c.FlyApp)
	case c.WebDir != "":
		return fmt.Errorf("CRITICAL_STORAGE=%s is for e2e runs only, refused in the production image (CRITICAL_WEB_DIR is set)", StorageMemory)
	}
	return nil
}

// loadConfig parses env (defaults come from envDefault tags) and validates.
func loadConfig() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse env: %w", err)
	}
	return cfg, cfg.Validate()
}

// resolveFromRepoRoot resolves a relative path against the repo root: the
// nearest directory at or above cwd holding a .git entry (a directory, or a
// file in a worktree). Outside a checkout (a deployment) it is left relative
// to cwd. An absolute path is returned as is.
func resolveFromRepoRoot(path, cwd string) string {
	if filepath.IsAbs(path) {
		return path
	}
	for dir := cwd; ; {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return filepath.Join(dir, path)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return path
		}
		dir = parent
	}
}

// Start boots the api: load env, build composition root, build router, run
// HTTP server, and shut everything down on SIGINT/SIGTERM. Returns the first
// non-shutdown error.
func Start() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	// A bad web dir stops the server before it touches the database.
	opts := router.Options{Commit: commit}
	if cfg.WebDir != "" {
		web, werr := webhdl.New(cfg.WebDir)
		if werr != nil {
			return werr
		}
		opts.Web = web
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("working directory: %w", err)
	}
	bs, err := composition.New(ctx, composition.Config{
		DSN: cfg.DSN, InMemory: cfg.Storage == StorageMemory,
		SeedsDir: cfg.SeedsDir, Catalogue: resolveFromRepoRoot(cfg.Catalogue, cwd),
	})
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
	srv := server.New(srvCfg, router.New(log, bs, opts))

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
