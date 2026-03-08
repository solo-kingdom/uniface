// Package aerospike provides a Aerospike-based sharded storage implementation.
// This file contains usage examples.
//
// 基于 prompts/features/rpc/governance/load-balancer/shard/aerospike/00-aerospike-shared-client.md 实现
package aerospike_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/wii/uniface/pkg/storage/kv/aerospike"
)

// ExampleNewShardClient 演示如何创建分片客户端
func ExampleNewShardClient() {
	// 定义 Aerospike 实例
	instances := []*aerospike.Instance{
		{
			ID:        "node-1",
			Host:      "192.168.1.1",
			Port:      3000,
			Namespace: "test",
			Set:       "users",
		},
		{
			ID:        "node-2",
			Host:      "192.168.1.2",
			Port:      3000,
			Namespace: "test",
			Set:       "users",
		},
		{
			ID:        "node-3",
			Host:      "192.168.1.3",
			Port:      3000,
			Namespace: "test",
			Set:       "users",
		},
	}

	// 创建分片客户端
	client, err := aerospike.NewShardClient(instances)
	if err != nil {
		log.Fatalf("创建客户端失败: %v", err)
	}
	defer client.Close()

	fmt.Println("分片客户端创建成功")
}

// ExampleNewShardClient_withOptions 演示如何使用配置选项创建客户端
func ExampleNewShardClient_withOptions() {
	instances := []*aerospike.Instance{
		{
			ID:        "node-1",
			Host:      "192.168.1.1",
			Port:      3000,
			Namespace: "test",
			Set:       "users",
		},
	}

	// 使用配置选项
	client, err := aerospike.NewShardClient(instances,
		aerospike.WithConnectTimeout(10*time.Second),
		aerospike.WithReadTimeout(5*time.Second),
		aerospike.WithWriteTimeout(5*time.Second),
		aerospike.WithPoolSize(20),
		aerospike.WithMinIdleConns(5),
		aerospike.WithAuth("user", "password"),
		aerospike.WithKeyPrefix("myapp:"),
	)
	if err != nil {
		log.Fatalf("创建客户端失败: %v", err)
	}
	defer client.Close()

	fmt.Println("带配置的分片客户端创建成功")
}

