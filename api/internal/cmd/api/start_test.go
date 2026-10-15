package api

import (
	"os"
	"path/filepath"
	"testing"
)

// clearEnv unsets the vars for the test and restores them afterwards.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"CRITICAL_DSN", "CRITICAL_SEEDS_DIR", "CRITICAL_ADDR", "CRITICAL_CATALOGUE"} {
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
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg != (Config{DSN: "/tmp/x.db", SeedsDir: "/tmp/seeds", Addr: ":9999", Catalogue: "/srv/catalogue.json"}) {
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
	for _, c := range []Config{{"", "s", ":1", "c"}, {"d", "", ":1", "c"}, {"d", "s", "", "c"}, {"d", "s", ":1", ""}} {
		if c.Validate() == nil {
			t.Fatalf("expected error for %+v", c)
		}
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
