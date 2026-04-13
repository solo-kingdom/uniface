# ShardManager 重构说明

## 重构概述

将过度设计的分片管理器重构为极简实现，从 **~800 行代码** 减少到 **~200 行代码**。

## 重构动机

原始实现过于复杂，引入了不必要的抽象层：
- ❌ Shard 结构（每个分片包含多个实例）
- ❌ Router 接口（支持多种路由策略）
- ❌ 3种路由器实现（HashRouter、RangeRouter、ManualRouter）
- ❌ 分片的动态添加/删除

实际需求很简单：
- ✅ 初始化时指定实例列表
- ✅ 根据 key 进行稳定路由
- ✅ 相同 key 始终路由到相同节点

**解决方案**：LoadBalancer + 一致性哈希已经完全满足需求，ShardManager 只需要简单封装。

## 重构内容

### 删除的文件（过度设计）

```
❌ router.go              # 路由器接口（不需要）
❌ shard.go               # Shard 结构（不需要）
❌ hash_router.go         # 哈希路由器（用 consistenthash）
❌ range_router.go        # 范围路由器（不需要）
❌ manual_router.go       # 手动路由器（不需要）
❌ router_test.go         # 路由器测试（不需要）
```

### 简化的文件

#### 1. `interface.go` - 极简接口

**之前**：~115 行，包含分片管理、路由器管理等复杂接口

**之后**：~50 行，只保留核心功能

```go
type Manager interface {
    Select(key string) (*Instance, error)
    SelectClient(key string, factory ClientFactory) (interface{}, error)
    Close() error
}
```

#### 2. `manager.go` - LoadBalancer 包装器

**之前**：~240 行，包含分片管理、路由器切换等复杂逻辑

**之后**：~90 行，简单封装

```go
type ShardManager struct {
    lb loadbalancer.Balancer[interface{}]
}

func NewShardManager(instances []*loadbalancer.Instance) *ShardManager {
    lb := consistenthash.New[interface{}](0, nil)
    // 添加所有实例
    return &ShardManager{lb: lb}
}

func (m *ShardManager) Select(key string) (*Instance, error) {
    return m.lb.Select(context.Background(), loadbalancer.WithKey(key))
}
```

#### 3. `errors.go` - 基本错误

**之前**：~60 行，包含 ShardError 等复杂错误类型

**之后**：~20 行，只保留基本错误

```go
var (
    ErrNoInstances   = errors.New("no instances available")
    ErrManagerClosed = errors.New("shard manager closed")
    ErrInvalidKey    = errors.New("invalid shard key")
    ErrNoFactory     = errors.New("client factory not provided")
)
```

#### 4. `manager_test.go` - 简化测试

**之前**：~220 行，测试分片管理、路由器等

**之后**：~160 行，只测试核心功能

关键测试：
- ✅ 稳定性测试：相同 key 始终路由到相同实例
- ✅ 分布测试：不同 key 均匀分布
- ✅ 客户端缓存测试
- ✅ 错误处理测试

#### 5. `example_test.go` - 更新示例

**之前**：~220 行，展示多种路由策略

**之后**：~110 行，只展示基本用法

## 核心变化对比

| 维度 | 之前 | 之后 |
|------|------|------|
| **文件数量** | 11 个 | **5 个** |
| **代码行数** | ~800 行 | **~200 行** |
| **接口方法** | 11 个 | **3 个** |
| **路由策略** | 3 种 | **1 种（一致性哈希）** |
| **实例管理** | 动态添加/删除 | **初始化时固定** |
| **分片概念** | 有（Shard 结构） | **无（直接管理实例）** |

## 架构对比

### 之前（过度设计）

```
ShardManager
    ├── Router (路由策略接口)
    │   ├── HashRouter
    │   ├── RangeRouter
    │   └── ManualRouter
    │
    └── Shards (分片集合)
        ├── Shard 0
        │   └── LoadBalancer
        │       ├── Instance 1
        │       └── Instance 2
        ├── Shard 1
        │   └── LoadBalancer
        │       └── Instance 3
        └── ...
```

