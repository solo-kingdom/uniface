package natsjetstream

import (
	"context"
	"fmt"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/solo-kingdom/uniface/pkg/messaging/queue"
)

// Queue 实现 queue.Queue[T] 接口，使用 NATS JetStream 作为消息代理。
// JetStream 提供持久化存储、ACK、重播等高级特性。
type Queue[T any] struct {
	conn   *nats.Conn
	js     jetstream.JetStream
	config *Config
	codec  queue.Codec
	mu     sync.RWMutex
	closed bool

	subscriptions map[string]*jsSubscription[T]
	subMu         sync.Mutex
}

// New 创建新的 JetStream 队列实例。
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

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, queue.NewQueueError("connect", "", err)
	}

	return &Queue[T]{
		conn:          nc,
		js:            js,
		config:        cfg,
		codec:         &queue.JSONCodec{},
		subscriptions: make(map[string]*jsSubscription[T]),
	}, nil
}

// getOrCreateStream 获取或创建 Stream。
func (q *Queue[T]) getOrCreateStream(ctx context.Context, subject string) (jetstream.Stream, error) {
	streamName := q.config.StreamName
	if streamName == "" {
		streamName = subjectToStreamName(subject)
	}

	stream, err := q.js.Stream(ctx, streamName)
	if err == nil {
		return stream, nil
	}

	// Stream 不存在，创建新的
	streamSubjects := q.config.StreamSubjects
	if len(streamSubjects) == 0 {
		streamSubjects = []string{subject}
	}

	streamConfig := jetstream.StreamConfig{
		Name:      streamName,
		Subjects:  streamSubjects,
		Replicas:  q.config.Replicas,
		Retention: jetstream.RetentionPolicy(q.config.Retention),
		MaxMsgs:   q.config.MaxMsgs,
		MaxBytes:  q.config.MaxBytes,
		MaxAge:    q.config.MaxAge,
	}

	stream, err = q.js.CreateStream(ctx, streamConfig)
	if err != nil {
		return nil, queue.NewQueueError("create_stream", subject, err)
	}

	return stream, nil
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

	// 确保 Stream 存在
	if _, err := q.getOrCreateStream(ctx, subject); err != nil {
		return err
	}

	data, err := codec.Encode(message.Value)
	if err != nil {
		return queue.NewQueueError("publish", subject, err)
	}

	_, err = q.js.Publish(ctx, subject, data)
	if err != nil {
		return queue.NewQueueError("publish", subject, err)
	}

	return nil
}

