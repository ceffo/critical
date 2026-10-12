package hpsvc

import "fmt"

// Config controls service-local knobs, parsed from the environment by New.
// Today only the history window is configurable.
type Config struct {
	HistoryLimit int `env:"HP_HISTORY_LIMIT" envDefault:"5"`
}

// DefaultConfig is the literal default (matches the envDefault tags); tests
// use it with NewWithConfig so they don't depend on the environment.
func DefaultConfig() *Config { return &Config{HistoryLimit: 5} }

// Validate enforces invariants.
func (c *Config) Validate() error {
	if c.HistoryLimit <= 0 {
		return fmt.Errorf("HistoryLimit must be > 0")
	}
	return nil
}
