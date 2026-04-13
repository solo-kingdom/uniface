# Config Storage

配置存储接口，支持直读、缓存读取、写入删除和变更监听。

- **接口定义**: `pkg/storage/config/interface.go`（也用于 `pkg/rpc/governance/config/`）
- **实现**: Consul (`pkg/rpc/governance/config/consul/`)

---

## 接口

```go
type Storage interface {
    Read(ctx context.Context, key string, value interface{}, opts ...Option) error
    ReadWithCache(ctx context.Context, key string, value interface{}, opts ...Option) error
    Write(ctx context.Context, key string, value interface{}, opts ...Option) error
    Delete(ctx context.Context, key string) error
    Watch(ctx context.Context, key string, handler Handler, opts ...Option) error
    Unwatch(key string) error
    WatchPrefix(ctx context.Context, prefix string, handler Handler, opts ...Option) error
    UnwatchPrefix(prefix string) error
    List(ctx context.Context, prefix string) ([]string, error)
    ClearCache(key string) error
    Close() error
}
```

## Handler 类型

```go
type Handler func(ctx context.Context, key string, value interface{}) error
```

配置变更时调用的处理器。`value` 可能为 nil（表示配置被删除）。

## 错误

| Sentinel 错误 | 说明 |
|---------------|------|
| `ErrConfigNotFound` | 配置不存在 |
| `ErrConfigAlreadyExists` | 配置已存在（排他模式下） |
| `ErrInvalidConfigKey` | 无效的配置键 |
| `ErrInvalidConfigValue` | 无效的配置值 |
| `ErrConfigFormat` | 配置格式错误 |
| `ErrVersionConflict` | 版本冲突 |
| `ErrWatchFailed` | 监听失败 |
| `ErrInvalidHandler` | 无效的处理器 |
| `ErrStorageClosed` | 存储已关闭 |
| `ErrCacheExpired` | 缓存已过期 |
| `ErrOperationFailed` | 操作失败 |

## 行为规格

### Requirement: Read 操作

系统 SHALL 支持通过 `Read` 直接从存储读取配置，不经过缓存。

#### Scenario: 读取已存在的配置
- **WHEN** 配置键 "app.timeout" 存在，调用 `Read(ctx, "app.timeout", &result)`
- **THEN** result 被设置为配置值，返回 nil

#### Scenario: 读取不存在的配置
- **WHEN** 配置键 "missing" 不存在，调用 `Read(ctx, "missing", &result)`
- **THEN** 返回 `ErrConfigNotFound` 错误

### Requirement: ReadWithCache 操作

系统 SHALL 支持通过 `ReadWithCache` 带缓存的读取。缓存命中时 SHALL 直接返回缓存值；缓存未命中时 SHALL 从存储读取并更新缓存。

#### Scenario: 缓存命中
- **WHEN** 配置 "app.timeout" 在缓存中存在且未过期，调用 `ReadWithCache(ctx, "app.timeout", &result)`
- **THEN** 直接从缓存返回值，不访问底层存储

#### Scenario: 缓存未命中
- **WHEN** 配置 "app.timeout" 不在缓存中，调用 `ReadWithCache(ctx, "app.timeout", &result)`
- **THEN** 从底层存储读取值，更新缓存，并返回值

### Requirement: Write 操作

系统 SHALL 支持通过 `Write` 写入配置。如果键已存在则更新；不存在则创建。写入后 SHALL 通知所有监听器。

#### Scenario: 写入新配置
- **WHEN** 调用 `Write(ctx, "app.timeout", 30)`
- **THEN** 配置被创建，监听该键的 handler 被调用

#### Scenario: 更新已有配置
- **WHEN** "app.timeout" 已存在值为 30，调用 `Write(ctx, "app.timeout", 60)`
- **THEN** 值被更新为 60，监听器被通知

### Requirement: Delete 操作

系统 SHALL 支持通过 `Delete` 删除配置并通知监听器。如果配置不存在，SHALL 不返回错误。

