# Load Balancer

泛型负载均衡器接口，支持实例选择、客户端管理和多种均衡策略。

- **接口定义**: `pkg/rpc/governance/loadbalancer/interface.go`
- **算法实现**: RoundRobin, Random, Weighted, ConsistentHash (`pkg/rpc/governance/loadbalancer/implementations/`)

---

## 接口

```go
type Balancer[T any] interface {
    Select(ctx context.Context, opts ...Option) (*Instance, error)
    SelectClient(ctx context.Context, opts ...Option) (T, error)
    Add(ctx context.Context, instance *Instance) error
    Remove(ctx context.Context, instanceID string) error
    Update(ctx context.Context, instance *Instance) error
    GetAll(ctx context.Context) ([]*Instance, error)
    Close() error
}
```

## Instance 结构

```go
type Instance struct {
    ID       string            // 唯一标识（必填）
    Address  string            // IP 地址或主机名（必填）
    Port     int               // 端口号（必填）
    Weight   int               // 权重，默认 1
    Metadata map[string]string // 附加元数据（可选）
}
```

## 配置选项

| 选项 | 类型 | 说明 |
|------|------|------|
| `WithKey(key string)` | Key | 一致性哈希键，非空时使用一致性哈希算法 |
| `WithClientFactory[T](fn)` | ClientFactory | 客户端工厂函数 |
| `WithFilter(fn)` | Filter | 实例过滤函数，仅返回 true 的实例参与选择 |

## 错误

| Sentinel 错误 | 说明 |
|---------------|------|
| `ErrNoInstances` | 没有可用实例 |
| `ErrInstanceNotFound` | 实例不存在 |
| `ErrInvalidInstance` | 无效的实例 |
| `ErrBalancerClosed` | 均衡器已关闭 |
| `ErrDuplicateInstance` | 重复实例 |
| `ErrNoClientFactory` | 未提供客户端工厂 |
| `ErrClientCreateFailed` | 客户端创建失败 |

## 行为规格

### Requirement: Select 操作

系统 SHALL 支持通过 `Select` 从可用实例中选择一个实例。当 `WithKey` 指定非空键时，SHALL 使用一致性哈希算法；未指定键或键为空时，SHALL 使用默认策略。

#### Scenario: 使用默认策略选择
- **WHEN** 有 3 个实例，调用 `Select(ctx)`
- **THEN** 根据算法策略（如 round-robin、random）返回一个实例

#### Scenario: 使用 Key 进行一致性哈希选择
- **WHEN** 有 3 个实例，调用 `Select(ctx, WithKey("user-123"))`
- **THEN** 使用一致性哈希算法选择实例，相同 Key 始终返回相同实例

#### Scenario: 无可用实例
- **WHEN** 没有实例，调用 `Select(ctx)`
- **THEN** 返回 `ErrNoInstances` 错误

#### Scenario: 使用 Filter 过滤实例
- **WHEN** 有 3 个实例（2 个 region=us-west，1 个 region=eu），调用 `Select(ctx, WithFilter(func(inst) bool { return inst.Metadata["region"] == "us-west" }))`
- **THEN** 仅从 2 个 region=us-west 的实例中选择

### Requirement: SelectClient 操作

系统 SHALL 支持通过 `SelectClient` 选择实例并返回对应的客户端。已缓存的客户端 SHALL 直接返回；未缓存的 SHALL 通过 ClientFactory 创建并缓存。同一实例 SHALL 始终返回同一客户端。

#### Scenario: 首次获取客户端
- **WHEN** 实例 "inst-1" 没有缓存客户端，调用 `SelectClient(ctx, WithClientFactory(factory))`
- **THEN** 调用 factory 创建客户端，缓存并返回

#### Scenario: 复用已缓存客户端
- **WHEN** 实例 "inst-1" 已有缓存客户端，调用 `SelectClient(ctx, WithClientFactory(factory))`
- **THEN** 直接返回缓存客户端，不调用 factory

