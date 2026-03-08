# Aerospike KV Interface 实现

## 概述

本包提供两种 API：

1. **ShardClient** - 原生 Aerospike API，高性能，支持直接操作 bins
2. **Storage** - 标准 KV 接口，提供通用性和可移植性

本文档专注于 **Storage** 的使用。

## 快速开始

### 基本使用

```go
package main

import (
    "context"
    "log"
    
    "github.com/wii/uniface/pkg/storage/kv/aerospike"
)

func main() {
    // 创建 Storage 实例
    storage, err := aerospike.NewStorage([]*aerospike.Instance{
        {ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "users"},
    })
    if err != nil {
        log.Fatal(err)
    }
    defer storage.Close()
    
    ctx := context.Background()
    
    // 写入
    user := User{Name: "Alice", Age: 30}
    if err := storage.Set(ctx, "user:123", user); err != nil {
        log.Fatal(err)
    }
    
    // 读取
    var result User
    if err := storage.Get(ctx, "user:123", &result); err != nil {
        log.Fatal(err)
    }
    
    log.Printf("User: %+v", result)
    
    // 删除
    if err := storage.Delete(ctx, "user:123"); err != nil {
        log.Fatal(err)
    }
    
    // 检查存在
    exists, _ := storage.Exists(ctx, "user:123")
    log.Printf("Exists: %v", exists)
}
```

### 自定义 Bin 名称

```go
// 默认 bin 名称: "data"
storage, _ := aerospike.NewStorage(instances, 
    aerospike.WithBinName("custom_bin"),
)
```

### TTL 支持

```go
// 写入 10 秒后过期
storage.Set(ctx, "session:abc", sessionData, kv.WithTTL(10*time.Second))
```

### Namespace 支持

```go
// 使用 namespace（实际存储的 key: "tenant1:user:123"）
storage.Set(ctx, "user:123", value, kv.WithNamespace("tenant1"))
```

### NoOverwrite 支持

```go
// 仅当 key 不存在时写入
err := storage.Set(ctx, "user:123", value, kv.WithNoOverwrite())
if errors.Is(err, kv.ErrKeyAlreadyExists) {
    // key 已存在
}
```

### 自定义序列化

```go
import "github.com/vmihailenco/msgpack/v5"

// 使用 MessagePack 序列化（更快，更小）
storage, _ := aerospike.NewStorage(instances,
    aerospike.WithSerializer(
        func(v interface{}) ([]byte, error) {
            return msgpack.Marshal(v)
        },
        func(data []byte, v interface{}) error {
            return msgpack.Unmarshal(data, v)
        },
    ),
)
```

### 全局 Key 前缀

```go
// 所有 key 都添加前缀
storage, _ := aerospike.NewStorage(instances,
    aerospike.WithStorageKeyPrefix("myapp:"),
)

// 实际存储: "myapp:user:123"
storage.Set(ctx, "user:123", value)
```

## 限制

⚠️ **批量操作不支持**

由于 Aerospike Go 客户端限制，以下方法会返回 `ErrBatchNotSupported` 错误：

- `BatchSet`
- `BatchGet`
- `BatchDelete`

### 替代方案

使用循环逐个操作：

```go
// 代替 BatchSet
for key, value := range items {
    if err := storage.Set(ctx, key, value); err != nil {
        // 处理错误
    }
}

// 代替 BatchGet
results := make(map[string]interface{})
for _, key := range keys {
    var value MyType
    if err := storage.Get(ctx, key, &value); err == nil {
        results[key] = value
    }
}

// 代替 BatchDelete
for _, key := range keys {
    if err := storage.Delete(ctx, key); err != nil {
        // 处理错误
    }
}
```

## API 参考

### 构造函数

#### `NewStorage(instances []*Instance, opts ...StorageOption) (*Storage, error)`

创建 Storage 实例。

**参数:**
- `instances` - Aerospike 实例列表
- `opts` - 配置选项

**返回:**
- `*Storage` - Storage 实例
- `error` - 错误

### 配置选项

#### Storage 专用选项

- `WithBinName(name string)` - 设置 bin 名称（默认 "data"）
- `WithSerializer(serialize, deserialize)` - 自定义序列化器
- `WithStorageKeyPrefix(prefix string)` - 全局 key 前缀

#### 继承自 ShardClient 的选项

