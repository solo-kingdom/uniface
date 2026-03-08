// Package config provides configuration storage interface with read, write, and watch capabilities.
// 此文件基于 prompts/features/storage/config/00-iface.md 实现
package config

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
)

// mockConfigStore 是一个用于测试的 Mock 实现，实现了 Storage 接口
type mockConfigStore struct {
	data       map[string]interface{}
	cache      map[string]cacheEntry
	watchers   map[string][]Handler
	cacheMutex sync.RWMutex
	watchMutex sync.RWMutex
	closed     bool
}

type cacheEntry struct {
	value     interface{}
	expiresAt time.Time
}

func newMockConfigStore() *mockConfigStore {
	return &mockConfigStore{
		data:     make(map[string]interface{}),
		cache:    make(map[string]cacheEntry),
		watchers: make(map[string][]Handler),
	}
}

func (m *mockConfigStore) Read(ctx context.Context, key string, value interface{}, opts ...Option) error {
	if m.closed {
		return ErrStorageClosed
	}
	if key == "" {
		return ErrInvalidConfigKey
	}

	m.cacheMutex.RLock()
	defer m.cacheMutex.RUnlock()

	storedValue, exists := m.data[key]
	if !exists {
		return ErrConfigNotFound
	}

	// 尝试将存储的值解码到输出参数
	switch v := value.(type) {
	case *interface{}:
		*v = storedValue
	default:
		data, err := json.Marshal(storedValue)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, value); err != nil {
			return err
		}
	}

	return nil
}

func (m *mockConfigStore) ReadWithCache(ctx context.Context, key string, value interface{}, opts ...Option) error {
	if m.closed {
		return ErrStorageClosed
	}
	if key == "" {
		return ErrInvalidConfigKey
	}

	options := DefaultOptions().Apply(opts...)

	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	// 检查缓存
	if entry, exists := m.cache[key]; exists {
		if time.Now().Before(entry.expiresAt) {
			// 缓存未过期，使用缓存值
			switch v := value.(type) {
			case *interface{}:
				*v = entry.value
			default:
				data, err := json.Marshal(entry.value)
				if err != nil {
					return err
				}
				if err := json.Unmarshal(data, value); err != nil {
					return err
				}
			}
			return nil
		}
		// 缓存过期，删除
		delete(m.cache, key)
	}

	// 缓存未命中，从数据源读取
	storedValue, exists := m.data[key]
	if !exists {
		return ErrConfigNotFound
	}

	// 写入缓存
	m.cache[key] = cacheEntry{
		value:     storedValue,
		expiresAt: time.Now().Add(options.CacheTTL),
	}

	// 解码值
	switch v := value.(type) {
	case *interface{}:
		*v = storedValue
	default:
		data, err := json.Marshal(storedValue)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, value); err != nil {
			return err
		}
	}

	return nil
}

func (m *mockConfigStore) Write(ctx context.Context, key string, value interface{}, opts ...Option) error {
	if m.closed {
		return ErrStorageClosed
	}
	if key == "" {
		return ErrInvalidConfigKey
	}
	if value == nil {
		return ErrInvalidConfigValue
	}

	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	// 写入数据
	m.data[key] = value

	// 清除缓存
	delete(m.cache, key)

	// 触发监听器
	m.notifyWatchers(key, value)

	return nil
}

func (m *mockConfigStore) Watch(ctx context.Context, key string, handler Handler, opts ...Option) error {
	if m.closed {
		return ErrStorageClosed
	}
	if key == "" {
		return ErrInvalidKey
	}
	if handler == nil {
		return ErrInvalidHandler
	}

	m.watchMutex.Lock()
	defer m.watchMutex.Unlock()

	m.watchers[key] = append(m.watchers[key], handler)

	return nil
}

func (m *mockConfigStore) notifyWatchers(key string, value interface{}) {
	m.watchMutex.RLock()
	defer m.watchMutex.RUnlock()

	if handlers, exists := m.watchers[key]; exists {
		for _, handler := range handlers {
			// 异步调用监听器
			go func(h Handler) {
				_ = h(context.Background(), key, value)
			}(handler)
		}
	}
}

func (m *mockConfigStore) Delete(ctx context.Context, key string) error {
	if m.closed {
		return ErrStorageClosed
	}
	if key == "" {
		return ErrInvalidConfigKey
	}

	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	delete(m.data, key)
	delete(m.cache, key)

	// 通知监听器（value 为 nil 表示删除）
	m.notifyWatchers(key, nil)

	return nil
}

func (m *mockConfigStore) Unwatch(key string) error {
	if m.closed {
		return ErrStorageClosed
	}

	m.watchMutex.Lock()
	defer m.watchMutex.Unlock()

	delete(m.watchers, key)
	return nil
}

