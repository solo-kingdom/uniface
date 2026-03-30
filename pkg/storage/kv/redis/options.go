// Package redis provides a Redis-based implementation of the KV storage interface.
// This package implements github.com/solo-kingdom/uniface/pkg/storage/kv.Storage interface.
package redis

import (
	"time"
)

// Config holds the configuration for Redis connection.
type Config struct {
	// Addr is the Redis server address in "host:port" format.
	Addr string

	// Password is the password for Redis authentication.
	Password string

	// DB is the Redis database number to use.
	DB int

	// PoolSize is the maximum number of socket connections.
	PoolSize int

	// MinIdleConns is the minimum number of idle connections.
	MinIdleConns int

	// MaxRetries is the maximum number of retries before giving up.
	MaxRetries int

	// DialTimeout is the timeout for establishing new connections.
	DialTimeout time.Duration

	// ReadTimeout is the timeout for socket reads.
	ReadTimeout time.Duration

	// WriteTimeout is the timeout for socket writes.
	WriteTimeout time.Duration

	// PoolTimeout is the timeout for getting a connection from the pool.
	PoolTimeout time.Duration

	// IdleTimeout is the timeout for closing idle connections.
	IdleTimeout time.Duration

	// KeyPrefix is a global prefix for all keys.
	KeyPrefix string
}

// DefaultConfig returns the default configuration for Redis.
func DefaultConfig() *Config {
	return &Config{
		Addr:         "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 5,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
		IdleTimeout:  5 * time.Minute,
		KeyPrefix:    "",
	}
}

// Option is a function that modifies Config.
type Option func(*Config)

// WithAddr sets the Redis server address.
func WithAddr(addr string) Option {
	return func(c *Config) {
		c.Addr = addr
	}
}

// WithPassword sets the Redis password.
func WithPassword(password string) Option {
	return func(c *Config) {
		c.Password = password
	}
}

// WithDB sets the Redis database number.
func WithDB(db int) Option {
	return func(c *Config) {
		c.DB = db
	}
}

// WithPoolSize sets the connection pool size.
func WithPoolSize(size int) Option {
	return func(c *Config) {
		c.PoolSize = size
	}
}

// WithMinIdleConns sets the minimum number of idle connections.
func WithMinIdleConns(n int) Option {
	return func(c *Config) {
		c.MinIdleConns = n
	}
}

// WithMaxRetries sets the maximum number of retries.
func WithMaxRetries(n int) Option {
	return func(c *Config) {
		c.MaxRetries = n
	}
}

// WithDialTimeout sets the dial timeout.
func WithDialTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.DialTimeout = d
	}
}

// WithReadTimeout sets the read timeout.
func WithReadTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.ReadTimeout = d
	}
}

// WithWriteTimeout sets the write timeout.
func WithWriteTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.WriteTimeout = d
	}
}

// WithPoolTimeout sets the pool timeout.
func WithPoolTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.PoolTimeout = d
	}
}

// WithIdleTimeout sets the idle timeout.
func WithIdleTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.IdleTimeout = d
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
