# 配置存储接口

配置存储接口，提供了完整的配置管理功能，支持直读、缓存、写入和监听变更。

## 特性

- ✅ **直读与缓存读取**：支持直接读取和带缓存的读取
- ✅ **智能缓存**：自动缓存管理，支持 TTL 过期
- ✅ **实时监听**：支持键级和前缀级配置变更监听
- ✅ **类型安全**：支持任意 Go 类型的配置值
- ✅ **原子操作**：写入操作自动清除缓存并通知监听器
- ✅ **错误处理**：详细的错误信息和错误包装
- ✅ **线程安全**：实现应保证线程安全
- ✅ **配置列表**：支持按前缀列出配置键

## 安装

```bash
go get github.com/wii/uniface/pkg/storage/config
```

## 快速开始

### 基本使用

```go
package main

import (
    "context"
    "fmt"
    "github.com/wii/uniface/pkg/storage/config"
)

func main() {
    ctx := context.Background()
    
    // 创建配置存储实例（需要实现 Storage 接口）
    var store config.Storage = NewYourConfigStorage()
    
    // 写入配置
    err := store.Write(ctx, "database.host", "localhost")
    if err != nil {
        panic(err)
    }
    
    // 直读配置
    var host string
    err = store.Read(ctx, "database.host", &host)
    if err != nil {
        panic(err)
    }
    fmt.Println("Database Host:", host)
    
    // 关闭存储
    store.Close()
}
```

### 使用缓存读取

```go
// 带缓存的读取（默认 TTL 5 分钟）
var port int
err := store.ReadWithCache(ctx, "database.port", &port)
if err != nil {
    panic(err)
}

// 自定义缓存 TTL
var timeout int
err = store.ReadWithCache(ctx, "database.timeout", &timeout, 
    config.WithCacheTTL(10*time.Minute))
```

### 监听配置变更

```go
// 监听单个配置键
go func() {
    handler := func(ctx context.Context, key string, value interface{}) error {
        fmt.Printf("配置 %s 变更为: %v\n", key, value)
        return nil
    }
    
    err := store.Watch(ctx, "database.host", handler)
    if err != nil {
        log.Printf("监听失败: %v", err)
    }
}()

// 监听前缀匹配的所有键
go func() {
    handler := func(ctx context.Context, key string, value interface{}) error {
        fmt.Printf("配置 %s 变更为: %v\n", key, value)
        return nil
    }
    
    err := store.WatchPrefix(ctx, "database", handler)
    if err != nil {
        log.Printf("监听失败: %v", err)
    }
}()
```

### 批量操作

```go
// 列出所有匹配前缀的配置
keys, err := store.List(ctx, "database")
if err != nil {
    panic(err)
}

for _, key := range keys {
    var value interface{}
    store.Read(ctx, key, &value)
    fmt.Printf("%s = %v\n", key, value)
}
```

## API 文档

### Storage 接口

```go
type Storage interface {
    // Read 直接从存储读取配置，不经过缓存
    Read(ctx context.Context, key string, value interface{}, opts ...Option) error
    
    // ReadWithCache 带缓存的读取配置
    ReadWithCache(ctx context.Context, key string, value interface{}, opts ...Option) error
    
    // Write 写入配置到存储，并通知所有监听器
    Write(ctx context.Context, key string, value interface{}, opts ...Option) error
    
    // Delete 删除配置，并通知所有监听器
    Delete(ctx context.Context, key string) error
    
    // Watch 监听指定配置键的变更
    Watch(ctx context.Context, key string, handler Handler, opts ...Option) error
    
    // Unwatch 取消对指定配置键的监听
    Unwatch(key string) error
    
    // WatchPrefix 监听指定前缀的所有配置键的变更
    WatchPrefix(ctx context.Context, prefix string, handler Handler, opts ...Option) error
    
    // UnwatchPrefix 取消对指定前缀的监听
    UnwatchPrefix(prefix string) error
    
    // List 列出所有匹配前缀的配置键
    List(ctx context.Context, prefix string) ([]string, error)
    
    // ClearCache 清除指定配置键的缓存
    ClearCache(key string) error
    
    // Close 关闭配置存储
    Close() error
}
```

### Handler 类型

```go
type Handler func(ctx context.Context, key string, value interface{}) error
```

配置变更处理器，当监听的配置发生变更时被调用：
- `ctx`：上下文
- `key`：发生变更的配置键
- `value`：新的配置值（可能为 nil，表示配置被删除）

### 选项

- `WithCacheTTL(d time.Duration)` - 设置缓存的过期时间
- `WithCacheEnabled(enabled bool)` - 启用或禁用缓存
- `WithForceRefresh()` - 强制从源读取，绕过缓存
- `WithWatchOnChange()` - 启用变更后自动监听
- `WithNamespace(ns string)` - 设置命名空间前缀
- `WithRetryCount(count int)` - 设置重试次数
- `WithRetryDelay(delay time.Duration)` - 设置重试延迟
- `WithNoOverwrite()` - 禁止覆盖已存在的配置

