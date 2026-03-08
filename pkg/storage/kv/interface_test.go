// Package kv provides a generic key-value storage interface.
// 此文件基于 prompts/features/kv-storage/00-iface.md 实现
package kv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"
)

// mockStore 是一个用于测试的 Mock 实现，实现了 Storage 接口
type mockStore struct {
	data    map[string]interface{}
	ttlData map[string]time.Time
	closed  bool
}

func newMockStore() *mockStore {
	return &mockStore{
		data:    make(map[string]interface{}),
		ttlData: make(map[string]time.Time),
	}
}

func (m *mockStore) Set(ctx context.Context, key string, value interface{}, opts ...Option) error {
	if m.closed {
		return ErrStorageClosed
	}
	if key == "" {
		return ErrInvalidKey
	}
	if value == nil {
		return ErrInvalidValue
	}

	// 处理选项
	options := DefaultOptions().Apply(opts...)

	// 检查 NoOverwrite 选项
	if options.NoOverwrite {
		if _, exists := m.data[key]; exists {
			return ErrKeyAlreadyExists
		}
	}

	// 应用命名空间
	fullKey := key
	if options.Namespace != "" {
		fullKey = options.Namespace + ":" + key
	}

	m.data[fullKey] = value

	// 处理 TTL 选项
	if options.TTL > 0 {
		m.ttlData[fullKey] = time.Now().Add(options.TTL)
	}

	return nil
}

func (m *mockStore) Get(ctx context.Context, key string, value interface{}) error {
	if m.closed {
		return ErrStorageClosed
	}
	if key == "" {
		return ErrInvalidKey
	}

	// 检查 TTL
	if exp, exists := m.ttlData[key]; exists {
		if time.Now().After(exp) {
			delete(m.data, key)
			delete(m.ttlData, key)
			return ErrKeyNotFound
		}
	}

	storedValue, exists := m.data[key]
	if !exists {
		return ErrKeyNotFound
	}

	// 尝试将存储的值解码到输出参数
	switch v := value.(type) {
	case *interface{}:
		*v = storedValue
	default:
		// 使用 JSON 编解码
		data, err := json.Marshal(storedValue)
		if err != nil {
			return fmt.Errorf("failed to encode value: %w", err)
		}
		if err := json.Unmarshal(data, value); err != nil {
			return fmt.Errorf("failed to decode value: %w", err)
		}
	}

	return nil
}

func (m *mockStore) Delete(ctx context.Context, key string) error {
	if m.closed {
		return ErrStorageClosed
	}
	if key == "" {
		return ErrInvalidKey
	}

	delete(m.data, key)
	delete(m.ttlData, key)
	return nil
}

func (m *mockStore) BatchSet(ctx context.Context, items map[string]interface{}, opts ...Option) error {
	if m.closed {
		return ErrStorageClosed
	}

	for key, value := range items {
		if err := m.Set(ctx, key, value, opts...); err != nil {
			return err
		}
	}
	return nil
}

func (m *mockStore) BatchGet(ctx context.Context, keys []string) (map[string]interface{}, error) {
	if m.closed {
		return nil, ErrStorageClosed
	}

	result := make(map[string]interface{})
	for _, key := range keys {
		var value interface{}
		if err := m.Get(ctx, key, &value); err != nil {
			if !errors.Is(err, ErrKeyNotFound) {
				return nil, err
			}
			continue
		}
		result[key] = value
	}
	return result, nil
}