#### Scenario: 未提供 ClientFactory
- **WHEN** 调用 `SelectClient(ctx)` 未提供 ClientFactory
- **THEN** 返回 `ErrNoClientFactory` 错误

#### Scenario: 客户端自动关闭
- **WHEN** 客户端实现了 `io.Closer`，实例被移除或均衡器被关闭
- **THEN** 系统自动调用客户端的 `Close()` 方法

### Requirement: Add 操作

系统 SHALL 支持通过 `Add` 添加实例。如果实例 ID 已存在，SHALL 返回 `ErrDuplicateInstance`。

#### Scenario: 添加新实例
- **WHEN** 调用 `Add(ctx, &Instance{ID: "inst-1", Address: "10.0.0.1", Port: 8080})`
- **THEN** 实例被添加到可用列表

#### Scenario: 添加重复实例
- **WHEN** "inst-1" 已存在，调用 `Add(ctx, &Instance{ID: "inst-1", ...})`
- **THEN** 返回 `ErrDuplicateInstance` 错误

### Requirement: Remove 操作

系统 SHALL 支持通过 `Remove` 移除实例，同时关闭并移除关联的客户端。如果实例不存在，SHALL 返回 `ErrInstanceNotFound`。

#### Scenario: 移除已存在的实例
- **WHEN** "inst-1" 存在且有关联客户端，调用 `Remove(ctx, "inst-1")`
- **THEN** 实例被移除，关联客户端被关闭

#### Scenario: 移除不存在的实例
- **WHEN** "missing" 不存在，调用 `Remove(ctx, "missing")`
- **THEN** 返回 `ErrInstanceNotFound` 错误

### Requirement: Update 操作

系统 SHALL 支持通过 `Update` 更新实例信息。如果实例不存在，SHALL 返回 `ErrInstanceNotFound`。

#### Scenario: 更新实例权重
- **WHEN** "inst-1" 存在，调用 `Update(ctx, &Instance{ID: "inst-1", Weight: 5})`
- **THEN** 实例的 Weight 被更新为 5

### Requirement: GetAll 操作

系统 SHALL 支持通过 `GetAll` 获取所有实例的副本。

#### Scenario: 获取所有实例
- **WHEN** 有 3 个实例，调用 `GetAll(ctx)`
- **THEN** 返回包含 3 个实例的切片（副本，修改不影响内部状态）

### Requirement: Close 操作

系统 SHALL 支持通过 `Close` 关闭均衡器并释放所有资源。关闭后所有其他操作 SHALL 返回 `ErrBalancerClosed`。

#### Scenario: 关闭后操作
- **WHEN** 调用 `Close()` 后，再调用 `Select(ctx)`
- **THEN** 返回 `ErrBalancerClosed` 错误

#### Scenario: 关闭时释放客户端
- **WHEN** 调用 `Close()`，且存在实现了 `io.Closer` 的缓存客户端
- **THEN** 所有缓存客户端的 `Close()` 方法被调用

### Requirement: 线程安全

所有 Balancer 实现 SHALL 保证线程安全，支持并发调用。

#### Scenario: 并发选择
- **WHEN** 多个 goroutine 同时调用 Select、Add、Remove
- **THEN** 不发生数据竞争，所有操作正确完成

## 算法实现

| 算法 | 包路径 | 特征 |
|------|--------|------|
| RoundRobin | `implementations/roundrobin/` | 轮询，依次选择实例 |
| Random | `implementations/random/` | 随机选择，支持 seed 确保确定性 |
| Weighted | `implementations/weighted/` | 加权轮询，高权重实例获得更多流量 |
| ConsistentHash | `implementations/consistenthash/` | 一致性哈希，相同 Key 路由到相同实例 |

## 实现要求

- 所有实现 MUST 使用 `sync.RWMutex` 保证线程安全
- 错误 MUST 使用 `BalancerError` 包装，支持 `errors.Is/As` 解包
- 客户端缓存 MUST 正确处理 `io.Closer` 自动检测
- `Close()` MUST 释放所有资源（缓存客户端、实例列表等）
