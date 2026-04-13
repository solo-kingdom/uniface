# KV Storage

泛型键值存储接口，提供基本 CRUD 操作和批量操作。

- **接口定义**: `pkg/storage/kv/interface.go`
- **实现**: Redis (`pkg/storage/kv/redis/`), Aerospike (`pkg/storage/kv/aerospike/`), BoltDB (`pkg/storage/kv/boltdb/`)

---

## 接口

```go
type Storage interface {
    Set(ctx context.Context, key string, value interface{}, opts ...Option) error
    Get(ctx context.Context, key string, value interface{}, opts ...Option) error
    Delete(ctx context.Context, key string, opts ...Option) error
    BatchSet(ctx context.Context, items map[string]interface{}, opts ...Option) error
    BatchGet(ctx context.Context, keys []string, opts ...Option) (map[string]interface{}, error)
    BatchDelete(ctx context.Context, keys []string, opts ...Option) error
    Exists(ctx context.Context, key string, opts ...Option) (bool, error)
    List(ctx context.Context, opts ...Option) ([]string, error)
    Close() error
}
```

## 配置选项

| 选项 | 类型 | 说明 |
|------|------|------|
| `WithTTL(d time.Duration)` | TTL | 键的存活时间，0 表示永不过期 |
| `WithNamespace(ns string)` | Namespace | 所有键的前缀 |
| `WithNoOverwrite()` | NoOverwrite | 禁止覆盖已存在的值 |
| `WithReadonly()` | Readonly | 标记为只读操作 |
| `WithCompress()` | Compress | 启用值压缩 |

## 错误

| Sentinel 错误 | 说明 |
|---------------|------|
| `ErrKeyNotFound` | 请求的键不存在 |
| `ErrKeyAlreadyExists` | 键已存在（排他模式下） |
| `ErrInvalidKey` | 无效的键 |
| `ErrInvalidValue` | 无效的值 |
| `ErrStorageClosed` | 存储已关闭 |
| `ErrOperationFailed` | 操作失败 |

## 行为规格

### Requirement: Set 操作

系统 SHALL 支持通过 `Set` 存储键值对。如果键已存在，SHALL 被覆盖（除非指定 `NoOverwrite`）。

#### Scenario: 写入新键
- **WHEN** 调用 `Set(ctx, "key1", "value1")`
- **THEN** 键值对被存储，返回 nil

#### Scenario: 覆盖已有键
- **WHEN** 键 "key1" 已存在，调用 `Set(ctx, "key1", "newValue")`
- **THEN** 值被更新为新值，返回 nil

#### Scenario: NoOverwrite 模式
- **WHEN** 键 "key1" 已存在，调用 `Set(ctx, "key1", "value", WithNoOverwrite())`
- **THEN** 返回 `ErrKeyAlreadyExists` 错误

#### Scenario: 设置 TTL
- **WHEN** 调用 `Set(ctx, "key1", "value1", WithTTL(time.Minute))`
- **THEN** 键在 1 分钟后自动过期

### Requirement: Get 操作

系统 SHALL 支持通过 `Get` 读取键对应的值。如果键不存在，SHALL 返回 `ErrKeyNotFound`。

#### Scenario: 读取已存在的键
- **WHEN** 键 "key1" 存在且值为 "value1"，调用 `Get(ctx, "key1", &result)`
- **THEN** result 被设置为 "value1"，返回 nil

#### Scenario: 读取不存在的键
- **WHEN** 键 "missing" 不存在，调用 `Get(ctx, "missing", &result)`
- **THEN** 返回 `ErrKeyNotFound` 错误

### Requirement: Delete 操作

系统 SHALL 支持通过 `Delete` 删除键值对。如果键不存在，SHALL 返回 nil（不报错）。

#### Scenario: 删除已存在的键
- **WHEN** 键 "key1" 存在，调用 `Delete(ctx, "key1")`
- **THEN** 键值对被删除，返回 nil

#### Scenario: 删除不存在的键
- **WHEN** 键 "missing" 不存在，调用 `Delete(ctx, "missing")`
- **THEN** 返回 nil

### Requirement: BatchSet 操作

系统 SHALL 支持通过 `BatchSet` 原子性地存储多个键值对。如果任一操作失败，SHALL 回滚整个批次。

#### Scenario: 批量写入多个键
- **WHEN** 调用 `BatchSet(ctx, map[string]interface{}{"k1": "v1", "k2": "v2"})`
- **THEN** 所有键值对被存储，返回 nil

### Requirement: BatchGet 操作

系统 SHALL 支持通过 `BatchGet` 批量读取多个键的值。不存在的键 SHALL 不出现在结果中。

#### Scenario: 批量读取
- **WHEN** 键 "k1" 存在、"k2" 不存在，调用 `BatchGet(ctx, []string{"k1", "k2"})`
- **THEN** 返回 `{"k1": value1}`，不包含 "k2"，返回 nil

### Requirement: BatchDelete 操作

系统 SHALL 支持通过 `BatchDelete` 原子性地删除多个键值对。不存在的键 SHALL 被忽略。

#### Scenario: 批量删除
- **WHEN** 调用 `BatchDelete(ctx, []string{"k1", "k2"})`
- **THEN** 所有指定键被删除（不存在的被忽略），返回 nil

### Requirement: Exists 操作

系统 SHALL 支持通过 `Exists` 检查键是否存在。

#### Scenario: 键存在
- **WHEN** 键 "key1" 存在，调用 `Exists(ctx, "key1")`
- **THEN** 返回 `(true, nil)`

#### Scenario: 键不存在
- **WHEN** 键 "missing" 不存在，调用 `Exists(ctx, "missing")`
- **THEN** 返回 `(false, nil)`

### Requirement: List 操作

系统 SHALL 支持通过 `List` 列出匹配选项的所有键。当指定 Namespace 时，SHALL 仅返回该命名空间下的键。

#### Scenario: 列出所有键
- **WHEN** 存储中有 "ns:k1"、"ns:k2"、"other:k3"，调用 `List(ctx, WithNamespace("ns"))`
- **THEN** 返回 `["ns:k1", "ns:k2"]`（仅命名空间 "ns" 下的键）

### Requirement: Close 操作

系统 SHALL 支持通过 `Close` 关闭存储并释放资源。关闭后所有其他操作 SHALL 返回 `ErrStorageClosed`。

#### Scenario: 关闭后操作
- **WHEN** 调用 `Close()` 后，再调用 `Get(ctx, "key1", &result)`
- **THEN** 返回 `ErrStorageClosed` 错误

### Requirement: 线程安全

所有 Storage 实现 SHALL 保证线程安全，支持并发调用。

#### Scenario: 并发读写
- **WHEN** 多个 goroutine 同时调用 Set、Get、Delete
- **THEN** 不发生数据竞争，所有操作正确完成

## 实现要求

- 所有实现 MUST 使用 `sync.RWMutex` 保证线程安全
- 所有实现 MUST 支持 Options 模式配置
- 错误 MUST 使用 `StorageError` 包装，支持 `errors.Is/As` 解包
- `Close()` MUST 正确释放所有资源（连接、缓存等）
