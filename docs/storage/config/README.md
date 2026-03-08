# 配置存储 (Config Storage)

统一配置存储接口，支持缓存、监听和动态更新。

---

## 接口定义

基于 `prompts/features/storage/config/00-iface.md` 实现。

### Storage 接口

```go
type Storage interface {
    // 读取操作
    Read(ctx context.Context, key string, value interface{}, opts ...Option) error
    ReadWithCache(ctx context.Context, key string, value interface{}, opts ...Option) error
    
    // 写入操作
    Write(ctx context.Context, key string, value interface{}, opts ...Option) error
    Delete(ctx context.Context, key string) error
    
    // 监听功能
    Watch(ctx context.Context, key string, handler Handler, opts ...Option) error
    Unwatch(key string) error
    WatchPrefix(ctx context.Context, prefix string, handler Handler, opts ...Option) error
    UnwatchPrefix(prefix string) error
    
    // 辅助操作
    List(ctx context.Context, prefix string) ([]string, error)
    ClearCache(key string) error
    Close() error
}
```

### Handler 类型

```go
type Handler func(ctx context.Context, key string, value interface{}) error
```

---

## 实现列表

| 实现 | 路径 | 说明 | 状态 |
|------|------|------|------|
| - | - | 待实现 | 🚧 进行中 |

---

## 核心特性

### 1. 直读与缓存读取

```go
// 直读 - 每次都从存储读取
err := store.Read(ctx, "config.timeout", &timeout)

// 缓存读取 - 优先从缓存读取，缓存未命中时回源
err := store.ReadWithCache(ctx, "config.timeout", &timeout)
```

### 2. 配置监听

```go
// 监听单个配置键
err := store.Watch(ctx, "config.timeout", func(ctx context.Context, key string, value interface{}) error {
    fmt.Printf("配置 %s 已更新: %v\n", key, value)
    // 处理配置变更
    return nil
})

// 监听配置前缀
err := store.WatchPrefix(ctx, "database.", func(ctx context.Context, key string, value interface{}) error {
    fmt.Printf("数据库配置 %s 已更新\n", key)
    return nil
})
```

### 3. 配置写入与通知

```go
// 写入配置（自动通知所有监听器）
err := store.Write(ctx, "config.timeout", 30*time.Second)
```

---

## 使用场景

1. **动态配置** - 运行时动态调整应用配置
2. **特性开关** - 动态控制功能启用/禁用
3. **多租户配置** - 按租户隔离配置
4. **分布式配置** - 集中式配置管理

---

## 相关文档

- [接口定义](../../../prompts/features/storage/config/00-iface.md)
- [代码实现](../../../pkg/storage/config/)