func (m *mockStore) BatchDelete(ctx context.Context, keys []string) error {
	if m.closed {
		return ErrStorageClosed
	}

	for _, key := range keys {
		if err := m.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

func (m *mockStore) Exists(ctx context.Context, key string) (bool, error) {
	if m.closed {
		return false, ErrStorageClosed
	}
	if key == "" {
		return false, ErrInvalidKey
	}

	// 检查 TTL
	if exp, exists := m.ttlData[key]; exists {
		if time.Now().After(exp) {
			delete(m.data, key)
			delete(m.ttlData, key)
			return false, nil
		}
	}

	_, exists := m.data[key]
	return exists, nil
}

func (m *mockStore) Close() error {
	m.closed = true
	m.data = nil
	m.ttlData = nil
	return nil
}

// 测试辅助函数

func assertStoreSet(t *testing.T, store Storage, ctx context.Context, key string, value interface{}) {
	t.Helper()
	if err := store.Set(ctx, key, value); err != nil {
		t.Fatalf("Set() failed: %v", err)
	}
}

func assertStoreGet(t *testing.T, store Storage, ctx context.Context, key string, want interface{}) interface{} {
	t.Helper()
	var got interface{}
	if err := store.Get(ctx, key, &got); err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	gotBytes, _ := json.Marshal(got)
	wantBytes, _ := json.Marshal(want)
	if string(gotBytes) != string(wantBytes) {
		t.Fatalf("Get() = %v, want %v", got, want)
	}

	return got
}

func TestStorage_Set(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		key     string
		value   interface{}
		opts    []Option
		wantErr error
	}{
		{
			name:    "正常设置",
			key:     "test-key",
			value:   "test-value",
			opts:    nil,
			wantErr: nil,
		},
		{
			name:    "设置整数值",
			key:     "int-key",
			value:   42,
			opts:    nil,
			wantErr: nil,
		},
		{
			name:    "设置结构体",
			key:     "struct-key",
			value:   struct{ Name string }{"test"},
			opts:    nil,
			wantErr: nil,
		},
		{
			name:    "空键",
			key:     "",
			value:   "test-value",
			opts:    nil,
			wantErr: ErrInvalidKey,
		},
		{
			name:    "nil 值",
			key:     "nil-key",
			value:   nil,
			opts:    nil,
			wantErr: ErrInvalidValue,
		},
		{
			name:    "带 TTL",
			key:     "test-key-ttl",
			value:   "test-value",
			opts:    []Option{WithTTL(time.Hour)},
			wantErr: nil,
		},
		{
			name:    "零 TTL",
			key:     "test-key-zero-ttl",
			value:   "test-value",
			opts:    []Option{WithTTL(0)},
			wantErr: nil,
		},
		{
			name:  "禁止覆盖",
			key:   "no-overwrite",
			value: "first",
			opts:  []Option{WithNoOverwrite()},
		},
		{
			name:    "带命名空间",
			key:     "ns-key",
			value:   "value",
			opts:    []Option{WithNamespace("mynamespace")},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMockStore()

			// 测试第一次设置
			err := store.Set(ctx, tt.key, tt.value, tt.opts...)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Set() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// 如果成功且不是空键，验证值是否正确设置
			if err == nil && tt.key != "" && tt.wantErr == nil {
				// 构造完整的键名（考虑命名空间）
				fullKey := tt.key
				opts := DefaultOptions().Apply(tt.opts...)
				if opts.Namespace != "" {
					fullKey = opts.Namespace + ":" + tt.key
				}

				var got interface{}
				if err := store.Get(ctx, fullKey, &got); err != nil {
					t.Errorf("Get() after Set() failed: %v", err)
					return
				}

				gotBytes, _ := json.Marshal(got)
				wantBytes, _ := json.Marshal(tt.value)
				if string(gotBytes) != string(wantBytes) {
					t.Errorf("Get() value = %v, want %v", got, tt.value)
				}
			}
		})
	}
}

func TestStorage_Set_NoOverwrite(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()

	key := "no-overwrite-key"

	// 第一次设置应该成功
	err := store.Set(ctx, key, "first-value", WithNoOverwrite())
	if err != nil {
		t.Fatalf("First Set() failed: %v", err)
	}

	// 第二次设置应该失败
	err = store.Set(ctx, key, "second-value", WithNoOverwrite())
	if !errors.Is(err, ErrKeyAlreadyExists) {
		t.Errorf("Second Set() with NoOverwrite should return ErrKeyAlreadyExists, got: %v", err)
	}

	// 验证值没有被覆盖
	var got string
	err = store.Get(ctx, key, &got)
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if got != "first-value" {
		t.Errorf("Value = %s, want first-value", got)
	}
}

