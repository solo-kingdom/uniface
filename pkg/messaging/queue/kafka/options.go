// Package kafka provides a Kafka-based implementation of the message queue interface.
// This package implements github.com/solo-kingdom/uniface/pkg/messaging/queue interfaces.
package kafka

import (
	"time"
)

// Config 保存 Kafka 连接级配置。
type Config struct {
	// Brokers 是 Kafka Broker 地址列表。
	Brokers []string

	// GroupID 是消费者组 ID。
	GroupID string

	// ClientID 是客户端标识。
	ClientID string

	// Version 是 Kafka 集群版本。
	// 为空时使用默认版本协商。
	Version string

	// AuthType 是认证类型："none"、"sasl_plain"、"sasl_scram"。
	AuthType string

	// AuthUser 是 SASL 认证用户名。
	AuthUser string

	// AuthPassword 是 SASL 认证密码。
	AuthPassword string

	// TLS 是是否启用 TLS。
	TLS bool

	// ProducerRequiredAcks 是生产者确认级别：0=NoResponse, 1=WaitForLocal, -1=WaitForAll。
	ProducerRequiredAcks int16

	// ProducerTimeout 是生产者超时。
	ProducerTimeout time.Duration

	// ConsumerOffsetsInitial 是消费者初始 offset 策略：-2=Oldest, -1=Newest。
	ConsumerOffsetsInitial int64

	// ConsumerMaxProcessingTime 是消费者最大处理时间。
	ConsumerMaxProcessingTime time.Duration

	// ConsumerFetchMin 是消费者最小拉取字节数。
	ConsumerFetchMin int32

	// ConsumerFetchDefault 是消费者默认拉取字节数。
	ConsumerFetchDefault int32

	// ChannelBufferSize 是内部通道缓冲区大小。
	ChannelBufferSize int
}

// DefaultConfig 返回默认的 Kafka 配置。
func DefaultConfig() *Config {
	return &Config{
		Brokers:                   []string{"localhost:9092"},
		GroupID:                   "",
		ClientID:                  "uniface-kafka",
		Version:                   "",
		AuthType:                  "none",
		TLS:                       false,
		ProducerRequiredAcks:      1, // WaitForLocal
		ProducerTimeout:           10 * time.Second,
		ConsumerOffsetsInitial:    -1, // Newest
		ConsumerMaxProcessingTime: 100 * time.Millisecond,
		ConsumerFetchMin:          1,
		ConsumerFetchDefault:      1024 * 1024, // 1MB
		ChannelBufferSize:         256,
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

// WithBrokers 设置 Kafka Broker 地址列表。
func WithBrokers(brokers []string) Option {
	return func(c *Config) {
		c.Brokers = brokers
	}
}

// WithGroupID 设置消费者组 ID。
func WithGroupID(groupID string) Option {
	return func(c *Config) {
		c.GroupID = groupID
	}
}

// WithClientID 设置客户端标识。
func WithClientID(clientID string) Option {
	return func(c *Config) {
		c.ClientID = clientID
	}
}

// WithVersion 设置 Kafka 集群版本。
func WithVersion(version string) Option {
	return func(c *Config) {
		c.Version = version
	}
}

// WithSASLPlain 启用 SASL/PLAIN 认证。
func WithSASLPlain(user, password string) Option {
	return func(c *Config) {
		c.AuthType = "sasl_plain"
		c.AuthUser = user
		c.AuthPassword = password
	}
}

// WithSASLSCRAM 启用 SASL/SCRAM 认证。
func WithSASLSCRAM(user, password string) Option {
	return func(c *Config) {
		c.AuthType = "sasl_scram"
		c.AuthUser = user
		c.AuthPassword = password
	}
}

// WithTLS 启用 TLS。
func WithTLS(enabled bool) Option {
	return func(c *Config) {
		c.TLS = enabled
	}
}

// WithProducerRequiredAcks 设置生产者确认级别。
func WithProducerRequiredAcks(acks int16) Option {
	return func(c *Config) {
		c.ProducerRequiredAcks = acks
	}
}

// WithProducerTimeout 设置生产者超时。
func WithProducerTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.ProducerTimeout = timeout
	}
}

// WithConsumerOffsetsInitial 设置消费者初始 offset 策略。
func WithConsumerOffsetsInitial(initial int64) Option {
	return func(c *Config) {
		c.ConsumerOffsetsInitial = initial
	}
}

// WithConsumerMaxProcessingTime 设置消费者最大处理时间。
func WithConsumerMaxProcessingTime(d time.Duration) Option {
	return func(c *Config) {
		c.ConsumerMaxProcessingTime = d
	}
}

// WithChannelBufferSize 设置内部通道缓冲区大小。
func WithChannelBufferSize(size int) Option {
	return func(c *Config) {
		c.ChannelBufferSize = size
	}
}
