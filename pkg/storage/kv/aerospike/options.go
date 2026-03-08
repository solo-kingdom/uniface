// Package aerospike provides a Aerospike-based sharded storage implementation.
// This package implements sharded access using the Shard Manager.
//
// 基于 prompts/features/rpc/governance/load-balancer/shard/aerospike/00-aerospike-shared-client.md 实现
package aerospike

import (
	"time"
)

// Config holds the configuration for Aerospike client.
type Config struct {
	// 连接超时
	ConnectTimeout time.Duration
	// 读写超时
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	// 连接池配置
	PoolSize     int
	MinIdleConns int
	MaxIdleConns int
	IdleTimeout  time.Duration
	MaxRetries   int
	RetryDelay   time.Duration
	// 认证配置
	User     string
	Password string
	// TLS 配置
	EnableTLS bool
	// 其他配置
	// 全局 key 前缀
	KeyPrefix string
}

// Option is a function that configures the Aerospike client.
type Option func(*Config)

// NewConfig creates a new Config with default values.
func NewConfig(opts ...Option) *Config {
	config := &Config{
		ConnectTimeout: 5 * time.Second,
		ReadTimeout:    3 * time.Second,
		WriteTimeout:   3 * time.Second,
		PoolSize:       10,
		MinIdleConns:   2,
		MaxIdleConns:   10,
		IdleTimeout:    5 * time.Minute,
		MaxRetries:     3,
		RetryDelay:     100 * time.Millisecond,
	}

	for _, opt := range opts {
		opt(config)
	}

	return config
}

// WithConnectTimeout sets the connection timeout.
func WithConnectTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.ConnectTimeout = timeout
	}
}

// WithReadTimeout sets the read timeout.
func WithReadTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.ReadTimeout = timeout
	}
}

// WithWriteTimeout sets the write timeout.
func WithWriteTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.WriteTimeout = timeout
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

// WithMaxIdleConns sets the maximum number of idle connections.
func WithMaxIdleConns(n int) Option {
	return func(c *Config) {
		c.MaxIdleConns = n
	}
}

// WithIdleTimeout sets the idle connection timeout.
func WithIdleTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.IdleTimeout = timeout
	}
}

// WithMaxRetries sets the maximum number of retries.
func WithMaxRetries(n int) Option {
	return func(c *Config) {
		c.MaxRetries = n
	}
}

// WithRetryDelay sets the retry delay.
func WithRetryDelay(delay time.Duration) Option {
	return func(c *Config) {
		c.RetryDelay = delay
	}
}

// WithAuth sets the authentication credentials.
func WithAuth(user, password string) Option {
	return func(c *Config) {
		c.User = user
		c.Password = password
	}
}

// WithTLS enables TLS.
func WithTLS(enable bool) Option {
	return func(c *Config) {
		c.EnableTLS = enable
	}
}

// WithKeyPrefix sets the global key prefix.
func WithKeyPrefix(prefix string) Option {
	return func(c *Config) {
		c.KeyPrefix = prefix
	}
}
