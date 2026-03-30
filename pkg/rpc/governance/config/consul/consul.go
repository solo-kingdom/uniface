// Package consul 提供基于 Consul 的配置存储实现。
// 实现了 config.Storage 接口，支持配置的读取、写入、监听等功能。
//
// 基于 specs/features/rpc/governance/config/01 consul.md 实现
package consul

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/solo-kingdom/uniface/pkg/rpc/governance/config"
)

// Storage 实现了基于 Consul 的配置存储。
type Storage struct {
	client   *api.Client
	kv       *api.KV
	opts     *Options
	cache    map[string]*cacheEntry
	cacheMu  sync.RWMutex
	watchers map[string][]watcherEntry
	watchMu  sync.RWMutex
	closed   bool
	closeMu  sync.RWMutex
}

type cacheEntry struct {
	value     []byte
	modifyIdx uint64
	expiresAt time.Time
}

type watcherEntry struct {
	handler config.Handler
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewStorage 创建一个新的 Consul 配置存储实例。
//
// 参数:
//   - opts: 可选配置项
//
// 返回:
//   - *Storage: Consul 配置存储实例
//   - error: 如果创建失败返回错误
func NewStorage(opts ...Option) (*Storage, error) {
	options := DefaultOptions().Apply(opts...)

	// 创建 Consul 客户端配置
	consulConfig := api.DefaultConfig()
	consulConfig.Address = options.Address
	consulConfig.Scheme = options.Scheme

	if options.Token != "" {
		consulConfig.Token = options.Token
	}
	if options.Namespace != "" {
		consulConfig.Namespace = options.Namespace
	}
	if options.Datacenter != "" {
		consulConfig.Datacenter = options.Datacenter
	}
	if options.TokenFile != "" {
		consulConfig.TokenFile = options.TokenFile
	}
	if options.TLSConfig != nil {
		consulConfig.TLSConfig.Address = options.TLSConfig.ServerName
		consulConfig.TLSConfig.CertFile = ""
		consulConfig.TLSConfig.KeyFile = ""
		consulConfig.TLSConfig.CAFile = ""
		consulConfig.TLSConfig.CAPath = ""
		consulConfig.TLSConfig.InsecureSkipVerify = options.TLSConfig.InsecureSkipVerify
	}
	if options.HttpClient != nil {
		consulConfig.HttpClient = options.HttpClient
	}
	if options.HttpAuth != nil {
		consulConfig.HttpAuth = &api.HttpBasicAuth{
			Username: options.HttpAuth.Username,
			Password: options.HttpAuth.Password,
		}
	}
	consulConfig.WaitTime = options.WaitTime
	// DisableKeepAlives 在 consul api v1.27.0 中不支持

	// 创建 Consul 客户端
	client, err := api.NewClient(consulConfig)
	if err != nil {
		return nil, config.NewConfigError("new_client", "", 0,
			fmt.Errorf("创建 Consul 客户端失败: %w", err))
	}

	return &Storage{
		client:   client,
		kv:       client.KV(),
		opts:     options,
		cache:    make(map[string]*cacheEntry),
		watchers: make(map[string][]watcherEntry),
	}, nil
}

// buildKey 构建完整的配置键（添加前缀）。
func (s *Storage) buildKey(key string) string {
	if s.opts.KeyPrefix == "" {
		return key
	}
	return s.opts.KeyPrefix + key
}

// Read 直接从 Consul 读取配置，不经过缓存。
func (s *Storage) Read(ctx context.Context, key string, value interface{}, opts ...config.Option) error {
	if err := s.checkClosed(); err != nil {
		return err
	}
	if key == "" {
		return config.ErrInvalidConfigKey
	}

	options := config.DefaultOptions().Apply(opts...)
	fullKey := s.buildKey(key)
	if options.Namespace != "" {
		fullKey = options.Namespace + key
	}

	pair, _, err := s.kv.Get(fullKey, nil)
	if err != nil {
		return config.NewConfigError("read", key, 0,
			fmt.Errorf("从 Consul 读取失败: %w", err))
	}

	if pair == nil {
		return config.NewConfigError("read", key, 0, config.ErrConfigNotFound)
	}

	// 解码值
	if err := s.decodeValue(pair.Value, value); err != nil {
		return config.NewConfigError("read", key, 0,
			fmt.Errorf("解码配置值失败: %w", err))
	}

	return nil
}

// ReadWithCache 带缓存的读取配置。
func (s *Storage) ReadWithCache(ctx context.Context, key string, value interface{}, opts ...config.Option) error {
	if err := s.checkClosed(); err != nil {
		return err
	}
	if key == "" {
		return config.ErrInvalidConfigKey
	}

	options := config.DefaultOptions().Apply(opts...)
	fullKey := s.buildKey(key)
	if options.Namespace != "" {
		fullKey = options.Namespace + key
	}

	// 检查缓存
	s.cacheMu.RLock()
	if entry, exists := s.cache[fullKey]; exists {
		if time.Now().Before(entry.expiresAt) && options.CacheEnabled {
			s.cacheMu.RUnlock()
			// 缓存未过期，使用缓存值
			if err := s.decodeValue(entry.value, value); err != nil {
				return config.NewConfigError("read_with_cache", key, 0,
					fmt.Errorf("解码缓存值失败: %w", err))
			}
			return nil
		}
	}
	s.cacheMu.RUnlock()

	// 从 Consul 读取
	pair, _, err := s.kv.Get(fullKey, nil)
	if err != nil {
		return config.NewConfigError("read_with_cache", key, 0,
			fmt.Errorf("从 Consul 读取失败: %w", err))
	}

	if pair == nil {
		return config.NewConfigError("read_with_cache", key, 0, config.ErrConfigNotFound)
	}

	// 更新缓存
	if options.CacheEnabled {
		s.cacheMu.Lock()
		s.cache[fullKey] = &cacheEntry{
			value:     pair.Value,
			modifyIdx: pair.ModifyIndex,
			expiresAt: time.Now().Add(options.CacheTTL),
		}
		s.cacheMu.Unlock()
	}

	// 解码值
	if err := s.decodeValue(pair.Value, value); err != nil {
		return config.NewConfigError("read_with_cache", key, 0,
			fmt.Errorf("解码配置值失败: %w", err))
	}

	return nil
}

// Write 写入配置到 Consul。
func (s *Storage) Write(ctx context.Context, key string, value interface{}, opts ...config.Option) error {
	if err := s.checkClosed(); err != nil {
		return err
	}
	if key == "" {
		return config.ErrInvalidConfigKey
	}
	if value == nil {
		return config.ErrInvalidConfigValue
	}

	options := config.DefaultOptions().Apply(opts...)
	fullKey := s.buildKey(key)
	if options.Namespace != "" {
		fullKey = options.Namespace + key
	}

	// 编码值
	data, err := s.encodeValue(value)
	if err != nil {
		return config.NewConfigError("write", key, 0,
			fmt.Errorf("编码配置值失败: %w", err))
	}

	// 检查是否不允许覆盖
	if options.NoOverwrite {
		existing, _, err := s.kv.Get(fullKey, nil)
		if err != nil {
			return config.NewConfigError("write", key, 0,
				fmt.Errorf("检查配置是否存在失败: %w", err))
		}
		if existing != nil {
			return config.NewConfigError("write", key, 0, config.ErrConfigAlreadyExists)
		}
	}

	// 写入 Consul
	pair := &api.KVPair{
		Key:   fullKey,
		Value: data,
	}

	var writeErr error

	for i := 0; i < options.RetryCount; i++ {
		_, writeErr = s.kv.Put(pair, nil)
		if writeErr == nil {
			break
		}
		if i < options.RetryCount-1 {
			time.Sleep(options.RetryDelay)
		}
	}

	if writeErr != nil {
		return config.NewConfigError("write", key, 0,
			fmt.Errorf("写入 Consul 失败: %w", writeErr))
	}

	// 清除缓存
	s.cacheMu.Lock()
	delete(s.cache, fullKey)
	s.cacheMu.Unlock()

	// 通知监听器
	s.notifyWatchers(key, value)

	return nil
}

// Delete 删除配置。
func (s *Storage) Delete(ctx context.Context, key string) error {
	if err := s.checkClosed(); err != nil {
		return err
	}
	if key == "" {
		return config.ErrInvalidConfigKey
	}

	fullKey := s.buildKey(key)

	_, err := s.kv.Delete(fullKey, nil)
	if err != nil {
		return config.NewConfigError("delete", key, 0,
			fmt.Errorf("从 Consul 删除失败: %w", err))
	}

	// 清除缓存
	s.cacheMu.Lock()
	delete(s.cache, fullKey)
	s.cacheMu.Unlock()

	// 通知监听器（value 为 nil 表示删除）
	s.notifyWatchers(key, nil)

	return nil
}

// Watch 监听指定配置键的变更。
func (s *Storage) Watch(ctx context.Context, key string, handler config.Handler, opts ...config.Option) error {
	if err := s.checkClosed(); err != nil {
		return err
	}
	if key == "" {
		return config.ErrInvalidConfigKey
	}
	if handler == nil {
		return config.ErrInvalidHandler
	}

	watchCtx, cancel := context.WithCancel(ctx)

	s.watchMu.Lock()
	entry := watcherEntry{
		handler: handler,
		ctx:     watchCtx,
		cancel:  cancel,
	}
	s.watchers[key] = append(s.watchers[key], entry)
	s.watchMu.Unlock()

	// 启动监听协程
	go s.watchKey(watchCtx, key, handler, opts...)

	return nil
}

// watchKey 监听单个键的变更。
func (s *Storage) watchKey(ctx context.Context, key string, handler config.Handler, opts ...config.Option) {
	options := config.DefaultOptions().Apply(opts...)
	fullKey := s.buildKey(key)
	if options.Namespace != "" {
		fullKey = options.Namespace + key
	}

	var lastIndex uint64

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 阻塞查询等待变更
		pair, meta, err := s.kv.Get(fullKey, &api.QueryOptions{
			WaitIndex: lastIndex,
			WaitTime:  s.opts.WaitTime,
		})

		if err != nil {
			// 出错时等待一段时间后重试
			time.Sleep(time.Second)
			continue
		}

		// 检查是否有变更
		if meta.LastIndex == lastIndex {
			continue
		}
		lastIndex = meta.LastIndex

		// 触发处理器
		if pair == nil {
			// 配置被删除
			go func() {
				_ = handler(ctx, key, nil)
			}()
		} else {
			// 配置变更
			var value interface{}
			if err := s.decodeValue(pair.Value, &value); err == nil {
				go func() {
					_ = handler(ctx, key, value)
				}()
			}
		}
	}
}

