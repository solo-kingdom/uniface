package boltdb

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/solo-kingdom/uniface/pkg/storage/kv"
	bolt "go.etcd.io/bbolt"
)

// defaultBucketName is used when no namespace is specified.
var defaultBucketName = []byte("_default")

// Storage implements kv.Storage interface using BoltDB.
// Namespace maps to BoltDB buckets: kv.WithNamespace("results") → "results" bucket.
type Storage struct {
	db     *bolt.DB
	config *Config
	mu     sync.RWMutex
	closed bool
}

// New creates a new BoltDB storage instance with the given options.
func New(opts ...Option) (*Storage, error) {
	config := NewConfig(opts...)

	dir := filepath.Dir(config.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, kv.NewStorageError("open", "", fmt.Errorf("create directory %s: %w", dir, err))
	}

	db, err := bolt.Open(config.Path, config.FileMode, &bolt.Options{Timeout: config.Timeout})
	if err != nil {
		return nil, kv.NewStorageError("open", "", err)
	}

	// Ensure default bucket exists
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(defaultBucketName)
		return err
	}); err != nil {
		db.Close()
		return nil, kv.NewStorageError("open", "", err)
	}

	return &Storage{
		db:     db,
		config: config,
	}, nil
}

// bucketName returns the BoltDB bucket name for the given options.
func (s *Storage) bucketName(opts *kv.Options) []byte {
	if opts != nil && opts.Namespace != "" {
		return []byte(opts.Namespace)
	}
	return defaultBucketName
}

// Set stores a key-value pair in the namespace's bucket.
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
	bucketName := s.bucketName(options)

	data, err := json.Marshal(value)
	if err != nil {
		return kv.NewStorageError("set", key, err)
	}

	err = s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketName)
		if err != nil {
			return err
		}
		if options.NoOverwrite {
			if existing := b.Get([]byte(key)); existing != nil {
				return kv.ErrKeyAlreadyExists
			}
		}
		return b.Put([]byte(key), data)
	})
	if err != nil {
		return kv.NewStorageError("set", key, err)
	}
	return nil
}

// Get retrieves a value by key from the namespace's bucket.
func (s *Storage) Get(ctx context.Context, key string, value interface{}, opts ...kv.Option) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return kv.NewStorageError("get", key, kv.ErrStorageClosed)
	}
	if key == "" {
		return kv.NewStorageError("get", key, kv.ErrInvalidKey)
	}

	options := kv.MergeOptions(opts...)
	bucketName := s.bucketName(options)

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucketName)
		}
		data := b.Get([]byte(key))
		if data == nil {
			return kv.ErrKeyNotFound
		}
		return json.Unmarshal(data, value)
	})
	if err != nil {
		return kv.NewStorageError("get", key, err)
	}
	return nil
}

// Delete removes a key from the namespace's bucket.
func (s *Storage) Delete(ctx context.Context, key string, opts ...kv.Option) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return kv.NewStorageError("delete", key, kv.ErrStorageClosed)
	}
	if key == "" {
		return kv.NewStorageError("delete", key, kv.ErrInvalidKey)
	}

	options := kv.MergeOptions(opts...)
	bucketName := s.bucketName(options)

	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		if b == nil {
			return nil // bucket doesn't exist, nothing to delete
		}
		return b.Delete([]byte(key))
	})
	if err != nil {
		return kv.NewStorageError("delete", key, err)
	}
	return nil
}

// BatchSet stores multiple key-value pairs atomically in one transaction.
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
	bucketName := s.bucketName(options)

	err := s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketName)
		if err != nil {
			return err
		}
		for key, value := range items {
			if key == "" {
				return kv.ErrInvalidKey
			}
			if options.NoOverwrite {
				if existing := b.Get([]byte(key)); existing != nil {
					return kv.NewStorageError("batch_set", key, kv.ErrKeyAlreadyExists)
				}
			}
			data, err := json.Marshal(value)
			if err != nil {
				return err
			}
			if err := b.Put([]byte(key), data); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return kv.NewStorageError("batch_set", "", err)
	}
	return nil
}

// BatchGet retrieves values for multiple keys from the namespace's bucket.
func (s *Storage) BatchGet(ctx context.Context, keys []string, opts ...kv.Option) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, kv.NewStorageError("batch_get", "", kv.ErrStorageClosed)
	}
	if len(keys) == 0 {
		return make(map[string]interface{}), nil
	}

	options := kv.MergeOptions(opts...)
	bucketName := s.bucketName(options)
	result := make(map[string]interface{})

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		if b == nil {
			return nil // empty bucket, return empty map
		}
		for _, key := range keys {
			data := b.Get([]byte(key))
			if data == nil {
				continue
			}
			var value interface{}
			if err := json.Unmarshal(data, &value); err != nil {
				result[key] = data
				continue
			}
			result[key] = value
		}
		return nil
	})
	if err != nil {
		return nil, kv.NewStorageError("batch_get", "", err)
	}
	return result, nil
}

// BatchDelete removes multiple keys atomically from the namespace's bucket.
func (s *Storage) BatchDelete(ctx context.Context, keys []string, opts ...kv.Option) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return kv.NewStorageError("batch_delete", "", kv.ErrStorageClosed)
	}
	if len(keys) == 0 {
		return nil
	}

	options := kv.MergeOptions(opts...)
	bucketName := s.bucketName(options)

	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		if b == nil {
			return nil
		}
		for _, key := range keys {
			if err := b.Delete([]byte(key)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return kv.NewStorageError("batch_delete", "", err)
	}
	return nil
}

// Exists checks if a key exists in the namespace's bucket.
func (s *Storage) Exists(ctx context.Context, key string, opts ...kv.Option) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return false, kv.NewStorageError("exists", key, kv.ErrStorageClosed)
	}
	if key == "" {
		return false, kv.NewStorageError("exists", key, kv.ErrInvalidKey)
	}

	options := kv.MergeOptions(opts...)
	bucketName := s.bucketName(options)
	exists := false

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		if b == nil {
			return nil
		}
		exists = b.Get([]byte(key)) != nil
		return nil
	})
	if err != nil {
		return false, kv.NewStorageError("exists", key, err)
	}
	return exists, nil
}

// List returns all keys in the namespace's bucket.
func (s *Storage) List(ctx context.Context, opts ...kv.Option) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, kv.NewStorageError("list", "", kv.ErrStorageClosed)
	}

	options := kv.MergeOptions(opts...)
	bucketName := s.bucketName(options)

	var keys []string
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		if b == nil {
			return nil // empty bucket
		}
		return b.ForEach(func(k, _ []byte) error {
			keys = append(keys, string(k))
			return nil
		})
	})
	if err != nil {
		return nil, kv.NewStorageError("list", "", err)
	}
	return keys, nil
}

// Close closes the storage and releases any held resources.
func (s *Storage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	if s.db != nil {
		if err := s.db.Close(); err != nil {
			return kv.NewStorageError("close", "", err)
		}
	}
	return nil
}

// DB returns the underlying BoltDB instance for advanced operations.
func (s *Storage) DB() *bolt.DB {
	return s.db
}
