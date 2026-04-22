// Package queue provides a generic message queue interface for application decoupling.
// This file defines common error types and error handling utilities for queue implementations.
package queue

import (
	"errors"
	"fmt"
)

var (
	// ErrTopicRequired 是主题为空时返回的错误。
	ErrTopicRequired = errors.New("topic is required")

	// ErrMessageTooLarge 是消息超过 Broker 限制时返回的错误。
	ErrMessageTooLarge = errors.New("message too large")

	// ErrQueueClosed 是在已关闭的队列上操作时返回的错误。
	ErrQueueClosed = errors.New("queue closed")

	// ErrSubscriptionClosed 是在已关闭的订阅上操作时返回的错误。
	ErrSubscriptionClosed = errors.New("subscription closed")

	// ErrPublishFailed 是消息发布失败时返回的错误。
	ErrPublishFailed = errors.New("publish failed")

	// ErrSubscribeFailed 是订阅失败时返回的错误。
	ErrSubscribeFailed = errors.New("subscribe failed")

	// ErrInvalidMessage 是消息内容无效时返回的错误。
	ErrInvalidMessage = errors.New("invalid message")

	// ErrCodecFailed 是编解码失败时返回的错误。
	ErrCodecFailed = errors.New("codec failed")

	// ErrBrokerUnavailable 是 Broker 不可用时返回的错误。
	ErrBrokerUnavailable = errors.New("broker unavailable")

	// ErrSubscriptionPaused 是订阅已暂停时返回的错误。
	ErrSubscriptionPaused = errors.New("subscription paused")
)

// QueueError 表示消息队列相关的错误，携带操作上下文信息。
type QueueError struct {
	// Op 是失败的操作名称（如 "publish"、"subscribe"、"ack"）。
	Op string

	// Topic 是相关的主题（可能为空）。
	Topic string

	// Err 是底层错误。
	Err error
}

// Error 返回格式化的错误信息。
func (e *QueueError) Error() string {
	if e.Topic == "" {
		return fmt.Sprintf("queue %s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("queue %s %q: %v", e.Op, e.Topic, e.Err)
}

// Unwrap 返回底层错误，支持 errors.Is 和 errors.As。
func (e *QueueError) Unwrap() error {
	return e.Err
}

// NewQueueError 创建一个新的 QueueError。
func NewQueueError(op, topic string, err error) error {
	return &QueueError{Op: op, Topic: topic, Err: err}
}
