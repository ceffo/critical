package api

import (
	"os"
	"path/filepath"
	"testing"
)

// devEnv is clearEnv plus CRITICAL_ENV=dev, the dev login of local runs,
// which needs no OAuth setting.
func devEnv(t *testing.T) {
	t.Helper()
	clearEnv(t)
	t.Setenv("CRITICAL_ENV", EnvDev)
}

// clearEnv unsets the vars for the test and restores them afterwards.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"CRITICAL_DSN", "CRITICAL_SEEDS_DIR", "CRITICAL_ADDR", "CRITICAL_CATALOGUE", "CRITICAL_WEB_DIR", "CRITICAL_STORAGE", "CRITICAL_ENV", "FLY_APP_NAME", "CRITICAL_GOOGLE_CLIENT_ID", "CRITICAL_GOOGLE_CLIENT_SECRET", "CRITICAL_PUBLIC_URL", "CRITICAL_BREVO_API_KEY", "CRITICAL_MAIL_FROM", "CRITICAL_MAIL_FROM_NAME"} {
		t.Setenv(k, "") // registers restore of the original value
		_ = os.Unsetenv(k)
	}
}

// devDSN is the default database: the compose PostgreSQL of `just dev`.
const devDSN = "postgres://critical:critical@localhost:5432/critical?sslmode=disable"

func TestLoadConfig_Defaults(t *testing.T) {
	devEnv(t)
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg != (Config{DSN: devDSN, SeedsDir: "seeds", Addr: ":8080", Catalogue: ".catalogue/catalogue.json", Env: EnvDev}) {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadConfig_Overrides(t *testing.T) {
	devEnv(t)
	t.Setenv("CRITICAL_DSN", "postgres://u:p@db:5432/x")
	t.Setenv("CRITICAL_SEEDS_DIR", "/tmp/seeds")
	t.Setenv("CRITICAL_ADDR", ":9999")
	t.Setenv("CRITICAL_CATALOGUE", "/srv/catalogue.json")
	t.Setenv("CRITICAL_WEB_DIR", "/app/web")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	want := Config{DSN: "postgres://u:p@db:5432/x", SeedsDir: "/tmp/seeds", Addr: ":9999", Catalogue: "/srv/catalogue.json", WebDir: "/app/web", Env: EnvDev}
	if cfg != want {
		t.Fatalf("overrides not applied: %+v", cfg)
	}
}

// Empty-but-set vars fall back to the default (same as the old os.Getenv chain).
func TestLoadConfig_EmptyUsesDefault(t *testing.T) {
	devEnv(t)
	t.Setenv("CRITICAL_DSN", "")
	cfg, err := loadConfig()
	if err != nil || cfg.DSN != devDSN {
		t.Fatalf("got %+v, %v", cfg, err)
	}
}

func TestConfigValidate_RejectsEmpty(t *testing.T) {
	for _, c := range []Config{
		{DSN: "", SeedsDir: "s", Addr: ":1", Catalogue: "c"},
		{DSN: "d", SeedsDir: "", Addr: ":1", Catalogue: "c"},
		{DSN: "d", SeedsDir: "s", Addr: "", Catalogue: "c"},
		{DSN: "d", SeedsDir: "s", Addr: ":1", Catalogue: ""},
	} {
		if c.Validate() == nil {
			t.Fatalf("expected error for %+v", c)
		}
	}
}

// ADR 0025 step 8: CRITICAL_STORAGE=memory selects the in-memory storage;
// step 8.3: it is refused on a fly.io machine (FLY_APP_NAME, set by fly on
// every machine of staging and production) and in the production image
// (CRITICAL_WEB_DIR), and any other value is refused.
func TestConfigValidate_Step8_3_InMemoryStorageOnlyOutsideFlyAndTheImage(t *testing.T) {
	base := Config{DSN: "d", SeedsDir: "s", Addr: ":1", Catalogue: "c", Env: EnvDev}
	with := func(f func(*Config)) Config {
		c := base
		f(&c)
		return c
	}
	cases := []struct {
		name string
		cfg  Config
		ok   bool
	}{
		{"database by default", base, true},
		{"memory for e2e", with(func(c *Config) { c.Storage = StorageMemory }), true},
		{"memory on fly", with(func(c *Config) {
			c.Storage, c.FlyApp, c.Env = StorageMemory, "critical-hit", ""
			c.GoogleClientID, c.GoogleClientSecret, c.PublicURL = "id", "secret", "https://critical-hit.fly.dev"
		}), false},
		{"memory in the production image", with(func(c *Config) { c.Storage, c.WebDir = StorageMemory, "/app/web" }), false},
		{"unknown storage", with(func(c *Config) { c.Storage = "sqlite" }), false},
		{"database on fly", with(func(c *Config) {
			c.FlyApp, c.WebDir, c.Env = "critical-hit", "/app/web", ""
			c.GoogleClientID, c.GoogleClientSecret, c.PublicURL = "id", "secret", "https://critical-hit.fly.dev"
		}), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.cfg.Validate(); (err == nil) != c.ok {
				t.Fatalf("Validate(%+v) = %v, want ok=%v", c.cfg, err, c.ok)
			}
		})
	}
}

// The storage and the fly marker are read from the environment, and the
// refusal is loadConfig's, before anything is opened.
func TestLoadConfig_InMemoryStorage(t *testing.T) {
	devEnv(t)
	t.Setenv("CRITICAL_STORAGE", "memory")
	cfg, err := loadConfig()
	if err != nil || cfg.Storage != StorageMemory {
		t.Fatalf("got %+v, %v", cfg, err)
	}
	t.Setenv("FLY_APP_NAME", "critical-hit-staging")
	if _, err := loadConfig(); err == nil {
		t.Fatal("CRITICAL_STORAGE=memory on a fly machine must be refused")
	}
}

// A relative catalogue path is resolved against the nearest directory with a
// .git entry (a worktree has a .git file); outside a checkout it stays
// relative; an absolute path is kept.
func TestResolveFromRepoRoot(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "api", "cmd")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: elsewhere"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ path, cwd, want string }{
		{".catalogue/catalogue.json", sub, filepath.Join(root, ".catalogue/catalogue.json")},
		{".catalogue/catalogue.json", root, filepath.Join(root, ".catalogue/catalogue.json")},
		{"/srv/catalogue.json", sub, "/srv/catalogue.json"},
	}
	for _, c := range cases {
		if got := resolveFromRepoRoot(c.path, c.cwd); got != c.want {
			t.Errorf("resolveFromRepoRoot(%q, %q) = %q, want %q", c.path, c.cwd, got, c.want)
		}
	}
	outside := t.TempDir()
	if got := resolveFromRepoRoot("c.json", outside); got != "c.json" {
		t.Errorf("outside a checkout: got %q, want it relative to cwd", got)
	}
}

