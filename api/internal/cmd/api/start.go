// Package api is the boot wiring for the api binary. cmd/api/main.go is a
// one-liner that calls Start(); everything else — env parsing, logger,
// composition root, signal handling — happens here.
package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/caarlos0/env/v11"

	"github.com/ceffo/critical/api/internal/adapters/changelogadpt"
	"github.com/ceffo/critical/api/internal/adapters/googleadpt"
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
	// Changelog is the JSON file the release build baked into the image
	// (scripts/changelog.sh, ADR 0039 step 6). Unset, as in `just dev`
	// where there is no tag history, the changelog is empty.
	Changelog string `env:"CRITICAL_CHANGELOG"`
	// Storage selects where the data lives: empty, the database at DSN;
	// StorageMemory, the in-memory storage of the e2e runs (ADR 0025 step 8),
	// which opens no database and keeps nothing after the process exits.
	Storage string `env:"CRITICAL_STORAGE"`
	// FlyApp is set by fly.io on every machine it runs (FLY_APP_NAME): the
	// in-memory storage is refused there (ADR 0025 step 8.3).
	FlyApp string `env:"FLY_APP_NAME"`
	// Env is the environment name: EnvDev (local runs), EnvStaging or
	// EnvProduction (the fly apps' [env], reported by /api/health, ADR 0024
	// step 1.4). Only EnvDev opts into the dev login when no OAuth setting is
	// set (ADR 0029 step 8.2). Unset or any other value, a missing OAuth setting stops the start, so
	// the image run anywhere without settings fails closed. `just dev`, the
	// e2e stacks and image-smoke set it; it is refused on fly.io.
	Env string `env:"CRITICAL_ENV"`
	// The Google login's OAuth client and the app's public URL (ADR 0029
	// step 8): all three set, Google login; none set, the dev login, only
	// with Env=EnvDev and never on a hosted app (FlyApp set); a partial
	// setting is refused everywhere. On fly they are the app's secrets,
	// never in fly.toml.
	GoogleClientID     string `env:"CRITICAL_GOOGLE_CLIENT_ID"`
	GoogleClientSecret string `env:"CRITICAL_GOOGLE_CLIENT_SECRET"`
	// PublicURL is the address users open (https://<host>, no path): the
	// Google redirect is PublicURL/api/auth/google/callback, the CSRF check
	// compares with its host, and https makes the cookies Secure.
	PublicURL string `env:"CRITICAL_PUBLIC_URL"`
	// The mail of the invites (ADR 0029 step 2.10): Brevo's API key and the
	// verified sender (address, and optionally its name). All unset, no mail
	// is sent and the server starts; on fly they are the app's secrets.
	BrevoAPIKey  string `env:"CRITICAL_BREVO_API_KEY"`
	MailFrom     string `env:"CRITICAL_MAIL_FROM"`
	MailFromName string `env:"CRITICAL_MAIL_FROM_NAME"`
}

// StorageMemory is the CRITICAL_STORAGE value of the in-memory storage.
const StorageMemory = "memory"

// EnvDev is the CRITICAL_ENV value of a local run (`just dev`, e2e,
// image-smoke): the only environment where the dev login may exist.
const EnvDev = "dev"

// The CRITICAL_ENV values of the two hosted apps (deploy/fly.*.toml). They
// only name the environment for /api/health: nothing else reads them.
const (
	EnvStaging    = "staging"
	EnvProduction = "production"
)

// reportedEnv is the environment /api/health reports: Env when it is one of
// the three known names, else empty (reported as "unknown", no pill).
func (c Config) reportedEnv() string {
	switch c.Env {
	case EnvDev, EnvStaging, EnvProduction:
		return c.Env
	}
	return ""
}

// commit is the commit the binary was built from, reported by /api/health
// (ADR 0024 step 1.4). The image build sets it with
// -ldflags "-X github.com/ceffo/critical/api/internal/cmd/api.commit=<sha>".
var commit = "unknown"

// version is the release version (X.Y.Z) the image was built as, reported by
// /api/health (ADR 0024 step 1.4). The release build computes it from the
// commits since the last version and sets it with
// -ldflags "-X github.com/ceffo/critical/api/internal/cmd/api.version=<X.Y.Z>";
// `just dev` and other builds leave it unknown.
var version = "unknown"

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
	if err := c.validateStorage(); err != nil {
		return err
	}
	if err := c.validateMail(); err != nil {
		return err
	}
	_, err := c.auth()
	return err
}

