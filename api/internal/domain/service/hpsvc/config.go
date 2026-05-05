package hpsvc

import "fmt"

// Config controls service-local knobs. Today only the history window is
// configurable; the struct is here so the shape is consistent with the other
// services and so future env-driven knobs land in one place.
type Config struct {
	HistoryLimit int `env:"HP_HISTORY_LIMIT"`
}

// DefaultConfig is what New uses when callers don't pass a literal Config.
func DefaultConfig() *Config { return &Config{HistoryLimit: 5} }

// Validate enforces invariants.
func (c *Config) Validate() error {
	if c.HistoryLimit <= 0 {
		return fmt.Errorf("HistoryLimit must be > 0")
	}
	return nil
}
