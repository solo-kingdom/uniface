// Package redis provides a Redis-based implementation of the KV storage interface.
// This package implements github.com/wii/uniface/pkg/storage/kv.Storage interface.
package redis

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/wii/uniface/pkg/storage/kv"
)

// Storage implements kv.Storage interface using Redis.
type Storage struct {
	client    *redis.Client
	config    *Config
	keyPrefix string
	mu        sync.RWMutex
	closed    bool
}

// New creates a new Redis storage instance with the given options.
func New(opts ...Option) (*Storage, error) {
	config := NewConfig(opts...)

	client := redis.NewClient(&redis.Options{
		Addr:         config.Addr,
		Password:     config.Password,
		DB:           config.DB,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
		MaxRetries:   config.MaxRetries,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		PoolTimeout:  config.PoolTimeout,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, kv.NewStorageError("connect", "", err)
	}

	return &Storage{
		client:    client,
		config:    config,
		keyPrefix: config.KeyPrefix,
	}, nil
}

// NewWithClient creates a new Redis storage instance with an existing client.
// This is useful when you want to share a Redis connection across multiple storage instances.
func NewWithClient(client *redis.Client, opts ...Option) (*Storage, error) {
	config := NewConfig(opts...)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, kv.NewStorageError("connect", "", err)
	}

	return &Storage{
		client:    client,
		config:    config,
		keyPrefix: config.KeyPrefix,
	}, nil
}

// buildKey constructs the final key with prefix and namespace.
func (s *Storage) buildKey(key string, opts *kv.Options) string {
	var sb strings.Builder

	// Add global prefix first
	if s.keyPrefix != "" {
		sb.WriteString(s.keyPrefix)
		sb.WriteString(":")
	}

	// Add namespace from options
	if opts != nil && opts.Namespace != "" {
		sb.WriteString(opts.Namespace)
		sb.WriteString(":")
	}

	sb.WriteString(key)
	return sb.String()
}

// Set stores a key-value pair.
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

	// Serialize value to JSON
	data, err := json.Marshal(value)
	if err != nil {
		return kv.NewStorageError("set", key, err)
	}

	// Check NoOverwrite option
	if options.NoOverwrite {
		exists, err := s.client.Exists(ctx, finalKey).Result()
		if err != nil {
			return kv.NewStorageError("set", key, err)
		}
		if exists > 0 {
			return kv.NewStorageError("set", key, kv.ErrKeyAlreadyExists)
		}
	}

	// Set with optional TTL
	if options.TTL > 0 {
		err = s.client.Set(ctx, finalKey, data, options.TTL).Err()
	} else {
		err = s.client.Set(ctx, finalKey, data, 0).Err()
	}

	if err != nil {
		return kv.NewStorageError("set", key, err)
	}

	return nil
}

// Get retrieves the value associated with the given key.
func (s *Storage) Get(ctx context.Context, key string, value interface{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return kv.NewStorageError("get", key, kv.ErrStorageClosed)
	}

	if key == "" {
		return kv.NewStorageError("get", key, kv.ErrInvalidKey)
	}

	finalKey := s.buildKey(key, nil)

	data, err := s.client.Get(ctx, finalKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return kv.NewStorageError("get", key, kv.ErrKeyNotFound)
		}
		return kv.NewStorageError("get", key, err)
	}

	if err := json.Unmarshal(data, value); err != nil {
		return kv.NewStorageError("get", key, err)
	}

	return nil
}

// Delete removes the key-value pair associated with the given key.
func (s *Storage) Delete(ctx context.Context, key string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return kv.NewStorageError("delete", key, kv.ErrStorageClosed)
	}

	if key == "" {
		return kv.NewStorageError("delete", key, kv.ErrInvalidKey)
	}

	finalKey := s.buildKey(key, nil)

	if err := s.client.Del(ctx, finalKey).Err(); err != nil {
		return kv.NewStorageError("delete", key, err)
	}

	return nil
}

