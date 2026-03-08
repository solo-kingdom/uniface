// Package shard provides simple sharding management based on load balancers.
// This file defines the ShardManager interface.
//
// 基于 prompts/features/rpc/governance/load-balancer/shard/00-shard-manager.md 实现
package shard

import (
	"github.com/wii/uniface/pkg/rpc/governance/loadbalancer"
)

// ClientFactory is a function that creates a client for an instance.
type ClientFactory func(*loadbalancer.Instance) (interface{}, error)

// Manager defines the shard manager interface.
// It provides simple key-based routing to service/database instances.
//
// 核心特性：
// - 基于 key 进行稳定路由（相同 key 始终路由到相同实例）
// - 初始化时指定实例列表，不支持动态修改
// - 线程安全
type Manager interface {
	// Select selects an instance based on the key using consistent hashing.
	// The same key always routes to the same instance (stability).
	//
	// 参数:
	//   - key: 分片键，用于确定路由到哪个实例
	//
	// 返回:
	//   - *Instance: 选中的实例
	//   - error: 如果选择失败返回错误
	Select(key string) (*loadbalancer.Instance, error)

	// SelectClient selects and returns a client for an instance based on the key.
	// If the client is already cached, it returns the cached client.
	// If not, it creates one using the factory and caches it.
	//
	// 参数:
	//   - key: 分片键
	//   - factory: 客户端工厂函数
	//
	// 返回:
	//   - interface{}: 客户端实例
	//   - error: 如果选择失败返回错误
	SelectClient(key string, factory ClientFactory) (interface{}, error)

	// Close closes the shard manager and releases all resources.
	Close() error
}