func TestStorage_Get(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()

	store.Set(ctx, "key1", "value1")
	store.Set(ctx, "key2", 42)
	store.Set(ctx, "key3", struct{ Name string }{"test"})

	tests := []struct {
		name    string
		key     string
		want    interface{}
		wantErr error
	}{
		{
			name:    "获取字符串值",
			key:     "key1",
			want:    "value1",
			wantErr: nil,
		},
		{
			name:    "获取整数值",
			key:     "key2",
			want:    42,
			wantErr: nil,
		},
		{
			name:    "获取结构体值",
			key:     "key3",
			want:    struct{ Name string }{"test"},
			wantErr: nil,
		},
		{
			name:    "获取不存在的键",
			key:     "nonexistent",
			want:    nil,
			wantErr: ErrKeyNotFound,
		},
		{
			name:    "空键",
			key:     "",
			want:    nil,
			wantErr: ErrInvalidKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got interface{}
			err := store.Get(ctx, tt.key, &got)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && tt.wantErr == nil {
				gotBytes, _ := json.Marshal(got)
				wantBytes, _ := json.Marshal(tt.want)
				if string(gotBytes) != string(wantBytes) {
					t.Errorf("Get() value = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestStorage_Delete(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()

	store.Set(ctx, "key1", "value1")
	store.Set(ctx, "key2", "value2")

	tests := []struct {
		name    string
		key     string
		wantErr error
	}{
		{
			name:    "删除存在的键",
			key:     "key1",
			wantErr: nil,
		},
		{
			name:    "删除不存在的键（应该成功，不报错）",
			key:     "nonexistent",
			wantErr: nil,
		},
		{
			name:    "删除空键",
			key:     "",
			wantErr: ErrInvalidKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.Delete(ctx, tt.key)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// 如果成功删除，验证键确实不存在
			if err == nil && tt.key != "" {
				var got interface{}
				err := store.Get(ctx, tt.key, &got)
				if !errors.Is(err, ErrKeyNotFound) {
					t.Errorf("Delete() failed: key still exists")
				}
			}
		})
	}
}

func TestStorage_BatchSet(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		items   map[string]interface{}
		opts    []Option
		wantErr error
	}{
		{
			name: "批量设置成功",
			items: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
				"key3": struct{ Name string }{"test"},
			},
			wantErr: nil,
		},
		{
			name: "批量设置包含空键",
			items: map[string]interface{}{
				"key1": "value1",
				"":     "value2",
			},
			wantErr: ErrInvalidKey,
		},
		{
			name:    "批量设置空映射",
			items:   map[string]interface{}{},
			wantErr: nil,
		},
		{
			name: "批量设置带命名空间",
			items: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			opts:    []Option{WithNamespace("test")},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMockStore()
			err := store.BatchSet(ctx, tt.items, tt.opts...)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("BatchSet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// 如果成功，验证所有键都已设置
			if err == nil && len(tt.items) > 0 {
				// 构造命名空间前缀
				namespace := ""
				opts := DefaultOptions().Apply(tt.opts...)
				if opts.Namespace != "" {
					namespace = opts.Namespace + ":"
				}

				for key, expectedValue := range tt.items {
					fullKey := namespace + key
					var got interface{}
					if err := store.Get(ctx, fullKey, &got); err != nil {
						t.Errorf("BatchSet() failed to get key %s: %v", fullKey, err)
						continue
					}

					gotBytes, _ := json.Marshal(got)
					wantBytes, _ := json.Marshal(expectedValue)
					if string(gotBytes) != string(wantBytes) {
						t.Errorf("BatchSet() key %s = %v, want %v", key, got, expectedValue)
					}
				}
			}
		})
	}
}

func TestStorage_BatchGet(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()

	store.BatchSet(ctx, map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"key3": struct{ Name string }{"test"},
	})

	tests := []struct {
		name    string
		keys    []string
		want    map[string]interface{}
		wantErr error
	}{
		{
			name: "批量获取成功",
			keys: []string{"key1", "key2"},
			want: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
			},
			wantErr: nil,
		},
		{
			name: "批量获取包含不存在的键",
			keys: []string{"key1", "nonexistent"},
			want: map[string]interface{}{
				"key1": "value1",
			},
			wantErr: nil,
		},
		{
			name:    "批量获取空键列表",
			keys:    []string{},
			want:    map[string]interface{}{},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.BatchGet(ctx, tt.keys)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("BatchGet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if len(got) != len(tt.want) {
					t.Errorf("BatchGet() len = %d, want %d", len(got), len(tt.want))
					return
				}
				for key, value := range got {
					gotBytes, _ := json.Marshal(value)
					wantBytes, _ := json.Marshal(tt.want[key])
					if string(gotBytes) != string(wantBytes) {
						t.Errorf("BatchGet() key %s = %v, want %v", key, value, tt.want[key])
					}
				}
			}
		})
	}
}