// Unwatch 取消对指定配置键的监听。
func (s *Storage) Unwatch(key string) error {
	if err := s.checkClosed(); err != nil {
		return err
	}

	s.watchMu.Lock()
	defer s.watchMu.Unlock()

	if watchers, exists := s.watchers[key]; exists {
		for _, w := range watchers {
			w.cancel()
		}
		delete(s.watchers, key)
	}

	return nil
}

// WatchPrefix 监听指定前缀的所有配置键的变更。
func (s *Storage) WatchPrefix(ctx context.Context, prefix string, handler config.Handler, opts ...config.Option) error {
	if err := s.checkClosed(); err != nil {
		return err
	}
	if prefix == "" {
		return config.ErrInvalidConfigKey
	}
	if handler == nil {
		return config.ErrInvalidHandler
	}

	watchCtx, cancel := context.WithCancel(ctx)

	s.watchMu.Lock()
	key := "prefix:" + prefix
	entry := watcherEntry{
		handler: handler,
		ctx:     watchCtx,
		cancel:  cancel,
	}
	s.watchers[key] = append(s.watchers[key], entry)
	s.watchMu.Unlock()

	// 启动前缀监听协程
	go s.watchPrefixKeys(watchCtx, prefix, handler, opts...)

	return nil
}

// watchPrefixKeys 监听前缀下所有键的变更。
func (s *Storage) watchPrefixKeys(ctx context.Context, prefix string, handler config.Handler, opts ...config.Option) {
	options := config.DefaultOptions().Apply(opts...)
	fullPrefix := s.buildKey(prefix)
	if options.Namespace != "" {
		fullPrefix = options.Namespace + prefix
	}

	var lastIndex uint64

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 阻塞查询等待变更
		pairs, meta, err := s.kv.List(fullPrefix, &api.QueryOptions{
			WaitIndex: lastIndex,
			WaitTime:  s.opts.WaitTime,
		})

		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		// 检查是否有变更
		if meta.LastIndex == lastIndex {
			continue
		}
		lastIndex = meta.LastIndex

		// 触发处理器
		for _, pair := range pairs {
			var value interface{}
			if err := s.decodeValue(pair.Value, &value); err == nil {
				// 移除前缀后的键名
				shortKey := pair.Key
				if s.opts.KeyPrefix != "" {
					shortKey = pair.Key[len(s.opts.KeyPrefix):]
				}
				go func(k string, v interface{}) {
					_ = handler(ctx, k, v)
				}(shortKey, value)
			}
		}
	}
}

