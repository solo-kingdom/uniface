// Package loadbalancer provides a generic load balancer interface for RPC services.
// This package defines the contract for load balancer implementations,
// supporting instance selection, client management, and multiple balancing strategies.
//
// 基于 prompts/features/service/governance/load-balancer/01-load-balancer-iface.md 实现
package loadbalancer

import "context"

// Instance represents an RPC service instance.
type Instance struct {
	// ID is the unique identifier for this instance (required).
	ID string

	// Address is the IP address or hostname of the instance (required).
	Address string

	// Port is the port number of the instance (required).
	Port int

	// Weight is used for weighted load balancing algorithms.
	// Higher weights receive more traffic. Default is 1.
	Weight int

	// Metadata contains additional information about the instance (optional).
	Metadata map[string]string
}

// Balancer[T] defines the load balancer interface with generic client type.
// T is the client type, such as *grpc.ClientConn, *http.Client, etc.
//
// 线程安全：所有实现必须是线程安全的
// 资源管理：Close() 必须正确释放所有资源
type Balancer[T any] interface {
	// ========== Instance Selection ==========

	// Select selects an instance from available instances.
	// If opts specifies a Key, consistent hashing algorithm is used.
	// If no Key or Key is empty, the default strategy is used.
	//
	// 参数:
	//   - ctx: 上下文，用于取消操作
	//   - opts: 可选配置项（如 Key、Filter）
	//
	// 返回:
	//   - *Instance: 选中的实例
	//   - error: 如果选择失败返回错误（如 ErrNoInstances、ErrBalancerClosed）
	Select(ctx context.Context, opts ...Option) (*Instance, error)

	// SelectClient selects and returns a client for an instance.
	// If the client is already cached, it returns the cached client.
	// If the client is not cached, it calls ClientFactory to create and cache it.
	// The same instance always returns the same client.
	//
	// 参数:
	//   - ctx: 上下文，用于取消操作
	//   - opts: 可选配置项（必须包含 ClientFactory）
	//
	// 返回:
	//   - T: 客户端实例
	//   - error: 如果选择失败返回错误（如 ErrNoClientFactory、ErrNoInstances）
	//
	// 注意:
	//   - ClientFactory 必须通过 WithClientFactory 选项提供
	//   - Client 会被缓存，相同实例复用相同的 client
	//   - 如果 client 实现了 io.Closer，会在 Remove/Close 时自动调用 Close()
	SelectClient(ctx context.Context, opts ...Option) (T, error)

	// ========== Instance Management ==========

	// Add adds an instance to the load balancer.
	// Returns ErrDuplicateInstance if the instance ID already exists.
	//
	// 参数:
	//   - ctx: 上下文
	//   - instance: 要添加的实例（ID、Address、Port 必须有效）
	//
	// 返回:
	//   - error: 如果添加失败返回错误
	Add(ctx context.Context, instance *Instance) error

	// Remove removes an instance from the load balancer.
	// Also closes and removes the associated client if it exists.
	// Returns ErrInstanceNotFound if the instance doesn't exist.
	//
	// 参数:
	//   - ctx: 上下文
	//   - instanceID: 要移除的实例 ID
	//
	// 返回:
	//   - error: 如果移除失败返回错误
	Remove(ctx context.Context, instanceID string) error

	// Update updates an instance's information.
	// Returns ErrInstanceNotFound if the instance doesn't exist.
	//
	// 参数:
	//   - ctx: 上下文
	//   - instance: 要更新的实例信息
	//
	// 返回:
	//   - error: 如果更新失败返回错误
	Update(ctx context.Context, instance *Instance) error

	// GetAll returns a copy of all instances.
	//
	// 参数:
	//   - ctx: 上下文
	//
	// 返回:
	//   - []*Instance: 所有实例的副本
	//   - error: 如果获取失败返回错误
	GetAll(ctx context.Context) ([]*Instance, error)

	// ========== Lifecycle ==========

	// Close closes the load balancer and releases all resources.
	// It closes all cached clients and clears all instances.
	// After calling Close, all other operations should return ErrBalancerClosed.
	//
	// 返回:
	//   - error: 如果关闭失败返回错误
	Close() error
}
