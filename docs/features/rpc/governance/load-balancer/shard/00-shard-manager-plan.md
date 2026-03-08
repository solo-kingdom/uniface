# Shard Manager 实现计划

## 需求概述

基于 load balancer，创建一个手动分片管理器，用于服务/数据库手动分片。

## 设计思路

### 1. 核心概念

**Shard（分片）**:
- 每个分片代表一个数据或服务的子集
- 每个分片对应一个 Load Balancer 实例
- 支持手动分配分片范围（如 user_id 范围、hash 范围等）

**Shard Key（分片键）**:
- 用于确定请求应该路由到哪个分片的键
- 支持多种类型：字符串、整数等

**Shard Manager（分片管理器）**:
- 管理多个分片
- 根据分片键路由请求到正确的分片
- 支持分片的动态添加和删除

### 2. 架构设计

```
┌─────────────────────────────────────────┐
│         Shard Manager[T]                │
│                                         │
│  ┌──────────────────────────────────┐   │
│  │  Shard Router (路由策略)         │   │
│  │  - RangeRouter (范围路由)        │   │
│  │  - HashRouter (哈希路由)         │   │
│  │  - ManualRouter (手动路由)       │   │
│  └──────────────────────────────────┘   │
│                                         │
│  ┌──────────────────────────────────┐   │
│  │  Shards (分片集合)               │   │
│  │  - Shard 0 → LoadBalancer       │   │
│  │  - Shard 1 → LoadBalancer       │   │
│  │  - Shard N → LoadBalancer       │   │
│  └──────────────────────────────────┘   │
└─────────────────────────────────────────┘
```

### 3. 接口设计

#### 3.1 Shard 接口

```go
// Shard 表示一个分片
type Shard struct {
    ID       string           // 分片 ID
    Range    *ShardRange      // 分片范围（可选）
    Metadata map[string]string // 元数据
    Balancer Balancer[T]      // 该分片的负载均衡器
}

// ShardRange 表示分片的键范围
type ShardRange struct {
    Min string // 最小值（包含）
    Max string // 最大值（包含）
}
```

#### 3.2 Router 接口

```go
// Router 分片路由器接口
type Router interface {
    // Route 根据分片键选择分片
    Route(ctx context.Context, key string, shards []*Shard) (*Shard, error)
}
```

#### 3.3 ShardManager 接口

```go
// ShardManager 分片管理器接口
type ShardManager[T any] interface {
    // Select 根据分片键选择实例
    Select(ctx context.Context, shardKey string, opts ...Option) (*Instance, error)
    
    // SelectClient 根据分片键选择客户端
    SelectClient(ctx context.Context, shardKey string, opts ...Option) (T, error)
    
    // AddShard 添加分片
    AddShard(ctx context.Context, shard *Shard) error
    
    // RemoveShard 移除分片
    RemoveShard(ctx context.Context, shardID string) error
    
    // GetShard 获取指定分片
    GetShard(ctx context.Context, shardID string) (*Shard, error)
    
    // GetAllShards 获取所有分片
    GetAllShards(ctx context.Context) ([]*Shard, error)
    
    // SetRouter 设置路由器
    SetRouter(router Router) error
    
    // Close 关闭管理器
    Close() error
}
```

### 4. 实现策略

#### 4.1 路由策略

1. **RangeRouter（范围路由）**:
   - 根据键的范围分配分片
   - 适用于有序键，如时间范围、ID 范围
   - 示例：user_id 1-10000 → Shard0, 10001-20000 → Shard1

2. **HashRouter（哈希路由）**:
   - 对分片键进行哈希计算
   - 使用一致性哈希或简单取模
   - 适用于均匀分布的场景

3. **ManualRouter（手动路由）**:
   - 完全手动指定分片映射
   - 适用于精确控制的场景

#### 4.2 分片管理

- 每个分片内部使用 LoadBalancer 管理多个实例
- 支持分片级别的实例管理
- 支持分片的动态扩缩容

### 5. 文件结构

```
pkg/rpc/governance/loadbalancer/
├── shard/
│   ├── interface.go          # 分片管理器接口
│   ├── shard.go              # Shard 结构定义
│   ├── router.go             # 路由器接口
│   ├── manager.go            # ShardManager 实现
│   ├── manager_test.go       # 测试
│   ├── routers/
│   │   ├── range_router.go   # 范围路由器
│   │   ├── hash_router.go    # 哈希路由器
│   │   └── manual_router.go  # 手动路由器
│   └── errors.go             # 错误定义
```

### 6. 使用示例

```go
// 创建分片管理器
manager := shard.NewShardManager[string](
    shard.WithRouter(shard.NewHashRouter()),
)

// 添加分片
shard0 := &shard.Shard{
    ID: "shard-0",
    Balancer: roundrobin.New[string](),
}
shard0.Balancer.Add(ctx, instance1)
shard0.Balancer.Add(ctx, instance2)

manager.AddShard(ctx, shard0)

// 路由请求
client, err := manager.SelectClient(ctx, "user-123",
    loadbalancer.WithClientFactory(factory),
)
```

### 7. 测试计划

- [ ] 分片路由测试（范围、哈希、手动）
- [ ] 分片管理测试（添加、删除、查询）
- [ ] 并发安全测试
- [ ] 错误处理测试
- [ ] 性能测试

### 8. 实施步骤

1. ✅ 创建计划文档
2. ⬜ 定义接口（interface.go）
3. ⬜ 实现 Shard 结构（shard.go）
4. ⬜ 实现路由器接口（router.go）
5. ⬜ 实现 ShardManager（manager.go）
6. ⬜ 实现各种路由器（routers/）
7. ⬜ 编写测试
8. ⬜ 编写文档和示例

## 关键特性

1. **基于 LoadBalancer**: 复用现有的负载均衡能力
2. **灵活的路由策略**: 支持多种路由算法
3. **手动管理**: 完全手动控制分片分配
4. **线程安全**: 所有操作都是并发安全的
5. **泛型支持**: 支持不同类型的客户端
6. **易扩展**: 容易添加新的路由策略

## 注意事项

1. 分片键的选择对性能影响很大
2. 分片重平衡需要谨慎处理
3. 需要考虑分片故障的容错
4. 监控和可观测性很重要