// ADR 0029 step 8.2: the dev login exists only when the environment is dev
// (CRITICAL_ENV=dev) and no OAuth setting is set; anywhere else (unset or
// another environment, the image run outside fly included) a missing or
// partial OAuth setting stops the start. On a hosted app (FLY_APP_NAME, set by
// fly on every machine) the dev environment itself is refused. All three
// settings select the Google login with the redirect, host and Secure cookies
// of the public URL.
func TestConfigAuth_Step8_2_RefusesToStartWithoutOAuthUnlessDev(t *testing.T) {
	base := Config{DSN: "d", SeedsDir: "s", Addr: ":1", Catalogue: "c"}
	full := func(c Config) Config {
		c.GoogleClientID, c.GoogleClientSecret, c.PublicURL = "id", "secret", "https://critical-hit.fly.dev"
		return c
	}
	dev := base
	dev.Env = EnvDev
	onFly := base
	onFly.FlyApp = "critical-hit"

	refused := map[string]Config{
		"no settings, no environment (fails closed)": base,
		"no settings, another environment":           func() Config { c := base; c.Env = "staging"; return c }(),
		"no settings, the image outside fly":         func() Config { c := base; c.WebDir = "/app/web"; return c }(),
		"no settings on fly":                         onFly,
		"dev environment on fly, no settings":        func() Config { c := onFly; c.Env = EnvDev; return c }(),
		"dev environment on fly, with settings":      func() Config { c := full(onFly); c.Env = EnvDev; return c }(),
		"partial setting on fly":                     func() Config { c := onFly; c.GoogleClientID = "id"; return c }(),
		"partial setting in dev":                     func() Config { c := dev; c.PublicURL = "http://localhost:8080"; return c }(),
	}
	for name, c := range refused {
		if err := c.Validate(); err == nil {
			t.Errorf("%s: must not start", name)
		}
	}
	for _, bad := range []string{"critical-hit.fly.dev", "ftp://x", "https://x/app", "https://"} {
		c := full(onFly)
		c.PublicURL = bad
		if err := c.Validate(); err == nil {
			t.Fatalf("public URL %q accepted", bad)
		}
	}

	d, err := dev.auth()
	if err != nil || d.google != nil || d.secure || d.host != "" || d.cookieApp != "dev_1" {
		t.Fatalf("no settings in dev must be the dev login, got %+v, %v", d, err)
	}
	devImage := dev
	devImage.WebDir = "/app/web"
	if err := devImage.Validate(); err != nil {
		t.Fatalf("the image with CRITICAL_ENV=dev (image-smoke) must start: %v", err)
	}
	g, err := full(onFly).auth()
	if err != nil {
		t.Fatal(err)
	}
	if g.google == nil || g.google.RedirectURL != "https://critical-hit.fly.dev/api/auth/google/callback" ||
		g.google.ClientID != "id" || g.google.ClientSecret != "secret" || g.host != "critical-hit.fly.dev" || !g.secure ||
		g.cookieApp != "critical-hit.fly.dev" {
		t.Fatalf("unexpected Google login config: %+v %+v", g, g.google)
	}
	local, err := Config{GoogleClientID: "id", GoogleClientSecret: "s", PublicURL: "http://localhost:8080/"}.auth()
	if err != nil || local.secure || local.host != "localhost:8080" || local.cookieApp != "localhost:8080" {
		t.Fatalf("plain HTTP public URL: %+v, %v", local, err)
	}
}

