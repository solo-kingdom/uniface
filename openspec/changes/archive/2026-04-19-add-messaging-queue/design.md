## Context

Uniface 项目的核心设计原则是接口优先、面向接口编程。每个领域在包根目录定义公开接口，实现放在子目录中作为独立 Go 子模块。现有领域（KV 存储、配置管理、负载均衡）均遵循统一的结构模式：

```
pkg/<domain>/<feature>/
  interface.go    # 公开接口定义
  options.go      # 函数式 Options 模式
  errors.go       # sentinel errors + 自定义错误类型
  <impl>/         # 具体实现（独立子模块）
```

消息队列的语义（发布-消费）与存储（存储-检索）有本质差异，应作为新的顶层领域 `pkg/messaging/`。

### 消息模型对比

| 特性 | Kafka | RabbitMQ | NATS Core | NATS JetStream |
|------|-------|----------|-----------|----------------|
| 模型 | Topic + Partition | Exchange + Queue | Subject | Stream + Consumer |
| 消费模式 | Consumer Group | Queue Consumer | Queue Group | Durable Consumer |
| 持久化 | Append-only log | 可选持久化 | 无 | Stream 存储 |
| 投递保证 | At-least-once | 支持三种 | At-most-once | At-least-once |
| 分区路由 | Key → Partition | Routing Key | N/A | N/A |
| 重播 | offset-based | 需 DLX | 无 | Sequence-based |

## Goals / Non-Goals

**Goals:**

- 定义泛型化的消息队列接口 `Publisher[T]`、`Subscriber[T]`、`Queue[T]`
- T 参数化为消息体类型，通过 Codec 实现编解码
- 拆分 Publisher/Subscriber 角色，Queue 为组合接口
- ACK/NACK 通过 Options 按需启用（默认 AutoAck）
- 消费者组保持简单（Group 字符串即可）
- Subscription 支持暂停/恢复消费
- 支持批量发布 BatchPublish
- 实现 4 个子模块：kafka、rabbitmq、nats（Core）、natsjetstream（JetStream）

**Non-Goals:**

- 不实现事务消息（Transactional Messaging）
- 不实现消息过滤/选择器（Message Selector/Filter）
- 不实现延迟消息（Delayed/Delayed Message）——可通过 Headers 扩展
- 不实现消息轨迹追踪（Message Tracing）
- 不实现跨 Broker 的消息转发

## Decisions

### D1: 领域包路径

**决策**: 新建 `pkg/messaging/queue/` 作为消息队列领域。

```
pkg/messaging/
  queue/
    interface.go
    options.go
    errors.go
    kafka/
    rabbitmq/
    nats/
    natsjetstream/
```

**理由**: 消息队列与存储（storage）语义不同，不宜放在 `pkg/storage/` 下。`messaging` 作为顶层领域，未来可扩展 `pubsub/` 等子领域。

**替代方案**: 放在 `pkg/storage/queue/`。放弃，因为消息队列的核心语义是发布-消费而非存储-检索。

### D2: 泛型参数化消息体类型

**决策**: 接口使用泛型参数 `T` 表示消息体类型，通过 `Codec` 接口实现编解码。

```go
type Codec interface {
    Encode(v any) ([]byte, error)
    Decode(data []byte, v any) error
}

type Publisher[T any] interface {
    Publish(ctx context.Context, topic string, message *Message[T], opts ...Option) error
    BatchPublish(ctx context.Context, topic string, messages []*Message[T], opts ...Option) error
    Close() error
}
```

**理由**: 泛型让用户可以选择 `Queue[[]byte]`（原始字节）或 `Queue[MyOrder]`（自动编解码），类型安全。Codec 在传输层统一为 `[]byte`，在应用层自动转换。

**替代方案**: 使用 `interface{}` 表示消息体。放弃，因为泛型提供更好的类型安全，且项目已有泛型使用先例（`Balancer[T]`）。