// UnwatchPrefix 取消对指定前缀的监听。
func (s *Storage) UnwatchPrefix(prefix string) error {
	if err := s.checkClosed(); err != nil {
		return err
	}

	s.watchMu.Lock()
	defer s.watchMu.Unlock()

	key := "prefix:" + prefix
	if watchers, exists := s.watchers[key]; exists {
		for _, w := range watchers {
			w.cancel()
		}
		delete(s.watchers, key)
	}

	return nil
}

// List 列出所有匹配前缀的配置键。
func (s *Storage) List(ctx context.Context, prefix string) ([]string, error) {
	if err := s.checkClosed(); err != nil {
		return nil, err
	}

	fullPrefix := s.buildKey(prefix)

	pairs, _, err := s.kv.List(fullPrefix, nil)
	if err != nil {
		return nil, config.NewConfigError("list", prefix, 0,
			fmt.Errorf("列出配置键失败: %w", err))
	}

	keys := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		// 移除前缀
		shortKey := pair.Key
		if s.opts.KeyPrefix != "" && len(pair.Key) > len(s.opts.KeyPrefix) {
			shortKey = pair.Key[len(s.opts.KeyPrefix):]
		}
		keys = append(keys, shortKey)
	}

	return keys, nil
}

// ClearCache 清除指定配置键的缓存。
func (s *Storage) ClearCache(key string) error {
	if err := s.checkClosed(); err != nil {
		return err
	}

	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	if key == "" {
		// 清除所有缓存
		s.cache = make(map[string]*cacheEntry)
	} else {
		fullKey := s.buildKey(key)
		delete(s.cache, fullKey)
	}

	return nil
}

