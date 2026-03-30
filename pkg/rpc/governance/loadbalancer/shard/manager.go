// Package shard provides simple sharding management based on load balancers.
// This file implements the ShardManager.
//
// 基于 prompts/features/rpc/governance/load-balancer/shard/00-shard-manager.md 实现
package shard

import (
	"context"

	"github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer"
	"github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer/implementations/consistenthash"
)

// ShardManager implements the Manager interface.
// It wraps a LoadBalancer with consistent hashing for stable key-based routing.
type ShardManager struct {
	lb loadbalancer.Balancer[interface{}]
}

// NewShardManager creates a new shard manager with the given instances.
// The instances are fixed at creation time and cannot be modified later.
//
// 示例:
//
//	manager := shard.NewShardManager([]*loadbalancer.Instance{
//	    {ID: "db-0", Address: "192.168.1.1", Port: 3306},
//	    {ID: "db-1", Address: "192.168.1.2", Port: 3306},
//	    {ID: "db-2", Address: "192.168.1.3", Port: 3306},
//	})
func NewShardManager(instances []*loadbalancer.Instance) *ShardManager {
	// Use consistent hash balancer for stable routing
	// 0 means use default virtual nodes (50), nil means use default hash function
	lb := consistenthash.New[interface{}](0, nil)
	ctx := context.Background()

	// Add all instances at creation time
	for _, inst := range instances {
		if err := lb.Add(ctx, inst); err != nil {
			// In production, you might want to handle this error
			// For simplicity, we just continue
			continue
		}
	}

	return &ShardManager{
		lb: lb,
	}
}

// Select selects an instance based on the key using consistent hashing.
// The same key always routes to the same instance (stability).
func (m *ShardManager) Select(key string) (*loadbalancer.Instance, error) {
	if key == "" {
		return nil, ErrInvalidKey
	}

	return m.lb.Select(context.Background(), loadbalancer.WithKey(key))
}

// SelectClient selects and returns a client for an instance based on the key.
// If the client is already cached, it returns the cached client.
// If not, it creates one using the factory and caches it.
func (m *ShardManager) SelectClient(key string, factory ClientFactory) (interface{}, error) {
	if key == "" {
		return nil, ErrInvalidKey
	}

	if factory == nil {
		return nil, ErrNoFactory
	}

	// Convert to loadbalancer.ClientFactory
	lbFactory := func(inst *loadbalancer.Instance) (interface{}, error) {
		return factory(inst)
	}

	return m.lb.SelectClient(
		context.Background(),
		loadbalancer.WithKey(key),
		loadbalancer.WithClientFactory(lbFactory),
	)
}

// Close closes the shard manager and releases all resources.
func (m *ShardManager) Close() error {
	if m.lb != nil {
		return m.lb.Close()
	}
	return nil
}
