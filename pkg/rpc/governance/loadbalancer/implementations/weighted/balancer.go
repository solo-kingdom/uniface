// Package weighted provides a weighted round-robin load balancer implementation.
// Weighted round-robin distributes traffic based on instance weights,
// where instances with higher weights receive more requests.
//
// This implementation uses the smooth weighted round-robin algorithm,
// which distributes requests more evenly over time.
//
// 基于 prompts/features/service/governance/load-balancer/01-load-balancer-iface.md 实现
package weighted

import (
	"context"
	"sync"

	"github.com/wii/uniface/pkg/rpc/governance/loadbalancer"
	"github.com/wii/uniface/pkg/rpc/governance/loadbalancer/base"
)

// WeightedInstance wraps an instance with weight tracking information.
type WeightedInstance struct {
	instance      *loadbalancer.Instance
	weight        int // Effective weight
	currentWeight int // Current weight (changes during selection)
}

// WeightedBalancer is a weighted round-robin load balancer.
// It uses the smooth weighted round-robin algorithm for better distribution.
type WeightedBalancer[T any] struct {
	*base.BaseBalancer[T]
	mu                sync.RWMutex
	weightedInstances map[string]*WeightedInstance // instanceID -> WeightedInstance
}

// New creates a new weighted round-robin load balancer.
//
// 示例:
//
//	lb := weighted.New[*grpc.ClientConn]()
func New[T any]() *WeightedBalancer[T] {
	return &WeightedBalancer[T]{
		BaseBalancer:      base.NewBaseBalancer[T](),
		weightedInstances: make(map[string]*WeightedInstance),
	}
}

// Add adds an instance to the load balancer.
// It overrides the base Add to also track weights.
func (b *WeightedBalancer[T]) Add(ctx context.Context, instance *loadbalancer.Instance) error {
	if err := b.BaseBalancer.Add(ctx, instance); err != nil {
		return err
	}

	// Get effective weight (default to 1 if not specified or <= 0)
	weight := instance.Weight
	if weight <= 0 {
		weight = 1
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.weightedInstances[instance.ID] = &WeightedInstance{
		instance:      instance,
		weight:        weight,
		currentWeight: 0,
	}

	return nil
}

// Remove removes an instance from the load balancer.
// It overrides the base Remove to also clean up weight tracking.
func (b *WeightedBalancer[T]) Remove(ctx context.Context, instanceID string) error {
	if err := b.BaseBalancer.Remove(ctx, instanceID); err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.weightedInstances, instanceID)

	return nil
}

// Update updates an instance's information.
// It overrides the base Update to also update weight tracking.
func (b *WeightedBalancer[T]) Update(ctx context.Context, instance *loadbalancer.Instance) error {
	if err := b.BaseBalancer.Update(ctx, instance); err != nil {
		return err
	}

	// Get effective weight (default to 1 if not specified or <= 0)
	weight := instance.Weight
	if weight <= 0 {
		weight = 1
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Preserve current weight if instance exists
	if wi, ok := b.weightedInstances[instance.ID]; ok {
		wi.instance = instance
		wi.weight = weight
	} else {
		b.weightedInstances[instance.ID] = &WeightedInstance{
			instance:      instance,
			weight:        weight,
			currentWeight: 0,
		}
	}

	return nil
}

// Select selects an instance using smooth weighted round-robin strategy.
// If opts specifies a Key, it will be ignored in this implementation.
// For key-based selection, use the consistent hash load balancer instead.
//
// Smooth weighted round-robin algorithm:
//  1. Every instance has effective weight and current weight
//  2. On each selection:
//     a. Add effective weight to current weight for all instances
//     b. Select the instance with the highest current weight
//     c. Subtract total effective weight from the selected instance's current weight
//
// This algorithm ensures smooth distribution and avoids bursts.
func (b *WeightedBalancer[T]) Select(ctx context.Context, opts ...loadbalancer.Option) (*loadbalancer.Instance, error) {
	options := loadbalancer.MergeOptions(opts...)

	// Check if balancer is closed
	if b.IsClosed() {
		return nil, loadbalancer.ErrBalancerClosed
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Apply filter if provided
	var candidates []*WeightedInstance
	if options.Filter != nil {
		for _, wi := range b.weightedInstances {
			if options.Filter(wi.instance) {
				candidates = append(candidates, wi)
			}
		}
		if len(candidates) == 0 {
			return nil, loadbalancer.ErrNoInstances
		}
	} else {
		// No filter, use all instances
		candidates = make([]*WeightedInstance, 0, len(b.weightedInstances))
		for _, wi := range b.weightedInstances {
			candidates = append(candidates, wi)
		}
	}

	if len(candidates) == 0 {
		return nil, loadbalancer.ErrNoInstances
	}

	// Calculate total weight and find instance with highest current weight
	var totalWeight int
	var selected *WeightedInstance

	for _, wi := range candidates {
		wi.currentWeight += wi.weight
		totalWeight += wi.weight

		if selected == nil || wi.currentWeight > selected.currentWeight {
			selected = wi
		}
	}

	// Subtract total weight from selected instance
	selected.currentWeight -= totalWeight

	return selected.instance, nil
}

// SelectClient selects and returns a client using weighted round-robin strategy.
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
func (b *WeightedBalancer[T]) SelectClient(ctx context.Context, opts ...loadbalancer.Option) (T, error) {
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
