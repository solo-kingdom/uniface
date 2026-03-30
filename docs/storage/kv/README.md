# KV 存储 (Key-Value Storage)

统一键值存储接口，支持多种底层实现。

---

## 接口定义

基于 `prompts/features/storage/kv/00-iface.md` 实现。

### Storage 接口

```go
type Storage interface {
    // 基础操作
    Set(ctx context.Context, key string, value interface{}, opts ...Option) error
    Get(ctx context.Context, key string, value interface{}) error
    Delete(ctx context.Context, key string) error
    
    // 批量操作
    BatchSet(ctx context.Context, items map[string]interface{}, opts ...Option) error
    BatchGet(ctx context.Context, keys []string) (map[string]interface{}, error)
    BatchDelete(ctx context.Context, keys []string) error
    
    // 辅助操作
    Exists(ctx context.Context, key string) (bool, error)
    Close() error
}
```

---

## 实现列表

| 实现 | 路径 | 说明 | 状态 |
|------|------|------|------|
| Redis | `pkg/storage/kv/redis/` | Redis 实现 | ✅ 已完成 |

---

## 使用示例

### Redis 实现

```go
package main

import (
    "context"
    "time"
    
    "github.com/solo-kingdom/uniface/pkg/storage/kv"
    "github.com/solo-kingdom/uniface/pkg/storage/kv/redis"
)

func main() {
    // 创建 Redis 存储
    store, err := redis.New(
        redis.WithAddr("localhost:6379"),
        redis.WithDB(0),
        redis.WithPoolSize(10),
    )
    if err != nil {
        panic(err)
    }
    defer store.Close()
    
    ctx := context.Background()
    
    // 设置值（带 TTL）
    err = store.Set(ctx, "user:123", "John", kv.WithTTL(10*time.Minute))
    if err != nil {
        panic(err)
    }
    
    // 获取值
    var value string
    err = store.Get(ctx, "user:123", &value)
    if err != nil {
        panic(err)
    }
    
    // 检查是否存在
    exists, _ := store.Exists(ctx, "user:123")
    println("exists:", exists)
}
```

---

## 特性

- ✅ 线程安全
- ✅ 支持泛型值存储
- ✅ 批量操作支持
- ✅ TTL 支持
- ✅ 连接池管理

---

## 相关文档

- [接口定义](../../../prompts/features/storage/kv/00-iface.md)
- [代码实现](../../../pkg/storage/kv/)
