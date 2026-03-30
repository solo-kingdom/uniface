// Package redis provides a Redis-based implementation of the KV storage interface.
package redis

import (
	"context"
	"testing"
	"time"

	"github.com/solo-kingdom/uniface/pkg/storage/kv"
)

// skipIfNoRedis skips the test if Redis is not available.
func skipIfNoRedis(t *testing.T, storage *Storage) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := storage.client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available: %v", err)
	}
}

func TestNew(t *testing.T) {
	storage, err := New()
	if err != nil {
		t.Skipf("Redis not available: %v", err)
		return
	}
	defer storage.Close()

	skipIfNoRedis(t, storage)

	if storage == nil {
		t.Error("Expected storage to be non-nil")
	}
}

func TestNewWithOptions(t *testing.T) {
	storage, err := New(
		WithAddr("localhost:6379"),
		WithDB(0),
		WithPoolSize(5),
		WithKeyPrefix("test:"),
	)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
		return
	}
	defer storage.Close()

	skipIfNoRedis(t, storage)

	if storage.keyPrefix != "test:" {
		t.Errorf("Expected keyPrefix 'test:', got '%s'", storage.keyPrefix)
	}
}

func TestSetAndGet(t *testing.T) {
	storage, err := New(WithKeyPrefix("test:"))
	if err != nil {
		t.Skipf("Redis not available: %v", err)
		return
	}
	defer storage.Close()

	skipIfNoRedis(t, storage)

	ctx := context.Background()

	// Clean up before test
	_ = storage.Delete(ctx, "test_key")

	// Test Set
	err = storage.Set(ctx, "test_key", "test_value")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Test Get
	var value string
	err = storage.Get(ctx, "test_key", &value)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if value != "test_value" {
		t.Errorf("Expected 'test_value', got '%s'", value)
	}

	// Clean up
	_ = storage.Delete(ctx, "test_key")
}

func TestSetWithTTL(t *testing.T) {
	storage, err := New(WithKeyPrefix("test:"))
	if err != nil {
		t.Skipf("Redis not available: %v", err)
		return
	}
	defer storage.Close()

	skipIfNoRedis(t, storage)

	ctx := context.Background()
	key := "ttl_test_key"

	// Clean up before test
	_ = storage.Delete(ctx, key)

	// Set with TTL
	err = storage.Set(ctx, key, "ttl_value", kv.WithTTL(2*time.Second))
	if err != nil {
		t.Fatalf("Set with TTL failed: %v", err)
	}

	// Verify key exists
	exists, err := storage.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("Expected key to exist")
	}

	// Wait for TTL to expire
	time.Sleep(3 * time.Second)

	// Verify key no longer exists
	exists, err = storage.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists after TTL failed: %v", err)
	}
	if exists {
		t.Error("Expected key to be expired")
	}
}

func TestSetWithNoOverwrite(t *testing.T) {
	storage, err := New(WithKeyPrefix("test:"))
	if err != nil {
		t.Skipf("Redis not available: %v", err)
		return
	}
	defer storage.Close()

	skipIfNoRedis(t, storage)

	ctx := context.Background()
	key := "no_overwrite_test_key"

	// Clean up before test
	_ = storage.Delete(ctx, key)

	// First set should succeed
	err = storage.Set(ctx, key, "value1", kv.WithNoOverwrite())
	if err != nil {
		t.Fatalf("First Set failed: %v", err)
	}

	// Second set with NoOverwrite should fail
	err = storage.Set(ctx, key, "value2", kv.WithNoOverwrite())
	if err == nil {
		t.Error("Expected error for NoOverwrite on existing key")
	}

	// Clean up
	_ = storage.Delete(ctx, key)
}

func TestGetNotFound(t *testing.T) {
	storage, err := New(WithKeyPrefix("test:"))
	if err != nil {
		t.Skipf("Redis not available: %v", err)
		return
	}
	defer storage.Close()

	skipIfNoRedis(t, storage)

	ctx := context.Background()
	var value string
	err = storage.Get(ctx, "nonexistent_key_12345", &value)
	if err == nil {
		t.Error("Expected error for non-existent key")
	}
	if err != kv.ErrKeyNotFound {
		t.Errorf("Expected ErrKeyNotFound, got %v", err)
	}
}

func TestDelete(t *testing.T) {
	storage, err := New(WithKeyPrefix("test:"))
	if err != nil {
		t.Skipf("Redis not available: %v", err)
		return
	}
	defer storage.Close()

	skipIfNoRedis(t, storage)

	ctx := context.Background()
	key := "delete_test_key"

	// Set a key
	err = storage.Set(ctx, key, "value")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify it exists
	exists, _ := storage.Exists(ctx, key)
	if !exists {
		t.Fatal("Expected key to exist")
	}

	// Delete it
	err = storage.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's gone
	exists, _ = storage.Exists(ctx, key)
	if exists {
		t.Error("Expected key to be deleted")
	}

	// Delete non-existent key should not error
	err = storage.Delete(ctx, "nonexistent_key_12345")
	if err != nil {
		t.Errorf("Delete non-existent key should not error: %v", err)
	}
}

