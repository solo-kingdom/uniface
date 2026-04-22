package nats

import (
	"context"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/solo-kingdom/uniface/pkg/messaging/queue"
)

// Queue 实现 queue.Queue[T] 接口，使用 NATS Core 作为消息代理。
// NATS Core 提供 at-most-once 语义，不支持 ACK 和持久化。
type Queue[T any] struct {
	conn   *nats.Conn
	config *Config
	codec  queue.Codec
	mu     sync.RWMutex
	closed bool

	subscriptions map[string]*natsSubscription[T]
	subMu         sync.Mutex
}

// New 创建新的 NATS Core 队列实例。
func New[T any](opts ...Option) (*Queue[T], error) {
	cfg := NewConfig(opts...)

	nc, err := nats.Connect(cfg.URL,
		nats.Name(cfg.Name),
		nats.MaxReconnects(cfg.MaxReconnect),
		nats.ReconnectWait(cfg.ReconnectWait),
		nats.Timeout(cfg.Timeout),
		nats.PingInterval(cfg.PingInterval),
	)
	if err != nil {
		return nil, queue.NewQueueError("connect", "", err)
	}

	return &Queue[T]{
		conn:          nc,
		config:        cfg,
		codec:         &queue.JSONCodec{},
		subscriptions: make(map[string]*natsSubscription[T]),
	}, nil
}

// getCodec 从选项中获取编解码器。
func (q *Queue[T]) getCodec(opts *queue.Options) queue.Codec {
	if opts != nil && opts.Codec != nil {
		return opts.Codec
	}
	return q.codec
}

// Publish 发布一条消息到指定主题。
func (q *Queue[T]) Publish(ctx context.Context, subject string, message *queue.Message[T], opts ...queue.Option) error {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.closed {
		return queue.NewQueueError("publish", subject, queue.ErrQueueClosed)
	}

	if subject == "" {
		return queue.NewQueueError("publish", "", queue.ErrTopicRequired)
	}

	options := queue.MergeOptions(opts...)
	codec := q.getCodec(options)

	data, err := codec.Encode(message.Value)
	if err != nil {
		return queue.NewQueueError("publish", subject, err)
	}

	// NATS Core 不支持 context 取消，直接发布
	if err := q.conn.Publish(subject, data); err != nil {
		return queue.NewQueueError("publish", subject, err)
	}

	return nil
}

// BatchPublish 批量发布消息到指定主题。
// NATS Core 不原生支持批量发布，逐条发送。
func (q *Queue[T]) BatchPublish(ctx context.Context, subject string, messages []*queue.Message[T], opts ...queue.Option) error {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.closed {
		return queue.NewQueueError("batch_publish", subject, queue.ErrQueueClosed)
	}

	if subject == "" {
		return queue.NewQueueError("batch_publish", "", queue.ErrTopicRequired)
	}

	if len(messages) == 0 {
		return nil
	}

	for _, message := range messages {
		if err := q.Publish(ctx, subject, message, opts...); err != nil {
			return err
		}
	}

	return nil
}

// Subscribe 订阅指定主题的消息。
func (q *Queue[T]) Subscribe(ctx context.Context, subject string, handler queue.Handler[T], opts ...queue.Option) (queue.Subscription, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.closed {
		return nil, queue.NewQueueError("subscribe", subject, queue.ErrQueueClosed)
	}

	if subject == "" {
		return nil, queue.NewQueueError("subscribe", "", queue.ErrTopicRequired)
	}

	options := queue.MergeOptions(opts...)
	codec := q.getCodec(options)

	// 使用 Queue Group（如果指定了 Group）
	queueGroup := options.Group

	sub := &natsSubscription[T]{
		subject:    subject,
		queueGroup: queueGroup,
		handler:    handler,
		codec:      codec,
		conn:       q.conn,
		paused:     false,
		closed:     false,
	}

	// 创建 NATS 订阅
	natsHandler := func(msg *nats.Msg) {
		// 检查暂停状态
		sub.mu.Lock()
		paused := sub.paused
		sub.mu.Unlock()

		if paused {
			return // 暂停时丢弃消息（NATS Core 语义：at-most-once）
		}

		// 解码消息
		var value T
		if err := codec.Decode(msg.Data, &value); err != nil {
			return
		}

		message := &queue.Message[T]{
			Topic:     msg.Subject,
			Value:     value,
			Timestamp: 0, // NATS Core 不提供时间戳
		}

		_ = handler(ctx, message)
	}

	var natsSub *nats.Subscription
	var err error

	if queueGroup != "" {
		natsSub, err = q.conn.QueueSubscribe(subject, queueGroup, natsHandler)
	} else {
		natsSub, err = q.conn.Subscribe(subject, natsHandler)
	}

	if err != nil {
		return nil, queue.NewQueueError("subscribe", subject, err)
	}

	sub.natsSub = natsSub

	// 注册订阅
	q.subMu.Lock()
	q.subscriptions[subject+"@"+queueGroup] = sub
	q.subMu.Unlock()

	return sub, nil
}

// Close 关闭队列并释放所有资源。
func (q *Queue[T]) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return nil
	}

	q.closed = true

	// 关闭所有订阅
	q.subMu.Lock()
	for _, sub := range q.subscriptions {
		_ = sub.close()
	}
	q.subscriptions = make(map[string]*natsSubscription[T])
	q.subMu.Unlock()

	if q.conn != nil {
		q.conn.Close()
	}

	return nil
}

// natsSubscription 实现 queue.Subscription 接口。
type natsSubscription[T any] struct {
	subject    string
	queueGroup string
	handler    queue.Handler[T]
	codec      queue.Codec
	conn       *nats.Conn
	natsSub    *nats.Subscription

	mu     sync.Mutex
	paused bool
	closed bool
}

// Unsubscribe 取消订阅。
func (s *natsSubscription[T]) Unsubscribe() error {
	return s.close()
}

// Pause 暂停消费。
// NATS Core 不原生支持暂停，暂停期间消息会被丢弃（at-most-once 语义）。
func (s *natsSubscription[T]) Pause() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return queue.NewQueueError("pause", s.subject, queue.ErrSubscriptionClosed)
	}

	s.paused = true
	return nil
}

// Resume 恢复消费。
func (s *natsSubscription[T]) Resume() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return queue.NewQueueError("resume", s.subject, queue.ErrSubscriptionClosed)
	}

	s.paused = false
	return nil
}

// close 内部关闭方法。
func (s *natsSubscription[T]) close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true

	if s.natsSub != nil {
		if err := s.natsSub.Unsubscribe(); err != nil {
			return queue.NewQueueError("unsubscribe", s.subject, err)
		}
	}

	return nil
}