// BatchSet stores multiple key-value pairs atomically using pipelining.
func (s *Storage) BatchSet(ctx context.Context, items map[string]interface{}, opts ...kv.Option) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return kv.NewStorageError("batch_set", "", kv.ErrStorageClosed)
	}

	if len(items) == 0 {
		return nil
	}

	options := kv.MergeOptions(opts...)

	// Use transaction for atomic batch set
	_, err := s.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		for key, value := range items {
			if key == "" {
				return kv.ErrInvalidKey
			}

			finalKey := s.buildKey(key, options)

			data, err := json.Marshal(value)
			if err != nil {
				return err
			}

			// Check NoOverwrite option
			if options.NoOverwrite {
				exists, err := s.client.Exists(ctx, finalKey).Result()
				if err != nil {
					return err
				}
				if exists > 0 {
					return kv.NewStorageError("batch_set", key, kv.ErrKeyAlreadyExists)
				}
			}

			if options.TTL > 0 {
				pipe.Set(ctx, finalKey, data, options.TTL)
			} else {
				pipe.Set(ctx, finalKey, data, 0)
			}
		}
		return nil
	})

	if err != nil {
		return kv.NewStorageError("batch_set", "", err)
	}

	return nil
}

// BatchGet retrieves values for multiple keys.
func (s *Storage) BatchGet(ctx context.Context, keys []string) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, kv.NewStorageError("batch_get", "", kv.ErrStorageClosed)
	}

	if len(keys) == 0 {
		return make(map[string]interface{}), nil
	}

	// Build final keys
	finalKeys := make([]string, len(keys))
	keyMap := make(map[string]string) // maps final key to original key

	for i, key := range keys {
		if key == "" {
			return nil, kv.NewStorageError("batch_get", key, kv.ErrInvalidKey)
		}
		finalKey := s.buildKey(key, nil)
		finalKeys[i] = finalKey
		keyMap[finalKey] = key
	}

	// Use MGET for batch retrieval
	results, err := s.client.MGet(ctx, finalKeys...).Result()
	if err != nil {
		return nil, kv.NewStorageError("batch_get", "", err)
	}

	output := make(map[string]interface{})
	for i, result := range results {
		if result == nil {
			continue // Skip missing keys
		}

		var value interface{}
		switch v := result.(type) {
		case string:
			if err := json.Unmarshal([]byte(v), &value); err != nil {
				// If JSON unmarshal fails, store as raw string
				value = v
			}
		default:
			value = result
		}

		originalKey := keyMap[finalKeys[i]]
		output[originalKey] = value
	}

	return output, nil
}

// BatchDelete removes multiple key-value pairs atomically.
func (s *Storage) BatchDelete(ctx context.Context, keys []string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return kv.NewStorageError("batch_delete", "", kv.ErrStorageClosed)
	}

	if len(keys) == 0 {
		return nil
	}

	finalKeys := make([]string, len(keys))
	for i, key := range keys {
		if key == "" {
			return kv.NewStorageError("batch_delete", key, kv.ErrInvalidKey)
		}
		finalKeys[i] = s.buildKey(key, nil)
	}

	if err := s.client.Del(ctx, finalKeys...).Err(); err != nil {
		return kv.NewStorageError("batch_delete", "", err)
	}

	return nil
}

// Exists checks if a key exists in the storage.
func (s *Storage) Exists(ctx context.Context, key string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return false, kv.NewStorageError("exists", key, kv.ErrStorageClosed)
	}

	if key == "" {
		return false, kv.NewStorageError("exists", key, kv.ErrInvalidKey)
	}

	finalKey := s.buildKey(key, nil)

	count, err := s.client.Exists(ctx, finalKey).Result()
	if err != nil {
		return false, kv.NewStorageError("exists", key, err)
	}

	return count > 0, nil
}

// Close closes the storage and releases any held resources.
func (s *Storage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true

	if s.client != nil {
		if err := s.client.Close(); err != nil {
			return kv.NewStorageError("close", "", err)
		}
	}

	return nil
}

// Client returns the underlying Redis client for advanced operations.
// Use with caution as direct client usage may bypass the storage abstraction.
func (s *Storage) Client() *redis.Client {
	return s.client
}
