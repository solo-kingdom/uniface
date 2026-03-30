// Package aerospike provides a Aerospike-based sharded storage implementation.
// This file implements the sharded client using Shard Manager.
//
// 基于 prompts/features/rpc/governance/load-balancer/shard/aerospike/00-aerospike-shared-client.md 实现
package aerospike

import (
	"context"
	"fmt"
	"sync"

	as "github.com/aerospike/aerospike-client-go/v7"
	"github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer"
	"github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer/shard"
)

// Instance represents an Aerospike instance configuration.
type Instance struct {
	// 实例标识
	ID string
	// 主机地址
	Host string
	// 端口号
	Port int
	// Aerospike 命名空间
	Namespace string
	// 默认 Set 名称
	Set string
	// 用户自定义元数据
	Metadata map[string]string
}

// toLoadBalancerInstance converts Instance to loadbalancer.Instance.
func (i *Instance) toLoadBalancerInstance() *loadbalancer.Instance {
	return &loadbalancer.Instance{
		ID:      i.ID,
		Address: i.Host,
		Port:    i.Port,
		Metadata: map[string]string{
			"namespace": i.Namespace,
			"set":       i.Set,
		},
	}
}

// ShardClient provides sharded access to Aerospike using Shard Manager.
// It routes requests to the appropriate Aerospike instance based on the key.
type ShardClient struct {
	manager shard.Manager
	config  *Config
	// 客户端缓存
	clients sync.Map // map[string]*as.Client
	mu      sync.RWMutex
	closed  bool
}

// NewShardClient creates a new sharded Aerospike client.
// It initializes the shard manager with the given instances.
//
// 示例:
//
//	client, err := aerospike.NewShardClient([]*aerospike.Instance{
//	    {ID: "node-1", Host: "192.168.1.1", Port: 3000, Namespace: "test", Set: "users"},
//	    {ID: "node-2", Host: "192.168.1.2", Port: 3000, Namespace: "test", Set: "users"},
//	    {ID: "node-3", Host: "192.168.1.3", Port: 3000, Namespace: "test", Set: "users"},
//	})
func NewShardClient(instances []*Instance, opts ...Option) (*ShardClient, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("至少需要一个 Aerospike 实例")
	}

	config := NewConfig(opts...)

	// 转换为 loadbalancer.Instance
	lbInstances := make([]*loadbalancer.Instance, len(instances))
	for i, inst := range instances {
		lbInstances[i] = inst.toLoadBalancerInstance()
	}

	// 创建 Shard Manager
	manager := shard.NewShardManager(lbInstances)

	return &ShardClient{
		manager: manager,
		config:  config,
	}, nil
}

// clientFactory creates an Aerospike client for the given instance.
func (c *ShardClient) clientFactory(inst *loadbalancer.Instance) (interface{}, error) {
	// 创建 Aerospike 客户端策略
	policy := as.NewClientPolicy()
	policy.Timeout = c.config.ConnectTimeout

	if c.config.User != "" {
		policy.User = c.config.User
		policy.Password = c.config.Password
	}

	if c.config.EnableTLS {
		// TLS 配置（需要进一步配置）
		policy.TlsConfig = nil // TODO: 添加 TLS 配置支持
	}

	// 创建 Aerospike 客户端
	client, err := as.NewClientWithPolicy(policy, inst.Address, inst.Port)
	if err != nil {
		return nil, fmt.Errorf("创建 Aerospike 客户端失败 [%s:%d]: %w", inst.Address, inst.Port, err)
	}

	return client, nil
}

// GetClient returns the Aerospike client for the given key.
// This is useful for advanced operations that require direct client access.
func (c *ShardClient) GetClient(ctx context.Context, key string) (*as.Client, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("客户端已关闭")
	}

	client, err := c.manager.SelectClient(key, c.clientFactory)
	if err != nil {
		return nil, fmt.Errorf("选择客户端失败: %w", err)
	}

	return client.(*as.Client), nil
}

// GetInstance returns the Aerospike instance for the given key.
func (c *ShardClient) GetInstance(key string) (*Instance, error) {
	inst, err := c.manager.Select(key)
	if err != nil {
		return nil, fmt.Errorf("选择实例失败: %w", err)
	}

	// 转换回 Instance
	return &Instance{
		ID:        inst.ID,
		Host:      inst.Address,
		Port:      inst.Port,
		Namespace: inst.Metadata["namespace"],
		Set:       inst.Metadata["set"],
		Metadata:  inst.Metadata,
	}, nil
}

// Close closes the shard client and releases all resources.
func (c *ShardClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true

	// 关闭所有客户端连接
	c.clients.Range(func(key, value interface{}) bool {
		if client, ok := value.(*as.Client); ok {
			client.Close()
		}
		return true
	})

	// 关闭 Shard Manager
	if c.manager != nil {
		if err := c.manager.Close(); err != nil {
			return fmt.Errorf("关闭 Shard Manager 失败: %w", err)
		}
	}

	return nil
}
