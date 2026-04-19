// Package queue provides a generic message queue interface for application decoupling.
// This package defines the contract for message queue implementations,
// supporting publish/subscribe with generic message types and pluggable codecs.
package queue

import "context"

// Message 表示消息队列中的一条消息。
// T 为消息体类型，通过 Codec 在传输层与应用层之间转换。
type Message[T any] struct {
	// Topic 是消息所属的主题（Kafka Topic、RabbitMQ Routing Key 依据、NATS Subject）。
	Topic string

	// Key 用于分区路由。
	// Kafka: 映射到 Partition 路由键；RabbitMQ: 映射到 Routing Key；NATS: 忽略。
	Key string

	// Value 是泛型消息体。
	// 通过 Codec 编码为 []byte 传输，接收时通过 Codec 解码。
	Value T

	// Headers 是消息头，用于传递元数据。
	Headers map[string]string

	// Timestamp 是消息的时间戳。
	Timestamp int64 // Unix 毫秒时间戳
}

// Handler 是消息处理回调函数。
// 返回 nil 表示 ACK（确认消费），返回 error 表示 NACK（重新入队）。
type Handler[T any] func(ctx context.Context, message *Message[T]) error

// Codec 定义消息体的编解码接口。
// 实现此接口可支持 JSON、Protobuf、MsgPack 等序列化格式。
type Codec interface {
	// Encode 将任意值编码为字节切片。
	Encode(v any) ([]byte, error)

	// Decode 将字节切片解码到目标值。
	Decode(data []byte, v any) error
}

// Publisher 定义消息发布者接口。
// T 为消息体类型。
type Publisher[T any] interface {
	// Publish 发布一条消息到指定主题。
	//
	// 参数:
	//   - ctx: 上下文，用于取消操作
	//   - topic: 主题名称，非空字符串
	//   - message: 消息内容
	//   - opts: 可选配置项（如 Key、Headers、Codec）
	//
	// 返回:
	//   - error: 如果发布失败返回错误
	Publish(ctx context.Context, topic string, message *Message[T], opts ...Option) error

	// BatchPublish 批量发布消息到指定主题。
	// 实现应尽可能使用原生批量接口以提高性能，不支持时降级为逐条发送。
	//
	// 参数:
	//   - ctx: 上下文，用于取消操作
	//   - topic: 主题名称，非空字符串
	//   - messages: 消息列表
	//   - opts: 可选配置项
	//
	// 返回:
	//   - error: 如果任一消息发布失败返回错误
	BatchPublish(ctx context.Context, topic string, messages []*Message[T], opts ...Option) error

	// Close 关闭发布者并释放资源。
	// 调用 Close 后，所有其他操作应返回错误。
	Close() error
}

// Subscriber 定义消息订阅者接口。
// T 为消息体类型。
type Subscriber[T any] interface {
	// Subscribe 订阅指定主题的消息。
	//
	// 参数:
	//   - ctx: 上下文，用于取消操作
	//   - topic: 主题名称，非空字符串
	//   - handler: 消息处理回调，返回 nil 为 ACK，返回 error 为 NACK
	//   - opts: 可选配置项（如 Group、AutoAck、AckTimeout、Codec）
	//
	// 返回:
	//   - Subscription: 订阅句柄，用于管理订阅生命周期
	//   - error: 如果订阅失败返回错误
	Subscribe(ctx context.Context, topic string, handler Handler[T], opts ...Option) (Subscription, error)

	// Close 关闭订阅者并释放资源。
	// 调用 Close 后，所有其他操作应返回错误。
	Close() error
}

// Subscription 表示一个订阅的句柄，用于管理订阅生命周期。
type Subscription interface {
	// Unsubscribe 取消订阅并释放资源。
	Unsubscribe() error

	// Pause 暂停消息消费，保持连接。
	// 暂停期间消息不投递到 Handler，但不丢失（取决于 Broker 能力）。
	Pause() error

	// Resume 恢复消息消费。
	// 恢复后暂停期间积压的消息将正常投递。
	Resume() error
}

// Queue 组合 Publisher 和 Subscriber 接口，提供完整的消息队列能力。
type Queue[T any] interface {
	Publisher[T]
	Subscriber[T]
}