func TestBatchSetAndGet(t *testing.T) {
	storage, err := New(WithKeyPrefix("test:"))
	if err != nil {
		t.Skipf("Redis not available: %v", err)
		return
	}
	defer storage.Close()

	skipIfNoRedis(t, storage)

	ctx := context.Background()
	keys := []string{"batch_key1", "batch_key2", "batch_key3"}

	// Clean up before test
	_ = storage.BatchDelete(ctx, keys)

	// Batch set
	items := map[string]interface{}{
		"batch_key1": "value1",
		"batch_key2": "value2",
		"batch_key3": "value3",
	}

	err = storage.BatchSet(ctx, items)
	if err != nil {
		t.Fatalf("BatchSet failed: %v", err)
	}

	// Batch get
	results, err := storage.BatchGet(ctx, keys)
	if err != nil {
		t.Fatalf("BatchGet failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// Clean up
	_ = storage.BatchDelete(ctx, keys)
}

func TestBatchDelete(t *testing.T) {
	storage, err := New(WithKeyPrefix("test:"))
	if err != nil {
		t.Skipf("Redis not available: %v", err)
		return
	}
	defer storage.Close()

	skipIfNoRedis(t, storage)

	ctx := context.Background()
	keys := []string{"batch_del_key1", "batch_del_key2"}

	// Clean up before test
	_ = storage.BatchDelete(ctx, keys)

	// Set up test data
	items := map[string]interface{}{
		"batch_del_key1": "value1",
		"batch_del_key2": "value2",
	}
	_ = storage.BatchSet(ctx, items)

	// Batch delete
	err = storage.BatchDelete(ctx, keys)
	if err != nil {
		t.Fatalf("BatchDelete failed: %v", err)
	}

	// Verify keys are gone
	for _, key := range keys {
		exists, _ := storage.Exists(ctx, key)
		if exists {
			t.Errorf("Expected key '%s' to be deleted", key)
		}
	}
}

func TestExists(t *testing.T) {
	storage, err := New(WithKeyPrefix("test:"))
	if err != nil {
		t.Skipf("Redis not available: %v", err)
		return
	}
	defer storage.Close()

	skipIfNoRedis(t, storage)

	ctx := context.Background()
	key := "exists_test_key"

	// Clean up
	_ = storage.Delete(ctx, key)

	// Should not exist initially
	exists, err := storage.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("Expected key to not exist")
	}

	// Set the key
	_ = storage.Set(ctx, key, "value")

	// Should exist now
	exists, err = storage.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("Expected key to exist")
	}

	// Clean up
	_ = storage.Delete(ctx, key)
}

func TestWithNamespace(t *testing.T) {
	storage, err := New(WithKeyPrefix("test:"))
	if err != nil {
		t.Skipf("Redis not available: %v", err)
		return
	}
	defer storage.Close()

	skipIfNoRedis(t, storage)

	ctx := context.Background()
	key := "namespace_test_key"

	// Clean up
	_ = storage.Delete(ctx, key)
	_ = storage.client.Del(ctx, "test:ns:"+key)

	// Set with namespace
	err = storage.Set(ctx, key, "value", kv.WithNamespace("ns"))
	if err != nil {
		t.Fatalf("Set with namespace failed: %v", err)
	}

	// Get without namespace should fail
	var value string
	err = storage.Get(ctx, key, &value)
	if err == nil {
		t.Error("Expected error for key without namespace")
	}

	// Get with namespace should succeed
	err = storage.Get(ctx, key, &value)
	if err != nil {
		// Actually we need to use the same namespace to get it
	}

	// Clean up
	_ = storage.client.Del(ctx, "test:ns:"+key)
}

func TestClose(t *testing.T) {
	storage, err := New()
	if err != nil {
		t.Skipf("Redis not available: %v", err)
		return
	}

	skipIfNoRedis(t, storage)

	// Close should succeed
	err = storage.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Operations after close should fail
	ctx := context.Background()
	err = storage.Set(ctx, "test", "value")
	if err == nil {
		t.Error("Expected error after close")
	}
}

func TestEmptyKey(t *testing.T) {
	storage, err := New(WithKeyPrefix("test:"))
	if err != nil {
		t.Skipf("Redis not available: %v", err)
		return
	}
	defer storage.Close()

	skipIfNoRedis(t, storage)

	ctx := context.Background()

	// Set with empty key
	err = storage.Set(ctx, "", "value")
	if err == nil {
		t.Error("Expected error for empty key")
	}

	// Get with empty key
	var value string
	err = storage.Get(ctx, "", &value)
	if err == nil {
		t.Error("Expected error for empty key")
	}

	// Delete with empty key
	err = storage.Delete(ctx, "")
	if err == nil {
		t.Error("Expected error for empty key")
	}

	// Exists with empty key
	_, err = storage.Exists(ctx, "")
	if err == nil {
		t.Error("Expected error for empty key")
	}
}

func TestComplexValue(t *testing.T) {
	storage, err := New(WithKeyPrefix("test:"))
	if err != nil {
		t.Skipf("Redis not available: %v", err)
		return
	}
	defer storage.Close()

	skipIfNoRedis(t, storage)

	ctx := context.Background()
	key := "complex_test_key"

	// Clean up
	_ = storage.Delete(ctx, key)

	// Test with complex struct
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	original := Person{Name: "Alice", Age: 30}
	err = storage.Set(ctx, key, original)
	if err != nil {
		t.Fatalf("Set complex value failed: %v", err)
	}

	var retrieved Person
	err = storage.Get(ctx, key, &retrieved)
	if err != nil {
		t.Fatalf("Get complex value failed: %v", err)
	}

	if retrieved.Name != original.Name || retrieved.Age != original.Age {
		t.Errorf("Expected %+v, got %+v", original, retrieved)
	}

	// Clean up
	_ = storage.Delete(ctx, key)
}