// authConfig is the login mode the settings select.
type authConfig struct {
	// google is nil in dev login mode.
	google *googleadpt.Config
	host   string
	secure bool
	// cookieApp names the auth cookies (middleware.SessionCookieName): the
	// public URL's host and port, or "dev_<listen port>" in dev login mode,
	// so apps reached on one host (localhost) keep separate sessions.
	cookieApp string
}

// auth checks the OAuth settings (ADR 0029 step 8): all or none. None is the
// dev login, which needs the explicit opt-in CRITICAL_ENV=dev (step 8.2: when
// the environment is not dev, missing OAuth is a start-up error). On a hosted
// app (FLY_APP_NAME, which fly.io sets on every machine of staging and
// production) the dev environment itself is refused, so the dev login can
// never be reachable there.
func (c Config) auth() (authConfig, error) {
	set := 0
	for _, v := range []string{c.GoogleClientID, c.GoogleClientSecret, c.PublicURL} {
		if v != "" {
			set++
		}
	}
	switch {
	case c.Env == EnvDev && c.FlyApp != "":
		return authConfig{}, fmt.Errorf("CRITICAL_ENV=%s is for local runs only, refused on fly.io (app %s)", EnvDev, c.FlyApp)
	case set == 0 && c.FlyApp != "":
		return authConfig{}, fmt.Errorf("CRITICAL_GOOGLE_CLIENT_ID, CRITICAL_GOOGLE_CLIENT_SECRET and CRITICAL_PUBLIC_URL are required on fly.io (app %s): the dev login is for local runs only (docs/deploy.md)", c.FlyApp)
	case set == 0 && c.Env != EnvDev:
		return authConfig{}, fmt.Errorf("CRITICAL_GOOGLE_CLIENT_ID, CRITICAL_GOOGLE_CLIENT_SECRET and CRITICAL_PUBLIC_URL are required unless CRITICAL_ENV=%s (the dev login of local runs; got CRITICAL_ENV=%q)", EnvDev, c.Env)
	case set == 0:
		return authConfig{cookieApp: "dev_" + listenPort(c.Addr)}, nil
	case set < 3:
		return authConfig{}, errors.New("CRITICAL_GOOGLE_CLIENT_ID, CRITICAL_GOOGLE_CLIENT_SECRET and CRITICAL_PUBLIC_URL must be set together (all for the Google login, none for the dev login)")
	}
	u, err := url.Parse(c.PublicURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return authConfig{}, fmt.Errorf("CRITICAL_PUBLIC_URL must be http(s)://<host>[:port] with no path, got %q", c.PublicURL)
	}
	base := u.Scheme + "://" + u.Host
	return authConfig{
		google: &googleadpt.Config{
			ClientID: c.GoogleClientID, ClientSecret: c.GoogleClientSecret,
			RedirectURL: base + "/api/auth/google/callback",
		},
		host:      u.Host,
		secure:    u.Scheme == "https",
		cookieApp: u.Host,
	}, nil
}

// login is the name /api/health reports for the login mode.
func (a authConfig) login() string {
	if a.google != nil {
		return "google"
	}
	return "dev"
}

// listenPort is the port of a listen address (":8080", "127.0.0.1:8080"),
// the address itself when it has none.
func listenPort(addr string) string {
	if _, port, err := net.SplitHostPort(addr); err == nil {
		return port
	}
	return addr
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
	auth, err := cfg.auth()
	if err != nil {
		return err
	}
	// A bad web dir stops the server before it touches the database.
	opts := router.Options{
		Commit: commit, Version: version, Env: cfg.reportedEnv(), Login: auth.login(), Host: auth.host, SecureCookies: auth.secure, CookieApp: auth.cookieApp,
	}
	// Like the web dir, a bad changelog stops the server before the database.
	opts.Changelog, err = changelogadpt.Load(cfg.Changelog)
	if err != nil {
		return err
	}
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
	mailer, err := cfg.notifier(log)
	if err != nil {
		return err
	}
	bs, err := composition.New(ctx, composition.Config{
		DSN: cfg.DSN, InMemory: cfg.Storage == StorageMemory,
		SeedsDir: cfg.SeedsDir, Catalogue: resolveFromRepoRoot(cfg.Catalogue, cwd),
		Google: auth.google, Mail: mailer,
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

	if auth.google == nil {
		log.Warn("auth: CRITICAL_ENV=dev without OAuth settings, dev login enabled (POST /api/auth/dev); never on a hosted app")
	} else {
		log.Info("auth: Google login", "public_url", cfg.PublicURL)
	}

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
