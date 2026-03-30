// Package roundrobin provides a round-robin load balancer implementation.
// Round-robin is a simple load balancing algorithm that selects instances in sequence.
//
// 基于 prompts/features/service/governance/load-balancer/01-load-balancer-iface.md 实现
package roundrobin

import (
	"context"
	"sync/atomic"

	"github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer"
	"github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer/base"
)

// RoundRobinBalancer is a round-robin load balancer.
// It selects instances in sequence, cycling through all available instances.
type RoundRobinBalancer[T any] struct {
	*base.BaseBalancer[T]
	counter uint64
}

// New creates a new round-robin load balancer.
//
// 示例:
//
//	lb := roundrobin.New[*grpc.ClientConn]()
func New[T any]() *RoundRobinBalancer[T] {
	return &RoundRobinBalancer[T]{
		BaseBalancer: base.NewBaseBalancer[T](),
	}
}

// Select selects an instance using round-robin strategy.
// If opts specifies a Key, it will be ignored in this implementation.
// For key-based selection, use the consistent hash load balancer instead.
//
// Round-robin algorithm:
//  1. Get all instances
//  2. Apply filter if provided
//  3. Select instance at position (counter % len(instances))
//  4. Increment counter
//
// This ensures fair distribution of requests across all instances.
func (b *RoundRobinBalancer[T]) Select(ctx context.Context, opts ...loadbalancer.Option) (*loadbalancer.Instance, error) {
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

	// Round-robin selection
	// atomic.AddUint64 returns the new value, so we subtract 1 to get the index
	index := atomic.AddUint64(&b.counter, 1) - 1
	selected := filtered[index%uint64(len(filtered))]

	return selected, nil
}

// SelectClient selects and returns a client using round-robin strategy.
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
func (b *RoundRobinBalancer[T]) SelectClient(ctx context.Context, opts ...loadbalancer.Option) (T, error) {
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
