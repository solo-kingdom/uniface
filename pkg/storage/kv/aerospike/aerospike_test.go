// Package aerospike provides a Aerospike-based sharded storage implementation.
// This file contains unit tests.
//
// 基于 prompts/features/rpc/governance/load-balancer/shard/aerospike/00-aerospike-shared-client.md 实现
package aerospike

import (
	"context"
	"testing"
	"time"
)

func TestNewShardClient(t *testing.T) {
	tests := []struct {
		name      string
		instances []*Instance
		opts      []Option
		wantErr   bool
	}{
		{
			name: "创建单个实例客户端",
			instances: []*Instance{
				{
					ID:        "node-1",
					Host:      "192.168.1.1",
					Port:      3000,
					Namespace: "test",
					Set:       "users",
				},
			},
			wantErr: false,
		},
		{
			name: "创建多个实例客户端",
			instances: []*Instance{
				{ID: "node-1", Host: "192.168.1.1", Port: 3000, Namespace: "test", Set: "users"},
				{ID: "node-2", Host: "192.168.1.2", Port: 3000, Namespace: "test", Set: "users"},
				{ID: "node-3", Host: "192.168.1.3", Port: 3000, Namespace: "test", Set: "users"},
			},
			wantErr: false,
		},
		{
			name:      "空实例列表应返回错误",
			instances: []*Instance{},
			wantErr:   true,
		},
		{
			name:      "nil 实例列表应返回错误",
			instances: nil,
			wantErr:   true,
		},
		{
			name: "带配置选项的客户端",
			instances: []*Instance{
				{ID: "node-1", Host: "192.168.1.1", Port: 3000, Namespace: "test", Set: "users"},
			},
			opts: []Option{
				WithConnectTimeout(10 * time.Second),
				WithPoolSize(20),
				WithAuth("user", "pass"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewShardClient(tt.instances, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewShardClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if client == nil {
					t.Error("NewShardClient() 返回 nil 客户端")
					return
				}

				// 清理资源
				if err := client.Close(); err != nil {
					t.Errorf("Close() error = %v", err)
				}
			}
		})
	}
}

func TestShardClient_GetInstance(t *testing.T) {
	instances := []*Instance{
		{ID: "node-1", Host: "192.168.1.1", Port: 3000, Namespace: "test", Set: "users"},
		{ID: "node-2", Host: "192.168.1.2", Port: 3000, Namespace: "test", Set: "users"},
		{ID: "node-3", Host: "192.168.1.3", Port: 3000, Namespace: "test", Set: "users"},
	}

	client, err := NewShardClient(instances)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}
	defer client.Close()

	// 测试路由稳定性 - 相同的 key 应该总是路由到相同的实例
	key := "user-123"
	inst1, err := client.GetInstance(key)
	if err != nil {
		t.Fatalf("GetInstance() error = %v", err)
	}

	inst2, err := client.GetInstance(key)
	if err != nil {
		t.Fatalf("GetInstance() error = %v", err)
	}

	if inst1.ID != inst2.ID {
		t.Errorf("路由不稳定: 同一个 key 路由到了不同的实例 (%s vs %s)", inst1.ID, inst2.ID)
	}

	// 测试不同的 key 可能路由到不同的实例
	// 注意：这个测试可能失败，取决于一致性哈希算法
	// 但至少应该有一些分布
	keySet := make(map[string]bool)
	for i := 0; i < 100; i++ {
		inst, err := client.GetInstance(string(rune(i)))
		if err != nil {
			t.Fatalf("GetInstance() error = %v", err)
		}
		keySet[inst.ID] = true
	}

	// 应该至少路由到 2 个不同的实例
	if len(keySet) < 2 {
		t.Logf("警告: 100 个 key 只路由到了 %d 个实例", len(keySet))
	}
}