func (m *mockConfigStore) WatchPrefix(ctx context.Context, prefix string, handler Handler, opts ...Option) error {
	if m.closed {
		return ErrStorageClosed
	}
	if prefix == "" {
		return ErrInvalidConfigKey
	}
	if handler == nil {
		return ErrInvalidHandler
	}

	// 在 mock 实现中，为了简化，我们将 WatchPrefix 作为单独的监听器
	// 实际实现中应该匹配前缀的所有键
	prefixKey := "prefix:" + prefix
	m.watchMutex.Lock()
	defer m.watchMutex.Unlock()

	m.watchers[prefixKey] = append(m.watchers[prefixKey], handler)

	return nil
}

func (m *mockConfigStore) UnwatchPrefix(prefix string) error {
	if m.closed {
		return ErrStorageClosed
	}

	m.watchMutex.Lock()
	defer m.watchMutex.Unlock()

	prefixKey := "prefix:" + prefix
	delete(m.watchers, prefixKey)
	return nil
}

func (m *mockConfigStore) List(ctx context.Context, prefix string) ([]string, error) {
	if m.closed {
		return nil, ErrStorageClosed
	}

	m.cacheMutex.RLock()
	defer m.cacheMutex.RUnlock()

	var keys []string
	for key := range m.data {
		if prefix == "" || len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			keys = append(keys, key)
		}
	}

	return keys, nil
}

func (m *mockConfigStore) ClearCache(key string) error {
	if m.closed {
		return ErrStorageClosed
	}

	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	if key == "" {
		// 清除所有缓存
		m.cache = make(map[string]cacheEntry)
	} else {
		delete(m.cache, key)
	}

	return nil
}

func (m *mockConfigStore) Close() error {
	m.closed = true
	m.data = nil
	m.cache = nil
	m.watchers = nil
	return nil
}

// 测试辅助函数

func assertConfigRead(t *testing.T, store Storage, ctx context.Context, key string, want interface{}) interface{} {
	t.Helper()
	var got interface{}
	if err := store.Read(ctx, key, &got); err != nil {
		t.Fatalf("Read() failed: %v", err)
	}

	gotBytes, _ := json.Marshal(got)
	wantBytes, _ := json.Marshal(want)
	if string(gotBytes) != string(wantBytes) {
		t.Fatalf("Read() = %v, want %v", got, want)
	}

	return got
}

func TestStorage_Read(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		key     string
		value   interface{}
		setup   func(*mockConfigStore)
		wantErr error
	}{
		{
			name:    "正常读取字符串",
			key:     "config-key",
			value:   "config-value",
			setup:   func(s *mockConfigStore) { s.Write(ctx, "config-key", "config-value") },
			wantErr: nil,
		},
		{
			name:    "读取整数配置",
			key:     "port",
			value:   8080,
			setup:   func(s *mockConfigStore) { s.Write(ctx, "port", 8080) },
			wantErr: nil,
		},
		{
			name:    "读取结构体配置",
			key:     "database",
			value:   struct{ Host string }{"localhost"},
			setup:   func(s *mockConfigStore) { s.Write(ctx, "database", struct{ Host string }{"localhost"}) },
			wantErr: nil,
		},
		{
			name:    "读取不存在的配置",
			key:     "nonexistent",
			value:   nil,
			setup:   func(s *mockConfigStore) {},
			wantErr: ErrConfigNotFound,
		},
		{
			name:    "空键",
			key:     "",
			value:   nil,
			setup:   func(s *mockConfigStore) {},
			wantErr: ErrInvalidConfigKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMockConfigStore()
			if tt.setup != nil {
				tt.setup(store)
			}

			var got interface{}
			err := store.Read(ctx, tt.key, &got)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Read() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && tt.wantErr == nil {
				gotBytes, _ := json.Marshal(got)
				wantBytes, _ := json.Marshal(tt.value)
				if string(gotBytes) != string(wantBytes) {
					t.Errorf("Read() = %v, want %v", got, tt.value)
				}
			}
		})
	}
}

