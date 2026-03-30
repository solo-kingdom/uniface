# Aerospike 分片客户端实现变更说明

## 变更概述

实现了基于 Shard Manager 的 Aerospike 分片客户端，作为独立的 go submodule，避免为主项目引入 Aerospike 依赖。

## 变更类型

- ✅ 新增功能
- ✅ 独立模块

## 变更内容

### 1. 新增文件

#### 独立子模块：`pkg/storage/kv/aerospike/`

- **go.mod** - 独立模块定义，依赖 Aerospike 客户端 v7.10.2
- **go.sum** - 依赖校验文件
- **options.go** - 配置选项定义，支持连接池、超时、认证等配置
- **client.go** - 分片客户端实现，基于 Shard Manager
- **aerospike.go** - CRUD 操作实现（Get, Put, PutWithTTL, Delete, Exists, BatchGet）
- **aerospike_test.go** - 单元测试
- **example_test.go** - 使用示例
- **README.md** - 完整的使用文档

### 2. 核心功能

#### ShardClient

```go
type ShardClient struct {
    manager shard.Manager
    config  *Config
    clients sync.Map
    mu      sync.RWMutex
    closed  bool
}
```

**核心方法**：
- `NewShardClient(instances []*Instance, opts ...Option)` - 创建分片客户端
- `Get(ctx, key string, binNames ...string)` - 读取记录
- `Put(ctx, key string, bins as.BinMap)` - 写入记录
- `PutWithTTL(ctx, key string, bins as.BinMap, ttl uint32)` - 带 TTL 写入
- `Delete(ctx, key string)` - 删除记录
- `Exists(ctx, key string)` - 检查记录是否存在
- `BatchGet(ctx, keys []string)` - 批量读取
- `GetInstance(key string)` - 获取路由信息
- `GetClient(ctx, key string)` - 获取底层客户端
- `Close()` - 关闭客户端

#### Instance 配置

```go
type Instance struct {
    ID        string
    Host      string
    Port      int
    Namespace string
    Set       string
    Metadata  map[string]string
}
```

#### 配置选项

```go
// 连接配置
WithConnectTimeout(timeout time.Duration)
WithReadTimeout(timeout time.Duration)
WithWriteTimeout(timeout time.Duration)

// 连接池配置
WithPoolSize(size int)
WithMinIdleConns(n int)
WithMaxIdleConns(n int)

// 认证
WithAuth(user, password string)

// TLS
WithTLS(enable bool)

// 其他
WithKeyPrefix(prefix string)
```

### 3. 使用示例

```go
// 创建分片客户端
client, _ := aerospike.NewShardClient([]*aerospike.Instance{
    {ID: "node-1", Host: "192.168.1.1", Port: 3000, Namespace: "test", Set: "users"},
    {ID: "node-2", Host: "192.168.1.2", Port: 3000, Namespace: "test", Set: "users"},
    {ID: "node-3", Host: "192.168.1.3", Port: 3000, Namespace: "test", Set: "users"},
})
defer client.Close()

// 写入数据
client.Put(ctx, "user-123", map[string]interface{}{
    "name":  "Alice",
    "email": "alice@example.com",
})

// 读取数据
record, _ := client.Get(ctx, "user-123")
```

## 技术亮点

### 1. 独立模块设计

- 使用 go submodule 独立管理依赖
- 主项目不引入 Aerospike 依赖
- 通过 `replace` 指令引用主项目

```go
module github.com/solo-kingdom/uniface/pkg/storage/kv/aerospike

require (
    github.com/aerospike/aerospike-client-go/v7 v7.10.2
    github.com/solo-kingdom/uniface v0.0.0
)

replace github.com/solo-kingdom/uniface => ../../../../
```

### 2. 基于 Shard Manager

- 复用现有的 Shard Manager 实现
- 基于一致性哈希实现稳定路由
- 自动管理客户端连接池

### 3. 完整的错误处理

- 所有错误使用 `fmt.Errorf` 包装上下文
- 区分不同类型的错误（连接、查询、删除等）
- 提供清晰的错误信息

### 4. 线程安全

- 使用 `sync.RWMutex` 保护并发访问
- 使用 `sync.Map` 缓存客户端连接
- 所有导出方法都是线程安全的

### 5. 资源管理

- 实现 `Close()` 方法释放资源
- 支持多次调用 `Close()`（幂等）
- 关闭后操作返回错误

## 测试覆盖

### 单元测试
- ✅ 客户端创建测试
- ✅ 路由稳定性测试
- ✅ 配置选项测试
- ✅ 资源关闭测试
- ✅ Instance 转换测试

### 集成测试
- ✅ CRUD 操作测试（需要 Aerospike 服务器）
- ⏳ 批量操作测试（TODO）

## 文档

- ✅ README.md - 完整的使用文档和最佳实践
- ✅ 计划文档 - `docs/features/rpc/governance/load-balancer/shard/aerospike/00-aerospike-shard-client-plan.md`
- ✅ 代码注释 - 所有导出项都有文档注释
- ✅ 使用示例 - 11 个完整的使用示例

## 依赖

### 直接依赖
- `github.com/aerospike/aerospike-client-go/v7` v7.10.2
- `github.com/solo-kingdom/uniface` (主项目)

### 间接依赖
- `github.com/cespare/xxhash/v2` - 哈希算法
- `golang.org/x/crypto` - 加密库
- `golang.org/x/exp` - 实验性功能

## 影响范围

### 新增内容
- ✅ 独立子模块：`pkg/storage/kv/aerospike/`
- ✅ 文档：`docs/features/rpc/governance/load-balancer/shard/aerospike/`

### 不影响
- ❌ 主项目依赖
- ❌ 现有代码
- ❌ 现有测试

## 后续工作

### 短期（P1）
- [ ] 添加更多集成测试
- [ ] 性能基准测试
- [ ] 连接池优化

### 中期（P2）
- [ ] 批量操作优化
- [ ] 健康检查
- [ ] 监控指标

### 长期（P3）
- [ ] 故障转移
- [ ] 读写分离
- [ ] 分布式追踪

## 使用建议

### 1. 基本配置

```go
client, _ := aerospike.NewShardClient(instances,
    aerospike.WithConnectTimeout(5*time.Second),
    aerospike.WithPoolSize(20),
    aerospike.WithMinIdleConns(5),
)
```

### 2. 实例配置

- 至少配置 3 个实例以实现高可用
- 确保实例分布在不同节点
- 合理配置 namespace 和 set

### 3. 错误处理

```go
record, err := client.Get(ctx, key)
if err != nil {
    // 处理错误
    log.Printf("读取失败: %v", err)
    return
}
```

### 4. 资源清理

```go
client, _ := aerospike.NewShardClient(instances)
defer client.Close() // 确保关闭
```

## 兼容性

- ✅ Go 1.24+
- ✅ Aerospike 4.x/5.x/6.x/7.x
- ✅ Linux/macOS/Windows

## 已知问题

1. **批量操作**：BatchGet 当前是简单实现，后续会优化为按分片分组
2. **TLS 配置**：TLS 配置尚未完全实现，需要后续补充
3. **监控指标**：尚未添加监控指标，需要后续补充

## 参考资料

- [Aerospike Go Client](https://github.com/aerospike/aerospike-client-go)
- [Shard Manager 设计](../00-shard-manager-plan.md)
- [Redis 子模块](../../../../storage/kv/redis/)
- [需求文档](../../../../prompts/features/rpc/governance/load-balancer/shard/aerospike/00-aerospike-shared-client.md)

## 变更日期

2026-03-08

## 变更作者

AI Assistant
