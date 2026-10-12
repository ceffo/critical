package api

import (
	"os"
	"testing"
)

// clearEnv unsets the vars for the test and restores them afterwards.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"CRITICAL_DSN", "CRITICAL_SEEDS_DIR", "CRITICAL_ADDR"} {
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
	if cfg != (Config{DSN: "critical.db", SeedsDir: "seeds", Addr: ":8080"}) {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadConfig_Overrides(t *testing.T) {
	clearEnv(t)
	t.Setenv("CRITICAL_DSN", "/tmp/x.db")
	t.Setenv("CRITICAL_SEEDS_DIR", "/tmp/seeds")
	t.Setenv("CRITICAL_ADDR", ":9999")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg != (Config{DSN: "/tmp/x.db", SeedsDir: "/tmp/seeds", Addr: ":9999"}) {
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
	for _, c := range []Config{{"", "s", ":1"}, {"d", "", ":1"}, {"d", "s", ""}} {
		if c.Validate() == nil {
			t.Fatalf("expected error for %+v", c)
		}
	}
}