### 错误类型

- `ErrConfigNotFound` - 配置不存在
- `ErrConfigAlreadyExists` - 配置已存在（使用 NoOverwrite 时）
- `ErrInvalidConfigKey` - 无效的配置键
- `ErrInvalidConfigValue` - 无效的配置值
- `ErrConfigFormat` - 配置格式错误
- `ErrVersionConflict` - 版本冲突
- `ErrWatchFailed` - 监听失败
- `ErrInvalidHandler` - 无效的处理器
- `ErrStorageClosed` - 存储已关闭
- `ErrCacheExpired` - 缓存已过期
- `ErrOperationFailed` - 操作失败

## 使用示例

### 数据库配置管理

```go
type DatabaseConfig struct {
    Host     string
    Port     int
    Username string
    Password string
    Database string
}

// 写入配置
dbConfig := DatabaseConfig{
    Host:     "localhost",
    Port:     5432,
    Username: "user",
    Password: "pass",
    Database: "mydb",
}

store.Write(ctx, "database", dbConfig)

// 读取配置
var config DatabaseConfig
store.ReadWithCache(ctx, "database", &config)

// 监听数据库配置变更
go store.Watch(ctx, "database", func(ctx context.Context, key string, value interface{}) error {
    if db, ok := value.(DatabaseConfig); ok {
        log.Printf("数据库配置已更新: %+v", db)
        // 重新连接数据库
    }
    return nil
})
```

### 应用设置管理

```go
// 写入应用设置
store.Write(ctx, "app.name", "MyApp")
store.Write(ctx, "app.version", "1.0.0")
store.Write(ctx, "app.debug", true)
store.Write(ctx, "app.maxConnections", 100)

// 列出所有应用配置
keys, _ := store.List(ctx, "app")
for _, key := range keys {
    var value interface{}
    store.Read(ctx, key, &value)
    fmt.Printf("%s = %v\n", key, value)
}

// 监听所有 app 前缀的配置变更
go store.WatchPrefix(ctx, "app", func(ctx context.Context, key string, value interface{}) error {
    log.Printf("应用配置变更: %s = %v", key, value)
    return nil
})
```

### 动态配置热更新

```go
// 配置结构
type AppConfig struct {
    LogLevel   string
    LogPath    string
    MaxWorkers int
}

// 监听配置变更并热更新
go func() {
    handler := func(ctx context.Context, key string, value interface{}) error {
        if key == "app.config" {
            if cfg, ok := value.(AppConfig); ok {
                // 更新日志级别
                setLogLevel(cfg.LogLevel)
                // 调整工作线程数
                adjustWorkerCount(cfg.MaxWorkers)
            }
        }
        return nil
    }
    
    // 注册监听器
    if err := store.Watch(ctx, "app.config", handler); err != nil {
        log.Fatalf("无法监听配置: %v", err)
    }
}()
```

## 错误处理

```go
import "errors"

// 检查特定错误
err := store.Read(ctx, "key", &value)
if errors.Is(err, config.ErrConfigNotFound) {
    fmt.Println("配置不存在，使用默认值")
    // 使用默认值
} else if err != nil {
    // 处理其他错误
    fmt.Printf("读取失败: %v\n", err)
}

// 获取详细的错误信息
if ce, ok := err.(*config.ConfigError); ok {
    fmt.Printf("操作: %s, 键: %s, 版本: %d, 错误: %v\n", 
        ce.Op, ce.Key, ce.Version, ce.Err)
}
```

## 实现指南

要实现自己的配置存储后端，
只需实现 `Storage` 接口：

