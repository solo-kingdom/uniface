// Package config provides configuration storage interface.
// This package defines common error types and error handling utilities for configuration storage implementations.
//
// 基于 prompts/features/storage/config/00-iface.md 实现
package config

import (
	"errors"
	"fmt"
)

var (
	// ErrConfigNotFound is returned when a requested configuration does not exist in the storage.
	ErrConfigNotFound = errors.New("配置不存在")

	// ErrConfigAlreadyExists is returned when attempting to write a config that already exists in exclusive mode.
	ErrConfigAlreadyExists = errors.New("配置已存在")

	// ErrInvalidConfigKey is returned when the provided config key is invalid (e.g., empty or malformed).
	ErrInvalidConfigKey = errors.New("无效的配置键")

	// ErrInvalidKey is an alias for ErrInvalidConfigKey for backward compatibility.
	ErrInvalidKey = ErrInvalidConfigKey

	// ErrInvalidConfigValue is returned when the provided config value is invalid or malformed.
	ErrInvalidConfigValue = errors.New("无效的配置值")

	// ErrInvalidValue is an alias for ErrInvalidConfigValue for backward compatibility.
	ErrInvalidValue = ErrInvalidConfigValue

	// ErrConfigFormat is returned when the config format is invalid or cannot be parsed.
	ErrConfigFormat = errors.New("配置格式错误")

	// ErrVersionConflict is returned when there's a version conflict during write operation.
	ErrVersionConflict = errors.New("版本冲突")

	// ErrWatchFailed is returned when watch operation fails.
	ErrWatchFailed = errors.New("监听失败")

	// ErrInvalidHandler is returned when an invalid handler is provided for watch operation.
	ErrInvalidHandler = errors.New("无效的处理器")

	// ErrStorageClosed is returned when attempting to operate on a closed storage.
	ErrStorageClosed = errors.New("存储已关闭")

	// ErrCacheExpired is returned when cached config has expired.
	ErrCacheExpired = errors.New("缓存已过期")

	// ErrOperationFailed is returned when a general storage operation fails.
	ErrOperationFailed = errors.New("操作失败")
)

// ConfigError represents a configuration storage-related error with additional context.
// It wraps the underlying error and provides information about the operation, key, and version involved.
type ConfigError struct {
	Op      string // Operation that failed (e.g., "Read", "Write", "Watch")
	Key     string // Config key involved in the operation
	Version int64  // Config version (0 if not applicable)
	Err     error  // Underlying error
}

// Error returns a formatted error message including operation, key, version, and underlying error.
func (e *ConfigError) Error() string {
	if e.Version > 0 {
		return fmt.Sprintf("config %s key=%q version=%d: %v", e.Op, e.Key, e.Version, e.Err)
	}
	if e.Key != "" {
		return fmt.Sprintf("config %s key=%q: %v", e.Op, e.Key, e.Err)
	}
	return fmt.Sprintf("config %s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error for use with errors.Is and errors.As.
func (e *ConfigError) Unwrap() error {
	return e.Err
}

// NewConfigError creates a new ConfigError with the given operation, key, version, and underlying error.
func NewConfigError(op, key string, version int64, err error) error {
	return &ConfigError{
		Op:      op,
		Key:     key,
		Version: version,
		Err:     err,
	}
}
