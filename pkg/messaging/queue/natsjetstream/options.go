// Package natsjetstream provides a NATS JetStream-based implementation of the message queue interface.
// This package implements github.com/solo-kingdom/uniface/pkg/messaging/queue interfaces.
// JetStream 提供持久化存储、ACK、重播等高级特性。
package natsjetstream

import (
	"time"
)

// Config 保存 NATS JetStream 连接级配置。
type Config struct {
	// URL 是 NATS 服务器地址（nats://host:port）。
	URL string

	// Name 是客户端连接名称。
	Name string

	// StreamName 是 JetStream Stream 名称。
	StreamName string

	// StreamSubjects 是 Stream 绑定的 Subject 列表。
	StreamSubjects []string

	// DurableName 是 Durable Consumer 名称。
	DurableName string

	// MaxReconnect 是最大重连次数。
	MaxReconnect int

	// ReconnectWait 是重连等待时间。
	ReconnectWait time.Duration

	// Timeout 是连接超时时间。
	Timeout time.Duration

	// PingInterval 是心跳间隔。
	PingInterval time.Duration

	// MaxDeliver 是消息最大投递次数。
	MaxDeliver int

	// AckWait 是 ACK 等待超时。
	AckWait time.Duration

	// Replicas 是 Stream 副本数。
	Replicas int

	// Retention 是 Stream 消息保留策略：0=limits, 1=interest, 2=workqueue。
	Retention int

	// MaxMsgs 是 Stream 最大消息数量。
	MaxMsgs int64

	// MaxBytes 是 Stream 最大存储字节数。
	MaxBytes int64

	// MaxAge 是 Stream 消息最大保留时间。
	MaxAge time.Duration
}

// DefaultConfig 返回默认的 JetStream 配置。
func DefaultConfig() *Config {
	return &Config{
		URL:            "nats://localhost:4222",
		Name:           "uniface-jetstream",
		StreamName:     "",
		StreamSubjects: nil,
		DurableName:    "",
		MaxReconnect:   60,
		ReconnectWait:  2 * time.Second,
		Timeout:        5 * time.Second,
		PingInterval:   20 * time.Second,
		MaxDeliver:     3,
		AckWait:        30 * time.Second,
		Replicas:       1,
		Retention:      0, // limits
		MaxMsgs:        -1,
		MaxBytes:       -1,
		MaxAge:         0, // 无限制
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

// WithStreamName 设置 JetStream Stream 名称。
func WithStreamName(name string) Option {
	return func(c *Config) {
		c.StreamName = name
	}
}

// WithStreamSubjects 设置 Stream 绑定的 Subject 列表。
func WithStreamSubjects(subjects []string) Option {
	return func(c *Config) {
		c.StreamSubjects = subjects
	}
}

// WithDurableName 设置 Durable Consumer 名称。
func WithDurableName(name string) Option {
	return func(c *Config) {
		c.DurableName = name
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

// WithMaxDeliver 设置消息最大投递次数。
func WithMaxDeliver(n int) Option {
	return func(c *Config) {
		c.MaxDeliver = n
	}
}

// WithAckWait 设置 ACK 等待超时。
func WithAckWait(d time.Duration) Option {
	return func(c *Config) {
		c.AckWait = d
	}
}

// WithReplicas 设置 Stream 副本数。
func WithReplicas(n int) Option {
	return func(c *Config) {
		c.Replicas = n
	}
}

// WithMaxMsgs 设置 Stream 最大消息数量。
func WithMaxMsgs(n int64) Option {
	return func(c *Config) {
		c.MaxMsgs = n
	}
}

// WithMaxBytes 设置 Stream 最大存储字节数。
func WithMaxBytes(n int64) Option {
	return func(c *Config) {
		c.MaxBytes = n
	}
}

// WithMaxAge 设置 Stream 消息最大保留时间。
func WithMaxAge(d time.Duration) Option {
	return func(c *Config) {
		c.MaxAge = d
	}
}