// ExampleShardClient_Put 演示如何写入数据
func ExampleShardClient_Put() {
	// 创建客户端
	instances := []*aerospike.Instance{
		{
			ID:        "node-1",
			Host:      "localhost",
			Port:      3000,
			Namespace: "test",
			Set:       "users",
		},
	}

	client, err := aerospike.NewShardClient(instances)
	if err != nil {
		log.Fatalf("创建客户端失败: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// 写入数据
	key := "user-123"
	bins := map[string]interface{}{
		"name":   "Alice",
		"email":  "alice@example.com",
		"age":    30,
		"active": true,
	}

	if err := client.Put(ctx, key, bins); err != nil {
		log.Fatalf("写入数据失败: %v", err)
	}

	fmt.Printf("数据写入成功: %s\n", key)
}

// ExampleShardClient_PutWithTTL 演示如何写入带 TTL 的数据
func ExampleShardClient_PutWithTTL() {
	// 创建客户端
	instances := []*aerospike.Instance{
		{
			ID:        "node-1",
			Host:      "localhost",
			Port:      3000,
			Namespace: "test",
			Set:       "sessions",
		},
	}

	client, err := aerospike.NewShardClient(instances)
	if err != nil {
		log.Fatalf("创建客户端失败: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// 写入会话数据，TTL 为 3600 秒（1 小时）
	key := "session-abc123"
	bins := map[string]interface{}{
		"user_id":    12345,
		"created_at": time.Now().Unix(),
		"expires_at": time.Now().Add(1 * time.Hour).Unix(),
	}

	if err := client.PutWithTTL(ctx, key, bins, 3600); err != nil {
		log.Fatalf("写入数据失败: %v", err)
	}

	fmt.Printf("会话数据写入成功，TTL=3600秒: %s\n", key)
}

// ExampleShardClient_Get 演示如何读取数据
func ExampleShardClient_Get() {
	// 创建客户端
	instances := []*aerospike.Instance{
		{
			ID:        "node-1",
			Host:      "localhost",
			Port:      3000,
			Namespace: "test",
			Set:       "users",
		},
	}

	client, err := aerospike.NewShardClient(instances)
	if err != nil {
		log.Fatalf("创建客户端失败: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	key := "user-123"

	// 读取数据
	record, err := client.Get(ctx, key)
	if err != nil {
		log.Fatalf("读取数据失败: %v", err)
	}

	fmt.Printf("读取数据成功: %+v\n", record.Bins)

	// 只读取特定的 bins
	record, err = client.Get(ctx, key, "name", "email")
	if err != nil {
		log.Fatalf("读取数据失败: %v", err)
	}

	fmt.Printf("读取指定字段: %+v\n", record.Bins)
}

// ExampleShardClient_Delete 演示如何删除数据
func ExampleShardClient_Delete() {
	// 创建客户端
	instances := []*aerospike.Instance{
		{
			ID:        "node-1",
			Host:      "localhost",
			Port:      3000,
			Namespace: "test",
			Set:       "users",
		},
	}

	client, err := aerospike.NewShardClient(instances)
	if err != nil {
		log.Fatalf("创建客户端失败: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	key := "user-123"

	// 删除数据
	if err := client.Delete(ctx, key); err != nil {
		log.Fatalf("删除数据失败: %v", err)
	}

	fmt.Printf("数据删除成功: %s\n", key)
}

// ExampleShardClient_Exists 演示如何检查数据是否存在
func ExampleShardClient_Exists() {
	// 创建客户端
	instances := []*aerospike.Instance{
		{
			ID:        "node-1",
			Host:      "localhost",
			Port:      3000,
			Namespace: "test",
			Set:       "users",
		},
	}

	client, err := aerospike.NewShardClient(instances)
	if err != nil {
		log.Fatalf("创建客户端失败: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	key := "user-123"

	// 检查是否存在
	exists, err := client.Exists(ctx, key)
	if err != nil {
		log.Fatalf("检查存在失败: %v", err)
	}

	if exists {
		fmt.Printf("数据存在: %s\n", key)
	} else {
		fmt.Printf("数据不存在: %s\n", key)
	}
}

// ExampleShardClient_GetInstance 演示如何获取路由信息
func ExampleShardClient_GetInstance() {
	// 创建客户端
	instances := []*aerospike.Instance{
		{ID: "node-1", Host: "192.168.1.1", Port: 3000, Namespace: "test", Set: "users"},
		{ID: "node-2", Host: "192.168.1.2", Port: 3000, Namespace: "test", Set: "users"},
		{ID: "node-3", Host: "192.168.1.3", Port: 3000, Namespace: "test", Set: "users"},
	}

	client, err := aerospike.NewShardClient(instances)
	if err != nil {
		log.Fatalf("创建客户端失败: %v", err)
	}
	defer client.Close()

	// 获取特定 key 路由到的实例
	key := "user-123"
	inst, err := client.GetInstance(key)
	if err != nil {
		log.Fatalf("获取实例失败: %v", err)
	}

	fmt.Printf("Key '%s' 路由到的实例:\n", key)
	fmt.Printf("  ID: %s\n", inst.ID)
	fmt.Printf("  Host: %s:%d\n", inst.Host, inst.Port)
	fmt.Printf("  Namespace: %s\n", inst.Namespace)
	fmt.Printf("  Set: %s\n", inst.Set)

	// 相同的 key 总是路由到相同的实例（稳定性）
	inst2, _ := client.GetInstance(key)
	fmt.Printf("路由稳定性验证: %v\n", inst.ID == inst2.ID)
}

// ExampleShardClient_BatchGet 演示如何批量读取数据
func ExampleShardClient_BatchGet() {
	// 创建客户端
	instances := []*aerospike.Instance{
		{
			ID:        "node-1",
			Host:      "localhost",
			Port:      3000,
			Namespace: "test",
			Set:       "users",
		},
	}

	client, err := aerospike.NewShardClient(instances)
	if err != nil {
		log.Fatalf("创建客户端失败: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// 批量读取多个 key
	keys := []string{"user-1", "user-2", "user-3"}
	records, err := client.BatchGet(ctx, keys)
	if err != nil {
		log.Fatalf("批量读取失败: %v", err)
	}

	for key, record := range records {
		fmt.Printf("Key: %s, Data: %+v\n", key, record.Bins)
	}
}

// ExampleShardClient_GetClient 演示如何获取底层客户端进行高级操作
func ExampleShardClient_GetClient() {
	// 创建客户端
	instances := []*aerospike.Instance{
		{
			ID:        "node-1",
			Host:      "localhost",
			Port:      3000,
			Namespace: "test",
			Set:       "users",
		},
	}

	client, err := aerospike.NewShardClient(instances)
	if err != nil {
		log.Fatalf("创建客户端失败: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	key := "user-123"

	// 获取底层 Aerospike 客户端
	asClient, err := client.GetClient(ctx, key)
	if err != nil {
		log.Fatalf("获取客户端失败: %v", err)
	}

	// 使用底层客户端进行高级操作
	// 注意：这是一个示例，实际使用需要更多代码
	_ = asClient

	fmt.Println("获取底层客户端成功")
}