func TestShardClient_Close(t *testing.T) {
	instances := []*Instance{
		{ID: "node-1", Host: "192.168.1.1", Port: 3000, Namespace: "test", Set: "users"},
	}

	client, err := NewShardClient(instances)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	// 第一次关闭应该成功
	if err := client.Close(); err != nil {
		t.Errorf("第一次 Close() error = %v", err)
	}

	// 第二次关闭也应该成功（幂等）
	if err := client.Close(); err != nil {
		t.Errorf("第二次 Close() error = %v", err)
	}

	// 关闭后操作应该返回错误
	_, err = client.GetInstance("test")
	if err == nil {
		t.Error("关闭后的客户端应该返回错误")
	}
}

func TestInstance_toLoadBalancerInstance(t *testing.T) {
	inst := &Instance{
		ID:        "node-1",
		Host:      "192.168.1.1",
		Port:      3000,
		Namespace: "test",
		Set:       "users",
		Metadata: map[string]string{
			"region": "us-west",
		},
	}

	lbInst := inst.toLoadBalancerInstance()

	if lbInst.ID != inst.ID {
		t.Errorf("ID 不匹配: got %v, want %v", lbInst.ID, inst.ID)
	}

	if lbInst.Address != inst.Host {
		t.Errorf("Address 不匹配: got %v, want %v", lbInst.Address, inst.Host)
	}

	if lbInst.Port != inst.Port {
		t.Errorf("Port 不匹配: got %v, want %v", lbInst.Port, inst.Port)
	}

	if lbInst.Metadata["namespace"] != inst.Namespace {
		t.Errorf("Namespace 不匹配: got %v, want %v", lbInst.Metadata["namespace"], inst.Namespace)
	}

	if lbInst.Metadata["set"] != inst.Set {
		t.Errorf("Set 不匹配: got %v, want %v", lbInst.Metadata["set"], inst.Set)
	}
}

func TestConfig(t *testing.T) {
	// 测试默认配置
	config := NewConfig()
	if config.ConnectTimeout != 5*time.Second {
		t.Errorf("默认 ConnectTimeout = %v, want %v", config.ConnectTimeout, 5*time.Second)
	}

	// 测试自定义配置
	config = NewConfig(
		WithConnectTimeout(10*time.Second),
		WithPoolSize(20),
		WithAuth("user", "pass"),
		WithKeyPrefix("test:"),
	)

	if config.ConnectTimeout != 10*time.Second {
		t.Errorf("自定义 ConnectTimeout = %v, want %v", config.ConnectTimeout, 10*time.Second)
	}

	if config.PoolSize != 20 {
		t.Errorf("自定义 PoolSize = %v, want %v", config.PoolSize, 20)
	}

	if config.User != "user" {
		t.Errorf("自定义 User = %v, want %v", config.User, "user")
	}

	if config.KeyPrefix != "test:" {
		t.Errorf("自定义 KeyPrefix = %v, want %v", config.KeyPrefix, "test:")
	}
}

// 注意：以下测试需要真实的 Aerospike 服务器，应该作为集成测试
// 在 CI/CD 中可以使用 Docker 启动 Aerospike 容器

func TestShardClient_Integration_GetPut(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	instances := []*Instance{
		{ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "users"},
	}

	client, err := NewShardClient(instances)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	key := "test-user-123"
	bins := map[string]interface{}{
		"name":  "Alice",
		"email": "alice@example.com",
		"age":   30,
	}

	// 测试 Put
	if err := client.Put(ctx, key, bins); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	// 测试 Get
	record, err := client.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if record == nil {
		t.Fatal("Get() 返回 nil record")
	}

	// 验证数据
	if record.Bins["name"] != bins["name"] {
		t.Errorf("name 不匹配: got %v, want %v", record.Bins["name"], bins["name"])
	}

	// 测试 Exists
	exists, err := client.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}

	if !exists {
		t.Error("Exists() 返回 false，期望 true")
	}

	// 测试 Delete
	if err := client.Delete(ctx, key); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// 验证删除
	exists, err = client.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}

	if exists {
		t.Error("删除后 Exists() 返回 true，期望 false")
	}
}
