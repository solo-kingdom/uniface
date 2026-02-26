# KV 存储接口

通用的键值（Key-Value）存储接口，提供了类型安全、高性能的存储抽象。

## 特性

- ✅ **类型安全**：支持任意 Go 类型的值存储
- ✅ **批量操作**：支持原子的批量 Set、Get、Delete 操作
- ✅ **TTL 支持**：支持设置键的过期时间
- ✅ **命名空间**：支持命名空间隔离
- ✅ **选项模式**：灵活的配置选项
- ✅ **错误处理**：详细的错误信息和错误包装
- ✅ **线程安全**：实现应保证线程安全

## 安装

```bash
go get github.com/wii/uniface/pkg/kv
```

## 快速开始

### 基本使用

```go
package main

import (
    "context"
    "fmt"
    "github.com/wii/uniface/pkg/kv"
)

func main() {
    ctx := context.Background()
    
    // 创建存储实例（需要实现 Storage 接口）
    var store kv.Storage = NewYourStorageImplementation()
    
    // 设置键值
    err := store.Set(ctx, "name", "Alice")
    if err != nil {
        panic(err)
    }
    
    // 获取值
    var value string
    err = store.Get(ctx, "name", &value)
    if err != nil {
        panic(err)
    }
    fmt.Println("Name:", value) // 输出: Name: Alice
    
    // 删除键
    err = store.Delete(ctx, "name")
    if err != nil {
        panic(err)
    }
}
```

### 使用选项

```go
import (
    "time"
)

// 设置带 TTL 的键
err := store.Set(ctx, "session", "session-data", kv.WithTTL(24*time.Hour))

// 在命名空间中设置键
err := store.Set(ctx, "user:123", userData, kv.WithNamespace("app"))

// 禁止覆盖已存在的键
err := store.Set(ctx, "unique-key", value, kv.WithNoOverwrite())
```

### 批量操作

```go
// 批量设置
items := map[string]interface{}{
    "key1": "value1",
    "key2": 42,
    "key3": struct{ Name string }{"Alice"},
}
err := store.BatchSet(ctx, items)

// 批量获取
keys := []string{"key1", "key2", "key3"}
values, err := store.BatchGet(ctx, keys)

// 批量删除
err := store.BatchDelete(ctx, []string{"key1", "key2"})
```

## API 文档

### Storage 接口

```go
type Storage interface {
    // Set 存储键值对
    Set(ctx context.Context, key string, value interface{}, opts ...Option) error
    
    // Get 获取键对应的值
    Get(ctx context.Context, key string, value interface{}) error
    
    // Delete 删除键
    Delete(ctx context.Context, key string) error
    
    // BatchSet 批量设置键值对
    BatchSet(ctx context.Context, items map[string]interface{}, opts ...Option) error
    
    // BatchGet 批量获取值
    BatchGet(ctx context.Context, keys []string) (map[string]interface{}, error)
    
    // BatchDelete 批量删除键
    BatchDelete(ctx context.Context, keys []string) error
    
    // Exists 检查键是否存在
    Exists(ctx context.Context, key string) (bool, error)
    
    // Close 关闭存储
    Close() error
}
```

### 选项

- `WithTTL(d time.Duration)` - 设置键的过期时间
- `WithNamespace(ns string)` - 设置命名空间前缀
- `WithNoOverwrite()` - 禁止覆盖已存在的键
- `WithReadonly()` - 标记为只读操作
- `WithCompress()` - 启用值压缩

### 错误类型

- `ErrKeyNotFound` - 键不存在
- `ErrKeyAlreadyExists` - 键已存在（使用 NoOverwrite 时）
- `ErrInvalidKey` - 无效的键
- `ErrInvalidValue` - 无效的值
- `ErrStorageClosed` - 存储已关闭
- `ErrOperationFailed` - 操作失败

## 错误处理

```go
import "errors"

// 检查特定错误
err := store.Get(ctx, "key", &value)
if errors.Is(err, kv.ErrKeyNotFound) {
    fmt.Println("键不存在")
} else if err != nil {
    // 处理其他错误
    fmt.Printf("获取失败: %v\n", err)
}

// 获取详细的错误信息
if se, ok := err.(*kv.StorageError); ok {
    fmt.Printf("操作: %s, 键: %s, 错误: %v\n", se.Op, se.Key, se.Err)
}
```

## 实现 Storage 接口

要实现自己的存储后端，只需实现 `Storage` 接口：

```go
type MyStorage struct {
    data map[string]interface{}
    mu   sync.RWMutex
}

func (m *MyStorage) Set(ctx context.Context, key string, value interface{}, opts ...kv.Option) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.data[key] = value
    return nil
}

// 实现其他方法...
```

## 测试

运行测试：

```bash
go test ./pkg/kv/...
```

运行基准测试：

```bash
go test ./pkg/kv/... -bench=. -benchmem
```

## 性能参考

基于 mock 实现的基准测试结果（Apple M5）：

```
BenchmarkStorage_Set-10         39065335    30.99 ns/op    56 B/op     2 allocs/op
BenchmarkStorage_Get-10          329128   3500 ns/op    2706 B/op    3 allocs/op
BenchmarkStorage_BatchSet-10      269504   4412 ns/op   12232 B/op  109 allocs/op
BenchmarkStorage_BatchGet-10      258896   4654 ns/op   10968 B/op  111 allocs/op
```

## 设计原则

1. **简单性**：接口简洁，易于理解和使用
2. **灵活性**：通过选项模式支持各种配置
3. **可扩展性**：易于添加新的存储后端
4. **类型安全**：利用 Go 的类型系统
5. **性能优先**：最小化开销，高效操作

## 使用场景

- 配置存储
- 会话管理
- 缓存层
- 临时数据存储
- 应用状态管理

## 注意事项

1. Get 操作的 value 参数必须是指针类型
2. 批量操作应该是原子的，要么全部成功，要么全部失败
3. TTL 是在存储层面实现的，不是严格的过期时间
4. 关闭存储后，所有操作都应该返回错误
5. 实现必须保证线程安全

## 示例项目

查看 `examples/` 目录获取更多使用示例。

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

本项目采用 MIT 许可证。详见 LICENSE 文件。

## 相关文档

- [AI 代码生成规则](../../docs/AI_CODING_RULES.md)
- [项目结构说明](../../docs/PROJECT_STRUCTURE.md)
- [需求文档](../../prompts/features/kv-storage/00-iface.md)