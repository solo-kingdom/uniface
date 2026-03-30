// Package consistenthash provides a consistent hash load balancer implementation.
// Consistent hash provides stable routing based on keys, where the same key
// always routes to the same instance (unless instances are added or removed).
//
// This implementation:
// - Uses consistent hash when a key is provided
// - Falls back to round-robin when no key is provided
// - Uses virtual nodes for better distribution
//
// 基于 prompts/features/service/governance/load-balancer/01-load-balancer-iface.md 实现
package consistenthash

import (
	"context"
	"sync/atomic"

	"github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer"
	"github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer/base"
)

// ConsistentHashBalancer is a load balancer that uses consistent hashing.
// When a key is provided, it uses consistent hash for stable routing.
// When no key is provided, it falls back to round-robin.
type ConsistentHashBalancer[T any] struct {
	*base.BaseBalancer[T]
	ring    *Ring
	counter uint64 // For round-robin fallback
}

// New creates a new consistent hash load balancer.
// virtualNodes is the number of virtual nodes per instance (default 50).
// More virtual nodes provide better distribution but use more memory.
//
// 示例:
//
//	// 默认 50 个虚拟节点
//	lb := consistenthash.New[*grpc.ClientConn](0, nil)
//
//	// 自定义 100 个虚拟节点
//	lb := consistenthash.New[*grpc.ClientConn](100, nil)
func New[T any](virtualNodes int, hashFunc HashFunc) *ConsistentHashBalancer[T] {
	return &ConsistentHashBalancer[T]{
		BaseBalancer: base.NewBaseBalancer[T](),
		ring:         NewRing(virtualNodes, hashFunc),
	}
}

// Add adds an instance to the load balancer.
// It overrides the base Add to also add to the hash ring.
func (b *ConsistentHashBalancer[T]) Add(ctx context.Context, instance *loadbalancer.Instance) error {
	if err := b.BaseBalancer.Add(ctx, instance); err != nil {
		return err
	}

	b.ring.Add(instance.ID)
	return nil
}

// Remove removes an instance from the load balancer.
// It overrides the base Remove to also remove from the hash ring.
func (b *ConsistentHashBalancer[T]) Remove(ctx context.Context, instanceID string) error {
	if err := b.BaseBalancer.Remove(ctx, instanceID); err != nil {
		return err
	}

	b.ring.Remove(instanceID)
	return nil
}

// Select selects an instance using consistent hash or round-robin.
// If opts specifies a Key, it uses consistent hash for stable routing.
// If no Key is provided, it falls back to round-robin.
//
// Consistent hash algorithm:
//  1. Hash the key
//  2. Find the first node in the ring with hash >= key hash
//  3. Return the instance at that node
//
// This ensures that the same key always routes to the same instance (stability).
func (b *ConsistentHashBalancer[T]) Select(ctx context.Context, opts ...loadbalancer.Option) (*loadbalancer.Instance, error) {
	options := loadbalancer.MergeOptions(opts...)

	// Check if balancer is closed
	if b.IsClosed() {
		return nil, loadbalancer.ErrBalancerClosed
	}

	// If key is provided, use consistent hash
	if options.Key != "" {
		return b.selectByKey(ctx, options)
	}

	// Otherwise, fall back to round-robin
	return b.selectRoundRobin(ctx, options)
}

// selectByKey selects an instance using consistent hash.
func (b *ConsistentHashBalancer[T]) selectByKey(ctx context.Context, options *loadbalancer.Options) (*loadbalancer.Instance, error) {
	// Get instance ID from hash ring
	instanceID := b.ring.Get(options.Key)
	if instanceID == "" {
		return nil, loadbalancer.ErrNoInstances
	}

	// Get the instance
	instance, ok := b.GetInstance(instanceID)
	if !ok {
		return nil, loadbalancer.ErrNoInstances
	}

	// Apply filter if provided
	if options.Filter != nil && !options.Filter(instance) {
		// If the selected instance doesn't pass filter, return error
		// In a more sophisticated implementation, we could try the next instance
		return nil, loadbalancer.ErrNoInstances
	}

	return instance, nil
}

// selectRoundRobin selects an instance using round-robin (fallback).
func (b *ConsistentHashBalancer[T]) selectRoundRobin(ctx context.Context, options *loadbalancer.Options) (*loadbalancer.Instance, error) {
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
	index := atomic.AddUint64(&b.counter, 1) - 1
	selected := filtered[index%uint64(len(filtered))]

	return selected, nil
}

// SelectClient selects and returns a client using consistent hash or round-robin.
// If opts specifies a Key, it uses consistent hash for stable routing.
// If no Key is provided, it falls back to round-robin.
//
// 示例:
//
//	// Key-based routing (consistent hash)
//	client, err := lb.SelectClient(ctx,
//	    loadbalancer.WithKey(userID),
//	    loadbalancer.WithClientFactory(factory),
//	)
//
//	// Round-robin routing (no key)
//	client, err := lb.SelectClient(ctx,
//	    loadbalancer.WithClientFactory(factory),
//	)
func (b *ConsistentHashBalancer[T]) SelectClient(ctx context.Context, opts ...loadbalancer.Option) (T, error) {
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
