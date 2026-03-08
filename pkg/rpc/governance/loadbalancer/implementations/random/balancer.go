// Package random provides a random load balancer implementation.
// Random load balancing selects instances randomly, which provides
// good distribution over time without maintaining state.
//
// 基于 prompts/features/service/governance/load-balancer/01-load-balancer-iface.md 实现
package random

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/wii/uniface/pkg/rpc/governance/loadbalancer"
	"github.com/wii/uniface/pkg/rpc/governance/loadbalancer/base"
)

// RandomBalancer is a random load balancer.
// It selects instances randomly from the available pool.
type RandomBalancer[T any] struct {
	*base.BaseBalancer[T]
	rand *rand.Rand
	mu   sync.Mutex
}

// New creates a new random load balancer.
//
// 示例:
//
//	lb := random.New[*grpc.ClientConn]()
func New[T any]() *RandomBalancer[T] {
	return &RandomBalancer[T]{
		BaseBalancer: base.NewBaseBalancer[T](),
		rand:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// NewWithSeed creates a new random load balancer with a custom seed.
// This is useful for deterministic testing.
//
// 示例:
//
//	lb := random.NewWithSeed[*grpc.ClientConn](12345)
func NewWithSeed[T any](seed int64) *RandomBalancer[T] {
	return &RandomBalancer[T]{
		BaseBalancer: base.NewBaseBalancer[T](),
		rand:         rand.New(rand.NewSource(seed)),
	}
}

// Select selects an instance using random strategy.
// If opts specifies a Key, it will be ignored in this implementation.
// For key-based selection, use the consistent hash load balancer instead.
//
// Random algorithm:
//  1. Get all instances
//  2. Apply filter if provided
//  3. Select a random instance from the filtered list
//
// Over time, this provides good distribution across all instances.
func (b *RandomBalancer[T]) Select(ctx context.Context, opts ...loadbalancer.Option) (*loadbalancer.Instance, error) {
	options := loadbalancer.MergeOptions(opts...)

	// Check if balancer is closed
	if b.IsClosed() {
		return nil, loadbalancer.ErrBalancerClosed
	}

	// Get all instances in order
	instances := b.GetOrderedInstances()
	if len(instances) == 0 {
		return nil, loadbalancer.ErrNoInstances
	}

	// Apply filter if provided
	var filtered []*loadbalancer.Instance
	if options.Filter != nil {
		for _, inst := range instances {
			if options.Filter(inst) {
				filtered = append(filtered, inst)
			}
		}
		if len(filtered) == 0 {
			return nil, loadbalancer.ErrNoInstances
		}
	} else {
		// No filter, use all instances
		filtered = instances
	}

	// Random selection
	b.mu.Lock()
	index := b.rand.Intn(len(filtered))
	b.mu.Unlock()

	selected := filtered[index]
	return selected, nil
}

// SelectClient selects and returns a client using random strategy.
// If the client is already cached, it returns the cached client.
// If the client is not cached, it calls ClientFactory to create and cache it.
//
// 示例:
//
//	client, err := lb.SelectClient(ctx,
//	    loadbalancer.WithClientFactory(func(inst *loadbalancer.Instance) (*grpc.ClientConn, error) {
//	        addr := fmt.Sprintf("%s:%d", inst.Address, inst.Port)
//	        return grpc.Dial(addr, grpc.WithInsecure())
//	    }),
//	)
func (b *RandomBalancer[T]) SelectClient(ctx context.Context, opts ...loadbalancer.Option) (T, error) {
	options := loadbalancer.MergeOptions(opts...)

	// Select instance
	instance, err := b.Select(ctx, opts...)
	if err != nil {
		var zero T
		return zero, err
	}

	// Get or create client
	return b.GetOrCreateClient(instance, options.ClientFactory)
}
