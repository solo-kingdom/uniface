// Package kv provides a universal key-value storage interface.
// This package defines common error types and error handling utilities for KV storage implementations.
//
// 基于 prompts/features/kv-storage/00-iface.md 实现
package kv

import (
	"errors"
	"fmt"
)

var (
	// ErrKeyNotFound is returned when a requested key does not exist in the storage.
	ErrKeyNotFound = errors.New("key not found")

	// ErrKeyAlreadyExists is returned when attempting to set a key that already exists in exclusive mode.
	ErrKeyAlreadyExists = errors.New("key already exists")

	// ErrInvalidKey is returned when the provided key is invalid (e.g., empty or malformed).
	ErrInvalidKey = errors.New("invalid key")

	// ErrInvalidValue is returned when the provided value is invalid.
	ErrInvalidValue = errors.New("invalid value")

	// ErrStorageClosed is returned when attempting to operate on a closed storage.
	ErrStorageClosed = errors.New("storage closed")

	// ErrOperationFailed is returned when a general storage operation fails.
	ErrOperationFailed = errors.New("operation failed")
)

// StorageError represents a storage-related error with additional context.
// It wraps the underlying error and provides information about the operation and key involved.
type StorageError struct {
	Op  string // Operation that failed (e.g., "Get", "Set", "Delete")
	Key string // Key involved in the operation (may be empty for batch operations)
	Err error  // Underlying error
}

// Error returns a formatted error message including operation, key, and underlying error.
func (e *StorageError) Error() string {
	if e.Key == "" {
		return fmt.Sprintf("kv %s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("kv %s %q: %v", e.Op, e.Key, e.Err)
}

// Unwrap returns the underlying error for use with errors.Is and errors.As.
func (e *StorageError) Unwrap() error {
	return e.Err
}

// NewStorageError creates a new StorageError with the given operation, key, and underlying error.
func NewStorageError(op, key string, err error) error {
	return &StorageError{
		Op:  op,
		Key: key,
		Err: err,
	}
}
