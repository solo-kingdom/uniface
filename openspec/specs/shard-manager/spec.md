# Shard Manager

基于 LoadBalancer + Consistent Hash 的简化分片管理器，提供稳定的基于 Key 的路由。

- **接口定义**: `pkg/rpc/governance/loadbalancer/shard/interface.go`
- **实现**: `pkg/rpc/governance/loadbalancer/shard/manager.go`

---

## 接口

```go
type Manager interface {
    Select(key string) (*loadbalancer.Instance, error)
    SelectClient(key string, factory ClientFactory) (interface{}, error)
    Close() error
}
```

## ClientFactory 类型

```go
type ClientFactory func(*loadbalancer.Instance) (interface{}, error)
```

## 错误

| Sentinel 错误 | 说明 |
|---------------|------|
| `ErrNoInstances` | 没有可用实例 |
| `ErrManagerClosed` | 分片管理器已关闭 |
| `ErrInvalidKey` | 无效的分片键 |
| `ErrNoFactory` | 未提供客户端工厂 |

## 行为规格

### Requirement: Select 操作

系统 SHALL 支持通过 `Select` 基于一致性哈希算法选择实例。相同的 Key SHALL 始终路由到相同的实例（稳定性）。

#### Scenario: 基于 Key 稳定路由
- **WHEN** 有 3 个实例，调用 `Select("user-123")`
- **THEN** 返回一致性哈希计算得到的实例

#### Scenario: 相同 Key 路由一致性
- **WHEN** 多次调用 `Select("user-123")`
- **THEN** 每次返回相同的实例

#### Scenario: 无可用实例
- **WHEN** 没有实例，调用 `Select("user-123")`
- **THEN** 返回 `ErrNoInstances` 错误

### Requirement: SelectClient 操作

系统 SHALL 支持通过 `SelectClient` 基于 Key 选择实例并返回客户端。已缓存的客户端 SHALL 直接返回；未缓存的 SHALL 通过 factory 创建并缓存。

#### Scenario: 首次获取客户端
- **WHEN** 调用 `SelectClient("user-123", factory)`
- **THEN** 通过一致性哈希选择实例，调用 factory 创建客户端，缓存并返回

#### Scenario: 复用已缓存客户端
- **WHEN** "user-123" 对应的实例已有缓存客户端，调用 `SelectClient("user-123", factory)`
- **THEN** 直接返回缓存客户端

#### Scenario: 未提供 Factory
- **WHEN** 调用 `SelectClient("user-123", nil)`
- **THEN** 返回 `ErrNoFactory` 错误

### Requirement: Close 操作

系统 SHALL 支持通过 `Close` 关闭分片管理器并释放所有资源。关闭后所有操作 SHALL 返回 `ErrManagerClosed`。

#### Scenario: 关闭后操作
- **WHEN** 调用 `Close()` 后，再调用 `Select("key")`
- **THEN** 返回 `ErrManagerClosed` 错误

### Requirement: 固定实例列表

ShardManager 的实例列表在初始化时指定，SHALL 不支持动态添加或移除实例。

#### Scenario: 初始化时指定实例
- **WHEN** 创建 ShardManager 时传入 3 个实例
- **THEN** 这 3 个实例在 Manager 生命周期内不变

### Requirement: 线程安全

ShardManager SHALL 保证线程安全，支持并发调用。

#### Scenario: 并发路由
- **WHEN** 多个 goroutine 同时调用 Select 和 SelectClient
- **THEN** 不发生数据竞争，所有操作正确完成

## 设计说明

ShardManager 是 LoadBalancer（ConsistentHash 算法）的简化包装：

- 初始化时创建 ConsistentHash 负载均衡器并添加所有实例
- `Select(key)` 等价于 `balancer.Select(ctx, WithKey(key))`
- `SelectClient(key, factory)` 等价于 `balancer.SelectClient(ctx, WithKey(key), WithClientFactory(factory))`
- 实例列表固定，不支持动态修改（YAGNI 原则）

## 实现要求

- MUST 基于 `loadbalancer.Balancer` 接口实现
- MUST 使用一致性哈希算法保证 Key 路由稳定性
- MUST 使用 `sync.RWMutex` 保证线程安全
- `Close()` MUST 调用底层 Balancer 的 `Close()` 释放资源
