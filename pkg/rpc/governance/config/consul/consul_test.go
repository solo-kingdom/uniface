// Package consul 提供基于 Consul 的配置存储实现测试。
//
// 基于 specs/features/rpc/governance/config/01 consul.md 实现
package consul

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/solo-kingdom/uniface/pkg/rpc/governance/config"
)

// 注意：这些测试需要运行 Consul 服务器。
// 可以使用 Docker 启动 Consul: docker run -d -p 8500:8500 consul
// 或者使用环境变量设置 Consul 地址: CONSUL_ADDR=192.168.1.100:8500

// skipIfNoConsul 如果 Consul 不可用则跳过测试。
func skipIfNoConsul(t *testing.T) *Storage {
	t.Helper()

	storage, err := NewStorage()
	if err != nil {
		t.Skipf("跳过测试：无法连接到 Consul: %v", err)
		return nil
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 尝试列出键来测试连接
	_, err = storage.List(ctx, "__test_connection__")
	if err != nil {
		t.Skipf("跳过测试：Consul 连接测试失败: %v", err)
		storage.Close()
		return nil
	}

	return storage
}

func TestNewStorage(t *testing.T) {
	tests := []struct {
		name    string
		opts    []Option
		wantErr bool
	}{
		{
			name:    "默认配置",
			opts:    nil,
			wantErr: false,
		},
		{
			name: "自定义地址",
			opts: []Option{
				WithAddress("127.0.0.1:8500"),
			},
			wantErr: false,
		},
		{
			name: "自定义前缀",
			opts: []Option{
				WithKeyPrefix("myapp/config/"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, err := NewStorage(tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewStorage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if storage != nil {
				storage.Close()
			}
		})
	}
}

func TestStorage_WriteRead(t *testing.T) {
	storage := skipIfNoConsul(t)
	if storage == nil {
		return
	}
	defer storage.Close()

	ctx := context.Background()

	tests := []struct {
		name  string
		key   string
		value interface{}
	}{
		{
			name:  "字符串值",
			key:   "test-string",
			value: "hello-world",
		},
		{
			name:  "整数值",
			key:   "test-int",
			value: 12345,
		},
		{
			name:  "布尔值",
			key:   "test-bool",
			value: true,
		},
		{
			name: "结构体值",
			key:  "test-struct",
			value: struct {
				Name string `json:"name"`
				Age  int    `json:"age"`
			}{
				Name: "test",
				Age:  30,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 清理测试数据
			defer storage.Delete(ctx, tt.key)

			// 写入
			err := storage.Write(ctx, tt.key, tt.value)
			if err != nil {
				t.Fatalf("Write() 失败: %v", err)
			}

			// 读取
			var got interface{}
			err = storage.Read(ctx, tt.key, &got)
			if err != nil {
				t.Fatalf("Read() 失败: %v", err)
			}

			t.Logf("成功读取配置: key=%s, value=%v", tt.key, got)
		})
	}
}

func TestStorage_ReadWithCache(t *testing.T) {
	storage := skipIfNoConsul(t)
	if storage == nil {
		return
	}
	defer storage.Close()

	ctx := context.Background()
	key := "test-cache-key"
	value := "cached-value"

	// 清理测试数据
	defer storage.Delete(ctx, key)

	// 写入初始值
	err := storage.Write(ctx, key, value)
	if err != nil {
		t.Fatalf("Write() 失败: %v", err)
	}

	// 第一次读取（建立缓存）
	var got1 string
	err = storage.ReadWithCache(ctx, key, &got1, config.WithCacheTTL(time.Hour))
	if err != nil {
		t.Fatalf("第一次 ReadWithCache() 失败: %v", err)
	}

	if got1 != value {
		t.Errorf("第一次读取 = %v, want %v", got1, value)
	}

	// 第二次读取（应该从缓存读取）
	var got2 string
	err = storage.ReadWithCache(ctx, key, &got2, config.WithCacheTTL(time.Hour))
	if err != nil {
		t.Fatalf("第二次 ReadWithCache() 失败: %v", err)
	}

	if got2 != value {
		t.Errorf("第二次读取 = %v, want %v", got2, value)
	}

	t.Log("缓存测试通过")
}

func TestStorage_Delete(t *testing.T) {
	storage := skipIfNoConsul(t)
	if storage == nil {
		return
	}
	defer storage.Close()

	ctx := context.Background()
	key := "test-delete-key"
	value := "to-be-deleted"

	// 写入
	err := storage.Write(ctx, key, value)
	if err != nil {
		t.Fatalf("Write() 失败: %v", err)
	}

	// 删除
	err = storage.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Delete() 失败: %v", err)
	}

	// 读取已删除的键应该返回 ErrConfigNotFound
	var got string
	err = storage.Read(ctx, key, &got)
	if !errors.Is(err, config.ErrConfigNotFound) {
		t.Errorf("删除后读取应该返回 ErrConfigNotFound, got: %v", err)
	}

	t.Log("删除测试通过")
}

func TestStorage_List(t *testing.T) {
	storage := skipIfNoConsul(t)
	if storage == nil {
		return
	}
	defer storage.Close()

	ctx := context.Background()
	prefix := "test-list-"

	// 写入多个键
	keys := []string{
		prefix + "key1",
		prefix + "key2",
		prefix + "key3",
	}

	for _, key := range keys {
		err := storage.Write(ctx, key, "value")
		if err != nil {
			t.Fatalf("Write(%s) 失败: %v", key, err)
		}
		defer storage.Delete(ctx, key)
	}

	// 列出键
	gotKeys, err := storage.List(ctx, prefix)
	if err != nil {
		t.Fatalf("List() 失败: %v", err)
	}

	// 验证返回的键
	if len(gotKeys) < len(keys) {
		t.Errorf("List() 返回 %d 个键, 至少需要 %d", len(gotKeys), len(keys))
	}

	t.Logf("列出的键: %v", gotKeys)
}

func TestStorage_Watch(t *testing.T) {
	storage := skipIfNoConsul(t)
	if storage == nil {
		return
	}
	defer storage.Close()

	ctx := context.Background()
	key := "test-watch-key"

	// 清理测试数据
	defer storage.Delete(ctx, key)

	// 创建变更通道
	changes := make(chan interface{}, 10)
	handler := func(ctx context.Context, k string, value interface{}) error {
		if k == key {
			changes <- value
		}
		return nil
	}

	// 注册监听器
	err := storage.Watch(ctx, key, handler)
	if err != nil {
		t.Fatalf("Watch() 失败: %v", err)
	}
	defer storage.Unwatch(key)

	// 等待监听器启动
	time.Sleep(100 * time.Millisecond)

	// 写入值触发变更
	err = storage.Write(ctx, key, "new-value")
	if err != nil {
		t.Fatalf("Write() 失败: %v", err)
	}

	// 等待通知
	select {
	case got := <-changes:
		t.Logf("收到变更通知: %v", got)
	case <-time.After(5 * time.Second):
		t.Error("监听器超时，未收到变更通知")
	}
}

func TestStorage_ClearCache(t *testing.T) {
	storage := skipIfNoConsul(t)
	if storage == nil {
		return
	}
	defer storage.Close()

	ctx := context.Background()
	key := "test-clear-cache"
	value := "cached-value"

	// 清理测试数据
	defer storage.Delete(ctx, key)

	// 写入
	err := storage.Write(ctx, key, value)
	if err != nil {
		t.Fatalf("Write() 失败: %v", err)
	}

	// 读取建立缓存
	var got1 string
	err = storage.ReadWithCache(ctx, key, &got1, config.WithCacheTTL(time.Hour))
	if err != nil {
		t.Fatalf("ReadWithCache() 失败: %v", err)
	}

	// 清除缓存
	err = storage.ClearCache(key)
	if err != nil {
		t.Fatalf("ClearCache() 失败: %v", err)
	}

	// 清除所有缓存
	err = storage.ClearCache("")
	if err != nil {
		t.Fatalf("ClearCache(all) 失败: %v", err)
	}

	t.Log("清除缓存测试通过")
}

func TestStorage_Close(t *testing.T) {
	storage := skipIfNoConsul(t)
	if storage == nil {
		return
	}

	ctx := context.Background()

	// 关闭存储
	err := storage.Close()
	if err != nil {
		t.Fatalf("Close() 失败: %v", err)
	}

	// 关闭后操作应该返回 ErrStorageClosed
	err = storage.Write(ctx, "key", "value")
	if !errors.Is(err, config.ErrStorageClosed) {
		t.Errorf("关闭后写入应该返回 ErrStorageClosed, got: %v", err)
	}

	var got string
	err = storage.Read(ctx, "key", &got)
	if !errors.Is(err, config.ErrStorageClosed) {
		t.Errorf("关闭后读取应该返回 ErrStorageClosed, got: %v", err)
	}

	t.Log("关闭测试通过")
}

func TestStorage_ErrorCases(t *testing.T) {
	storage := skipIfNoConsul(t)
	if storage == nil {
		return
	}
	defer storage.Close()

	ctx := context.Background()

	tests := []struct {
		name    string
		op      func() error
		wantErr error
	}{
		{
			name: "读取空键",
			op: func() error {
				var v interface{}
				return storage.Read(ctx, "", &v)
			},
			wantErr: config.ErrInvalidConfigKey,
		},
		{
			name: "写入空键",
			op: func() error {
				return storage.Write(ctx, "", "value")
			},
			wantErr: config.ErrInvalidConfigKey,
		},
		{
			name: "写入 nil 值",
			op: func() error {
				return storage.Write(ctx, "key", nil)
			},
			wantErr: config.ErrInvalidConfigValue,
		},
		{
			name: "监听空键",
			op: func() error {
				return storage.Watch(ctx, "", func(ctx context.Context, key string, value interface{}) error {
					return nil
				})
			},
			wantErr: config.ErrInvalidConfigKey,
		},
		{
			name: "监听空处理器",
			op: func() error {
				return storage.Watch(ctx, "key", nil)
			},
			wantErr: config.ErrInvalidHandler,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.op()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("操作错误 = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// 基准测试

func BenchmarkStorage_Read(b *testing.B) {
	storage, err := NewStorage()
	if err != nil {
		b.Skipf("跳过基准测试：无法连接到 Consul: %v", err)
		return
	}
	defer storage.Close()

	ctx := context.Background()
	key := "benchmark-read-key"
	value := "benchmark-value"

	storage.Write(ctx, key, value)
	defer storage.Delete(ctx, key)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var got string
		storage.Read(ctx, key, &got)
	}
}

func BenchmarkStorage_ReadWithCache(b *testing.B) {
	storage, err := NewStorage()
	if err != nil {
		b.Skipf("跳过基准测试：无法连接到 Consul: %v", err)
		return
	}
	defer storage.Close()

	ctx := context.Background()
	key := "benchmark-cache-key"
	value := "benchmark-value"

	storage.Write(ctx, key, value)
	defer storage.Delete(ctx, key)

	// 预热缓存
	var got string
	storage.ReadWithCache(ctx, key, &got, config.WithCacheTTL(time.Hour))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		storage.ReadWithCache(ctx, key, &got, config.WithCacheTTL(time.Hour))
	}
}

func BenchmarkStorage_Write(b *testing.B) {
	storage, err := NewStorage()
	if err != nil {
		b.Skipf("跳过基准测试：无法连接到 Consul: %v", err)
		return
	}
	defer storage.Close()

	ctx := context.Background()
	key := "benchmark-write-key"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		storage.Write(ctx, key, "value")
	}
	defer storage.Delete(ctx, key)
}
