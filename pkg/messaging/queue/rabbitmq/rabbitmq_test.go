package rabbitmq

import (
	"context"
	"testing"

	"github.com/solo-kingdom/uniface/pkg/messaging/queue"
)

// TestConfigDefaults 测试默认配置。
func TestConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.URL != "amqp://guest:guest@localhost:5672/" {
		t.Errorf("Expected default URL, got %s", cfg.URL)
	}
	if cfg.ExchangeType != "topic" {
		t.Errorf("Expected ExchangeType topic, got %s", cfg.ExchangeType)
	}
	if cfg.DeliveryMode != 2 {
		t.Errorf("Expected DeliveryMode 2, got %d", cfg.DeliveryMode)
	}
	if cfg.PrefetchCount != 10 {
		t.Errorf("Expected PrefetchCount 10, got %d", cfg.PrefetchCount)
	}
	if !cfg.Durable {
		t.Error("Expected Durable true")
	}
}

// TestConfigWithOption 测试函数式选项。
func TestConfigWithOption(t *testing.T) {
	cfg := NewConfig(
		WithURL("amqp://user:pass@rabbit:5672/vhost"),
		WithExchange("test-exchange"),
		WithExchangeType("direct"),
		WithQueuePrefix("app-"),
		WithDurable(false),
		WithPrefetchCount(50),
		WithDeliveryMode(1),
	)

	if cfg.URL != "amqp://user:pass@rabbit:5672/vhost" {
		t.Errorf("Expected custom URL, got %s", cfg.URL)
	}
	if cfg.Exchange != "test-exchange" {
		t.Errorf("Expected exchange test-exchange, got %s", cfg.Exchange)
	}
	if cfg.ExchangeType != "direct" {
		t.Errorf("Expected ExchangeType direct, got %s", cfg.ExchangeType)
	}
	if cfg.QueuePrefix != "app-" {
		t.Errorf("Expected QueuePrefix app-, got %s", cfg.QueuePrefix)
	}
	if cfg.Durable {
		t.Error("Expected Durable false")
	}
	if cfg.PrefetchCount != 50 {
		t.Errorf("Expected PrefetchCount 50, got %d", cfg.PrefetchCount)
	}
	if cfg.DeliveryMode != 1 {
		t.Errorf("Expected DeliveryMode 1, got %d", cfg.DeliveryMode)
	}
}

// TestQueueClosedOnPublish 测试关闭状态下发布返回错误。
func TestQueueClosedOnPublish(t *testing.T) {
	q := &Queue[any]{
		closed:        true,
		codec:         &queue.JSONCodec{},
		subscriptions: make(map[string]*rabbitSubscription[any]),
	}
	err := q.Publish(nil, "test", &queue.Message[any]{})
	if err == nil {
		t.Error("Expected error on closed queue")
	}
}

// TestQueueClosedOnSubscribe 测试关闭状态下订阅返回错误。
func TestQueueClosedOnSubscribe(t *testing.T) {
	q := &Queue[any]{
		closed:        true,
		codec:         &queue.JSONCodec{},
		subscriptions: make(map[string]*rabbitSubscription[any]),
	}
	_, err := q.Subscribe(nil, "test", func(ctx context.Context, msg *queue.Message[any]) error { return nil })
	if err == nil {
		t.Error("Expected error on closed queue")
	}
}

// TestSubscriptionPauseResume 测试订阅暂停/恢复。
func TestSubscriptionPauseResume(t *testing.T) {
	sub := &rabbitSubscription[any]{
		topic:  "test",
		closed: false,
		paused: false,
	}

	// 暂停
	if err := sub.Pause(); err != nil {
		t.Errorf("Unexpected error on Pause: %v", err)
	}
	if !sub.paused {
		t.Error("Expected paused true")
	}

	// 恢复
	if err := sub.Resume(); err != nil {
		t.Errorf("Unexpected error on Resume: %v", err)
	}
	if sub.paused {
		t.Error("Expected paused false")
	}
}

// TestSubscriptionClosedOperations 测试已关闭订阅的操作。
func TestSubscriptionClosedOperations(t *testing.T) {
	sub := &rabbitSubscription[any]{
		topic:  "test",
		closed: true,
	}

	if err := sub.Pause(); err == nil {
		t.Error("Expected error on Pause for closed subscription")
	}
	if err := sub.Resume(); err == nil {
		t.Error("Expected error on Resume for closed subscription")
	}
}