func TestStorage_BatchDelete(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()

	store.BatchSet(ctx, map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	})

	tests := []struct {
		name    string
		keys    []string
		wantErr error
	}{
		{
			name:    "批量删除成功",
			keys:    []string{"key1", "key2"},
			wantErr: nil,
		},
		{
			name:    "批量删除包含不存在的键",
			keys:    []string{"key3", "nonexistent"},
			wantErr: nil,
		},
		{
			name:    "批量删除空键列表",
			keys:    []string{},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.BatchDelete(ctx, tt.keys)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("BatchDelete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// 验证键已被删除
			if err == nil {
				for _, key := range tt.keys {
					if key == "" {
						continue
					}
					var got interface{}
					err := store.Get(ctx, key, &got)
					if !errors.Is(err, ErrKeyNotFound) {
						t.Errorf("BatchDelete() failed: key %s still exists", key)
					}
				}
			}
		})
	}
}

func TestStorage_Exists(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()

	store.Set(ctx, "key1", "value1")

	tests := []struct {
		name    string
		key     string
		want    bool
		wantErr error
	}{
		{
			name:    "检查存在的键",
			key:     "key1",
			want:    true,
			wantErr: nil,
		},
		{
			name:    "检查不存在的键",
			key:     "nonexistent",
			want:    false,
			wantErr: nil,
		},
		{
			name:    "检查空键",
			key:     "",
			want:    false,
			wantErr: ErrInvalidKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.Exists(ctx, tt.key)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Exists() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("Exists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStorage_TTL(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()

	// 设置一个短 TTL 的键
	key := "expiring-key"
	value := "value"
	err := store.Set(ctx, key, value, WithTTL(10*time.Millisecond))
	if err != nil {
		t.Fatalf("Set() with TTL failed: %v", err)
	}

	// 立即获取应该成功
	var got string
	err = store.Get(ctx, key, &got)
	if err != nil {
		t.Errorf("Get() before TTL expired failed: %v", err)
	}
	if got != value {
		t.Errorf("Get() value = %s, want %s", got, value)
	}

	// 等待 TTL 过期
	time.Sleep(15 * time.Millisecond)

	// 再次获取应该失败
	err = store.Get(ctx, key, &got)
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("Get() after TTL expired should return ErrKeyNotFound, got: %v", err)
	}

	// Exists 也应该返回 false
	exists, err := store.Exists(ctx, key)
	if err != nil {
		t.Errorf("Exists() after TTL expired failed: %v", err)
	}
	if exists {
		t.Errorf("Exists() after TTL expired should return false, got true")
	}
}

func TestStorage_Close(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()

	// 正常操作
	err := store.Set(ctx, "key1", "value1")
	if err != nil {
		t.Fatalf("Set() before Close() failed: %v", err)
	}

	// 关闭存储
	err = store.Close()
	if err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	// 关闭后操作应该返回 ErrStorageClosed
	err = store.Set(ctx, "key2", "value2")
	if !errors.Is(err, ErrStorageClosed) {
		t.Errorf("Set() after Close() should return ErrStorageClosed, got: %v", err)
	}

	var got string
	err = store.Get(ctx, "key1", &got)
	if !errors.Is(err, ErrStorageClosed) {
		t.Errorf("Get() after Close() should return ErrStorageClosed, got: %v", err)
	}

	err = store.Delete(ctx, "key1")
	if !errors.Is(err, ErrStorageClosed) {
		t.Errorf("Delete() after Close() should return ErrStorageClosed, got: %v", err)
	}

	_, err = store.Exists(ctx, "key1")
	if !errors.Is(err, ErrStorageClosed) {
		t.Errorf("Exists() after Close() should return ErrStorageClosed, got: %v", err)
	}

	_, err = store.BatchGet(ctx, []string{"key1"})
	if !errors.Is(err, ErrStorageClosed) {
		t.Errorf("BatchGet() after Close() should return ErrStorageClosed, got: %v", err)
	}

	err = store.BatchSet(ctx, map[string]interface{}{"key3": "value3"})
	if !errors.Is(err, ErrStorageClosed) {
		t.Errorf("BatchSet() after Close() should return ErrStorageClosed, got: %v", err)
	}

	err = store.BatchDelete(ctx, []string{"key1"})
	if !errors.Is(err, ErrStorageClosed) {
		t.Errorf("BatchDelete() after Close() should return ErrStorageClosed, got: %v", err)
	}
}

func TestOption_WithTTL(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()

	tests := []struct {
		name    string
		ttl     time.Duration
		wantErr bool
	}{
		{
			name:    "正常 TTL",
			ttl:     time.Hour,
			wantErr: false,
		},
		{
			name:    "零 TTL",
			ttl:     0,
			wantErr: false,
		},
		{
			name:    "负 TTL",
			ttl:     -1,
			wantErr: false, // 负 TTL 被视为零
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "test-ttl"
			value := "test-value"

			// 设置带 TTL 的键
			err := store.Set(ctx, key, value, WithTTL(tt.ttl))
			if (err != nil) != tt.wantErr {
				t.Errorf("Set() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// 验证键可以立即获取（除非 TTL 为负且立即过期）
			if tt.ttl >= 0 {
				var got string
				err := store.Get(ctx, key, &got)
				if err != nil {
					t.Errorf("Get() failed: %v", err)
					return
				}
				if got != value {
					t.Errorf("Get() value = %s, want %s", got, value)
				}
			}
		})
	}
}

func TestOption_WithNamespace(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()

	// 在命名空间中设置键
	key := "test-key"
	value := "test-value"
	namespace := "mynamespace"

	err := store.Set(ctx, key, value, WithNamespace(namespace))
	if err != nil {
		t.Fatalf("Set() with namespace failed: %v", err)
	}

	// 使用命名空间获取
	fullKey := namespace + ":" + key
	var got string
	err = store.Get(ctx, fullKey, &got)
	if err != nil {
		t.Errorf("Get() with full key failed: %v", err)
	}
	if got != value {
		t.Errorf("Get() value = %s, want %s", got, value)
	}

	// 直接获取应该失败
	err = store.Get(ctx, key, &got)
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("Get() without namespace should return ErrKeyNotFound, got: %v", err)
	}
}

func BenchmarkStorage_Set(b *testing.B) {
	ctx := context.Background()
	store := newMockStore()
	key := "benchmark-key"
	value := make([]byte, 1024) // 1KB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Set(ctx, key, value)
	}
}

func BenchmarkStorage_Get(b *testing.B) {
	ctx := context.Background()
	store := newMockStore()
	key := "benchmark-key"
	value := make([]byte, 1024) // 1KB
	store.Set(ctx, key, value)

	var got []byte
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Get(ctx, key, &got)
	}
}

func BenchmarkStorage_BatchSet(b *testing.B) {
	ctx := context.Background()
	items := make(map[string]interface{}, 100)
	for i := 0; i < 100; i++ {
		items[fmt.Sprintf("key%d", i)] = make([]byte, 1024)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store := newMockStore()
		store.BatchSet(ctx, items)
	}
}

func BenchmarkStorage_BatchGet(b *testing.B) {
	ctx := context.Background()
	store := newMockStore()
	items := make(map[string]interface{}, 100)
	keys := make([]string, 100)
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key%d", i)
		keys[i] = key
		items[key] = make([]byte, 1024)
	}
	store.BatchSet(ctx, items)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.BatchGet(ctx, keys)
	}
}
