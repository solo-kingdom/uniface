## Why

Uniface 已提供 KV 存储、配置管理、负载均衡等基础设施抽象，但缺少消息队列（Messaging Queue）领域的统一接口。应用间解耦是微服务架构的核心需求，而 Kafka、RabbitMQ、NATS 等消息中间件各有不同的 API 和语义模型，导致应用代码与具体实现强耦合。

需要新增消息队列领域的统一接口，遵循项目既有的接口优先设计模式，屏蔽底层实现差异，使应用可以无成本地在不同消息中间件之间切换。

## What Changes

- 在 `pkg/messaging/queue/` 下新增消息队列领域的接口定义（interface.go、options.go、errors.go）
- 接口设计为泛型 `Publisher[T]`、`Subscriber[T]`、`Queue[T]`，T 为消息体类型
- 拆分 Publisher/Subscriber 角色，Queue 为组合接口
- ACK/NACK 语义通过 Options 按需启用
- 订阅支持暂停/恢复消费
- 支持批量发布（BatchPublish）
- 新增 Kafka 实现子模块（`pkg/messaging/queue/kafka/`）
- 新增 RabbitMQ 实现子模块（`pkg/messaging/queue/rabbitmq/`）
- 新增 NATS Core 实现子模块（`pkg/messaging/queue/nats/`，at-most-once）
- 新增 NATS JetStream 实现子模块（`pkg/messaging/queue/natsjetstream/`，持久化 + ACK）

## Capabilities

### New Capabilities

- `messaging-queue`: 消息队列统一接口，提供泛型化的发布/订阅/批量发布能力，支持消费者组、ACK/NACK、暂停/恢复消费、消息编解码

## Impact

- **新增领域**：`pkg/messaging/` 为新的顶层领域，与 `pkg/storage/`、`pkg/rpc/` 并列
- **新增子模块**：4 个独立 Go 子模块（kafka、rabbitmq、nats、natsjetstream），各有独立 `go.mod`
- **不涉及现有代码变更**：纯新增，不修改已有的 KV、Config、LoadBalancer 接口和实现