// The dev login's opt-in is read from the environment: without
// CRITICAL_ENV=dev and without OAuth settings loadConfig refuses.
func TestLoadConfig_Step8_2_DevLoginNeedsCriticalEnvDev(t *testing.T) {
	clearEnv(t)
	if _, err := loadConfig(); err == nil {
		t.Fatal("no OAuth settings and no CRITICAL_ENV must not start")
	}
	t.Setenv("CRITICAL_ENV", EnvDev)
	if _, err := loadConfig(); err != nil {
		t.Fatalf("CRITICAL_ENV=dev must start in dev login mode: %v", err)
	}
}

// ADR 0024 step 8.3: the login mode /api/health reports is the Google login
// only when all three settings are set; a hosted app never has the dev login.
func TestConfigAuth_Step8_3_HealthLoginMode(t *testing.T) {
	dev := Config{DSN: "d", SeedsDir: "s", Addr: ":1", Catalogue: "c", Env: EnvDev}
	a, err := dev.auth()
	if err != nil || a.login() != "dev" {
		t.Fatalf("dev: %q, %v", a.login(), err)
	}
	g := Config{
		DSN: "d", SeedsDir: "s", Addr: ":1", Catalogue: "c", FlyApp: "critical-hit",
		GoogleClientID: "id", GoogleClientSecret: "secret", PublicURL: "https://critical-hit.fly.dev",
	}
	a, err = g.auth()
	if err != nil || a.login() != "google" {
		t.Fatalf("google: %q, %v", a.login(), err)
	}
}

// ADR 0024 step 1.4: /api/health reports the environment, from CRITICAL_ENV:
// dev, staging and production as named, anything else as unset.
func TestConfig_Step1_4_ReportedEnv(t *testing.T) {
	for env, want := range map[string]string{
		EnvDev: "dev", EnvStaging: "staging", EnvProduction: "production",
		"": "", "prod": "", "Staging": "", "qa": "",
	} {
		if got := (Config{Env: env}).reportedEnv(); got != want {
			t.Errorf("CRITICAL_ENV=%q reports %q, want %q", env, got, want)
		}
	}
}

// ADR 0029 step 8.2 with ADR 0024 step 1.4: the staging and production
// names change nothing about the login: without OAuth settings they are
// refused like any other value, only dev opts into the dev login.
func TestConfigAuth_Step8_2_StagingAndProductionNamesAreNotTheDevLogin(t *testing.T) {
	for _, env := range []string{EnvStaging, EnvProduction} {
		c := Config{DSN: "d", SeedsDir: "s", Addr: ":1", Catalogue: "c", Env: env}
		if err := c.Validate(); err == nil {
			t.Errorf("CRITICAL_ENV=%s without OAuth settings must not start", env)
		}
		c.GoogleClientID, c.GoogleClientSecret, c.PublicURL = "id", "secret", "https://critical-hit.fly.dev"
		c.FlyApp = "critical-hit"
		a, err := c.auth()
		if err != nil || a.login() != "google" {
			t.Errorf("CRITICAL_ENV=%s with settings: %q, %v", env, a.login(), err)
		}
	}
}
