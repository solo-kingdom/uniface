// Package aerospike provides a Aerospike-based sharded storage implementation.
// This file implements the kv.Storage interface adapter.
//
// 基于 prompts/features/rpc/governance/load-balancer/shard/aerospike/00-aerospike-shared-client.md 实现
package aerospike

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	as "github.com/aerospike/aerospike-client-go/v7"
	"github.com/solo-kingdom/uniface/pkg/storage/kv"
)

// ErrBatchNotSupported 批量操作不支持错误
var ErrBatchNotSupported = errors.New("batch operations are not supported by Aerospike client")

// StorageConfig 存储 Storage 的配置
type StorageConfig struct {
	*Config // 继承基础配置

	// BinName 存储序列化数据的 bin 名称
	// 默认: "data"
	BinName string

	// SerializeFunc 自定义序列化函数
	// 默认: JSON 序列化
	SerializeFunc func(interface{}) ([]byte, error)

	// DeserializeFunc 自定义反序列化函数
	// 默认: JSON 反序列化
	DeserializeFunc func([]byte, interface{}) error

	// KeyPrefix 全局 key 前缀
	KeyPrefix string
}

// StorageOption 配置选项
type StorageOption func(*StorageConfig)

// NewStorageConfig 创建默认配置
func NewStorageConfig(opts ...StorageOption) *StorageConfig {
	config := &StorageConfig{
		Config:          NewConfig(),
		BinName:         "data",
		SerializeFunc:   json.Marshal,
		DeserializeFunc: json.Unmarshal,
		KeyPrefix:       "",
	}

	for _, opt := range opts {
		opt(config)
	}

	return config
}

// WithBinName 设置 bin 名称
func WithBinName(name string) StorageOption {
	return func(c *StorageConfig) {
		c.BinName = name
	}
}

// WithSerializer 设置自定义序列化器
func WithSerializer(serialize func(interface{}) ([]byte, error), deserialize func([]byte, interface{}) error) StorageOption {
	return func(c *StorageConfig) {
		c.SerializeFunc = serialize
		c.DeserializeFunc = deserialize
	}
}

// WithStorageKeyPrefix 设置 key 前缀
func WithStorageKeyPrefix(prefix string) StorageOption {
	return func(c *StorageConfig) {
		c.KeyPrefix = prefix
	}
}

// Storage 实现 kv.Storage 接口
type Storage struct {
	client *ShardClient
	config *StorageConfig
	mu     sync.RWMutex
	closed bool
}

// NewStorage 创建 Storage 实例
//
// 示例:
//
//	storage, _ := aerospike.NewStorage([]*aerospike.Instance{
//	    {ID: "node-1", Host: "localhost", Port: 3000, Namespace: "test", Set: "users"},
//	})
func NewStorage(instances []*Instance, opts ...StorageOption) (*Storage, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("至少需要一个 Aerospike 实例")
	}

	config := NewStorageConfig(opts...)

	// 将 Config 转换为选项
	clientOpts := configToOptions(config.Config)

	client, err := NewShardClient(instances, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("创建 ShardClient 失败: %w", err)
	}

	return &Storage{
		client: client,
		config: config,
	}, nil
}

// configToOptions 将 Config 转换为 Option 切片
func configToOptions(c *Config) []Option {
	var opts []Option

	if c.ConnectTimeout != 0 {
		opts = append(opts, WithConnectTimeout(c.ConnectTimeout))
	}
	if c.ReadTimeout != 0 {
		opts = append(opts, WithReadTimeout(c.ReadTimeout))
	}
	if c.WriteTimeout != 0 {
		opts = append(opts, WithWriteTimeout(c.WriteTimeout))
	}
	if c.PoolSize != 0 {
		opts = append(opts, WithPoolSize(c.PoolSize))
	}
	if c.MinIdleConns != 0 {
		opts = append(opts, WithMinIdleConns(c.MinIdleConns))
	}
	if c.MaxIdleConns != 0 {
		opts = append(opts, WithMaxIdleConns(c.MaxIdleConns))
	}
	if c.IdleTimeout != 0 {
		opts = append(opts, WithIdleTimeout(c.IdleTimeout))
	}
	if c.MaxRetries != 0 {
		opts = append(opts, WithMaxRetries(c.MaxRetries))
	}
	if c.RetryDelay != 0 {
		opts = append(opts, WithRetryDelay(c.RetryDelay))
	}
	if c.User != "" {
		opts = append(opts, WithAuth(c.User, c.Password))
	}
	if c.EnableTLS {
		opts = append(opts, WithTLS(c.EnableTLS))
	}
	if c.KeyPrefix != "" {
		opts = append(opts, WithKeyPrefix(c.KeyPrefix))
	}

	return opts
}

// buildKey 构建 key（添加前缀和 namespace）
func (s *Storage) buildKey(key string, opts *kv.Options) string {
	var parts []string

	if s.config.KeyPrefix != "" {
		parts = append(parts, s.config.KeyPrefix)
	}

	if opts != nil && opts.Namespace != "" {
		parts = append(parts, opts.Namespace)
	}

	parts = append(parts, key)

	return strings.Join(parts, ":")
}

