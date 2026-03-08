// Package aerospike provides a Aerospike-based sharded storage implementation.
// This file contains unit tests for Storage.
//
// 基于 prompts/features/rpc/governance/load-balancer/shard/aerospike/00-aerospike-shared-client.md 实现
package aerospike

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wii/uniface/pkg/storage/kv"
)

// TestNewStorage 测试创建 Storage
func TestNewStorage(t *testing.T) {
	instances := []*Instance{
		{ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "users"},
	}

	// 测试默认配置
	storage, err := NewStorage(instances)
	if err != nil {
		t.Fatalf("NewStorage() error = %v", err)
	}
	defer storage.Close()

	// 验证默认 bin 名称
	if storage.config.BinName != "data" {
		t.Errorf("默认 BinName = %v, want data", storage.config.BinName)
	}

	// 测试空实例列表
	_, err = NewStorage([]*Instance{})
	if err == nil {
		t.Error("空实例列表应该返回错误")
	}

	// 测试 nil 实例列表
	_, err = NewStorage(nil)
	if err == nil {
		t.Error("nil 实例列表应该返回错误")
	}
}

// TestStorage_CustomBinName 测试自定义 bin 名称
func TestStorage_CustomBinName(t *testing.T) {
	instances := []*Instance{
		{ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "test"},
	}

	// 自定义 bin 名称
	storage, err := NewStorage(instances, WithBinName("custom_bin"))
	if err != nil {
		t.Fatalf("NewStorage() error = %v", err)
	}
	defer storage.Close()

	if storage.config.BinName != "custom_bin" {
		t.Errorf("BinName = %v, want custom_bin", storage.config.BinName)
	}
}

// TestStorage_BatchNotSupported 测试批量操作不支持
func TestStorage_BatchNotSupported(t *testing.T) {
	instances := []*Instance{
		{ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "test"},
	}

	storage, err := NewStorage(instances)
	if err != nil {
		t.Fatalf("NewStorage() error = %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// BatchSet 应该返回错误
	err = storage.BatchSet(ctx, map[string]interface{}{"key": "value"})
	if err == nil {
		t.Error("BatchSet() 应该返回错误")
	}
	if !errors.Is(err, ErrBatchNotSupported) {
		t.Errorf("BatchSet() error = %v, want ErrBatchNotSupported", err)
	}

	// BatchGet 应该返回错误
	_, err = storage.BatchGet(ctx, []string{"key"})
	if err == nil {
		t.Error("BatchGet() 应该返回错误")
	}
	if !errors.Is(err, ErrBatchNotSupported) {
		t.Errorf("BatchGet() error = %v, want ErrBatchNotSupported", err)
	}

	// BatchDelete 应该返回错误
	err = storage.BatchDelete(ctx, []string{"key"})
	if err == nil {
		t.Error("BatchDelete() 应该返回错误")
	}
	if !errors.Is(err, ErrBatchNotSupported) {
		t.Errorf("BatchDelete() error = %v, want ErrBatchNotSupported", err)
	}
}

// TestStorage_CRUD 测试基本 CRUD（集成测试）
func TestStorage_CRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	instances := []*Instance{
		{ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "test"},
	}

	storage, err := NewStorage(instances)
	if err != nil {
		t.Fatalf("NewStorage() error = %v", err)
	}
	defer storage.Close()

	ctx := context.Background()
	key := "test-crud"
	value := map[string]interface{}{"name": "Alice", "age": float64(30)}

	// Set
	if err := storage.Set(ctx, key, value); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Get
	var result map[string]interface{}
	if err := storage.Get(ctx, key, &result); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if result["name"] != "Alice" {
		t.Errorf("Get() = %v, want name=Alice", result)
	}

	// Exists
	exists, err := storage.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("Exists() = false, want true")
	}

	// Delete
	if err := storage.Delete(ctx, key); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// 再次 Exists
	exists, err = storage.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if exists {
		t.Error("Delete 后 Exists() = true, want false")
	}
}