```go
type MyConfigStorage struct {
    data       map[string]interface{}
    cache      map[string]cacheEntry
    watchers   map[string][]Handler
    dataMu     sync.RWMutex
    cacheMu    sync.RWMutex
    watcherMu  sync.RWMutex
    closed     bool
}

func (m *MyConfigStorage) Read(ctx context.Context, key string, value interface{}, opts ...config.Option) error {
    m.dataMu.RLock()
    defer m.dataMu.RUnlock()
    
    if m.closed {
        return config.ErrStorageClosed
    }
    
    // 从数据源读取
    data, exists := m.data[key]
    if !exists {
        return config.ErrConfigNotFound
    }
    
    // 解码值
    return decode(data, value)
}

func (m *MyConfigStorage) ReadWithCache(ctx context.Context, key string, value interface{}, opts ...config.Option) error {
    if m.closed {
        return config.ErrStorageClosed
    }
    
    options := config.DefaultOptions().Apply(opts...)
    
    m.cacheMu.Lock()
    defer m.cacheMu.Unlock()
    
    // 检查缓存
    if entry, exists := m.cache[key]; exists && time.Now().Before(entry.ExpiresAt) {
        return decode(entry.Value, value)
    }
    
    // 从数据源读取
    m.dataMu.RLock()
    data, exists := m.data[key]
    m.dataMu.RUnlock()
    
    if !exists {
        return config.ErrConfigNotFound
    }
    
    // 更新缓存
    m.cache[key] = cacheEntry{
        Value:     data,
        ExpiresAt: time.Now().Add(options.CacheTTL),
    }
    
    return decode(data, value)
}

func (m *MyConfigStorage) Write(ctx context.Context, key string, value interface{}, opts ...config.Option) error {
    m.dataMu.Lock()
    defer m.dataMu.Unlock()
    
    if m.closed {
        return config.ErrStorageClosed
    }
    
    // 写入数据
    m.data[key] = value
    
    // 清除缓存
    m.cacheMu.Lock()
    delete(m.cache, key)
    m.cacheMu.Unlock()
    
    // 通知监听器
    m.notifyWatchers(key, value)
    
    return nil
}

// 实现其他方法...
```

## 测试

运行测试：

```bash
go test ./pkg/storage/config/...
```

运行基准测试：

```bash
go test ./pkg/storage/config/... -bench=. -benchmem
```

## 性能参考

基于 mock 实现的基准测试结果（Apple M5）：

```
BenchmarkStorage_Read-10                   9,003,643    129.8 ns/op    184 B/op    3 allocs/op
BenchmarkStorage_ReadWithCache-10          6,824,592    175.8 ns/op    248 B/op    4 allocs/op
BenchmarkStorage_Write-10                 50,339,352     23.74 ns/op     0 B/op    0 allocs/op
BenchmarkStorage_ReadWithCache_Miss-10     6,952,168    171.9 ns/op    224 B/op    4 allocs/op
BenchmarkStorage_ReadWithCache_Hit-10      6,842,808    176.1 ns/op    248 B/op    4 allocs/op
```

## 设计原则

1. **简单性**：接口简洁，易于理解和使用
2. **灵活性**：通过选项模式支持各种配置
3. **性能优先**：缓存机制减少数据源访问
4. **实时性**：监听机制提供配置变更通知
5. **一致性**：写入操作自动清除缓存并通知监听器
6. **类型安全**：利用 Go 的类型系统

## 使用场景

- **应用配置**：动态加载和更新应用配置
- **服务发现**：监听服务配置变更
- **功能开关**：实时控制功能开关
- **热更新**：无需重启更新配置
- **环境管理**：不同环境的配置管理
- **多租户**：基于命名空间的配置隔离

## 注意事项

1. **缓存一致性**：写入操作会自动清除相关缓存
2. **监听器阻塞**：监听器应该在单独的 goroutine 中运行
3. **上下文取消**：正确处理上下文取消
4. **并发安全**：实现必须保证线程安全
5. **资源清理**：关闭存储后释放所有资源
6. **处理器错误**：处理器中的错误不应影响其他监听器

## 最佳实践

### 1. 缓存策略

```go
// 高频访问的配置，使用较长 TTL
err := store.ReadWithCache(ctx, "static.config", &value, 
    config.WithCacheTTL(1*time.Hour))

// 低频访问的配置，使用较短 TTL
err := store.ReadWithCache(ctx, "dynamic.config", &value,
    config.WithCacheTTL(1*time.Minute))

// 实时性要求高的配置，禁用缓存
err := store.ReadWithCache(ctx, "realtime.config", &value,
    config.DisableCache())
```

### 2. 错误重试

```go
// 配置重试策略
opts := []config.Option{
    config.WithRetryCount(5),
    config.WithRetryDelay(200*time.Millisecond),
}

err := store.Read(ctx, "unstable.config", &value, opts...)
```

### 3. 配置命名空间

```go
// 使用命名空间隔离不同模块的配置
store.Write(ctx, "module1.setting", "value1", config.WithNamespace("app"))
store.Write(ctx, "module2.setting", "value2", config.WithNamespace("app"))

// 监听整个命名空间
go store.WatchPrefix(ctx, "app:module1", handler)
```

### 4. 优雅关闭

```go
// 注册信号处理器
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

// 等待信号
<-sigChan

// 优雅关闭
log.Println("正在关闭配置存储...")
if err := store.Close(); err != nil {
    log.Printf("关闭失败: %v", err)
}
```

## 相关文档

- [AI 代码生成规则](../../../docs/AI_CODING_RULES.md)
- [项目结构说明](../../../docs/PROJECT_STRUCTURE.md)
- [需求文档](../../../prompts/features/storage/config/00-iface.md)
- [KV 存储接口](../kv/README.md)

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

本项目采用 MIT 许可证。详见 LICENSE 文件。