#### Scenario: 删除已存在的配置
- **WHEN** "app.timeout" 存在，调用 `Delete(ctx, "app.timeout")`
- **THEN** 配置被删除，监听器收到 value 为 nil 的通知

#### Scenario: 删除不存在的配置
- **WHEN** "missing" 不存在，调用 `Delete(ctx, "missing")`
- **THEN** 返回 nil，不报错

### Requirement: Watch 操作

系统 SHALL 支持通过 `Watch` 监听指定键的变更。当配置发生写入或删除时，SHALL 调用处理器。此方法 SHALL 阻塞直到上下文取消或返回错误。

#### Scenario: 监听键变更
- **WHEN** 调用 `Watch(ctx, "app.timeout", handler)` 后，其他客户端写入 "app.timeout"
- **THEN** handler 被调用，参数为 (ctx, "app.timeout", newValue)

#### Scenario: 取消监听
- **WHEN** 传入的 ctx 被取消
- **THEN** Watch 方法返回，不再调用 handler

### Requirement: Unwatch 操作

系统 SHALL 支持通过 `Unwatch` 取消对指定键的监听。如果键未被监听，SHALL 不返回错误。

#### Scenario: 取消已有监听
- **WHEN** "app.timeout" 正在被监听，调用 `Unwatch("app.timeout")`
- **THEN** 该键的监听被停止

### Requirement: WatchPrefix 操作

系统 SHALL 支持通过 `WatchPrefix` 监听指定前缀下所有键的变更。匹配前缀的任何键变更时 SHALL 调用处理器。此方法 SHALL 阻塞直到上下文取消。

#### Scenario: 前缀监听
- **WHEN** 调用 `WatchPrefix(ctx, "app.", handler)` 后，"app.timeout" 被写入
- **THEN** handler 被调用，参数为 (ctx, "app.timeout", newValue)

### Requirement: UnwatchPrefix 操作

系统 SHALL 支持通过 `UnwatchPrefix` 取消前缀监听。如果前缀未被监听，SHALL 不返回错误。

#### Scenario: 取消前缀监听
- **WHEN** "app." 正在被监听，调用 `UnwatchPrefix("app.")`
- **THEN** 该前缀的监听被停止

### Requirement: List 操作

系统 SHALL 支持通过 `List` 列出匹配前缀的配置键。空前缀 SHALL 列出所有键。

#### Scenario: 列出指定前缀的键
- **WHEN** 存在 "app.timeout"、"app.name"、"db.host"，调用 `List(ctx, "app.")`
- **THEN** 返回 `["app.timeout", "app.name"]`

#### Scenario: 列出所有键
- **WHEN** 调用 `List(ctx, "")`
- **THEN** 返回所有配置键

### Requirement: ClearCache 操作

系统 SHALL 支持通过 `ClearCache` 清除缓存。指定键时清除该键的缓存；空键名时清除所有缓存。

#### Scenario: 清除指定键的缓存
- **WHEN** 调用 `ClearCache("app.timeout")`
- **THEN** 仅 "app.timeout" 的缓存被清除

#### Scenario: 清除所有缓存
- **WHEN** 调用 `ClearCache("")`
- **THEN** 所有缓存被清除

### Requirement: Close 操作

系统 SHALL 支持通过 `Close` 关闭存储并释放所有资源。关闭后所有操作 SHALL 返回 `ErrStorageClosed`。

#### Scenario: 关闭后操作
- **WHEN** 调用 `Close()` 后，再调用 `Read(ctx, "key", &result)`
- **THEN** 返回 `ErrStorageClosed` 错误

### Requirement: 线程安全

所有 Storage 实现 SHALL 保证线程安全，支持并发调用。

#### Scenario: 并发读写和监听
- **WHEN** 多个 goroutine 同时调用 Read、Write、Watch
- **THEN** 不发生数据竞争，所有操作正确完成

## 实现要求

- 所有实现 MUST 使用 `sync.RWMutex` 保证线程安全
- 错误 MUST 使用 `ConfigError` 包装，支持 `errors.Is/As` 解包
- Watch/WatchPrefix MUST 正确处理上下文取消
- `Close()` MUST 停止所有监听并释放资源
