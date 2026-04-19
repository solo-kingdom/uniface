package kafka

import (
	"context"
	"testing"

	"github.com/solo-kingdom/uniface/pkg/messaging/queue"
)

// TestConfigDefaults 测试默认配置。
func TestConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if len(cfg.Brokers) != 1 || cfg.Brokers[0] != "localhost:9092" {
		t.Errorf("Expected default broker localhost:9092, got %v", cfg.Brokers)
	}
	if cfg.ClientID != "uniface-kafka" {
		t.Errorf("Expected clientID uniface-kafka, got %s", cfg.ClientID)
	}
	if cfg.ProducerRequiredAcks != 1 {
		t.Errorf("Expected ProducerRequiredAcks 1, got %d", cfg.ProducerRequiredAcks)
	}
	if cfg.ConsumerOffsetsInitial != -1 {
		t.Errorf("Expected ConsumerOffsetsInitial -1, got %d", cfg.ConsumerOffsetsInitial)
	}
	if cfg.AuthType != "none" {
		t.Errorf("Expected AuthType none, got %s", cfg.AuthType)
	}
}

// TestConfigWithOption 测试函数式选项。
func TestConfigWithOption(t *testing.T) {
	cfg := NewConfig(
		WithBrokers([]string{"broker1:9092", "broker2:9092"}),
		WithGroupID("test-group"),
		WithClientID("test-client"),
		WithSASLPlain("user", "pass"),
		WithTLS(true),
		WithProducerRequiredAcks(-1),
		WithConsumerOffsetsInitial(-2),
	)

	if len(cfg.Brokers) != 2 {
		t.Errorf("Expected 2 brokers, got %d", len(cfg.Brokers))
	}
	if cfg.GroupID != "test-group" {
		t.Errorf("Expected groupID test-group, got %s", cfg.GroupID)
	}
	if cfg.AuthType != "sasl_plain" {
		t.Errorf("Expected AuthType sasl_plain, got %s", cfg.AuthType)
	}
	if cfg.AuthUser != "user" || cfg.AuthPassword != "pass" {
		t.Errorf("Expected user/pass, got %s/%s", cfg.AuthUser, cfg.AuthPassword)
	}
	if !cfg.TLS {
		t.Error("Expected TLS true")
	}
	if cfg.ProducerRequiredAcks != -1 {
		t.Errorf("Expected ProducerRequiredAcks -1, got %d", cfg.ProducerRequiredAcks)
	}
	if cfg.ConsumerOffsetsInitial != -2 {
		t.Errorf("Expected ConsumerOffsetsInitial -2, got %d", cfg.ConsumerOffsetsInitial)
	}
}

// TestBuildSaramaConfig 测试 sarama 配置构建。
func TestBuildSaramaConfig(t *testing.T) {
	cfg := NewConfig(
		WithClientID("test-client"),
		WithProducerRequiredAcks(-1),
	)
	sc := buildSaramaConfig(cfg)

	if sc.ClientID != "test-client" {
		t.Errorf("Expected ClientID test-client, got %s", sc.ClientID)
	}
	if sc.Producer.RequiredAcks != -1 {
		t.Errorf("Expected RequiredAcks -1, got %d", sc.Producer.RequiredAcks)
	}
	if !sc.Producer.Return.Successes {
		t.Error("Expected Return.Successes true")
	}
	if !sc.Producer.Return.Errors {
		t.Error("Expected Return.Errors true")
	}
}

// TestQueueErrors 测试错误类型。
func TestQueueErrors(t *testing.T) {
	// QueueError 格式化
	err := queue.NewQueueError("publish", "test-topic", queue.ErrQueueClosed)
	if err.Error() != `queue publish "test-topic": queue closed` {
		t.Errorf("Unexpected error format: %s", err.Error())
	}

	// QueueError 不带 Topic
	err2 := queue.NewQueueError("connect", "", queue.ErrBrokerUnavailable)
	if err2.Error() != "queue connect: broker unavailable" {
		t.Errorf("Unexpected error format: %s", err2.Error())
	}
}

// TestQueueClosedOnPublish 测试关闭状态下发布返回错误。
func TestQueueClosedOnPublish(t *testing.T) {
	q := &Queue[any]{
		closed:   true,
		codec:    &queue.JSONCodec{},
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
		subscriptions: make(map[string]*kafkaSubscription[any]),
	}
	_, err := q.Subscribe(nil, "test", func(ctx context.Context, msg *queue.Message[any]) error { return nil })
	if err == nil {
		t.Error("Expected error on closed queue")
	}
}
