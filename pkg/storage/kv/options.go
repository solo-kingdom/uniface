// Package kv provides a universal key-value storage interface.
// This file contains configuration options for KV operations.
//
// 基于 prompts/features/kv-storage/00-iface.md 实现
package kv

import "time"

// Options represents the configuration options for KV operations.
type Options struct {
	// TTL specifies the time-to-live for stored keys.
	TTL time.Duration

	// Namespace is a prefix added to all keys in this operation.
	Namespace string

	// NoOverwrite prevents overwriting existing values.
	NoOverwrite bool

	// Readonly indicates this is a read-only operation.
	Readonly bool

	// Compress indicates if values should be compressed.
	Compress bool
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

// DefaultOptions returns the default options for KV operations.
func DefaultOptions() *Options {
	return &Options{
		TTL:         0,
		Namespace:   "",
		NoOverwrite: false,
		Readonly:    false,
		Compress:    false,
	}
}

// WithTTL sets the time-to-live for stored keys.
func WithTTL(ttl time.Duration) Option {
	return func(o *Options) {
		o.TTL = ttl
	}
}

// WithNamespace sets a namespace prefix for all keys.
func WithNamespace(ns string) Option {
	return func(o *Options) {
		o.Namespace = ns
	}
}

// WithNoOverwrite prevents overwriting existing values.
func WithNoOverwrite() Option {
	return func(o *Options) {
		o.NoOverwrite = true
	}
}

// WithReadonly marks the operation as read-only.
func WithReadonly() Option {
	return func(o *Options) {
		o.Readonly = true
	}
}

// WithCompress enables value compression.
func WithCompress() Option {
	return func(o *Options) {
		o.Compress = true
	}
}

// MergeOptions merges multiple Options into one.
// Later options override earlier ones.
func MergeOptions(opts ...Option) *Options {
	return DefaultOptions().Apply(opts...)
}
