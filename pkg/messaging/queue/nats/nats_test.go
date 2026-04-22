package nats

import (
	"context"
	"testing"

	"github.com/solo-kingdom/uniface/pkg/messaging/queue"
)

// TestConfigDefaults 测试默认配置。
func TestConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.URL != "nats://localhost:4222" {
		t.Errorf("Expected default URL, got %s", cfg.URL)
	}
	if cfg.Name != "uniface-nats" {
		t.Errorf("Expected Name uniface-nats, got %s", cfg.Name)
	}
	if cfg.MaxReconnect != 60 {
		t.Errorf("Expected MaxReconnect 60, got %d", cfg.MaxReconnect)
	}
}

// TestConfigWithOption 测试函数式选项。
func TestConfigWithOption(t *testing.T) {
	cfg := NewConfig(
		WithURL("nats://nats:4222"),
		WithName("test-app"),
		WithMaxReconnect(10),
	)

	if cfg.URL != "nats://nats:4222" {
		t.Errorf("Expected custom URL, got %s", cfg.URL)
	}
	if cfg.Name != "test-app" {
		t.Errorf("Expected Name test-app, got %s", cfg.Name)
	}
	if cfg.MaxReconnect != 10 {
		t.Errorf("Expected MaxReconnect 10, got %d", cfg.MaxReconnect)
	}
}

// TestQueueClosedOnPublish 测试关闭状态下发布返回错误。
func TestQueueClosedOnPublish(t *testing.T) {
	q := &Queue[any]{
		closed:        true,
		codec:         &queue.JSONCodec{},
		subscriptions: make(map[string]*natsSubscription[any]),
	}
	err := q.Publish(nil, "test.subject", &queue.Message[any]{})
	if err == nil {
		t.Error("Expected error on closed queue")
	}
}

// TestQueueClosedOnSubscribe 测试关闭状态下订阅返回错误。
func TestQueueClosedOnSubscribe(t *testing.T) {
	q := &Queue[any]{
		closed:        true,
		codec:         &queue.JSONCodec{},
		subscriptions: make(map[string]*natsSubscription[any]),
	}
	_, err := q.Subscribe(nil, "test.subject", func(ctx context.Context, msg *queue.Message[any]) error { return nil })
	if err == nil {
		t.Error("Expected error on closed queue")
	}
}

// TestSubscriptionPauseResume 测试订阅暂停/恢复。
func TestSubscriptionPauseResume(t *testing.T) {
	sub := &natsSubscription[any]{
		subject: "test",
		paused:  false,
		closed:  false,
	}

	if err := sub.Pause(); err != nil {
		t.Errorf("Unexpected error on Pause: %v", err)
	}
	if !sub.paused {
		t.Error("Expected paused true")
	}

	if err := sub.Resume(); err != nil {
		t.Errorf("Unexpected error on Resume: %v", err)
	}
	if sub.paused {
		t.Error("Expected paused false")
	}
}