// Set stores a key-value pair.
// 实现 kv.Storage 接口
func (s *Storage) Set(ctx context.Context, key string, value interface{}, opts ...kv.Option) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return kv.NewStorageError("set", key, kv.ErrStorageClosed)
	}

	if key == "" {
		return kv.NewStorageError("set", key, kv.ErrInvalidKey)
	}

	options := kv.MergeOptions(opts...)
	finalKey := s.buildKey(key, options)

	// NoOverwrite 检查
	if options.NoOverwrite {
		exists, err := s.client.Exists(ctx, finalKey)
		if err != nil {
			return kv.NewStorageError("set", key, err)
		}
		if exists {
			return kv.NewStorageError("set", key, kv.ErrKeyAlreadyExists)
		}
	}

	// 序列化
	data, err := s.config.SerializeFunc(value)
	if err != nil {
		return kv.NewStorageError("set", key, err)
	}

	// 构建 bins
	bins := as.BinMap{
		s.config.BinName: data,
	}

	// 写入（支持 TTL）
	if options.TTL > 0 {
		err = s.client.PutWithTTL(ctx, finalKey, bins, uint32(options.TTL.Seconds()))
	} else {
		err = s.client.Put(ctx, finalKey, bins)
	}

	if err != nil {
		return kv.NewStorageError("set", key, err)
	}

	return nil
}

// Get retrieves the value associated with the given key.
// 实现 kv.Storage 接口
func (s *Storage) Get(ctx context.Context, key string, value interface{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return kv.NewStorageError("get", key, kv.ErrStorageClosed)
	}

	if key == "" {
		return kv.NewStorageError("get", key, kv.ErrInvalidKey)
	}

	// 只读取指定的 bin
	record, err := s.client.Get(ctx, key, s.config.BinName)
	if err != nil {
		if err.Error() == "记录不存在: "+key {
			return kv.NewStorageError("get", key, kv.ErrKeyNotFound)
		}
		return kv.NewStorageError("get", key, err)
	}

	// 提取 bin 数据
	data, ok := record.Bins[s.config.BinName]
	if !ok {
		return kv.NewStorageError("get", key, fmt.Errorf("bin %s not found", s.config.BinName))
	}

	dataBytes, ok := data.([]byte)
	if !ok {
		return kv.NewStorageError("get", key, fmt.Errorf("invalid bin data type: expected []byte, got %T", data))
	}

	// 反序列化
	if err := s.config.DeserializeFunc(dataBytes, value); err != nil {
		return kv.NewStorageError("get", key, err)
	}

	return nil
}

// Delete removes the key-value pair associated with the given key.
// 实现 kv.Storage 接口
func (s *Storage) Delete(ctx context.Context, key string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return kv.NewStorageError("delete", key, kv.ErrStorageClosed)
	}

	if key == "" {
		return kv.NewStorageError("delete", key, kv.ErrInvalidKey)
	}

	if err := s.client.Delete(ctx, key); err != nil {
		return kv.NewStorageError("delete", key, err)
	}

	return nil
}

// BatchSet stores multiple key-value pairs atomically.
// 注意：Aerospike 客户端不支持批量操作，此方法始终返回 ErrBatchNotSupported
func (s *Storage) BatchSet(ctx context.Context, items map[string]interface{}, opts ...kv.Option) error {
	return kv.NewStorageError("batch_set", "", ErrBatchNotSupported)
}

// BatchGet retrieves values for multiple keys.
// 注意：Aerospike 客户端不支持批量操作，此方法始终返回 ErrBatchNotSupported
func (s *Storage) BatchGet(ctx context.Context, keys []string) (map[string]interface{}, error) {
	return nil, kv.NewStorageError("batch_get", "", ErrBatchNotSupported)
}

// BatchDelete removes multiple key-value pairs atomically.
// 注意：Aerospike 客户端不支持批量操作，此方法始终返回 ErrBatchNotSupported
func (s *Storage) BatchDelete(ctx context.Context, keys []string) error {
	return kv.NewStorageError("batch_delete", "", ErrBatchNotSupported)
}

// Exists checks if a key exists in the storage.
// 实现 kv.Storage 接口
func (s *Storage) Exists(ctx context.Context, key string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return false, kv.NewStorageError("exists", key, kv.ErrStorageClosed)
	}

	if key == "" {
		return false, kv.NewStorageError("exists", key, kv.ErrInvalidKey)
	}

	exists, err := s.client.Exists(ctx, key)
	if err != nil {
		return false, kv.NewStorageError("exists", key, err)
	}

	return exists, nil
}

// Close closes the storage and releases any held resources.
// 实现 kv.Storage 接口
func (s *Storage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true

	if s.client != nil {
		return s.client.Close()
	}

	return nil
}
