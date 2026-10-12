package actionlogsvc

import "fmt"

// Config holds the list-size knobs.
type Config struct {
	DefaultLimit int `env:"ACTION_LOG_DEFAULT_LIMIT" envDefault:"50"`
	MaxLimit     int `env:"ACTION_LOG_MAX_LIMIT" envDefault:"500"`
}

// DefaultConfig matches the envDefault tags; tests use it with NewWithConfig.
func DefaultConfig() *Config { return &Config{DefaultLimit: 50, MaxLimit: 500} }

// Validate enforces invariants.
func (c *Config) Validate() error {
	if c.DefaultLimit <= 0 || c.MaxLimit <= 0 {
		return fmt.Errorf("DefaultLimit and MaxLimit must be > 0")
	}
	if c.DefaultLimit > c.MaxLimit {
		return fmt.Errorf("DefaultLimit must be <= MaxLimit")
	}
	return nil
}

// clampLimit maps a requested limit (<= 0 meaning "default") into
// [1, MaxLimit].
func (c *Config) clampLimit(limit int) int {
	if limit <= 0 {
		return c.DefaultLimit
	}
	return min(limit, c.MaxLimit)
}