func TestStorage_ReadWithCache(t *testing.T) {
	ctx := context.Background()

	t.Run("缓存命中", func(t *testing.T) {
		store := newMockConfigStore()
		key := "cache-key"

		// 写入初始值
		store.Write(ctx, key, "cached-value")

		// 第一次读取，应该从数据源读取并缓存
		var got1 string
		err := store.ReadWithCache(ctx, key, &got1, WithCacheTTL(time.Hour))
		if err != nil {
			t.Fatalf("First ReadWithCache() failed: %v", err)
		}

		// 第二次读取，应该从缓存读取（因为缓存未过期且没有被清除）
		var got2 string
		err = store.ReadWithCache(ctx, key, &got2, WithCacheTTL(time.Hour))
		if err != nil {
			t.Fatalf("Second ReadWithCache() failed: %v", err)
		}

		if got2 != got1 {
			t.Errorf("ReadWithCache() returned %s (from cache), expected %s", got2, got1)
		}
	})

	t.Run("缓存过期", func(t *testing.T) {
		store := newMockConfigStore()
		key := "expiring-cache-key"
		value := "original-value"

		// 写入配置
		store.Write(ctx, key, value)

		// 第一次读取并设置短 TTL
		var got1 string
		err := store.ReadWithCache(ctx, key, &got1, WithCacheTTL(10*time.Millisecond))
		if err != nil {
			t.Fatalf("First ReadWithCache() failed: %v", err)
		}

		// 等待缓存过期
		time.Sleep(20 * time.Millisecond)

		// 修改数据源
		store.Write(ctx, key, "new-value")

		// 第二次读取，应该从数据源读取新值
		var got2 string
		err = store.ReadWithCache(ctx, key, &got2, WithCacheTTL(time.Hour))
		if err != nil {
			t.Fatalf("Second ReadWithCache() failed: %v", err)
		}

		if got2 == got1 {
			t.Errorf("ReadWithCache() should return new value after cache expired")
		}
	})

	t.Run("缓存失效（写入后）", func(t *testing.T) {
		store := newMockConfigStore()
		key := "invalidate-cache-key"
		value := "original-value"

		// 写入并读取（建立缓存）
		store.Write(ctx, key, value)
		var got1 string
		store.ReadWithCache(ctx, key, &got1, WithCacheTTL(time.Hour))

		// 写入新值（应该清除缓存）
		store.Write(ctx, key, "new-value")

		// 再次读取，应该得到新值
		var got2 string
		err := store.ReadWithCache(ctx, key, &got2, WithCacheTTL(time.Hour))
		if err != nil {
			t.Fatalf("ReadWithCache() after write failed: %v", err)
		}

		if got2 != "new-value" {
			t.Errorf("ReadWithCache() = %s, want new-value", got2)
		}
	})
}

func TestStorage_Write(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		key     string
		value   interface{}
		wantErr error
	}{
		{
			name:    "正常写入字符串",
			key:     "key1",
			value:   "value1",
			wantErr: nil,
		},
		{
			name:    "写入整数",
			key:     "port",
			value:   8080,
			wantErr: nil,
		},
		{
			name:    "写入结构体",
			key:     "config",
			value:   struct{ Name string }{"test"},
			wantErr: nil,
		},
		{
			name:    "空键",
			key:     "",
			value:   "value",
			wantErr: ErrInvalidKey,
		},
		{
			name:    "nil 值",
			key:     "key",
			value:   nil,
			wantErr: ErrInvalidValue,
		},
		{
			name:    "覆盖已存在的键",
			key:     "overwrite-key",
			value:   "new-value",
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMockConfigStore()

			// 如果是覆盖测试，先写入一个值
			if tt.key == "overwrite-key" {
				store.Write(ctx, tt.key, "old-value")
			}

			err := store.Write(ctx, tt.key, tt.value)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Write() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// 如果成功，验证值是否正确写入
			if err == nil && tt.wantErr == nil {
				var got interface{}
				if err := store.Read(ctx, tt.key, &got); err != nil {
					t.Errorf("Read() after Write() failed: %v", err)
					return
				}

				gotBytes, _ := json.Marshal(got)
				wantBytes, _ := json.Marshal(tt.value)
				if string(gotBytes) != string(wantBytes) {
					t.Errorf("Read() = %v, want %v", got, tt.value)
				}
			}
		})
	}
}

