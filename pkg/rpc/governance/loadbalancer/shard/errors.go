// Package shard provides simple sharding management based on load balancers.
// This file defines error types for shard management.
//
// 基于 prompts/features/rpc/governance/load-balancer/shard/00-shard-manager.md 实现
package shard

import "errors"

var (
	// ErrNoInstances is returned when there are no instances available.
	ErrNoInstances = errors.New("no instances available")

	// ErrManagerClosed is returned when attempting to operate on a closed shard manager.
	ErrManagerClosed = errors.New("shard manager closed")

	// ErrInvalidKey is returned when the shard key is invalid or empty.
	ErrInvalidKey = errors.New("invalid shard key")

	// ErrNoFactory is returned when no client factory is provided.
	ErrNoFactory = errors.New("client factory not provided")
)