- `WithConnectTimeout(d time.Duration)` - 连接超时
- `WithReadTimeout(d time.Duration)` - 读取超时
- `WithWriteTimeout(d time.Duration)` - 写入超时
- `WithPoolSize(size int)` - 连接池大小
- `WithMinIdleConns(n int)` - 最小空闲连接数
- `WithMaxIdleConns(n int)` - 最大空闲连接数
- `WithIdleTimeout(d time.Duration)` - 空闲连接超时
- `WithMaxRetries(n int)` - 最大重试次数
- `WithRetryDelay(d time.Duration)` - 重试延迟
- `WithAuth(user, password string)` - 认证
- `WithTLS(enable bool)` - 启用 TLS
- `WithKeyPrefix(prefix string)` - 全局 key 前缀（ShardClient 级别）

### 实现的方法

✅ **支持的方法**

#### `Set(ctx context.Context, key string, value interface{}, opts ...kv.Option) error`

写入数据。

**支持的选项:**
- `kv.WithTTL(duration)` - 设置过期时间
- `kv.WithNamespace(ns)` - 设置命名空间
- `kv.WithNoOverwrite()` - 不覆盖已存在的 key

#### `Get(ctx context.Context, key string, value interface{}) error`

读取数据。

**注意:** `value` 必须是指针类型。

#### `Delete(ctx context.Context, key string) error`

删除数据。

#### `Exists(ctx context.Context, key string) (bool, error)`

检查 key 是否存在。

#### `Close() error`

关闭 Storage 并释放资源。

❌ **不支持的方法**

#### `BatchSet(ctx context.Context, items map[string]interface{}, opts ...kv.Option) error`

始终返回 `ErrBatchNotSupported`。

#### `BatchGet(ctx context.Context, keys []string) (map[string]interface{}, error)`

始终返回 `ErrBatchNotSupported`。

#### `BatchDelete(ctx context.Context, keys []string) error`

始终返回 `ErrBatchNotSupported`。

## 选择指南

### 使用 Storage 当：

- ✅ 需要统一的 KV 存储接口
- ✅ 需要与 Redis 等实现互换
- ✅ 数据结构简单，不需要部分 bin 操作
- ✅ 不需要批量操作
- ✅ 需要灵活切换底层存储

### 使用 ShardClient 当：

- ✅ 需要批量操作
- ✅ 需要直接操作 Aerospike bins
- ✅ 需要部分 bin 读取（指定 binNames）
- ✅ 需要最高性能（避免序列化开销）
- ✅ 使用 Aerospike 特有功能

## 数据存储格式

Storage 将数据序列化后存储在单个 bin 中：

```
Aerospike Record:
┌────────────────────────────────────┐
│ Key: "user:123"                    │
├────────────────────────────────────┤
│ Bins:                              │
│  - "data": <JSON/serialized data> │
└────────────────────────────────────┘
```

**默认 bin 名称**: `"data"`（可通过 `WithBinName()` 配置）

**序列化方式**: JSON（可通过 `WithSerializer()` 配置）

## 错误处理

所有错误都使用 `kv.StorageError` 包装：

```go
err := storage.Get(ctx, "key", &value)
if err != nil {
    var storageErr *kv.StorageError
    if errors.As(err, &storageErr) {
        log.Printf("操作: %s, Key: %s, 错误: %v",
            storageErr.Op, storageErr.Key, storageErr.Err)
    }
    
    // 检查特定错误
    if errors.Is(err, kv.ErrKeyNotFound) {
        // key 不存在
    }
    if errors.Is(err, kv.ErrStorageClosed) {
        // Storage 已关闭
    }
    if errors.Is(err, kv.ErrInvalidKey) {
        // 无效的 key
    }
    if errors.Is(err, aerospike.ErrBatchNotSupported) {
        // 不支持批量操作
    }
}
```

### 常见错误

| 错误 | 说明 | 解决方案 |
|------|------|----------|
| `ErrKeyNotFound` | key 不存在 | 检查 key 是否正确 |
| `ErrKeyAlreadyExists` | key 已存在（NoOverwrite） | 检查是否需要覆盖 |
| `ErrInvalidKey` | 无效的 key（空字符串） | 提供有效的 key |
| `ErrStorageClosed` | Storage 已关闭 | 不要在关闭后使用 |
| `ErrBatchNotSupported` | 不支持批量操作 | 使用循环代替 |

## 性能考虑

### 序列化开销

Storage 使用序列化（默认 JSON），会有性能开销：

| 序列化方式 | 性能 | 大小 | 推荐场景 |
|-----------|------|------|----------|
| JSON | 中等 | 大 | 通用场景，调试方便 |
| MessagePack | 快 | 小 | 高性能场景 |
| Protobuf | 最快 | 最小 | 需要预定义 schema |

