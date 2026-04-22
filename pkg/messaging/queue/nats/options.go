// Package nats provides a NATS Core-based implementation of the message queue interface.
// This package implements github.com/solo-kingdom/uniface/pkg/messaging/queue interfaces.
// NATS Core 提供 at-most-once 语义（fire-and-forget）。
package nats

import (
	"time"
)

// Config 保存 NATS Core 连接级配置。
type Config struct {
	// URL 是 NATS 服务器地址（nats://host:port）。
	URL string

	// Name 是客户端连接名称。
	Name string

	// MaxReconnect 是最大重连次数。
	MaxReconnect int

	// ReconnectWait 是重连等待时间。
	ReconnectWait time.Duration

	// Timeout 是连接超时时间。
	Timeout time.Duration

	// PingInterval 是心跳间隔。
	PingInterval time.Duration
}

// DefaultConfig 返回默认的 NATS 配置。
func DefaultConfig() *Config {
	return &Config{
		URL:           "nats://localhost:4222",
		Name:          "uniface-nats",
		MaxReconnect:  60,
		ReconnectWait: 2 * time.Second,
		Timeout:       5 * time.Second,
		PingInterval:  20 * time.Second,
	}
}

// Option 是修改 Config 的函数类型。
type Option func(*Config)

// Apply 应用给定的选项到当前 Config。
func (c *Config) Apply(opts ...Option) *Config {
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// NewConfig 使用给定的选项创建新的 Config。
func NewConfig(opts ...Option) *Config {
	return DefaultConfig().Apply(opts...)
}

// WithURL 设置 NATS 服务器地址。
func WithURL(url string) Option {
	return func(c *Config) {
		c.URL = url
	}
}

// WithName 设置客户端连接名称。
func WithName(name string) Option {
	return func(c *Config) {
		c.Name = name
	}
}

// WithMaxReconnect 设置最大重连次数。
func WithMaxReconnect(n int) Option {
	return func(c *Config) {
		c.MaxReconnect = n
	}
}

// WithReconnectWait 设置重连等待时间。
func WithReconnectWait(d time.Duration) Option {
	return func(c *Config) {
		c.ReconnectWait = d
	}
}

// WithTimeout 设置连接超时时间。
func WithTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.Timeout = d
	}
}

// WithPingInterval 设置心跳间隔。
func WithPingInterval(d time.Duration) Option {
	return func(c *Config) {
		c.PingInterval = d
	}
}