### 之后（极简设计）

```
ShardManager
    └── LoadBalancer (一致性哈希)
        ├── Instance 0
        ├── Instance 1
        └── Instance 2

本质：LoadBalancer 的简单封装
```

## 使用示例对比

### 之前（复杂）

```go
// 1. 创建管理器
manager := shard.NewShardManager(
    shard.WithRouter(shard.NewHashRouter()),
)

// 2. 创建分片
shard0 := &shard.Shard{
    ID:       "shard-0",
    Balancer: roundrobin.New[interface{}](),
}

// 3. 添加实例到分片
shard0.Balancer.Add(ctx, instance1)

// 4. 添加分片到管理器
manager.AddShard(ctx, shard0)

// 5. 路由
instance, _ := manager.Select(ctx, "user-123")
```

### 之后（简单）

```go
// 1. 创建管理器（直接指定实例）
manager := shard.NewShardManager([]*loadbalancer.Instance{
    {ID: "inst-0", Address: "192.168.1.1", Port: 8080},
    {ID: "inst-1", Address: "192.168.1.2", Port: 8080},
})

// 2. 路由
instance, _ := manager.Select("user-123")
```

**代码减少 70%+**

## 测试结果

```bash
=== RUN   TestShardManager_Select
=== RUN   TestShardManager_Select/stable_routing_-_same_key_always_routes_to_same_instance
=== RUN   TestShardManager_Select/different_keys_distribute_across_instances
=== RUN   TestShardManager_Select/empty_key_returns_error
--- PASS: TestShardManager_Select (0.00s)

=== RUN   TestShardManager_SelectClient
=== RUN   TestShardManager_SelectClient/create_and_cache_client
=== RUN   TestShardManager_SelectClient/empty_key_returns_error
=== RUN   TestShardManager_SelectClient/nil_factory_returns_error
--- PASS: TestShardManager_SelectClient (0.00s)

=== RUN   TestShardManager_Close
--- PASS: TestShardManager_Close (0.00s)

=== RUN   TestShardManager_EmptyInstances
--- PASS: TestShardManager_EmptyInstances (0.00s)

PASS
```

✅ **所有测试通过**

## 设计原则

### 1. YAGNI (You Aren't Gonna Need It)

不预先设计不需要的功能：
- 不需要多种路由策略 → 只用一致性哈希
- 不需要分片概念 → 直接管理实例
- 不需要动态修改 → 初始化时固定

### 2. KISS (Keep It Simple, Stupid)

保持最简单的实现：
- ShardManager = LoadBalancer 的简单封装
- 复用现有代码，不重复造轮子
- 代码清晰易懂，一目了然

### 3. 组合优于继承

通过组合复用功能：
- 组合 `consistenthash.Balancer`
- 代理调用底层方法
- 不引入复杂的继承层次

## 后续维护

### 极简代码的好处

1. **易于理解**：新成员可以快速上手
2. **易于测试**：测试用例简单直接
3. **易于维护**：修改影响范围小
4. **不易出错**：代码少，bug 少

### 如果需要扩展

如果未来真的需要更复杂的功能：
- **范围路由**：可以在应用层实现，或创建新的 Manager
- **动态实例**：可以暴露底层的 LoadBalancer 供外部调用
- **多分片**：可以在更高层次实现，ShardManager 保持简单

**关键**：只在真正需要时才增加复杂度，而不是预先设计。

## 总结

这次重构完美诠释了 **"Less is More"** 的理念：

- 从 800 行减少到 200 行（**减少 75%**）
- 从 11 个文件减少到 5 个文件（**减少 55%**）
- 功能完全满足需求
- 测试全部通过
- 代码更清晰易维护

**最重要的**：符合用户的真实需求，而不是过度设计。
