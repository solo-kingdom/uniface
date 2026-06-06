package wiring

import (
	"fmt"
	"strings"

	"github.com/solo-kingdom/uniface/pkg/messaging/queue"
	"github.com/solo-kingdom/uniface/pkg/messaging/queue/kafka"
	"github.com/solo-kingdom/uniface/pkg/messaging/queue/nats"
	"github.com/solo-kingdom/uniface/pkg/messaging/queue/natsjetstream"
	"github.com/solo-kingdom/uniface/pkg/messaging/queue/rabbitmq"
)

// QueuePair 发布者与订阅者组合。
type QueuePair struct {
	Publisher  queue.Publisher[map[string]any]
	Subscriber queue.Subscriber[map[string]any]
	Closer     func() error
}

// NewQueue 根据配置创建消息队列。
func NewQueue(cfg QueueConfig) (*QueuePair, string, error) {
	impl := strings.ToLower(strings.TrimSpace(cfg.Impl))
	if impl == "" {
		impl = "nats"
	}

	switch impl {
	case "nats":
		q, err := nats.New[map[string]any](nats.WithURL(cfg.Addr))
		if err != nil {
			return nil, impl, err
		}
		return &QueuePair{Publisher: q, Subscriber: q, Closer: q.Close}, impl, nil
	case "natsjetstream", "jetstream":
		q, err := natsjetstream.New[map[string]any](natsjetstream.WithURL(cfg.Addr))
		if err != nil {
			return nil, impl, err
		}
		return &QueuePair{Publisher: q, Subscriber: q, Closer: q.Close}, impl, nil
	case "kafka":
		brokers := cfg.Brokers
		if len(brokers) == 0 {
			brokers = []string{"localhost:9092"}
		}
		q, err := kafka.New[map[string]any](kafka.WithBrokers(brokers))
		if err != nil {
			return nil, impl, err
		}
		return &QueuePair{Publisher: q, Subscriber: q, Closer: q.Close}, impl, nil
	case "rabbitmq", "amqp":
		url := cfg.Addr
		if url == "" {
			user := cfg.Username
			if user == "" {
				user = "guest"
			}
			pass := cfg.Password
			if pass == "" {
				pass = "guest"
			}
			url = fmt.Sprintf("amqp://%s:%s@localhost:5672/", user, pass)
		}
		q, err := rabbitmq.New[map[string]any](rabbitmq.WithURL(url))
		if err != nil {
			return nil, impl, err
		}
		return &QueuePair{Publisher: q, Subscriber: q, Closer: q.Close}, impl, nil
	default:
		return nil, impl, fmt.Errorf("unsupported queue impl: %s", impl)
	}
}
