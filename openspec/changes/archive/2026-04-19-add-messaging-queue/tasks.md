## 1. 接口定义

- [x] 1.1 创建 `pkg/messaging/queue/interface.go`，定义 `Message[T]`、`Handler[T]`、`Codec`、`Publisher[T]`、`Subscriber[T]`、`Queue[T]`、`Subscription` 接口
- [x] 1.2 创建 `pkg/messaging/queue/options.go`，定义 `Options`、`Option`、`DefaultOptions`、`MergeOptions`、`WithXxx` 函数式选项（Group、AutoAck、AckTimeout、Codec、Headers 等）
- [x] 1.3 创建 `pkg/messaging/queue/errors.go`，定义 sentinel errors（ErrTopicRequired、ErrMessageTooLarge、ErrBrokerClosed、ErrSubscriptionClosed、ErrPublishFailed、ErrSubscribeFailed 等）和 `QueueError` 自定义错误类型
- [x] 1.4 创建内置编解码器 `pkg/messaging/queue/codec.go`，实现 `JSONCodec`

## 2. Kafka 实现

- [x] 2.1 创建 `pkg/messaging/queue/kafka/` 子模块（go.mod、独立依赖）
- [x] 2.2 实现 `Config` 连接级配置（Brokers、GroupID、Auth 等）和 `Option` 函数式选项
- [x] 2.3 实现 `kafka.Queue[T]`（嵌入 Publisher + Subscriber），使用 `github.com/IBM/sarama` 或 `github.com/segmentio/kafka-go`
- [x] 2.4 实现 Publish/BatchPublish（Key → Partition 路由）
- [x] 2.5 实现 Subscribe（Consumer Group、offset commit、AutoAck/手动 ACK）
- [x] 2.6 实现 Subscription 的 Pause/Resume（暂停分区消费）
- [x] 2.7 编写单元测试

## 3. RabbitMQ 实现

- [x] 3.1 创建 `pkg/messaging/queue/rabbitmq/` 子模块（go.mod、独立依赖）
- [x] 3.2 实现 `Config` 连接级配置（URL、Exchange、Queue 等）和 `Option` 函数式选项
- [x] 3.3 实现 `rabbitmq.Queue[T]`，使用 `github.com/rabbitmq/amqp091-go`
- [x] 3.4 实现 Publish/BatchPublish（Key → Routing Key、Exchange 声明）
- [x] 3.5 实现 Subscribe（Queue 绑定、Consumer、ACK/NACK/Reject）
- [x] 3.6 实现 Subscription 的 Pause/Resume（channel flow control）
- [x] 3.7 编写单元测试

## 4. NATS Core 实现

- [x] 4.1 创建 `pkg/messaging/queue/nats/` 子模块（go.mod、独立依赖）
- [x] 4.2 实现 `Config` 连接级配置（URL、Name 等）和 `Option` 函数式选项
- [x] 4.3 实现 `nats.Queue[T]`，使用 `github.com/nats-io/nats.go`
- [x] 4.4 实现 Publish/BatchPublish（Subject 发布，BatchPublish 降级为逐条）
- [x] 4.5 实现 Subscribe（Queue Group、at-most-once）
- [x] 4.6 实现 Subscription 的 Pause/Resume（unsubscribe/resubscribe 模拟）
- [x] 4.7 编写单元测试

## 5. NATS JetStream 实现

- [x] 5.1 创建 `pkg/messaging/queue/natsjetstream/` 子模块（go.mod、独立依赖）
- [x] 5.2 实现 `Config` 连接级配置（URL、Stream 名称、Durable Name 等）和 `Option` 函数式选项
- [x] 5.3 实现 `natsjetstream.Queue[T]`，使用 `github.com/nats-io/nats.go` JetStream API
- [x] 5.4 实现 Publish/BatchPublish（Stream 发布、去重）
- [x] 5.5 实现 Subscribe（Durable Consumer、ACK、重播）
- [x] 5.6 实现 Subscription 的 Pause/Resume（pull consumer 暂停/恢复）
- [x] 5.7 编写单元测试

## 6. 构建集成

- [x] 6.1 更新 `Makefile`，将新的子模块纳入 `make mod`、`make build`、`make test` 范围
- [x] 6.2 更新根 `go.mod`（如有需要）