func TestStorage_Watch(t *testing.T) {
	ctx := context.Background()

	t.Run("监听配置变更", func(t *testing.T) {
		store := newMockConfigStore()
		key := "watch-key"

		// 创建监听通道
		changes := make(chan interface{}, 10)
		handler := func(ctx context.Context, k string, value interface{}) error {
			if k == key {
				changes <- value
			}
			return nil
		}

		// 注册监听器
		err := store.Watch(ctx, key, handler)
		if err != nil {
			t.Fatalf("Watch() failed: %v", err)
		}

		// 写入新值
		go func() {
			time.Sleep(10 * time.Millisecond)
			store.Write(ctx, key, "new-value-1")
		}()

		// 等待通知
		select {
		case got := <-changes:
			if got != "new-value-1" {
				t.Errorf("Watch handler received %v, want new-value-1", got)
			}
		case <-time.After(time.Second):
			t.Error("Watch handler timeout")
		}

		// 再次写入
		go func() {
			time.Sleep(10 * time.Millisecond)
			store.Write(ctx, key, "new-value-2")
		}()

		select {
		case got := <-changes:
			if got != "new-value-2" {
				t.Errorf("Watch handler received %v, want new-value-2", got)
			}
		case <-time.After(time.Second):
			t.Error("Watch handler timeout")
		}
	})

	t.Run("多个监听器", func(t *testing.T) {
		store := newMockConfigStore()
		key := "multi-watch-key"

		handler1Called := false
		handler2Called := false

		handler1 := func(ctx context.Context, k string, value interface{}) error {
			if k == key {
				handler1Called = true
			}
			return nil
		}

		handler2 := func(ctx context.Context, k string, value interface{}) error {
			if k == key {
				handler2Called = true
			}
			return nil
		}

		// 注册多个监听器
		store.Watch(ctx, key, handler1)
		store.Watch(ctx, key, handler2)

		// 写入新值
		store.Write(ctx, key, "test-value")

		// 等待异步处理
		time.Sleep(50 * time.Millisecond)

		if !handler1Called {
			t.Error("Handler 1 was not called")
		}
		if !handler2Called {
			t.Error("Handler 2 was not called")
		}
	})

	t.Run("无效监听器", func(t *testing.T) {
		store := newMockConfigStore()

		err := store.Watch(ctx, "key", nil)
		if !errors.Is(err, ErrInvalidHandler) {
			t.Errorf("Watch() with nil handler should return ErrInvalidHandler, got: %v", err)
		}
	})
}

func TestStorage_Close(t *testing.T) {
	ctx := context.Background()
	store := newMockConfigStore()

	// 正常操作
	err := store.Write(ctx, "key1", "value1")
	if err != nil {
		t.Fatalf("Write() before Close() failed: %v", err)
	}

	// 关闭存储
	err = store.Close()
	if err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	// 关闭后操作应该返回 ErrStorageClosed
	err = store.Write(ctx, "key2", "value2")
	if !errors.Is(err, ErrStorageClosed) {
		t.Errorf("Write() after Close() should return ErrStorageClosed, got: %v", err)
	}

	var got string
	err = store.Read(ctx, "key1", &got)
	if !errors.Is(err, ErrStorageClosed) {
		t.Errorf("Read() after Close() should return ErrStorageClosed, got: %v", err)
	}

	err = store.ReadWithCache(ctx, "key1", &got)
	if !errors.Is(err, ErrStorageClosed) {
		t.Errorf("ReadWithCache() after Close() should return ErrStorageClosed, got: %v", err)
	}

	err = store.Watch(ctx, "key", func(ctx context.Context, key string, value interface{}) error {
		return nil
	})
	if !errors.Is(err, ErrStorageClosed) {
		t.Errorf("Watch() after Close() should return ErrStorageClosed, got: %v", err)
	}
}

func BenchmarkStorage_Read(b *testing.B) {
	ctx := context.Background()
	store := newMockConfigStore()
	key := "benchmark-key"
	store.Write(ctx, key, "benchmark-value")

	var got string
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Read(ctx, key, &got)
	}
}

func BenchmarkStorage_ReadWithCache(b *testing.B) {
	ctx := context.Background()
	store := newMockConfigStore()
	key := "benchmark-cache-key"
	store.Write(ctx, key, "benchmark-value")

	var got string
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.ReadWithCache(ctx, key, &got, WithCacheTTL(time.Hour))
	}
}

func BenchmarkStorage_Write(b *testing.B) {
	ctx := context.Background()
	store := newMockConfigStore()
	key := "benchmark-key"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Write(ctx, key, "value")
	}
}

func BenchmarkStorage_ReadWithCache_Miss(b *testing.B) {
	ctx := context.Background()
	store := newMockConfigStore()
	keys := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		keys[i] = "key" + string(rune(i))
		store.Write(ctx, keys[i], "value")
	}

	var got string
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.ReadWithCache(ctx, keys[i%1000], &got, WithCacheTTL(time.Millisecond))
	}
}

func BenchmarkStorage_ReadWithCache_Hit(b *testing.B) {
	ctx := context.Background()
	store := newMockConfigStore()
	key := "benchmark-hit-key"
	store.Write(ctx, key, "benchmark-value")
	// 预热缓存
	var got string
	store.ReadWithCache(ctx, key, &got, WithCacheTTL(time.Hour))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.ReadWithCache(ctx, key, &got, WithCacheTTL(time.Hour))
	}
}
