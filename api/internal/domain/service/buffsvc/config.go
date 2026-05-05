package buffsvc

// Config is reserved for future env-driven knobs.
type Config struct{}

// DefaultConfig returns the zero-value config.
func DefaultConfig() *Config { return &Config{} }

// Validate is a no-op today.
func (c *Config) Validate() error { return nil }
