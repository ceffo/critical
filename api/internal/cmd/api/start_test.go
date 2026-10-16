package api

import (
	"os"
	"path/filepath"
	"testing"
)

// clearEnv unsets the vars for the test and restores them afterwards.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"CRITICAL_DSN", "CRITICAL_SEEDS_DIR", "CRITICAL_ADDR", "CRITICAL_CATALOGUE", "CRITICAL_WEB_DIR", "CRITICAL_STORAGE", "FLY_APP_NAME"} {
		t.Setenv(k, "") // registers restore of the original value
		_ = os.Unsetenv(k)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	clearEnv(t)
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg != (Config{DSN: "critical.db", SeedsDir: "seeds", Addr: ":8080", Catalogue: ".catalogue/catalogue.json"}) {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadConfig_Overrides(t *testing.T) {
	clearEnv(t)
	t.Setenv("CRITICAL_DSN", "/tmp/x.db")
	t.Setenv("CRITICAL_SEEDS_DIR", "/tmp/seeds")
	t.Setenv("CRITICAL_ADDR", ":9999")
	t.Setenv("CRITICAL_CATALOGUE", "/srv/catalogue.json")
	t.Setenv("CRITICAL_WEB_DIR", "/app/web")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	want := Config{DSN: "/tmp/x.db", SeedsDir: "/tmp/seeds", Addr: ":9999", Catalogue: "/srv/catalogue.json", WebDir: "/app/web"}
	if cfg != want {
		t.Fatalf("overrides not applied: %+v", cfg)
	}
}

// Empty-but-set vars fall back to the default (same as the old os.Getenv chain).
func TestLoadConfig_EmptyUsesDefault(t *testing.T) {
	clearEnv(t)
	t.Setenv("CRITICAL_DSN", "")
	cfg, err := loadConfig()
	if err != nil || cfg.DSN != "critical.db" {
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
	base := Config{DSN: "d", SeedsDir: "s", Addr: ":1", Catalogue: "c"}
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
		{"memory on fly", with(func(c *Config) { c.Storage, c.FlyApp = StorageMemory, "critical-prod" }), false},
		{"memory in the production image", with(func(c *Config) { c.Storage, c.WebDir = StorageMemory, "/app/web" }), false},
		{"unknown storage", with(func(c *Config) { c.Storage = "sqlite" }), false},
		{"database on fly", with(func(c *Config) { c.FlyApp, c.WebDir = "critical-prod", "/app/web" }), true},
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
	clearEnv(t)
	t.Setenv("CRITICAL_STORAGE", "memory")
	cfg, err := loadConfig()
	if err != nil || cfg.Storage != StorageMemory {
		t.Fatalf("got %+v, %v", cfg, err)
	}
	t.Setenv("FLY_APP_NAME", "critical-staging")
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
