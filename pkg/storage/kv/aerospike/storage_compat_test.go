// Package aerospike provides a Aerospike-based sharded storage implementation.
// This file contains kv.Storage interface compatibility tests.
//
// 基于 prompts/features/rpc/governance/load-balancer/shard/aerospike/00-aerospike-shared-client.md 实现
package aerospike

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/solo-kingdom/uniface/pkg/storage/kv"
)

// skipIfNoAerospike 跳过无 Aerospike 的测试
func skipIfNoAerospike(t *testing.T, storage *Storage) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 尝试连接测试
	err := storage.client.Put(ctx, "__test_compat__", map[string]interface{}{"test": 1})
	if err != nil {
		t.Skipf("Aerospike not available: %v", err)
	}
	storage.client.Delete(ctx, "__test_compat__")
}

// TestKVInterface_Compatibility 测试 KV 接口兼容性
// 这个测试确保 Storage 完全实现了 kv.Storage 接口
func TestKVInterface_Compatibility(t *testing.T) {
	instances := []*Instance{
		{ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "compat"},
	}

	storage, err := NewStorage(instances)
	if err != nil {
		t.Skipf("Aerospike not available: %v", err)
		return
	}
	defer storage.Close()

	skipIfNoAerospike(t, storage)

	// 验证实现了 kv.Storage 接口
	// 这个编译时检查确保 Storage 完全实现了 kv.Storage
	var _ kv.Storage = storage
}

// TestKVInterface_Set 测试 Set 方法
func TestKVInterface_Set(t *testing.T) {
	instances := []*Instance{
		{ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "compat"},
	}

	storage, err := NewStorage(instances)
	if err != nil {
		t.Skipf("Aerospike not available: %v", err)
		return
	}
	defer storage.Close()

	skipIfNoAerospike(t, storage)

	ctx := context.Background()

	tests := []struct {
		name    string
		key     string
		value   interface{}
		opts    []kv.Option
		wantErr error
	}{
		{
			name:  "正常写入字符串",
			key:   "compat:string",
			value: "hello world",
		},
		{
			name:  "正常写入数字",
			key:   "compat:number",
			value: 42,
		},
		{
			name:  "正常写入 struct",
			key:   "compat:struct",
			value: map[string]interface{}{"name": "Alice", "age": 30},
		},
		{
			name:    "空 key",
			key:     "",
			value:   "value",
			wantErr: kv.ErrInvalidKey,
		},
		{
			name:  "带 TTL",
			key:   "compat:ttl",
			value: "temporary",
			opts:  []kv.Option{kv.WithTTL(10 * time.Second)},
		},
		{
			name:  "带 Namespace",
			key:   "compat:ns",
			value: "namespaced",
			opts:  []kv.Option{kv.WithNamespace("tenant1")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := storage.Set(ctx, tt.key, tt.value, tt.opts...)
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("Set() 应该返回错误")
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Set() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("Set() error = %v", err)
			}
		})
	}
}

// TestKVInterface_Set_NoOverwrite 测试 NoOverwrite 选项
func TestKVInterface_Set_NoOverwrite(t *testing.T) {
	instances := []*Instance{
		{ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "compat"},
	}

	storage, err := NewStorage(instances)
	if err != nil {
		t.Skipf("Aerospike not available: %v", err)
		return
	}
	defer storage.Close()

	skipIfNoAerospike(t, storage)

	ctx := context.Background()
	key := "compat:no-overwrite"

	// 第一次写入
	if err := storage.Set(ctx, key, "value1"); err != nil {
		t.Fatalf("第一次 Set() error = %v", err)
	}

	// 第二次写入（不使用 NoOverwrite，应该成功）
	if err := storage.Set(ctx, key, "value2"); err != nil {
		t.Fatalf("第二次 Set() error = %v", err)
	}

	// 第三次写入（使用 NoOverwrite，应该失败）
	err = storage.Set(ctx, key, "value3", kv.WithNoOverwrite())
	if err == nil {
		t.Error("NoOverwrite Set() 应该返回错误")
	}
	if !errors.Is(err, kv.ErrKeyAlreadyExists) {
		t.Errorf("Set() error = %v, want ErrKeyAlreadyExists", err)
	}

	// 清理
	storage.Delete(ctx, key)
}

