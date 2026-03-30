// Package base provides base implementation for load balancers.
// This file contains common functionality shared by all load balancer implementations,
// including instance management, client caching, and lifecycle management.
//
// 基于 prompts/features/service/governance/load-balancer/01-load-balancer-iface.md 实现
package base

import (
	"context"
	"io"
	"sync"

	"github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer"
)

// BaseBalancer provides base implementation for load balancers.
// It includes instance management, client caching, and lifecycle management.
// Concrete implementations should embed this struct and implement the Select method.
type BaseBalancer[T any] struct {
	mu            sync.RWMutex
	closed        bool
	instances     map[string]*loadbalancer.Instance // instanceID -> Instance
	instanceOrder []string                          // ordered list of instance IDs
	clients       map[string]T                      // instanceID -> Client
}

// NewBaseBalancer creates a new base load balancer.
func NewBaseBalancer[T any]() *BaseBalancer[T] {
	return &BaseBalancer[T]{
		instances: make(map[string]*loadbalancer.Instance),
		clients:   make(map[string]T),
	}
}

// Add adds an instance to the load balancer.
func (b *BaseBalancer[T]) Add(ctx context.Context, instance *loadbalancer.Instance) error {
	if err := b.validateInstance(instance); err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return loadbalancer.ErrBalancerClosed
	}

	if _, exists := b.instances[instance.ID]; exists {
		return loadbalancer.NewBalancerError("Add", instance.ID, loadbalancer.ErrDuplicateInstance)
	}

	b.instances[instance.ID] = instance
	b.instanceOrder = append(b.instanceOrder, instance.ID)
	return nil
}

// Remove removes an instance from the load balancer.
func (b *BaseBalancer[T]) Remove(ctx context.Context, instanceID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return loadbalancer.ErrBalancerClosed
	}

	if _, exists := b.instances[instanceID]; !exists {
		return loadbalancer.NewBalancerError("Remove", instanceID, loadbalancer.ErrInstanceNotFound)
	}

	// Close and remove client
	if client, ok := b.clients[instanceID]; ok {
		b.closeClient(client)
		delete(b.clients, instanceID)
	}

	delete(b.instances, instanceID)

	// Remove from ordered list
	for i, id := range b.instanceOrder {
		if id == instanceID {
			b.instanceOrder = append(b.instanceOrder[:i], b.instanceOrder[i+1:]...)
			break
		}
	}

	return nil
}

// Update updates an instance's information.
func (b *BaseBalancer[T]) Update(ctx context.Context, instance *loadbalancer.Instance) error {
	if err := b.validateInstance(instance); err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return loadbalancer.ErrBalancerClosed
	}

	if _, exists := b.instances[instance.ID]; !exists {
		return loadbalancer.NewBalancerError("Update", instance.ID, loadbalancer.ErrInstanceNotFound)
	}

	b.instances[instance.ID] = instance
	return nil
}

// GetAll returns a copy of all instances.
func (b *BaseBalancer[T]) GetAll(ctx context.Context) ([]*loadbalancer.Instance, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return nil, loadbalancer.ErrBalancerClosed
	}

	instances := make([]*loadbalancer.Instance, 0, len(b.instances))
	for _, inst := range b.instances {
		// Return a copy
		copy := *inst
		instances = append(instances, &copy)
	}

	return instances, nil
}

// GetInstance gets an instance by ID (internal method).
// Returns the instance and whether it exists.
func (b *BaseBalancer[T]) GetInstance(instanceID string) (*loadbalancer.Instance, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	inst, ok := b.instances[instanceID]
	return inst, ok
}

// GetInstances gets all instances (internal method).
// Returns a copy of the instances map.
// DEPRECATED: Use GetOrderedInstances for deterministic order.
func (b *BaseBalancer[T]) GetInstances() map[string]*loadbalancer.Instance {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make(map[string]*loadbalancer.Instance, len(b.instances))
	for k, v := range b.instances {
		result[k] = v
	}
	return result
}

// GetOrderedInstances gets all instances in insertion order (internal method).
// Returns instances in the order they were added.
// This is important for algorithms like round-robin that need deterministic order.
func (b *BaseBalancer[T]) GetOrderedInstances() []*loadbalancer.Instance {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]*loadbalancer.Instance, 0, len(b.instanceOrder))
	for _, id := range b.instanceOrder {
		if inst, ok := b.instances[id]; ok {
			result = append(result, inst)
		}
	}
	return result
}

// GetOrCreateClient gets or creates a client for an instance (core logic).
// This method implements the client caching mechanism with double-checked locking.
//
// The process:
//  1. Check cache with read lock
//  2. If not in cache, acquire write lock
//  3. Double-check cache (another goroutine might have created it)
//  4. Create client using factory
//  5. Cache the client
//
// Parameters:
//   - instance: The instance to get/create a client for
//   - factory: The client factory function (may be nil)
//
// Returns:
//   - T: The client
//   - error: If creation fails or factory is nil
func (b *BaseBalancer[T]) GetOrCreateClient(
	instance *loadbalancer.Instance,
	factory func(*loadbalancer.Instance) (interface{}, error),
) (T, error) {
	var zero T

	// 1. Check cache with read lock
	b.mu.RLock()
	if client, ok := b.clients[instance.ID]; ok {
		b.mu.RUnlock()
		return client, nil
	}
	b.mu.RUnlock()

	// 2. Create client with write lock
	b.mu.Lock()
	defer b.mu.Unlock()

	// 3. Double-check cache
	if client, ok := b.clients[instance.ID]; ok {
		return client, nil
	}

	if factory == nil {
		return zero, loadbalancer.ErrNoClientFactory
	}

	clientInterface, err := factory(instance)
	if err != nil {
		return zero, loadbalancer.NewBalancerError("CreateClient", instance.ID, err)
	}

	client, ok := clientInterface.(T)
	if !ok {
		return zero, loadbalancer.ErrClientCreateFailed
	}

	b.clients[instance.ID] = client
	return client, nil
}

// Close closes the load balancer and releases all resources.
func (b *BaseBalancer[T]) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil
	}

	b.closed = true

	// Close all clients
	for _, client := range b.clients {
		b.closeClient(client)
	}

	b.clients = make(map[string]T)
	b.instances = make(map[string]*loadbalancer.Instance)
	b.instanceOrder = nil

	return nil
}

// IsClosed checks if the balancer is closed.
func (b *BaseBalancer[T]) IsClosed() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.closed
}

// validateInstance validates an instance.
func (b *BaseBalancer[T]) validateInstance(instance *loadbalancer.Instance) error {
	if instance == nil {
		return loadbalancer.ErrInvalidInstance
	}
	if instance.ID == "" {
		return loadbalancer.NewBalancerError("Validate", "", loadbalancer.ErrInvalidInstance)
	}
	if instance.Address == "" {
		return loadbalancer.NewBalancerError("Validate", instance.ID, loadbalancer.ErrInvalidInstance)
	}
	if instance.Port <= 0 {
		return loadbalancer.NewBalancerError("Validate", instance.ID, loadbalancer.ErrInvalidInstance)
	}
	return nil
}

// closeClient closes a client (automatically detects io.Closer).
// Uses any() for type assertion because T is a generic type parameter.
func (b *BaseBalancer[T]) closeClient(client T) {
	if closer, ok := any(client).(io.Closer); ok {
		closer.Close()
	}
}
