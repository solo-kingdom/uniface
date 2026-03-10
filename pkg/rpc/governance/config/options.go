// Package config provides a configuration storage interface.
// This file contains configuration options for config storage operations.
//
// 基于 prompts/features/storage/config/00-iface.md 实现
package config

import "time"

// Options represents the configuration options for config storage operations.
type Options struct {
	// CacheTTL specifies the time-to-live for cached config values.
	CacheTTL time.Duration

	// CacheEnabled enables or disables caching.
	CacheEnabled bool

	// ForceRefresh forces reading from the source, bypassing cache.
	ForceRefresh bool

	// WatchOnChange enables watching for changes after initial read.
	WatchOnChange bool

	// Namespace is a prefix added to all config keys in this operation.
	Namespace string

	// RetryCount specifies the number of retry attempts for read/write operations.
	RetryCount int

	// RetryDelay specifies the delay between retry attempts.
	RetryDelay time.Duration

	// NoOverwrite prevents overwriting existing config values.
	NoOverwrite bool
}

// Option is a function that modifies Options.
type Option func(*Options)

// Apply applies the given options to the current Options.
func (o *Options) Apply(opts ...Option) *Options {
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// DefaultOptions returns the default options for config storage operations.
func DefaultOptions() *Options {
	return &Options{
		CacheTTL:      5 * time.Minute,
		CacheEnabled:  true,
		ForceRefresh:  false,
		WatchOnChange: false,
		Namespace:     "",
		RetryCount:    3,
		RetryDelay:    100 * time.Millisecond,
		NoOverwrite:   false,
	}
}

// WithCacheTTL sets the time-to-live for cached config values.
func WithCacheTTL(ttl time.Duration) Option {
	return func(o *Options) {
		o.CacheTTL = ttl
	}
}

// WithCacheEnabled enables or disables caching.
func WithCacheEnabled(enabled bool) Option {
	return func(o *Options) {
		o.CacheEnabled = enabled
	}
}

// WithForceRefresh forces reading from the source, bypassing cache.
func WithForceRefresh() Option {
	return func(o *Options) {
		o.ForceRefresh = true
	}
}

// WithWatchOnChange enables watching for changes after initial read.
func WithWatchOnChange() Option {
	return func(o *Options) {
		o.WatchOnChange = true
	}
}

// WithNamespace sets a namespace prefix for all config keys.
func WithNamespace(ns string) Option {
	return func(o *Options) {
		o.Namespace = ns
	}
}

// WithRetryCount specifies the number of retry attempts.
func WithRetryCount(count int) Option {
	return func(o *Options) {
		if count > 0 {
			o.RetryCount = count
		}
	}
}

// WithRetryDelay specifies the delay between retry attempts.
func WithRetryDelay(delay time.Duration) Option {
	return func(o *Options) {
		o.RetryDelay = delay
	}
}

// WithNoOverwrite prevents overwriting existing config values.
func WithNoOverwrite() Option {
	return func(o *Options) {
		o.NoOverwrite = true
	}
}

// DisableCache disables caching for the operation.
func DisableCache() Option {
	return WithCacheEnabled(false)
}

// EnableCache enables caching for the operation.
func EnableCache() Option {
	return WithCacheEnabled(true)
}