// TestKVInterface_Get 测试 Get 方法
func TestKVInterface_Get(t *testing.T) {
	instances := []*Instance{
		{ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "compat"},
	}

	storage, err := NewStorage(instances)
	if err != nil {
		t.Skipf("Aerospike not available: %v", err)
		return
	}
	defer storage.Close()

	skipIfNoAerospike(t, storage)

	ctx := context.Background()

	// 准备数据
	storage.Set(ctx, "compat:get:string", "hello")
	storage.Set(ctx, "compat:get:struct", map[string]interface{}{"name": "Bob"})

	tests := []struct {
		name    string
		key     string
		wantErr error
	}{
		{
			name: "读取存在的 key",
			key:  "compat:get:string",
		},
		{
			name:    "读取不存在的 key",
			key:     "compat:get:notexist",
			wantErr: kv.ErrKeyNotFound,
		},
		{
			name:    "空 key",
			key:     "",
			wantErr: kv.ErrInvalidKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var value interface{}
			err := storage.Get(ctx, tt.key, &value)
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("Get() 应该返回错误")
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Get() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("Get() error = %v", err)
			}
		})
	}
}

// TestKVInterface_Delete 测试 Delete 方法
func TestKVInterface_Delete(t *testing.T) {
	instances := []*Instance{
		{ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "compat"},
	}

	storage, err := NewStorage(instances)
	if err != nil {
		t.Skipf("Aerospike not available: %v", err)
		return
	}
	defer storage.Close()

	skipIfNoAerospike(t, storage)

	ctx := context.Background()
	key := "compat:delete"

	// 写入数据
	if err := storage.Set(ctx, key, "value"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// 删除存在的 key
	if err := storage.Delete(ctx, key); err != nil {
		t.Errorf("Delete() error = %v", err)
	}

	// 验证已删除
	exists, _ := storage.Exists(ctx, key)
	if exists {
		t.Error("删除后 Exists() 应该返回 false")
	}

	// 删除不存在的 key（应该成功）
	if err := storage.Delete(ctx, "compat:delete:notexist"); err != nil {
		t.Errorf("删除不存在的 key 应该成功，error = %v", err)
	}

	// 删除空 key
	err = storage.Delete(ctx, "")
	if err == nil {
		t.Error("空 key 的 Delete() 应该返回错误")
	}
	if !errors.Is(err, kv.ErrInvalidKey) {
		t.Errorf("Delete() error = %v, want ErrInvalidKey", err)
	}
}

// TestKVInterface_Exists 测试 Exists 方法
func TestKVInterface_Exists(t *testing.T) {
	instances := []*Instance{
		{ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "compat"},
	}

	storage, err := NewStorage(instances)
	if err != nil {
		t.Skipf("Aerospike not available: %v", err)
		return
	}
	defer storage.Close()

	skipIfNoAerospike(t, storage)

	ctx := context.Background()
	key := "compat:exists"

	// 存在的 key
	storage.Set(ctx, key, "value")
	exists, err := storage.Exists(ctx, key)
	if err != nil {
		t.Errorf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("Exists() 应该返回 true")
	}

	// 不存在的 key
	exists, err = storage.Exists(ctx, "compat:exists:notexist")
	if err != nil {
		t.Errorf("Exists() error = %v", err)
	}
	if exists {
		t.Error("不存在的 key，Exists() 应该返回 false")
	}

	// 空 key
	_, err = storage.Exists(ctx, "")
	if err == nil {
		t.Error("空 key 的 Exists() 应该返回错误")
	}
	if !errors.Is(err, kv.ErrInvalidKey) {
		t.Errorf("Exists() error = %v, want ErrInvalidKey", err)
	}

	// 清理
	storage.Delete(ctx, key)
}

// TestKVInterface_TTL 测试 TTL 功能
func TestKVInterface_TTL(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要等待的测试")
	}

	instances := []*Instance{
		{ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "compat"},
	}

	storage, err := NewStorage(instances)
	if err != nil {
		t.Skipf("Aerospike not available: %v", err)
		return
	}
	defer storage.Close()

	skipIfNoAerospike(t, storage)

	ctx := context.Background()
	key := "compat:ttl"

	// 写入 1 秒过期
	if err := storage.Set(ctx, key, "value", kv.WithTTL(1*time.Second)); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// 立即读取应该成功
	var value string
	if err := storage.Get(ctx, key, &value); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	// 等待过期
	time.Sleep(2 * time.Second)

	// 过期后读取应该失败
	err = storage.Get(ctx, key, &value)
	if err == nil {
		t.Error("过期后 Get() 应该返回错误")
	}
	if !errors.Is(err, kv.ErrKeyNotFound) {
		t.Errorf("Get() error = %v, want ErrKeyNotFound", err)
	}
}

