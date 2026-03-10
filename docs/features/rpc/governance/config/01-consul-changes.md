# Config 代码迁移与 Consul 实现变更说明

## 变更概述
将配置管理代码从 `pkg/storage/config` 迁移到 `pkg/rpc/governance/config`，并实现了基于 Consul 的配置存储子模块。

## 变更日期
2026-03-10

## 变更类型
- 代码迁移
- 新功能实现
- 子模块创建

## 详细变更

### 1. 代码迁移
**原路径**: `pkg/storage/config`
**新路径**: `pkg/rpc/governance/config`

迁移文件：
- `interface.go` - Storage 接口定义
- `options.go` - 通用配置选项
- `errors.go` - 错误类型定义
- `interface_test.go` - 接口测试

### 2. Consul 子模块实现
**路径**: `pkg/rpc/governance/config/consul`

创建文件：
- `go.mod` - 子模块配置（使用 consul api v1.27.0）
- `consul.go` - Consul Storage 实现（约 640 行）
- `options.go` - Consul 特定选项（约 160 行）
- `consul_test.go` - Consul 实现测试（约 400 行）
- `example_test.go` - 使用示例（约 300 行）

### 3. 实现特性

#### Consul Storage 功能
- ✅ Read/ReadWithCache - 配置读取（支持缓存）
- ✅ Write/Delete - 配置写入和删除
- ✅ Watch/WatchPrefix - 配置变更监听
- ✅ Unwatch/UnwatchPrefix - 取消监听
- ✅ List - 列出配置键
- ✅ ClearCache - 清除缓存
- ✅ Close - 关闭存储

#### Consul 特定选项
- `WithAddress` - 设置 Consul 地址
- `WithScheme` - 设置协议（http/https）
- `WithToken` - 设置 ACL Token
- `WithNamespace` - 设置命名空间（Enterprise）
- `WithDatacenter` - 设置数据中心
- `WithKeyPrefix` - 设置配置键前缀
- `WithTLSConfig` - 设置 TLS 配置
- `WithHttpClient` - 设置自定义 HTTP 客户端
- `WithHttpAuth` - 设置 HTTP 基础认证
- `WithWaitTime` - 设置阻塞查询等待时间

### 4. 子模块配置
```go
module github.com/wii/uniface/pkg/rpc/governance/config/consul

go 1.24

require (
    github.com/hashicorp/consul/api v1.27.0
    github.com/wii/uniface v0.0.0
)

replace github.com/wii/uniface => ../../../../../
```

## 目录结构
```
pkg/rpc/governance/config/
├── interface.go          # Storage 接口定义（148 行）
├── options.go            # 通用配置选项（127 行）
├── errors.go             # 错误类型定义（86 行）
├── interface_test.go     # 接口测试（774 行）
└── consul/               # Consul 实现（子模块）
    ├── go.mod            # 子模块配置
    ├── go.sum            # 依赖锁定
    ├── consul.go         # Consul 实现（640 行）
    ├── options.go        # Consul 选项（160 行）
    ├── consul_test.go    # Consul 测试（400 行）
    └── example_test.go   # 使用示例（300 行）
```

## 技术亮点

### 1. 缓存机制
- 支持 TTL 缓存
- 写入时自动清除缓存
- 可配置是否启用缓存

### 2. 监听机制
- 基于 Consul 的阻塞查询实现
- 支持单键和前缀监听
- 使用 goroutine 异步通知

### 3. 错误处理
- 统一的错误类型（ConfigError）
- 错误包装和上下文信息
- 兼容 Go 1.13+ 错误处理

### 4. 线程安全
- 使用 sync.RWMutex 保护共享资源
- 细粒度锁控制
- 优雅关闭处理

## 兼容性说明
- Go 版本：1.24+
- Consul API 版本：v1.27.0
- 原路径 `pkg/storage/config` 的文件保持不变，确保向后兼容

## 测试覆盖
- Mock 实现测试：完整的 Storage 接口测试
- Consul 集成测试：需要运行 Consul 服务器
- 基准测试：Read/ReadWithCache/Write 性能测试

## 使用示例
```go
// 创建 Consul 配置存储
storage, err := consul.NewStorage(
    consul.WithAddress("127.0.0.1:8500"),
    consul.WithKeyPrefix("myapp/"),
)
if err != nil {
    log.Fatal(err)
}
defer storage.Close()

// 写入配置
ctx := context.Background()
err = storage.Write(ctx, "database/host", "localhost")

// 读取配置（带缓存）
var host string
err = storage.ReadWithCache(ctx, "database/host", &host,
    config.WithCacheTTL(5*time.Minute),
)

// 监听配置变更
storage.Watch(ctx, "database/host", func(ctx context.Context, key string, value interface{}) error {
    log.Printf("配置 %s 变更: %v", key, value)
    return nil
})
```

## 后续工作建议
1. 为其他配置中心（如 etcd、ZooKeeper）创建类似的子模块
2. 添加配置加密/解密支持
3. 实现配置版本管理
4. 添加配置变更历史记录
5. 实现配置回滚功能

## 参考文档
- 需求文档：`specs/features/rpc/governance/config/01 consul.md`
- 实施计划：`docs/features/rpc/governance/config/01-consul-plan.md`
- Consul API 文档：https://www.consul.io/api-docs