// TestStorage_TTL 测试 TTL（集成测试）
func TestStorage_TTL(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	instances := []*Instance{
		{ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "test"},
	}

	storage, err := NewStorage(instances)
	if err != nil {
		t.Fatalf("NewStorage() error = %v", err)
	}
	defer storage.Close()

	ctx := context.Background()
	key := "test-ttl"

	// 写入 2 秒过期
	if err := storage.Set(ctx, key, "value", kv.WithTTL(2*time.Second)); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// 立即读取应该成功
	var result string
	if err := storage.Get(ctx, key, &result); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	// 等待过期
	time.Sleep(3 * time.Second)

	// 过期后读取应该失败
	err = storage.Get(ctx, key, &result)
	if err == nil {
		t.Error("过期后 Get() 应该返回错误")
	}
	if !errors.Is(err, kv.ErrKeyNotFound) {
		t.Errorf("Get() error = %v, want ErrKeyNotFound", err)
	}
}

// TestStorage_NoOverwrite 测试 NoOverwrite（集成测试）
func TestStorage_NoOverwrite(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	instances := []*Instance{
		{ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "test"},
	}

	storage, err := NewStorage(instances)
	if err != nil {
		t.Fatalf("NewStorage() error = %v", err)
	}
	defer storage.Close()

	ctx := context.Background()
	key := "test-no-overwrite"

	// 第一次写入
	if err := storage.Set(ctx, key, "value1"); err != nil {
		t.Fatalf("第一次 Set() error = %v", err)
	}

	// 第二次写入（应该失败）
	err = storage.Set(ctx, key, "value2", kv.WithNoOverwrite())
	if err == nil {
		t.Error("NoOverwrite Set() 应该返回错误")
	}
	if !errors.Is(err, kv.ErrKeyAlreadyExists) {
		t.Errorf("Set() error = %v, want ErrKeyAlreadyExists", err)
	}

	// 清理
	storage.Delete(ctx, key)
}

// TestStorage_Namespace 测试 Namespace（集成测试）
func TestStorage_Namespace(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	instances := []*Instance{
		{ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "test"},
	}

	storage, err := NewStorage(instances)
	if err != nil {
		t.Fatalf("NewStorage() error = %v", err)
	}
	defer storage.Close()

	ctx := context.Background()
	key := "test-namespace"
	value := "value"

	// 带 namespace 写入
	if err := storage.Set(ctx, key, value, kv.WithNamespace("ns1")); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// 带 namespace 读取（注意：Get 不接受 opts，需要手动构建 key）
	// 这里验证 namespace 功能通过 buildKey 实现
	var result string
	// 手动构建带 namespace 的 key
	fullKey := storage.buildKey(key, &kv.Options{Namespace: "ns1"})
	if err := storage.Get(ctx, fullKey, &result); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if result != value {
		t.Errorf("Get() = %v, want %v", result, value)
	}

	// 清理
	storage.Delete(ctx, fullKey)
}

// TestStorage_KeyPrefix 测试全局 key 前缀
func TestStorage_KeyPrefix(t *testing.T) {
	instances := []*Instance{
		{ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "test"},
	}

	// 带 key 前缀
	storage, err := NewStorage(instances, WithStorageKeyPrefix("myapp"))
	if err != nil {
		t.Fatalf("NewStorage() error = %v", err)
	}
	defer storage.Close()

	if storage.config.KeyPrefix != "myapp" {
		t.Errorf("KeyPrefix = %v, want myapp", storage.config.KeyPrefix)
	}

	// 验证 buildKey
	key := storage.buildKey("test", nil)
	if key != "myapp:test" {
		t.Errorf("buildKey() = %v, want myapp:test", key)
	}

	// 带 namespace
	key = storage.buildKey("test", &kv.Options{Namespace: "ns1"})
	if key != "myapp:ns1:test" {
		t.Errorf("buildKey() with namespace = %v, want myapp:ns1:test", key)
	}
}