// TestKVInterface_BatchOperationsNotSupported 测试批量操作不支持
func TestKVInterface_BatchOperationsNotSupported(t *testing.T) {
	instances := []*Instance{
		{ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "compat"},
	}

	storage, err := NewStorage(instances)
	if err != nil {
		t.Skipf("Aerospike not available: %v", err)
		return
	}
	defer storage.Close()

	skipIfNoAerospike(t, storage)

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

// TestKVInterface_Close 测试关闭
func TestKVInterface_Close(t *testing.T) {
	instances := []*Instance{
		{ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "compat"},
	}

	storage, _ := NewStorage(instances)

	// 第一次关闭
	if err := storage.Close(); err != nil {
		t.Errorf("第一次 Close() error = %v", err)
	}

	// 第二次关闭（幂等）
	if err := storage.Close(); err != nil {
		t.Errorf("第二次 Close() error = %v", err)
	}

	// 关闭后操作应该返回错误
	ctx := context.Background()
	err := storage.Set(ctx, "key", "value")
	if err == nil {
		t.Error("关闭后 Set() 应该返回错误")
	}
	if !errors.Is(err, kv.ErrStorageClosed) {
		t.Errorf("Set() error = %v, want ErrStorageClosed", err)
	}

	err = storage.Get(ctx, "key", nil)
	if err == nil {
		t.Error("关闭后 Get() 应该返回错误")
	}
	if !errors.Is(err, kv.ErrStorageClosed) {
		t.Errorf("Get() error = %v, want ErrStorageClosed", err)
	}
}

// TestKVInterface_ErrorHandling 测试错误处理
func TestKVInterface_ErrorHandling(t *testing.T) {
	instances := []*Instance{
		{ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "compat"},
	}

	storage, err := NewStorage(instances)
	if err != nil {
		t.Skipf("Aerospike not available: %v", err)
		return
	}
	defer storage.Close()

	skipIfNoAerospike(t, storage)

	ctx := context.Background()

	// 测试 StorageError 包装
	err = storage.Set(ctx, "", "value")
	if err == nil {
		t.Fatal("Set() 应该返回错误")
	}

	var storageErr *kv.StorageError
	if !errors.As(err, &storageErr) {
		t.Error("错误应该是 StorageError 类型")
	} else {
		if storageErr.Op != "set" {
			t.Errorf("StorageError.Op = %v, want set", storageErr.Op)
		}
		if storageErr.Key != "" {
			t.Errorf("StorageError.Key = %v, want empty", storageErr.Key)
		}
		if !errors.Is(storageErr.Err, kv.ErrInvalidKey) {
			t.Errorf("StorageError.Err = %v, want ErrInvalidKey", storageErr.Err)
		}
	}
}
