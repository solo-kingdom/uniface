// Package kv provides a generic key-value storage interface.
// This package defines the contract for KV storage implementations,
// supporting basic CRUD operations and batch operations.
//
// 基于 prompts/features/kv-storage/00-iface.md 实现
package kv

import "context"

// Storage defines the interface for key-value storage operations.
// Implementations should be thread-safe and handle errors appropriately.
type Storage interface {
	// Set stores a key-value pair.
	// If the key already exists, it will be overwritten.
	//
	// 参数:
	//   - ctx: 上下文，用于取消操作
	//   - key: 键名，非空字符串
	//   - value: 要存储的值
	//   - opts: 可选配置项（如 TTL）
	//
	// 返回:
	//   - error: 如果操作失败返回错误
	Set(ctx context.Context, key string, value interface{}, opts ...Option) error

	// Get retrieves the value associated with the given key.
	// If the key does not exist, returns ErrNotFound.
	//
	// 参数:
	//   - ctx: 上下文
	//   - key: 键名
	//   - value: 输出参数，用于存储获取的值
	//
	// 返回:
	//   - error: 如果操作失败返回错误
	Get(ctx context.Context, key string, value interface{}) error

	// Delete removes the key-value pair associated with the given key.
	// If the key does not exist, returns nil (no error).
	//
	// 参数:
	//   - ctx: 上下文
	//   - key: 键名
	//
	// 返回:
	//   - error: 如果操作失败返回错误
	Delete(ctx context.Context, key string) error

	// BatchSet stores multiple key-value pairs atomically.
	// If any operation fails, the entire batch should be rolled back.
	//
	// 参数:
	//   - ctx: 上下文
	//   - items: 键值对映射
	//   - opts: 可选配置项
	//
	// 返回:
	//   - error: 如果操作失败返回错误
	BatchSet(ctx context.Context, items map[string]interface{}, opts ...Option) error

	// BatchGet retrieves values for multiple keys.
	// Missing keys will not appear in the result.
	//
	// 参数:
	//   - ctx: 上下文
	//   - keys: 键名列表
	//
	// 返回:
	//   - map[string]interface{}: 键值对映射
	//   - error: 如果操作失败返回错误
	BatchGet(ctx context.Context, keys []string) (map[string]interface{}, error)

	// BatchDelete removes multiple key-value pairs atomically.
	// If a key does not exist, it is ignored (no error).
	//
	// 参数:
	//   - ctx: 上下文
	//   - keys: 键名列表
	//
	// 返回:
	//   - error: 如果操作失败返回错误
	BatchDelete(ctx context.Context, keys []string) error

	// Exists checks if a key exists in the storage.
	//
	// 参数:
	//   - ctx: 上下文
	//   - key: 键名
	//
	// 返回:
	//   - bool: 键是否存在
	//   - error: 如果操作失败返回错误
	Exists(ctx context.Context, key string) (bool, error)

	// Close closes the storage and releases any held resources.
	// After calling Close, all other operations should return errors.
	Close() error
}
