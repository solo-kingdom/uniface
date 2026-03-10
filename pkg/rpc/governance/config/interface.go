// Package config 提供配置存储接口，支持直读、缓存、写入和监听功能。
//
// 基于 prompts/features/storage/config/00-iface.md 实现
package config

import "context"

// Handler 是配置变更的处理器类型。
// 当监听的配置发生变更时，会调用此处理器。
//
// 参数:
//   - ctx: 上下文
//   - key: 发生变更的配置键
//   - value: 新的配置值（可能为 nil，表示配置被删除）
//
// 返回:
//   - error: 处理器执行过程中返回的错误
type Handler func(ctx context.Context, key string, value interface{}) error

// Storage 定义了配置存储接口，提供了配置管理的完整功能。
// 实现应该保证线程安全，并正确处理上下文取消。
type Storage interface {
	// Read 直接从存储读取配置，不经过缓存。
	//
	// 参数:
	//   - ctx: 上下文，用于取消操作
	//   - key: 配置键名
	//   - value: 输出参数，用于存储读取的配置值（必须是指针类型）
	//   - opts: 可选配置项
	//
	// 返回:
	//   - error: 如果操作失败返回错误，如 ErrNotFound
	Read(ctx context.Context, key string, value interface{}, opts ...Option) error

	// ReadWithCache 带缓存的读取配置。
	// 首先尝试从缓存读取，如果缓存未命中，则从存储读取并更新缓存。
	//
	// 参数:
	//   - ctx: 上下文
	//   - key: 配置键名
	//   - value: 输出参数，用于存储读取的配置值（必须是指针类型）
	//   - opts: 可选配置项，可覆盖默认缓存 TTL
	//
	// 返回:
	//   - error: 如果操作失败返回错误
	ReadWithCache(ctx context.Context, key string, value interface{}, opts ...Option) error

	// Write 写入配置到存储，并通知所有监听器。
	// 如果键已存在，则更新值；如果不存在，则创建新配置。
	//
	// 参数:
	//   - ctx: 上下文
	//   - key: 配置键名
	//   - value: 要写入的配置值
	//   - opts: 可选配置项
	//
	// 返回:
	//   - error: 如果操作失败返回错误
	Write(ctx context.Context, key string, value interface{}, opts ...Option) error

	// Delete 删除配置，并通知所有监听器。
	// 如果配置不存在，不返回错误。
	//
	// 参数:
	//   - ctx: 上下文
	//   - key: 配置键名
	//
	// 返回:
	//   - error: 如果操作失败返回错误
	Delete(ctx context.Context, key string) error

	// Watch 监听指定配置键的变更。
	// 当配置发生变更（写入、删除）时，会调用处理器。
	//
	// 注意：此方法会阻塞，直到上下文被取消或返回错误。
	// 建议在单独的 goroutine 中调用。
	//
	// 参数:
	//   - ctx: 上下文，用于取消监听
	//   - key: 要监听的配置键名
	//   - handler: 配置变更处理器
	//   - opts: 可选配置项
	//
	// 返回:
	//   - error: 如果监听启动失败返回错误
	Watch(ctx context.Context, key string, handler Handler, opts ...Option) error

	// Unwatch 取消对指定配置键的监听。
	// 如果键没有被监听，不返回错误。
	//
	// 参数:
	//   - key: 配置键名
	//
	// 返回:
	//   - error: 如果操作失败返回错误
	Unwatch(key string) error

	// WatchPrefix 监听指定前缀的所有配置键的变更。
	// 匹配前缀的所有键发生变更时，都会调用处理器。
	//
	// 注意：此方法会阻塞，直到上下文被取消或返回错误。
	// 建议在单独的 goroutine 中调用。
	//
	// 参数:
	//   - ctx: 上下文，用于取消监听
	//   - prefix: 要监听的配置键前缀
	//   - handler: 配置变更处理器
	//   - opts: 可选配置项
	//
	// 返回:
	//   - error: 如果监听启动失败返回错误
	WatchPrefix(ctx context.Context, prefix string, handler Handler, opts ...Option) error

	// UnwatchPrefix 取消对指定前缀的监听。
	// 如果前缀没有被监听，不返回错误。
	//
	// 参数:
	//   - prefix: 配置键前缀
	//
	// 返回:
	//   - error: 如果操作失败返回错误
	UnwatchPrefix(prefix string) error

	// List 列出所有匹配前缀的配置键。
	//
	// 参数:
	//   - ctx: 上下文
	//   - prefix: 配置键前缀，空字符串表示所有键
	//
	// 返回:
	//   - []string: 匹配的配置键列表
	//   - error: 如果操作失败返回错误
	List(ctx context.Context, prefix string) ([]string, error)

	// ClearCache 清除指定配置键的缓存。
	// 如果键名为空，清除所有缓存。
	//
	// 参数:
	//   - key: 配置键名，空字符串表示清除所有缓存
	//
	// 返回:
	//   - error: 如果操作失败返回错误
	ClearCache(key string) error

	// Close 关闭配置存储，释放所有资源。
	// 关闭后，所有操作应该返回错误。
	Close() error
}
