// Package aerospike provides a Aerospike-based sharded storage implementation.
// This file implements the core CRUD operations.
//
// 基于 prompts/features/rpc/governance/load-balancer/shard/aerospike/00-aerospike-shared-client.md 实现
package aerospike

import (
	"context"
	"fmt"

	as "github.com/aerospike/aerospike-client-go/v7"
)

// Get retrieves a record from Aerospike by key.
// The key is used to route to the appropriate shard.
//
// 参数:
//   - ctx: 上下文
//   - key: 记录的键（用于路由和数据存储）
//   - binNames: 可选，指定要获取的 bin 名称，为空则获取所有 bins
//
// 返回:
//   - *as.Record: 获取的记录
//   - error: 如果操作失败返回错误
func (c *ShardClient) Get(ctx context.Context, key string, binNames ...string) (*as.Record, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("客户端已关闭")
	}

	if key == "" {
		return nil, fmt.Errorf("key 不能为空")
	}

	// 获取客户端和实例
	client, err := c.GetClient(ctx, key)
	if err != nil {
		return nil, err
	}

	inst, err := c.GetInstance(key)
	if err != nil {
		return nil, err
	}

	// 构建 Aerospike key
	asKey, err := as.NewKey(inst.Namespace, inst.Set, key)
	if err != nil {
		return nil, fmt.Errorf("创建 Aerospike key 失败: %w", err)
	}

	// 执行查询
	policy := as.NewPolicy()
	policy.SocketTimeout = c.config.ReadTimeout

	record, err := client.Get(policy, asKey, binNames...)

	if err != nil {
		if err == as.ErrKeyNotFound {
			return nil, fmt.Errorf("记录不存在: %s", key)
		}
		return nil, fmt.Errorf("获取记录失败 [%s]: %w", key, err)
	}

	return record, nil
}

// Put stores a record in Aerospike.
// The key is used to route to the appropriate shard.
//
// 参数:
//   - ctx: 上下文
//   - key: 记录的键
//   - bins: 要存储的 bins
//
// 返回:
//   - error: 如果操作失败返回错误
func (c *ShardClient) Put(ctx context.Context, key string, bins as.BinMap) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("客户端已关闭")
	}

	if key == "" {
		return fmt.Errorf("key 不能为空")
	}

	// 获取客户端和实例
	client, err := c.GetClient(ctx, key)
	if err != nil {
		return err
	}

	inst, err := c.GetInstance(key)
	if err != nil {
		return err
	}

	// 构建 Aerospike key
	asKey, err := as.NewKey(inst.Namespace, inst.Set, key)
	if err != nil {
		return fmt.Errorf("创建 Aerospike key 失败: %w", err)
	}

	// 执行写入
	policy := as.NewWritePolicy(0, 0)
	policy.SocketTimeout = c.config.WriteTimeout

	err = client.Put(policy, asKey, bins)
	if err != nil {
		return fmt.Errorf("写入记录失败 [%s]: %w", key, err)
	}

	return nil
}

// PutWithTTL stores a record in Aerospike with a TTL.
//
// 参数:
//   - ctx: 上下文
//   - key: 记录的键
//   - bins: 要存储的 bins
//   - ttl: 生存时间（秒），0 表示永不过期
//
// 返回:
//   - error: 如果操作失败返回错误
func (c *ShardClient) PutWithTTL(ctx context.Context, key string, bins as.BinMap, ttl uint32) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("客户端已关闭")
	}

	if key == "" {
		return fmt.Errorf("key 不能为空")
	}

	// 获取客户端和实例
	client, err := c.GetClient(ctx, key)
	if err != nil {
		return err
	}

	inst, err := c.GetInstance(key)
	if err != nil {
		return err
	}

	// 构建 Aerospike key
	asKey, err := as.NewKey(inst.Namespace, inst.Set, key)
	if err != nil {
		return fmt.Errorf("创建 Aerospike key 失败: %w", err)
	}

	// 执行写入
	policy := as.NewWritePolicy(0, ttl)
	policy.SocketTimeout = c.config.WriteTimeout

	err = client.Put(policy, asKey, bins)
	if err != nil {
		return fmt.Errorf("写入记录失败 [%s]: %w", key, err)
	}

	return nil
}

// Delete removes a record from Aerospike.
//
// 参数:
//   - ctx: 上下文
//   - key: 记录的键
//
// 返回:
//   - error: 如果操作失败返回错误
func (c *ShardClient) Delete(ctx context.Context, key string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("客户端已关闭")
	}

	if key == "" {
		return fmt.Errorf("key 不能为空")
	}

	// 获取客户端和实例
	client, err := c.GetClient(ctx, key)
	if err != nil {
		return err
	}

	inst, err := c.GetInstance(key)
	if err != nil {
		return err
	}

	// 构建 Aerospike key
	asKey, err := as.NewKey(inst.Namespace, inst.Set, key)
	if err != nil {
		return fmt.Errorf("创建 Aerospike key 失败: %w", err)
	}

	// 执行删除
	policy := as.NewWritePolicy(0, 0)
	policy.SocketTimeout = c.config.WriteTimeout

	_, err = client.Delete(policy, asKey)
	if err != nil {
		return fmt.Errorf("删除记录失败 [%s]: %w", key, err)
	}

	return nil
}

// Exists checks if a record exists in Aerospike.
//
// 参数:
//   - ctx: 上下文
//   - key: 记录的键
//
// 返回:
//   - bool: 记录是否存在
//   - error: 如果操作失败返回错误
func (c *ShardClient) Exists(ctx context.Context, key string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return false, fmt.Errorf("客户端已关闭")
	}

	if key == "" {
		return false, fmt.Errorf("key 不能为空")
	}

	// 获取客户端和实例
	client, err := c.GetClient(ctx, key)
	if err != nil {
		return false, err
	}

	inst, err := c.GetInstance(key)
	if err != nil {
		return false, err
	}

	// 构建 Aerospike key
	asKey, err := as.NewKey(inst.Namespace, inst.Set, key)
	if err != nil {
		return false, fmt.Errorf("创建 Aerospike key 失败: %w", err)
	}

	// 检查是否存在
	policy := as.NewPolicy()
	policy.SocketTimeout = c.config.ReadTimeout

	exists, err := client.Exists(policy, asKey)
	if err != nil {
		return false, fmt.Errorf("检查记录是否存在失败 [%s]: %w", key, err)
	}

	return exists, nil
}

// BatchGet retrieves multiple records by keys.
// Note: This operation may not be optimal for sharded scenarios.
//
// 参数:
//   - ctx: 上下文
//   - keys: 记录的键列表
//
// 返回:
//   - map[string]*as.Record: 键到记录的映射
//   - error: 如果操作失败返回错误
func (c *ShardClient) BatchGet(ctx context.Context, keys []string) (map[string]*as.Record, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("客户端已关闭")
	}

	if len(keys) == 0 {
		return make(map[string]*as.Record), nil
	}

	// TODO: 优化批量操作
	// 当前实现是简单的逐个获取，后续可以优化为按分片分组批量获取
	results := make(map[string]*as.Record)
	for _, key := range keys {
		record, err := c.Get(ctx, key)
		if err != nil {
			// 记录错误但继续处理其他键
			continue
		}
		results[key] = record
	}

	return results, nil
}
