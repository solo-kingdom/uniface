// Package loadbalancer provides a universal load balancer interface.
// This file contains configuration options for load balancer operations.
//
// 基于 prompts/features/service/governance/load-balancer/01-load-balancer-iface.md 实现
package loadbalancer

// Options represents the configuration options for load balancer operations.
type Options struct {
	// Key is used for consistent hashing selection (optional).
	// If specified and non-empty, consistent hashing algorithm is used.
	// The same Key always selects the same instance (stability).
	Key string

	// ClientFactory is a factory function to create clients (optional).
	// If nil, SelectClient returns ErrNoClientFactory.
	// The factory should return a client of type T.
	ClientFactory func(*Instance) (interface{}, error)

	// Filter is an instance filter function (optional).
	// Only instances for which Filter returns true will be selected.
	Filter func(*Instance) bool
}

// Option is a function that modifies Options.
type Option func(*Options)

// Apply applies the given options to the current Options.
func (o *Options) Apply(opts ...Option) *Options {
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// DefaultOptions returns the default options for load balancer operations.
func DefaultOptions() *Options {
	return &Options{
		Key:           "",
		ClientFactory: nil,
		Filter:        nil,
	}
}

// MergeOptions merges multiple options into one.
// Later options override earlier ones.
func MergeOptions(opts ...Option) *Options {
	return DefaultOptions().Apply(opts...)
}

// WithKey sets the key for consistent hashing.
// When a key is provided, the same key always selects the same instance.
// This is useful for sharding scenarios where you need stable routing.
//
// 示例:
//
//	// 用户 ID 分片
//	client, err := lb.SelectClient(ctx,
//	    loadbalancer.WithKey(userID),
//	    loadbalancer.WithClientFactory(factory),
//	)
func WithKey(key string) Option {
	return func(o *Options) {
		o.Key = key
	}
}

// WithClientFactory sets the client factory function.
// T is the concrete client type.
//
// The factory function is called when:
//   - SelectClient is called for the first time for an instance
//   - The client for that instance is not yet cached
//
// 示例:
//
//	// gRPC client factory
//	factory := loadbalancer.WithClientFactory(func(inst *loadbalancer.Instance) (*grpc.ClientConn, error) {
//	    addr := fmt.Sprintf("%s:%d", inst.Address, inst.Port)
//	    return grpc.Dial(addr, grpc.WithInsecure())
//	})
func WithClientFactory[T any](factory func(*Instance) (T, error)) Option {
	return func(o *Options) {
		o.ClientFactory = func(inst *Instance) (interface{}, error) {
			return factory(inst)
		}
	}
}

// WithFilter sets the instance filter function.
// Only instances for which filter returns true will be selected.
//
// 示例:
//
//	// 只选择包含特定标签的实例
//	filter := loadbalancer.WithFilter(func(inst *loadbalancer.Instance) bool {
//	    return inst.Metadata["region"] == "us-west"
//	})
func WithFilter(filter func(*Instance) bool) Option {
	return func(o *Options) {
		o.Filter = filter
	}
}
