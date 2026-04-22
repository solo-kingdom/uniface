// Package rabbitmq provides a RabbitMQ-based implementation of the message queue interface.
// This package implements github.com/solo-kingdom/uniface/pkg/messaging/queue interfaces.
package rabbitmq

import (
	"time"
)

// Config 保存 RabbitMQ 连接级配置。
type Config struct {
	// URL 是 RabbitMQ 连接地址（amqp://user:pass@host:port/vhost）。
	URL string

	// Exchange 是默认交换机名称。
	Exchange string

	// ExchangeType 是交换机类型（direct、fanout、topic、headers）。
	ExchangeType string

	// QueuePrefix 是队列名称前缀。
	QueuePrefix string

	// Durable 是否持久化交换机和队列。
	Durable bool

	// AutoDelete 是否自动删除不使用的队列。
	AutoDelete bool

	// ConnectionTimeout 是连接超时时间。
	ConnectionTimeout time.Duration

	// Heartbeat 是心跳间隔。
	Heartbeat time.Duration

	// PrefetchCount 是消费者预取数量。
	PrefetchCount int

	// PrefetchSize 是消费者预取大小（字节）。
	PrefetchSize int

	// DeliveryMode 是消息投递模式：1=非持久化，2=持久化。
	DeliveryMode uint8

	// MaxReconnectAttempts 是最大重连次数。
	MaxReconnectAttempts int
}

// DefaultConfig 返回默认的 RabbitMQ 配置。
func DefaultConfig() *Config {
	return &Config{
		URL:                  "amqp://guest:guest@localhost:5672/",
		Exchange:             "",
		ExchangeType:         "topic",
		QueuePrefix:          "",
		Durable:              true,
		AutoDelete:           false,
		ConnectionTimeout:    30 * time.Second,
		Heartbeat:            10 * time.Second,
		PrefetchCount:        10,
		PrefetchSize:         0,
		DeliveryMode:         2, // 持久化
		MaxReconnectAttempts: 3,
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

// WithURL 设置 RabbitMQ 连接地址。
func WithURL(url string) Option {
	return func(c *Config) {
		c.URL = url
	}
}

// WithExchange 设置默认交换机名称。
func WithExchange(exchange string) Option {
	return func(c *Config) {
		c.Exchange = exchange
	}
}

// WithExchangeType 设置交换机类型。
func WithExchangeType(exchangeType string) Option {
	return func(c *Config) {
		c.ExchangeType = exchangeType
	}
}

// WithQueuePrefix 设置队列名称前缀。
func WithQueuePrefix(prefix string) Option {
	return func(c *Config) {
		c.QueuePrefix = prefix
	}
}

// WithDurable 设置是否持久化。
func WithDurable(durable bool) Option {
	return func(c *Config) {
		c.Durable = durable
	}
}

// WithConnectionTimeout 设置连接超时时间。
func WithConnectionTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.ConnectionTimeout = timeout
	}
}

// WithHeartbeat 设置心跳间隔。
func WithHeartbeat(heartbeat time.Duration) Option {
	return func(c *Config) {
		c.Heartbeat = heartbeat
	}
}

// WithPrefetchCount 设置消费者预取数量。
func WithPrefetchCount(count int) Option {
	return func(c *Config) {
		c.PrefetchCount = count
	}
}

// WithDeliveryMode 设置消息投递模式。
func WithDeliveryMode(mode uint8) Option {
	return func(c *Config) {
		c.DeliveryMode = mode
	}
}

// WithMaxReconnectAttempts 设置最大重连次数。
func WithMaxReconnectAttempts(n int) Option {
	return func(c *Config) {
		c.MaxReconnectAttempts = n
	}
}