### 连接池优化

合理配置连接池参数：

```go
storage, _ := aerospike.NewStorage(instances,
    aerospike.WithPoolSize(20),        // 连接池大小
    aerospike.WithMinIdleConns(5),     // 最小空闲连接
    aerospike.WithMaxIdleConns(15),    // 最大空闲连接
    aerospike.WithIdleTimeout(5*time.Minute),
)
```

### 避免批量操作

由于不支持批量操作，在高性能场景下：
- 考虑使用 ShardClient
- 使用 goroutine 并发执行单个操作

## 兼容性

Storage 完全实现了 `kv.Storage` 接口，可以无缝切换：

```go
import "github.com/wii/uniface/pkg/storage/kv"

func UseStorage(storage kv.Storage) {
    // 可以是 Redis、Aerospike 或其他实现
    storage.Set(ctx, "key", value)
}

// 使用 Redis
redisStorage, _ := redis.New(redis.WithAddr("localhost:6379"))
UseStorage(redisStorage)

// 切换到 Aerospike
aerospikeStorage, _ := aerospike.NewStorage([]*aerospike.Instance{
    {ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "cache"},
})
UseStorage(aerospikeStorage)
```

## 最佳实践

1. **选择合适的 API**
   - 需要通用性：使用 Storage
   - 需要性能和批量操作：使用 ShardClient

2. **配置 bin 名称**
   - 根据业务语义配置有意义的 bin 名称
   - 例如：`"data"`、`"cache"`、`"session"`

3. **优化序列化**
   - 高性能场景使用 MessagePack
   - 调试场景使用 JSON

4. **合理配置连接池**
   - 根据并发量调整连接池大小
   - 避免连接池过大或过小

5. **正确处理错误**
   - 检查特定错误类型
   - 提供有意义的错误处理

6. **避免批量操作**
   - 使用循环代替批量操作
   - 必要时使用 goroutine 并发

7. **使用 TTL 管理临时数据**
   - 会话、缓存等临时数据使用 TTL
   - 避免手动清理

## 迁移指南

### 从 ShardClient 迁移到 Storage

```go
// ShardClient
client.Put(ctx, "user:123", as.BinMap{
    "name": "Alice",
    "age":  30,
})
record, _ := client.Get(ctx, "user:123")
name := record.Bins["name"]

// Storage
storage.Set(ctx, "user:123", User{Name: "Alice", Age: 30})
var user User
storage.Get(ctx, "user:123", &user)
name := user.Name
```

### 从 Redis 迁移到 Aerospike

```go
// 只需更换创建方式，其他代码完全相同

// Redis
storage, _ := redis.New(redis.WithAddr("localhost:6379"))

// Aerospike
storage, _ := aerospike.NewStorage([]*aerospike.Instance{
    {ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "cache"},
})

// 其他代码保持不变！
storage.Set(ctx, "key", value)
storage.Get(ctx, "key", &value)
storage.Delete(ctx, "key")
```

## 常见问题

### Q: 为什么不支持批量操作？

A: Aerospike Go 客户端本身不支持批量操作。如果需要批量操作，请使用 ShardClient 或使用循环代替。

### Q: 默认 bin 名称是什么？

A: 默认是 `"data"`，可以通过 `WithBinName()` 配置。

### Q: 如何选择 Storage 和 ShardClient？

A: 
- 需要统一接口和可移植性：使用 Storage
- 需要批量操作和高性能：使用 ShardClient

### Q: 数据存储在哪里？

A: Storage 将序列化后的数据存储在单个 bin 中（默认名称 "data"）。

### Q: 可以修改 bin 名称吗？

A: 可以，使用 `WithBinName()` 配置。但注意：不同 bin 名称的数据不能互通。

### Q: 如何提高性能？

A: 
1. 使用更快的序列化器（如 MessagePack）
2. 合理配置连接池
3. 考虑使用 ShardClient（避免序列化开销）

### Q: 支持哪些序列化方式？

A: 支持任何序列化方式，通过 `WithSerializer()` 配置。默认使用 JSON。

### Q: TTL 精度如何？

A: TTL 精度为秒级，由 Aerospike 服务器控制。

## 参考

- [ShardClient 文档](./README.md) - 原生 Aerospike API
- [KV Interface 文档](../README.md) - 通用 KV 存储接口
- [Redis 实现](../redis/) - Redis KV 实现
- [Aerospike 官方文档](https://docs.aerospike.com/)
- [Aerospike Go Client](https://github.com/aerospike/aerospike-client-go)

## License

MIT