// Close 关闭配置存储，释放所有资源。
func (s *Storage) Close() error {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true

	// 取消所有监听器
	s.watchMu.Lock()
	for _, watchers := range s.watchers {
		for _, w := range watchers {
			w.cancel()
		}
	}
	s.watchers = nil
	s.watchMu.Unlock()

	// 清除缓存
	s.cacheMu.Lock()
	s.cache = nil
	s.cacheMu.Unlock()

	return nil
}

// checkClosed 检查存储是否已关闭。
func (s *Storage) checkClosed() error {
	s.closeMu.RLock()
	defer s.closeMu.RUnlock()

	if s.closed {
		return config.ErrStorageClosed
	}
	return nil
}

// notifyWatchers 通知监听器。
func (s *Storage) notifyWatchers(key string, value interface{}) {
	s.watchMu.RLock()
	defer s.watchMu.RUnlock()

	if watchers, exists := s.watchers[key]; exists {
		for _, w := range watchers {
			go func(handler config.Handler, ctx context.Context) {
				_ = handler(ctx, key, value)
			}(w.handler, w.ctx)
		}
	}
}

// decodeValue 解码配置值。
func (s *Storage) decodeValue(data []byte, value interface{}) error {
	// 尝试 JSON 解码
	if err := json.Unmarshal(data, value); err != nil {
		// 如果 JSON 解码失败，尝试作为字符串处理
		switch v := value.(type) {
		case *interface{}:
			*v = string(data)
		case *string:
			*v = string(data)
		default:
			return fmt.Errorf("无法解码配置值: %w", err)
		}
	}
	return nil
}

// encodeValue 编码配置值。
func (s *Storage) encodeValue(value interface{}) ([]byte, error) {
	switch v := value.(type) {
	case string:
		return []byte(v), nil
	case []byte:
		return v, nil
	default:
		return json.Marshal(v)
	}
}
