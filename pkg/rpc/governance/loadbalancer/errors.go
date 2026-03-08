// Package loadbalancer provides a universal load balancer interface.
// This package defines common error types and error handling utilities for load balancer implementations.
//
// 基于 prompts/features/service/governance/load-balancer/01-load-balancer-iface.md 实现
package loadbalancer

import (
	"errors"
	"fmt"
)

var (
	// ErrNoInstances is returned when there are no available instances to select from.
	ErrNoInstances = errors.New("no instances available")

	// ErrInstanceNotFound is returned when a requested instance does not exist.
	ErrInstanceNotFound = errors.New("instance not found")

	// ErrInvalidInstance is returned when the provided instance is invalid.
	ErrInvalidInstance = errors.New("invalid instance")

	// ErrBalancerClosed is returned when attempting to operate on a closed load balancer.
	ErrBalancerClosed = errors.New("balancer closed")

	// ErrDuplicateInstance is returned when attempting to add an instance that already exists.
	ErrDuplicateInstance = errors.New("duplicate instance")

	// ErrNoClientFactory is returned when SelectClient is called without a client factory.
	ErrNoClientFactory = errors.New("client factory not provided")

	// ErrClientCreateFailed is returned when client creation fails.
	ErrClientCreateFailed = errors.New("failed to create client")
)

// BalancerError represents a load balancer error with additional context.
// It wraps the underlying error and provides information about the operation and instance involved.
type BalancerError struct {
	Op         string // Operation that failed (e.g., "Add", "Remove", "Select")
	InstanceID string // Instance ID involved in the operation (may be empty)
	Err        error  // Underlying error
}

// Error returns a formatted error message including operation, instance ID, and underlying error.
func (e *BalancerError) Error() string {
	if e.InstanceID == "" {
		return fmt.Sprintf("loadbalancer %s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("loadbalancer %s %q: %v", e.Op, e.InstanceID, e.Err)
}

// Unwrap returns the underlying error for use with errors.Is and errors.As.
func (e *BalancerError) Unwrap() error {
	return e.Err
}

// NewBalancerError creates a new BalancerError with the given operation, instance ID, and underlying error.
func NewBalancerError(op, instanceID string, err error) error {
	return &BalancerError{
		Op:         op,
		InstanceID: instanceID,
		Err:        err,
	}
}
