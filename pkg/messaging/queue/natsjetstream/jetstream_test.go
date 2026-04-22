package natsjetstream

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
	if cfg.Name != "uniface-jetstream" {
		t.Errorf("Expected Name uniface-jetstream, got %s", cfg.Name)
	}
	if cfg.MaxDeliver != 3 {
		t.Errorf("Expected MaxDeliver 3, got %d", cfg.MaxDeliver)
	}
	if cfg.Replicas != 1 {
		t.Errorf("Expected Replicas 1, got %d", cfg.Replicas)
	}
}

// TestConfigWithOption 测试函数式选项。
func TestConfigWithOption(t *testing.T) {
	cfg := NewConfig(
		WithURL("nats://nats:4222"),
		WithName("test-js"),
		WithStreamName("TEST_STREAM"),
		WithStreamSubjects([]string{"test.>"}),
		WithDurableName("test-durable"),
		WithMaxDeliver(5),
		WithReplicas(3),
		WithMaxMsgs(10000),
		WithMaxBytes(1024 * 1024),
	)

	if cfg.URL != "nats://nats:4222" {
		t.Errorf("Expected custom URL, got %s", cfg.URL)
	}
	if cfg.StreamName != "TEST_STREAM" {
		t.Errorf("Expected StreamName TEST_STREAM, got %s", cfg.StreamName)
	}
	if cfg.DurableName != "test-durable" {
		t.Errorf("Expected DurableName test-durable, got %s", cfg.DurableName)
	}
	if cfg.MaxDeliver != 5 {
		t.Errorf("Expected MaxDeliver 5, got %d", cfg.MaxDeliver)
	}
	if cfg.Replicas != 3 {
		t.Errorf("Expected Replicas 3, got %d", cfg.Replicas)
	}
	if cfg.MaxMsgs != 10000 {
		t.Errorf("Expected MaxMsgs 10000, got %d", cfg.MaxMsgs)
	}
}

// TestQueueClosedOnPublish 测试关闭状态下发布返回错误。
func TestQueueClosedOnPublish(t *testing.T) {
	q := &Queue[any]{
		closed:        true,
		codec:         &queue.JSONCodec{},
		subscriptions: make(map[string]*jsSubscription[any]),
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
		subscriptions: make(map[string]*jsSubscription[any]),
	}
	_, err := q.Subscribe(nil, "test.subject", func(ctx context.Context, msg *queue.Message[any]) error { return nil })
	if err == nil {
		t.Error("Expected error on closed queue")
	}
}

// TestSubscriptionPauseResume 测试订阅暂停/恢复。
func TestSubscriptionPauseResume(t *testing.T) {
	sub := &jsSubscription[any]{
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

// TestSubscriptionClosedOperations 测试已关闭订阅的操作。
func TestSubscriptionClosedOperations(t *testing.T) {
	sub := &jsSubscription[any]{
		subject: "test",
		closed:  true,
	}

	if err := sub.Pause(); err == nil {
		t.Error("Expected error on Pause for closed subscription")
	}
	if err := sub.Resume(); err == nil {
		t.Error("Expected error on Resume for closed subscription")
	}
}

// TestSubjectToStreamName 测试 subject 到 stream 名称的转换。
func TestSubjectToStreamName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"orders.created", "ORDERS_CREATED"},
		{"test.subject", "TEST_SUBJECT"},
		{"foo.bar.baz", "FOO_BAR_BAZ"},
		{"test.>", "TEST__"},
		{"test.*", "TEST__"},
		{"UPPER.CASE", "UPPER_CASE"},
	}

	for _, tt := range tests {
		result := subjectToStreamName(tt.input)
		if result != tt.expected {
			t.Errorf("subjectToStreamName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
