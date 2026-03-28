// Package boltdb provides a BoltDB-based implementation of the KV storage interface.
// This package implements github.com/wii/uniface/pkg/storage/kv.Storage interface.
package boltdb

import (
	"os"
	"time"
)

// Config holds the configuration for BoltDB storage.
type Config struct {
	// Path is the file path for the BoltDB database file.
	Path string

	// Timeout is the maximum time to wait for the database to open.
	Timeout time.Duration

	// FileMode is the file mode for the database file.
	FileMode os.FileMode

	// KeyPrefix is a global prefix for all keys.
	KeyPrefix string
}

// DefaultConfig returns the default configuration for BoltDB.
func DefaultConfig() *Config {
	return &Config{
		Path:      "data.db",
		Timeout:   1 * time.Second,
		FileMode:  0600,
		KeyPrefix: "",
	}
}

// Option is a function that modifies Config.
type Option func(*Config)

// WithPath sets the database file path.
func WithPath(path string) Option {
	return func(c *Config) {
		c.Path = path
	}
}

// WithTimeout sets the database open timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.Timeout = d
	}
}

// WithFileMode sets the database file mode.
func WithFileMode(mode os.FileMode) Option {
	return func(c *Config) {
		c.FileMode = mode
	}
}

// WithKeyPrefix sets a global prefix for all keys.
func WithKeyPrefix(prefix string) Option {
	return func(c *Config) {
		c.KeyPrefix = prefix
	}
}

// Apply applies the given options to the current Config.
func (c *Config) Apply(opts ...Option) *Config {
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// NewConfig creates a new Config with the given options.
func NewConfig(opts ...Option) *Config {
	return DefaultConfig().Apply(opts...)
}