### D3: Publisher/Subscriber 拆分

**决策**: 定义独立的 `Publisher[T]` 和 `Subscriber[T]` 接口，`Queue[T]` 作为组合接口。

```go
type Publisher[T any] interface { ... }
type Subscriber[T any] interface { ... }
type Queue[T any] interface {
    Publisher[T]
    Subscriber[T]
}
```

**理由**: 实际场景中经常只需要其中一侧——纯生产者不需要 Subscribe，纯消费者不需要 Publish。拆开后依赖注入更精确。

**替代方案**: 单一 Queue 接口。放弃，因为违反接口隔离原则（ISP）。

### D4: ACK/NACK 在 Options 中按需启用

**决策**: ACK 行为通过 Options 控制。

```go
type Options struct {
    AutoAck    bool          // 默认 true
    AckTimeout time.Duration // ACK 超时
    // ...
}
```

当 `AutoAck=false` 时，Handler 返回 error 即为 NACK（重新入队），返回 nil 即为 ACK。如需更细粒度控制，可通过 Message 上的方法扩展。

**理由**: 简化常见用例（自动确认），同时保留显式确认能力。不同 Broker 的 ACK 机制差异大（Kafka offset commit vs RabbitMQ explicit ACK），统一为最简模型。

**替代方案**: 在 Message 上暴露显式 Ack()/Nack() 方法。现阶段放弃，因为 Handler 返回值已能表达 ACK/NACK 语义，且避免了 goroutine 安全问题。未来如需更复杂场景可扩展。

### D5: NATS Core 和 JetStream 分开实现

**决策**: `nats/` 实现 Core NATS（at-most-once），`natsjetstream/` 实现 JetStream（持久化 + ACK）。

**理由**: 两者语义差异显著——Core NATS 是 fire-and-forget，JetStream 提供持久化存储和重播。分开实现使依赖隔离（JetStream 需要流管理 API），也使用户选择更明确。

**替代方案**: 在同一子模块中通过配置切换。放弃，因为会导致可选依赖和条件逻辑复杂化。

### D6: Subscription 暂停/恢复

**决策**: Subscription 接口包含 `Pause()` 和 `Resume()` 方法。

```go
type Subscription interface {
    Unsubscribe() error
    Pause() error
    Resume() error
}
```

**理由**: 实际场景中经常需要临时停止消费（如下游服务过载），而不完全断开连接。

**替代方案**: 仅提供 Unsubscribe。放弃，因为重连成本高且用户明确需要此能力。

### D7: 统一 Key 字段用于分区路由

**决策**: Message 结构体使用 `Key string` 字段，映射到各 Broker 的路由机制。

| Broker | Key 映射 |
|--------|---------|
| Kafka | Partition 路由键 |
| RabbitMQ | Routing Key |
| NATS Core | 忽略（无分区概念） |
| NATS JetStream | 忽略 |

**理由**: Key 是消息队列中最普遍的路由概念，统一后接口简洁。各实现自行决定如何使用或忽略。

### D8: 编解码器设计

**决策**: 内置 `JSONCodec` 作为默认编解码器，用户可通过 `WithCodec` 选项替换。

**理由**: JSON 是最通用的编码格式，覆盖大多数场景。用户可自定义 Protobuf、MsgPack 等编解码器。

## Risks / Trade-offs

- **[泛型 T 可能导致实现复杂]** → Codec 接口将泛型与传输层解耦，实现只需处理 `[]byte`
- **[统一 ACK 模型可能无法覆盖所有 Broker 特性]** → 保持最简模型，通过 Options 扩展；RabbitMQ 的 Reject/DLX 等高级特性暂不支持
- **[BatchPublish 在 NATS Core 上意义有限]** → 实现可降级为逐条发送，接口统一
- **[Pause/Resume 在 Kafka 中需额外实现]** → Kafka 通过暂停分区消费实现，复杂度可控
