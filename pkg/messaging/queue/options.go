// Package queue provides a generic message queue interface for application decoupling.
// This file contains configuration options for queue operations.
package queue

import "time"

// Options 表示消息队列操作的配置选项。
type Options struct {
	// Group 是消费者组名称。
	// Kafka: Consumer Group ID；RabbitMQ: Queue 名称后缀；NATS: Queue Group Name。
	Group string

	// AutoAck 控制是否自动确认消息。
	// 默认 true，消息由 Handler 返回值决定 ACK/NACK。
	// 设为 false 时需通过 Handler 返回值手动确认。
	AutoAck bool

	// AckTimeout 是消息确认超时时间。
	// 超时未确认的消息将被重新投递（取决于 Broker 能力）。
	// 0 表示不超时。
	AckTimeout time.Duration

	// Codec 是消息编解码器。
	// 为 nil 时使用默认的 JSONCodec。
	Codec Codec

	// Headers 是消息头，用于传递元数据。
	Headers map[string]string

	// Key 是消息路由键（操作级别，覆盖 Message 中的 Key）。
	Key string

	// MaxRetries 是消息消费失败后的最大重试次数。
	// 仅在 AutoAck=false 时有效。
	MaxRetries int

	// BatchSize 是批量操作的单批最大数量。
	BatchSize int
}

// Option 是修改 Options 的函数类型。
type Option func(*Options)

// Apply 应用给定的选项到当前 Options。
func (o *Options) Apply(opts ...Option) *Options {
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// DefaultOptions 返回默认的消息队列操作选项。
func DefaultOptions() *Options {
	return &Options{
		Group:      "",
		AutoAck:    true,
		AckTimeout: 0,
		Codec:      nil, // 使用时由实现层填充默认 JSONCodec
		Headers:    nil,
		Key:        "",
		MaxRetries: 3,
		BatchSize:  100,
	}
}

// MergeOptions 合并多个选项，后面的覆盖前面的。
func MergeOptions(opts ...Option) *Options {
	return DefaultOptions().Apply(opts...)
}

// WithGroup 设置消费者组名称。
func WithGroup(group string) Option {
	return func(o *Options) {
		o.Group = group
	}
}

// WithAutoAck 设置是否自动确认消息。
func WithAutoAck(auto bool) Option {
	return func(o *Options) {
		o.AutoAck = auto
	}
}

// WithAckTimeout 设置消息确认超时时间。
func WithAckTimeout(timeout time.Duration) Option {
	return func(o *Options) {
		o.AckTimeout = timeout
	}
}

// WithCodec 设置消息编解码器。
func WithCodec(codec Codec) Option {
	return func(o *Options) {
		o.Codec = codec
	}
}

// WithHeaders 设置消息头。
func WithHeaders(headers map[string]string) Option {
	return func(o *Options) {
		o.Headers = headers
	}
}

// WithKey 设置消息路由键。
func WithKey(key string) Option {
	return func(o *Options) {
		o.Key = key
	}
}

// WithMaxRetries 设置最大重试次数。
func WithMaxRetries(n int) Option {
	return func(o *Options) {
		o.MaxRetries = n
	}
}

// WithBatchSize 设置批量操作的单批最大数量。
func WithBatchSize(size int) Option {
	return func(o *Options) {
		o.BatchSize = size
	}
}