// TestStorage_Close 测试关闭
func TestStorage_Close(t *testing.T) {
	instances := []*Instance{
		{ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "test"},
	}

	storage, _ := NewStorage(instances)

	// 第一次关闭应该成功
	if err := storage.Close(); err != nil {
		t.Errorf("第一次 Close() error = %v", err)
	}

	// 第二次关闭也应该成功（幂等）
	if err := storage.Close(); err != nil {
		t.Errorf("第二次 Close() error = %v", err)
	}

	// 关闭后操作应该返回错误
	ctx := context.Background()
	err := storage.Set(ctx, "key", "value")
	if err == nil {
		t.Error("关闭后的 Set() 应该返回错误")
	}
	if !errors.Is(err, kv.ErrStorageClosed) {
		t.Errorf("Set() error = %v, want ErrStorageClosed", err)
	}
}

// TestStorage_EmptyKey 测试空 key
func TestStorage_EmptyKey(t *testing.T) {
	instances := []*Instance{
		{ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "test"},
	}

	storage, _ := NewStorage(instances)
	defer storage.Close()

	ctx := context.Background()

	// Set 空 key
	err := storage.Set(ctx, "", "value")
	if err == nil {
		t.Error("空 key 的 Set() 应该返回错误")
	}
	if !errors.Is(err, kv.ErrInvalidKey) {
		t.Errorf("Set() error = %v, want ErrInvalidKey", err)
	}

	// Get 空 key
	var value interface{}
	err = storage.Get(ctx, "", &value)
	if err == nil {
		t.Error("空 key 的 Get() 应该返回错误")
	}
	if !errors.Is(err, kv.ErrInvalidKey) {
		t.Errorf("Get() error = %v, want ErrInvalidKey", err)
	}

	// Delete 空 key
	err = storage.Delete(ctx, "")
	if err == nil {
		t.Error("空 key 的 Delete() 应该返回错误")
	}
	if !errors.Is(err, kv.ErrInvalidKey) {
		t.Errorf("Delete() error = %v, want ErrInvalidKey", err)
	}

	// Exists 空 key
	_, err = storage.Exists(ctx, "")
	if err == nil {
		t.Error("空 key 的 Exists() 应该返回错误")
	}
	if !errors.Is(err, kv.ErrInvalidKey) {
		t.Errorf("Exists() error = %v, want ErrInvalidKey", err)
	}
}

// TestConfigToOptions 测试 configToOptions
func TestConfigToOptions(t *testing.T) {
	config := &Config{
		ConnectTimeout: 10 * time.Second,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		PoolSize:       20,
		MinIdleConns:   5,
		MaxIdleConns:   15,
		IdleTimeout:    10 * time.Minute,
		MaxRetries:     5,
		RetryDelay:     200 * time.Millisecond,
		User:           "testuser",
		Password:       "testpass",
		EnableTLS:      true,
		KeyPrefix:      "test:",
	}

	opts := configToOptions(config)

	if len(opts) == 0 {
		t.Error("configToOptions() 应该返回选项")
	}

	// 验证转换后的配置
	newConfig := NewConfig(opts...)

	if newConfig.ConnectTimeout != config.ConnectTimeout {
		t.Errorf("ConnectTimeout = %v, want %v", newConfig.ConnectTimeout, config.ConnectTimeout)
	}
	if newConfig.PoolSize != config.PoolSize {
		t.Errorf("PoolSize = %v, want %v", newConfig.PoolSize, config.PoolSize)
	}
	if newConfig.User != config.User {
		t.Errorf("User = %v, want %v", newConfig.User, config.User)
	}
	if newConfig.EnableTLS != config.EnableTLS {
		t.Errorf("EnableTLS = %v, want %v", newConfig.EnableTLS, config.EnableTLS)
	}
	if newConfig.KeyPrefix != config.KeyPrefix {
		t.Errorf("KeyPrefix = %v, want %v", newConfig.KeyPrefix, config.KeyPrefix)
	}
}
