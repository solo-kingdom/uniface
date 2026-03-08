# ShardManager - 简单分片管理器

基于 LoadBalancer + 一致性哈希的极简分片管理器。

## 概述

ShardManager 提供了一个简单的基于 key 的分片路由功能，确保相同的 key 始终路由到相同的实例（稳定性）。

**本质上是 LoadBalancer + 一致性哈希的简单封装**。

## 核心特性

- ✅ **基于 key 进行稳定路由**：相同的 key 始终路由到相同实例
- ✅ **初始化时指定实例**：简单直接，不支持动态修改
- ✅ **线程安全**：所有操作都是并发安全的
- ✅ **客户端缓存**：自动缓存和复用客户端连接

## 快速开始

```go
import (
    "github.com/wii/uniface/pkg/rpc/governance/loadbalancer"
    "github.com/wii/uniface/pkg/rpc/governance/loadbalancer/shard"
)

// 1. 创建分片管理器（初始化时指定实例）
manager := shard.NewShardManager([]*loadbalancer.Instance{
    {ID: "db-0", Address: "192.168.1.1", Port: 3306},
    {ID: "db-1", Address: "192.168.1.2", Port: 3306},
    {ID: "db-2", Address: "192.168.1.3", Port: 3306},
})
defer manager.Close()

// 2. 根据 key 路由（保证稳定性）
instance, err := manager.Select("user-123")

// 3. 获取客户端
client, err := manager.SelectClient("user-123", func(inst *loadbalancer.Instance) (interface{}, error) {
    // 创建实际的客户端连接
    return createClient(inst)
})
```

## API

### `Select(key string) (*Instance, error)`

根据 key 选择实例。相同的 key 始终返回相同的实例。

```go
instance, _ := manager.Select("user-123")
```

### `SelectClient(key string, factory ClientFactory) (interface{}, error)`

根据 key 选择客户端。如果客户端已缓存则返回缓存的客户端，否则使用 factory 创建并缓存。

```go
client, _ := manager.SelectClient("user-123", func(inst *loadbalancer.Instance) (interface{}, error) {
    return &MyClient{addr: inst.Address}, nil
})
```

### `Close() error`

关闭管理器，释放所有资源。

## 使用场景

### 1. 数据库分片

```go
// 创建3个数据库分片
manager := shard.NewShardManager([]*loadbalancer.Instance{
    {ID: "db-0", Address: "db0.example.com", Port: 5432},
    {ID: "db-1", Address: "db1.example.com", Port: 5432},
    {ID: "db-2", Address: "db2.example.com", Port: 5432},
})

// 按用户ID路由
instance, _ := manager.Select(userID)
```

### 2. 服务分片

```go
// 为服务创建多个实例
manager := shard.NewShardManager([]*loadbalancer.Instance{
    {ID: "service-0", Address: "192.168.1.1", Port: 8080},
    {ID: "service-1", Address: "192.168.1.2", Port: 8080},
})

// 按租户ID路由
client, _ := manager.SelectClient(tenantID, factory)
```

## 工作原理

```
┌─────────────────────────────────────────┐
│         ShardManager                     │
│                                          │
│  ┌────────────────────────────────────┐  │
│  │  ConsistentHashBalancer            │  │
│  │  (底层使用一致性哈希)              │  │
│  └────────────────────────────────────┘  │
│                                          │
│  Instances:                              │
│  - Instance 0                            │
│  - Instance 1                            │
│  - Instance 2                            │
└─────────────────────────────────────────┘

路由流程：
1. 输入 key（如 "user-123"）
2. 一致性哈希计算
3. 返回对应的实例
4. 相同 key 始终返回相同实例 ✓
```

## 文件结构

```
pkg/rpc/governance/loadbalancer/shard/
├── interface.go          # Manager 接口定义
├── manager.go            # ShardManager 实现
├── errors.go             # 错误定义
├── manager_test.go       # 测试
└── example_test.go       # 使用示例
```

## 测试

```bash
# 运行测试
go test ./pkg/rpc/governance/loadbalancer/shard/...

# 查看覆盖率
go test -cover ./pkg/rpc/governance/loadbalancer/shard/...
```

## 设计理念

### 简单至上

- **不引入 Shard 概念**：直接管理实例
- **不引入 Router 接口**：直接用一致性哈希
- **不支持动态修改**：初始化时固定实例列表

### 复用现有实现

完全复用 `consistenthash` 包的功能：
- 一致性哈希算法
- 虚拟节点
- 客户端缓存
- 线程安全

### 代码极简

整个实现只有 **~200 行代码**，清晰易懂。

## 文档

- [实现计划](./00-shard-manager-plan.md) - 原始设计
- [重构说明](./02-shard-manager-refactor.md) - 本次重构
- [需求文档](../../../prompts/features/rpc/governance/load-balancer/shard/00-shard-manager.md)

## License

MIT
