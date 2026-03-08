# Aerospike 分片客户端

基于 Shard Manager 的 Aerospike 分片访问实现，以独立的 go submodule 方式组织。

## 概述

本包提供了基于一致性哈希的 Aerospike 分片客户端，能够根据 key 自动路由到对应的 Aerospike 实例。

**核心特性**：

- ✅ **自动分片**: 基于一一致性哈希自动路由请求
- ✅ **稳定路由**: 相同的 key 总是路由到相同的实例
- ✅ **连接池**: 自动管理客户端连接池
- ✅ **线程安全**: 所有操作都是并发安全的
- ✅ **独立模块**: 不污染主项目依赖

## 快速开始

### 安装

```bash
# 作为独立模块使用
go get github.com/wii/uniface/pkg/storage/kv/aerospike
```

### 基本使用

```go
package main

import (
    "context"
    "log"
    
    "github.com/wii/uniface/pkg/storage/kv/aerospike"
)

func main() {
    // 1. 定义 Aerospike 实例
    instances := []*aerospike.Instance{
        {
            ID:        "node-1",
            Host:      "192.168.1.1",
            Port:      3000,
            Namespace: "test",
            Set:       "users",
        },
        {
            ID:        "node-2",
            Host:      "192.168.1.2",
            Port:      3000,
            Namespace: "test",
            Set:       "users",
        },
        {
            ID:        "node-3",
            Host:      "192.168.1.3",
            Port:      3000,
            Namespace: "test",
            Set:       "users",
        },
    }

    // 2. 创建分片客户端
    client, err := aerospike.NewShardClient(instances)
    if err != nil {
        log.Fatalf("创建客户端失败: %v", err)
    }
    defer client.Close()

    ctx := context.Background()

    // 3. 写入数据
    err = client.Put(ctx, "user-123", map[string]interface{}{
        "name":  "Alice",
        "email": "alice@example.com",
        "age":   30,
    })
    if err != nil {
        log.Fatalf("写入失败: %v", err)
    }

    // 4. 读取数据
    record, err := client.Get(ctx, "user-123")
    if err != nil {
        log.Fatalf("读取失败: %v", err)
    }

    log.Printf("读取数据: %+v", record.Bins)
}
```

## API 文档

### 创建客户端

#### `NewShardClient(instances []*Instance, opts ...Option) (*ShardClient, error)`

创建分片客户端。

```go
// 简单创建
client, err := aerospike.NewShardClient(instances)

// 带配置创建
client, err := aerospike.NewShardClient(instances,
    aerospike.WithConnectTimeout(10*time.Second),
    aerospike.WithPoolSize(20),
    aerospike.WithAuth("user", "password"),
)
```

### 配置选项

```go
// 连接超时
aerospike.WithConnectTimeout(timeout time.Duration)

// 读写超时
aerospike.WithReadTimeout(timeout time.Duration)
aerospike.WithWriteTimeout(timeout time.Duration)

// 连接池配置
aerospike.WithPoolSize(size int)
aerospike.WithMinIdleConns(n int)
aerospike.WithMaxIdleConns(n int)

// 认证
aerospike.WithAuth(user, password string)

// TLS
aerospike.WithTLS(enable bool)

// Key 前缀
aerospike.WithKeyPrefix(prefix string)
```

### 数据操作

#### `Put(ctx context.Context, key string, bins as.BinMap) error`

写入数据。

```go
err := client.Put(ctx, "user-123", map[string]interface{}{
    "name":  "Alice",
    "email": "alice@example.com",
})
```

#### `PutWithTTL(ctx context.Context, key string, bins as.BinMap, ttl uint32) error`

写入数据并设置 TTL。

```go
// 写入数据，1 小时后过期
err := client.PutWithTTL(ctx, "session-abc", bins, 3600)
```

#### `Get(ctx context.Context, key string, binNames ...string) (*as.Record, error)`

读取数据。

```go
// 读取所有字段
record, err := client.Get(ctx, "user-123")

// 只读取特定字段
record, err := client.Get(ctx, "user-123", "name", "email")
```

#### `Delete(ctx context.Context, key string) error`

删除数据。

```go
err := client.Delete(ctx, "user-123")
```

#### `Exists(ctx context.Context, key string) (bool, error)`

检查数据是否存在。

```go
exists, err := client.Exists(ctx, "user-123")
```

#### `BatchGet(ctx context.Context, keys []string) (map[string]*as.Record, error)`

