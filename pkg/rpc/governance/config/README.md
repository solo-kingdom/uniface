# Config 配置管理模块

基于 `specs/features/rpc/governance/config/01 consul.md` 实现

## 概述

Config 模块提供了统一的配置存储接口，支持多种配置中心实现。目前实现了 Consul 配置中心，未来可以扩展支持 etcd、ZooKeeper 等。

## 目录结构

```
config/
├── interface.go          # Storage 接口定义
├── options.go            # 通用配置选项
├── errors.go             # 错误类型定义
├── interface_test.go     # 接口测试
└── consul/               # Consul 实现（子模块）
    ├── go.mod            # 子模块配置
    ├── consul.go         # Consul 实现
    ├── options.go        # Consul 选项
    ├── consul_test.go    # Consul 测试
    └── example_test.go   # 使用示例
```

## 功能特性

### 核心功能
- ✅ 配置读取（Read/ReadWithCache）
- ✅ 配置写入（Write）
- ✅ 配置删除（Delete）
- ✅ 配置监听（Watch/WatchPrefix）
- ✅ 配置列表（List）
- ✅ 缓存管理（ClearCache）
- ✅ 资源清理（Close）

### Consul 实现特性
- 基于 Consul KV 存储实现
- 支持阻塞查询实现实时监听
- 支持缓存机制
- 支持 ACL Token 认证
- 支持 TLS 加密
- 支持命名空间（Enterprise）
- 支持多数据中心

## 快速开始

### 安装

```bash
# 安装 Consul 子模块
go get github.com/solo-kingdom/uniface/pkg/rpc/governance/config/consul
```

### 基本使用

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/solo-kingdom/uniface/pkg/rpc/governance/config"
    "github.com/solo-kingdom/uniface/pkg/rpc/governance/config/consul"
)

func main() {
    // 1. 创建 Consul 配置存储
    storage, err := consul.NewStorage(
        consul.WithAddress("127.0.0.1:8500"),
        consul.WithKeyPrefix("myapp/"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer storage.Close()

    ctx := context.Background()

    // 2. 写入配置
    err = storage.Write(ctx, "database/host", "localhost")
    if err != nil {
        log.Fatal(err)
    }

    // 3. 读取配置（带缓存）
    var host string
    err = storage.ReadWithCache(ctx, "database/host", &host,
        config.WithCacheTTL(5*time.Minute),
    )
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("数据库主机: %s", host)

    // 4. 监听配置变更
    err = storage.Watch(ctx, "database/host", 
        func(ctx context.Context, key string, value interface{}) error {
            log.Printf("配置 %s 变更: %v", key, value)
            return nil
        },
    )
    if err != nil {
        log.Fatal(err)
    }
    defer storage.Unwatch("database/host")

    // 保持程序运行
    select {}
}
```

## 配置选项

### Consul 选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `WithAddress(addr string)` | Consul 服务器地址 | `127.0.0.1:8500` |
| `WithScheme(scheme string)` | 协议类型 | `http` |
| `WithToken(token string)` | ACL Token | 空 |
| `WithNamespace(ns string)` | 命名空间（Enterprise） | 空 |
| `WithDatacenter(dc string)` | 数据中心 | 空 |
| `WithKeyPrefix(prefix string)` | 配置键前缀 | `config/` |
| `WithTLSConfig(tls *tls.Config)` | TLS 配置 | nil |
| `WithHttpClient(client *http.Client)` | 自定义 HTTP 客户端 | 默认客户端 |
| `WithHttpAuth(username, password string)` | HTTP 基础认证 | 空 |
| `WithWaitTime(d time.Duration)` | 阻塞查询等待时间 | `30s` |

### 通用选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `WithCacheTTL(ttl time.Duration)` | 缓存 TTL | `5m` |
| `WithCacheEnabled(enabled bool)` | 是否启用缓存 | `true` |
| `WithForceRefresh()` | 强制刷新 | `false` |
| `WithNamespace(ns string)` | 命名空间前缀 | 空 |
| `WithRetryCount(count int)` | 重试次数 | `3` |
| `WithRetryDelay(delay time.Duration)` | 重试延迟 | `100ms` |
| `WithNoOverwrite()` | 不覆盖已存在的配置 | `false` |

## 高级用法

### 存储复杂配置

```go
type DatabaseConfig struct {
    Host     string `json:"host"`
    Port     int    `json:"port"`
    Username string `json:"username"`
    Password string `json:"password"`
    Database string `json:"database"`
}

// 写入
config := DatabaseConfig{
    Host:     "localhost",
    Port:     5432,
    Username: "user",
    Password: "pass",
    Database: "mydb",
}
err := storage.Write(ctx, "database/config", config)

// 读取
var loadedConfig DatabaseConfig
err = storage.Read(ctx, "database/config", &loadedConfig)
```

### 监听前缀

```go
// 监听所有以 "services/" 开头的配置
err := storage.WatchPrefix(ctx, "services/", 
    func(ctx context.Context, key string, value interface{}) error {
        log.Printf("服务配置 %s 变更: %v", key, value)
        return nil
    },
)
```

### 列出配置

```go
// 列出所有以 "database/" 开头的配置键
keys, err := storage.List(ctx, "database/")
for _, key := range keys {
    log.Printf("配置键: %s", key)
}
```

## 错误处理

```go
var value string
err := storage.Read(ctx, "nonexistent/key", &value)
if err != nil {
    if errors.Is(err, config.ErrConfigNotFound) {
        // 配置不存在
        log.Println("配置不存在，使用默认值")
    } else {
        // 其他错误
        log.Printf("读取配置失败: %v", err)
    }
}
```

## 运行测试

### 单元测试

```bash
cd pkg/rpc/governance/config/consul
go test -v
```

### 集成测试

集成测试需要运行 Consul 服务器：

```bash
# 使用 Docker 启动 Consul
docker run -d -p 8500:8500 --name consul consul:latest

# 运行测试
go test -v

# 停止 Consul
docker stop consul
docker rm consul
```

### 基准测试

```bash
go test -bench=. -benchmem
```

## 扩展其他配置中心

可以参考 Consul 实现，为其他配置中心创建类似的子模块：

1. 创建新的子模块目录（如 `etcd/`）
2. 实现 `Storage` 接口
3. 创建 `go.mod` 文件
4. 编写测试和示例

## 最佳实践

1. **使用缓存**：对于频繁读取的配置，启用缓存可以显著提高性能
2. **监听变更**：使用 Watch 机制实时响应配置变更
3. **错误处理**：妥善处理配置不存在的情况
4. **资源清理**：使用 defer 确保 Close 被调用
5. **键命名规范**：使用有意义的键名，如 `database/host`、`cache/ttl`

## 文档

- [实施计划](./01-consul-plan.md)
- [变更说明](./01-consul-changes.md)
- [需求文档](../../../specs/features/rpc/governance/config/01 consul.md)

## 许可证

Apache License 2.0
