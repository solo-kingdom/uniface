# Aerospike 分片客户端实现计划

## 需求概述

基于 Shard Manager 实现 Aerospike 的分片访问实现，以独立的 go submodule 方式组织，避免为主项目引入太多依赖。

## 背景

### 现有架构

- `pkg/rpc/governance/loadbalancer/shard/` - 已实现的 Shard Manager
- `pkg/storage/kv/` - KV 存储接口定义
- `pkg/storage/kv/redis/` - Redis 独立子模块参考实现

### 设计目标

1. **独立性**: 作为独立 go submodule，不污染主项目依赖
2. **复用性**: 基于 Shard Manager 接口，复用现有分片能力
3. **简洁性**: 提供简单的 API，隐藏分片复杂性
4. **可扩展**: 支持自定义配置和客户端工厂

## 技术方案

### 目录结构

```
pkg/storage/kv/aerospike/
├── go.mod              # 独立模块定义
├── go.sum              # 依赖校验
├── aerospike.go        # 核心实现
├── options.go          # 配置选项
├── client.go           # 分片客户端封装
├── aerospike_test.go   # 单元测试
└── example_test.go     # 使用示例
```

### 核心组件

#### 1. ShardClient - 分片客户端

```go
// ShardClient 提供基于 Shard Manager 的 Aerospike 分片客户端
type ShardClient struct {
    manager shard.Manager
    // ...
}

// NewShardClient 创建分片客户端
func NewShardClient(instances []*Instance, opts ...Option) (*ShardClient, error)

// 基本操作
func (c *ShardClient) Get(ctx context.Context, key string) (*Record, error)
func (c *ShardClient) Put(ctx context.Context, key string, record *Record) error
func (c *ShardClient) Delete(ctx context.Context, key string) error
```

#### 2. Instance - 实例定义

```go
// Instance 表示一个 Aerospike 实例配置
type Instance struct {
    ID       string
    Host     string
    Port     int
    Namespace string
    Set      string
    // 用户自定义元数据
    Metadata map[string]string
}
```

#### 3. Options - 配置选项

```go
type Config struct {
    // 连接超时
    ConnectTimeout time.Duration
    // 读写超时
    ReadTimeout    time.Duration
    WriteTimeout   time.Duration
    // 连接池配置
    PoolSize       int
    // 认证配置
    User           string
    Password       string
    // 其他配置
    // ...
}

type Option func(*Config)
```

### 工作流程

```
用户请求
    ↓
ShardClient.Get(key)
    ↓
ShardManager.Select(key)
    ↓
ConsistentHash 算法
    ↓
选择目标 Instance
    ↓
获取/创建 Client 连接
    ↓
执行 Aerospike 操作
    ↓
返回结果
```

## 实现步骤

### 第一阶段：基础框架

1. ✅ 创建目录结构
2. ✅ 创建 go.mod 独立模块
3. ✅ 实现 Instance 定义
4. ✅ 实现 Options 配置

### 第二阶段：核心实现

1. ✅ 实现 ShardClient 结构
2. ✅ 实现客户端工厂
3. ✅ 实现基本 CRUD 操作
4. ✅ 实现连接管理

### 第三阶段：测试与文档

1. ✅ 编写单元测试
2. ✅ 编写使用示例
3. ✅ 编写 README
4. ✅ 编写变更说明

## 依赖管理

### go.mod 配置

```go
module github.com/solo-kingdom/uniface/pkg/storage/kv/aerospike

go 1.24

require (
    github.com/aerospike/aerospike-client-go/v7 v7.x.x
    github.com/solo-kingdom/uniface v0.0.0
)

replace github.com/solo-kingdom/uniface => ../../../../
```

### 依赖说明

- `aerospike-client-go` - Aerospike 官方 Go 客户端
- `solo-kingdom/uniface` - 主项目，提供 Shard Manager 接口

## API 设计

### 初始化

```go
// 方式1：简单初始化
client, err := aerospike.NewShardClient([]*aerospike.Instance{
    {ID: "node-1", Host: "192.168.1.1", Port: 3000, Namespace: "test"},
    {ID: "node-2", Host: "192.168.1.2", Port: 3000, Namespace: "test"},
    {ID: "node-3", Host: "192.168.1.3", Port: 3000, Namespace: "test"},
})

// 方式2：带配置初始化
client, err := aerospike.NewShardClient(instances,
    aerospike.WithConnectTimeout(5*time.Second),
    aerospike.WithPoolSize(10),
)
```

### 基本操作

```go
// 写入
err := client.Put(ctx, "user:123", &aerospike.Record{
    BinMap: map[string]interface{}{
        "name":  "Alice",
        "email": "alice@example.com",
    },
})

// 读取
record, err := client.Get(ctx, "user:123")

// 删除
err := client.Delete(ctx, "user:123")

// 批量操作（TODO: 后续扩展）
```

### 高级特性

```go
// 自定义客户端工厂
client, err := aerospike.NewShardClientWithFactory(
    instances,
    aerospike.DefaultClientFactory,
    opts...,
)

// 获取底层客户端（高级用法）
rawClient, err := client.GetClient(ctx, "user:123")
```

## 测试策略

### 单元测试

- ✅ 测试分片路由稳定性
- ✅ 测试客户端创建和缓存
- ✅ 测试基本 CRUD 操作
- ✅ 测试错误处理

### 集成测试

- 需要 Aerospike 集群环境
- 可使用 Docker Compose 启动测试集群
- 测试真实分片场景

## 注意事项

1. **依赖隔离**: Aerospike 依赖只在子模块中，不影响主项目
2. **错误处理**: 所有错误使用 fmt.Errorf 包装，提供上下文
3. **资源管理**: 实现 Close() 方法，确保连接正确释放
4. **线程安全**: 所有导出方法必须是线程安全的
5. **文档规范**: 遵循 Go 文档规范，添加中文注释

## 后续扩展

- [ ] 批量操作支持
- [ ] 连接池优化
- [ ] 监控指标
- [ ] 健康检查
- [ ] 故障转移

## 参考资料

- [Aerospike Go Client](https://github.com/aerospike/aerospike-client-go)
- [Shard Manager 设计文档](../00-shard-manager-plan.md)
- [Redis 子模块实现](../../../../storage/kv/redis/)