批量读取数据。

```go
records, err := client.BatchGet(ctx, []string{"user-1", "user-2", "user-3"})
```

### 路由信息

#### `GetInstance(key string) (*Instance, error)`

获取 key 路由到的实例信息。

```go
inst, err := client.GetInstance("user-123")
fmt.Printf("路由到实例: %s (%s:%d)\n", inst.ID, inst.Host, inst.Port)
```

#### `GetClient(ctx context.Context, key string) (*as.Client, error)`

获取底层 Aerospike 客户端（高级用法）。

```go
asClient, err := client.GetClient(ctx, "user-123")
// 使用 asClient 进行高级操作
```

## 工作原理

```
用户请求 (key="user-123")
    ↓
ShardClient.Get("user-123")
    ↓
ShardManager.Select("user-123")
    ↓
一致性哈希算法
    ↓
选择实例 node-2 (192.168.1.2:3000)
    ↓
获取/创建 Aerospike 客户端连接
    ↓
执行 Aerospike Get 操作
    ↓
返回结果
```

**关键特性**：
- 相同的 key 总是路由到相同的实例（稳定性）
- 客户端连接自动缓存和复用
- 线程安全的操作

## Instance 配置

```go
type Instance struct {
    ID        string            // 实例标识
    Host      string            // 主机地址
    Port      int               // 端口号
    Namespace string            // Aerospike 命名空间
    Set       string            // 默认 Set 名称
    Metadata  map[string]string // 用户自定义元数据
}
```

## 最佳实践

### 1. 合理配置实例数量

```go
// 推荐：至少 3 个实例以实现高可用
instances := []*aerospike.Instance{
    {ID: "node-1", Host: "192.168.1.1", Port: 3000, ...},
    {ID: "node-2", Host: "192.168.1.2", Port: 3000, ...},
    {ID: "node-3", Host: "192.168.1.3", Port: 3000, ...},
}
```

### 2. 配置连接池

```go
client, err := aerospike.NewShardClient(instances,
    aerospike.WithPoolSize(20),
    aerospike.WithMinIdleConns(5),
    aerospike.WithIdleTimeout(5*time.Minute),
)
```

### 3. 设置合理的超时

```go
client, err := aerospike.NewShardClient(instances,
    aerospike.WithConnectTimeout(5*time.Second),
    aerospike.WithReadTimeout(3*time.Second),
    aerospike.WithWriteTimeout(3*time.Second),
)
```

### 4. 使用 TTL 管理临时数据

```go
// 会话数据 1 小时后自动过期
client.PutWithTTL(ctx, "session-"+sessionID, sessionData, 3600)
```

### 5. 正确关闭客户端

```go
client, _ := aerospike.NewShardClient(instances)
defer client.Close() // 确保资源释放
```

## 测试

```bash
# 运行单元测试
go test -v ./pkg/storage/kv/aerospike/

# 运行集成测试（需要 Aerospike 服务器）
go test -v -tags=integration ./pkg/storage/kv/aerospike/
```

## 注意事项

1. **依赖隔离**: Aerospike 依赖只在本子模块中，不影响主项目
2. **实例固定**: 创建后实例列表不可修改，需要修改请重新创建客户端
3. **错误处理**: 所有错误都包含上下文信息，便于排查问题
4. **资源管理**: 记得调用 Close() 释放资源
5. **批量操作**: BatchGet 当前是简单实现，后续会优化

## 故障排查

### 连接失败

```
错误: 创建 Aerospike 客户端失败 [192.168.1.1:3000]: connection refused
```

**解决方案**：
- 检查 Aerospike 服务是否运行
- 检查网络连接和防火墙配置
- 增加连接超时时间

### Key 不存在

```
错误: 记录不存在: user-123
```

**解决方案**：
- 检查 key 是否正确
- 检查 namespace 和 set 是否正确
- 使用 Exists() 先检查是否存在

### 性能问题

**优化建议**：
- 增加连接池大小
- 减少网络延迟（使用更近的实例）
- 使用批量操作代替多次单独操作

## 文档

- [实现计划](./docs/features/rpc/governance/load-balancer/shard/aerospike/00-aerospike-shard-client-plan.md)
- [需求文档](./prompts/features/rpc/governance/load-balancer/shard/aerospike/00-aerospike-shared-client.md)
- [Shard Manager 文档](../shard/README.md)

## License

MIT
