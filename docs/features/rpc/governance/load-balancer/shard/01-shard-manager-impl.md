# Shard Manager 实现变更说明

## 变更概述

基于 load balancer 实现了手动分片管理器，用于服务/数据库的手动分片管理。

## 新增文件

### 核心实现

1. **pkg/rpc/governance/loadbalancer/shard/interface.go**
   - 定义 `Manager` 接口
   - 定义分片管理器的主要操作

2. **pkg/rpc/governance/loadbalancer/shard/shard.go**
   - 定义 `Shard` 结构
   - 定义 `ShardRange` 结构
   - 实现分片验证逻辑

3. **pkg/rpc/governance/loadbalancer/shard/router.go**
   - 定义 `Router` 接口
   - 分片路由策略的基础接口

4. **pkg/rpc/governance/loadbalancer/shard/manager.go**
   - 实现 `ShardManager` 结构
   - 实现分片管理的核心逻辑
   - 支持分片的增删改查
   - 支持基于分片键的路由

5. **pkg/rpc/governance/loadbalancer/shard/errors.go**
   - 定义分片管理相关的错误类型
   - 实现 `ShardError` 错误包装

### 路由器实现

1. **pkg/rpc/governance/loadbalancer/shard/hash_router.go**
   - 实现 `HashRouter`
   - 基于 FNV-1a 哈希的分片路由
   - 适用于均匀分布的场景

2. **pkg/rpc/governance/loadbalancer/shard/range_router.go**
   - 实现 `RangeRouter`
   - 基于键范围的分片路由
   - 适用于有序键的场景（如 ID 范围）

3. **pkg/rpc/governance/loadbalancer/shard/manual_router.go**
   - 实现 `ManualRouter`
   - 完全手动的键到分片映射
   - 适用于精确控制的场景

### 测试文件

1. **pkg/rpc/governance/loadbalancer/shard/manager_test.go**
   - 测试分片管理器的基本功能
   - 测试分片的增删改查
   - 测试生命周期管理

2. **pkg/rpc/governance/loadbalancer/shard/router_test.go**
   - 测试三种路由器的实现
   - 测试路由逻辑的正确性
   - 测试 Select 方法的路由功能

## 核心特性

### 1. 分片管理

- **添加分片**: `AddShard(ctx, shard)`
- **删除分片**: `RemoveShard(ctx, shardID)`
- **查询分片**: `GetShard(ctx, shardID)` / `GetAllShards(ctx)`
- **设置路由器**: `SetRouter(router)`

### 2. 请求路由

- **选择实例**: `Select(ctx, shardKey, opts...)`
- **选择客户端**: `SelectClient(ctx, shardKey, opts...)`
- 基于分片键自动路由到正确的分片
- 每个分片内部使用 LoadBalancer 进行负载均衡

### 3. 多种路由策略

1. **HashRouter**: 哈希分片，均匀分布
2. **RangeRouter**: 范围分片，适用于有序数据
3. **ManualRouter**: 手动映射，精确控制

### 4. 线程安全

- 所有操作都是并发安全的
- 使用读写锁优化性能
- 支持高并发场景

## 使用示例

```go
// 1. 创建分片管理器
manager := shard.NewShardManager(
    shard.WithRouter(shard.NewHashRouter()),
)

// 2. 创建分片
shard0 := &shard.Shard{
    ID:       "shard-0",
    Balancer: roundrobin.New[interface{}](),
}
shard0.Balancer.Add(ctx, &loadbalancer.Instance{
    ID:      "inst-1",
    Address: "192.168.1.1",
    Port:    8080,
})

// 3. 添加分片
manager.AddShard(ctx, shard0)

// 4. 路由请求
instance, err := manager.Select(ctx, "user-123")
client, err := manager.SelectClient(ctx, "user-123",
    loadbalancer.WithClientFactory(factory),
)
```

## 架构设计

```
ShardManager
    ├── Router (路由策略)
    │   ├── HashRouter
    │   ├── RangeRouter
    │   └── ManualRouter
    │
    └── Shards (分片集合)
        ├── Shard 0 → LoadBalancer
        ├── Shard 1 → LoadBalancer
        └── Shard N → LoadBalancer
```

## 测试覆盖

- ✅ 分片管理测试（添加、删除、查询）
- ✅ 路由器测试（哈希、范围、手动）
- ✅ Select 方法测试
- ✅ 错误处理测试
- ✅ 并发安全测试

## 性能考虑

1. **读写锁**: 使用 RWMutex 优化读多写少的场景
2. **分片列表缓存**: 维护有序分片列表，避免每次遍历 map
3. **路由器抽象**: 可以轻松切换路由策略而不影响业务代码

## 后续优化方向

1. 支持分片健康检查
2. 支持分片权重
3. 支持分片重平衡
4. 添加监控指标
5. 实现一致性哈希路由器

## 文档

- 计划文档: `docs/features/rpc/governance/load-balancer/shard/00-shard-manager-plan.md`
- 需求文档: `prompts/features/rpc/governance/load-balancer/shard/00-shard-manager.md`