// BatchPublish 批量发布消息到指定主题。
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

	// 确保 Stream 存在
	if _, err := q.getOrCreateStream(ctx, subject); err != nil {
		return err
	}

	options := queue.MergeOptions(opts...)
	codec := q.getCodec(options)

	for _, message := range messages {
		data, err := codec.Encode(message.Value)
		if err != nil {
			return queue.NewQueueError("batch_publish", subject, err)
		}

		_, err = q.js.Publish(ctx, subject, data)
		if err != nil {
			return queue.NewQueueError("batch_publish", subject, err)
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

	// 获取或创建 Stream
	stream, err := q.getOrCreateStream(ctx, subject)
	if err != nil {
		return nil, err
	}

	// 创建消费者
	durableName := q.config.DurableName
	if options.Group != "" {
		durableName = options.Group
	}

	consumerConfig := jetstream.ConsumerConfig{
		Durable:     durableName,
		MaxDeliver:  q.config.MaxDeliver,
		AckWait:     q.config.AckWait,
		AckPolicy:   jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverAllPolicy,
		FilterSubject: subject,
	}

	if durableName == "" {
		// 非持久消费者
		consumerConfig.Durable = ""
	}

	consumer, err := stream.CreateOrUpdateConsumer(ctx, consumerConfig)
	if err != nil {
		return nil, queue.NewQueueError("subscribe", subject, err)
	}

	subCtx, cancel := context.WithCancel(ctx)

	sub := &jsSubscription[T]{
		subject:    subject,
		js:         q.js,
		consumer:   consumer,
		handler:    handler,
		codec:      codec,
		autoAck:    options.AutoAck,
		paused:     false,
		closed:     false,
		cancelCtx:  cancel,
	}

	// 启动消费 goroutine（使用 pull 模式）
	go sub.consume(subCtx)

	// 注册订阅
	q.subMu.Lock()
	q.subscriptions[subject+"@"+durableName] = sub
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
	q.subscriptions = make(map[string]*jsSubscription[T])
	q.subMu.Unlock()

	if q.conn != nil {
		q.conn.Close()
	}

	return nil
}

// jsSubscription 实现 queue.Subscription 接口。
type jsSubscription[T any] struct {
	subject   string
	js        jetstream.JetStream
	consumer  jetstream.Consumer
	handler   queue.Handler[T]
	codec     queue.Codec
	autoAck   bool

	mu        sync.Mutex
	paused    bool
	closed    bool
	cancelCtx context.CancelFunc
}

// consume 使用 pull 模式消费消息。
func (s *jsSubscription[T]) consume(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 检查暂停状态
		s.mu.Lock()
		paused := s.paused
		s.mu.Unlock()

		if paused {
			select {
			case <-ctx.Done():
				return
			case <-func() <-chan struct{} {
				ch := make(chan struct{})
				go func() {
					close(ch)
				}()
				return ch
			}():
			}
			continue
		}

		// Pull 消息
		msgs, err := s.consumer.Fetch(10, jetstream.FetchMaxWait(5*1e9)) // 5s timeout
		if err != nil {
			continue
		}

		for msg := range msgs.Messages() {
			// 解码消息
			var value T
			if err := s.codec.Decode(msg.Data(), &value); err != nil {
				_ = msg.Nak()
				continue
			}

			headers := make(map[string]string)
			for key := range msg.Headers() {
				headers[key] = msg.Headers().Get(key)
			}

			message := &queue.Message[T]{
				Topic:     msg.Subject(),
				Value:     value,
				Headers:   headers,
				Timestamp: 0, // JetStream metadata 提供时间信息
			}

			// 调用用户 Handler
			if err := s.handler(ctx, message); err != nil {
				_ = msg.Nak()
			} else {
				if s.autoAck {
					_ = msg.Ack()
				}
			}
		}
	}
}

// Unsubscribe 取消订阅。
func (s *jsSubscription[T]) Unsubscribe() error {
	return s.close()
}

// Pause 暂停消费。
func (s *jsSubscription[T]) Pause() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return queue.NewQueueError("pause", s.subject, queue.ErrSubscriptionClosed)
	}

	s.paused = true
	return nil
}

// Resume 恢复消费。
func (s *jsSubscription[T]) Resume() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return queue.NewQueueError("resume", s.subject, queue.ErrSubscriptionClosed)
	}

	s.paused = false
	return nil
}

// close 内部关闭方法。
func (s *jsSubscription[T]) close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	if s.cancelCtx != nil {
		s.cancelCtx()
	}

	return nil
}

// subjectToStreamName 将 subject 转换为合法的 Stream 名称。
// 例如 "orders.created" → "ORDERS_CREATED"
func subjectToStreamName(subject string) string {
	result := make([]byte, 0, len(subject))
	for i := 0; i < len(subject); i++ {
		c := subject[i]
		if c == '.' || c == '*' || c == '>' {
			result = append(result, '_')
		} else if c >= 'a' && c <= 'z' {
			result = append(result, c-32)
		} else {
			result = append(result, c)
		}
	}
	return string(result)
}

// ensure interface compliance at compile time.
var _ queue.Queue[any] = (*Queue[any])(nil)
var _ queue.Subscription = (*jsSubscription[any])(nil)

// suppress unused import
var _ = fmt.Sprintf
