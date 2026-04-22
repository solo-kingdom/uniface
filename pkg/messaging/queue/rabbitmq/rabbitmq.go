package rabbitmq

import (
	"context"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/solo-kingdom/uniface/pkg/messaging/queue"
)

// Queue 实现 queue.Queue[T] 接口，使用 RabbitMQ 作为消息代理。
type Queue[T any] struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	config  *Config
	codec   queue.Codec
	mu      sync.RWMutex
	closed  bool

	subscriptions map[string]*rabbitSubscription[T]
	subMu         sync.Mutex
}

// New 创建新的 RabbitMQ 队列实例。
func New[T any](opts ...Option) (*Queue[T], error) {
	cfg := NewConfig(opts...)

	conn, err := amqp.DialConfig(cfg.URL, amqp.Config{
		Dial:      nil,
		Heartbeat: cfg.Heartbeat,
		Locale:    "en_US",
	})
	if err != nil {
		return nil, queue.NewQueueError("connect", "", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, queue.NewQueueError("connect", "", err)
	}

	// 设置 QoS
	if err := ch.Qos(cfg.PrefetchCount, cfg.PrefetchSize, false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, queue.NewQueueError("connect", "", err)
	}

	return &Queue[T]{
		conn:          conn,
		channel:       ch,
		config:        cfg,
		codec:         &queue.JSONCodec{},
		subscriptions: make(map[string]*rabbitSubscription[T]),
	}, nil
}

// getCodec 从选项中获取编解码器。
func (q *Queue[T]) getCodec(opts *queue.Options) queue.Codec {
	if opts != nil && opts.Codec != nil {
		return opts.Codec
	}
	return q.codec
}

// declareExchange 声明交换机（如果配置了的话）。
func (q *Queue[T]) declareExchange(exchange, exchangeType string) error {
	if exchange == "" {
		return nil
	}
	return q.channel.ExchangeDeclare(
		exchange,
		exchangeType,
		q.config.Durable,
		q.config.AutoDelete,
		false,
		false,
		nil,
	)
}

// Publish 发布一条消息到指定主题。
func (q *Queue[T]) Publish(ctx context.Context, topic string, message *queue.Message[T], opts ...queue.Option) error {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.closed {
		return queue.NewQueueError("publish", topic, queue.ErrQueueClosed)
	}

	if topic == "" {
		return queue.NewQueueError("publish", "", queue.ErrTopicRequired)
	}

	options := queue.MergeOptions(opts...)
	codec := q.getCodec(options)

	data, err := codec.Encode(message.Value)
	if err != nil {
		return queue.NewQueueError("publish", topic, err)
	}

	exchange := q.config.Exchange
	key := message.Key
	if options.Key != "" {
		key = options.Key
	}
	// 当没有指定交换机时，使用默认交换机，routing key 即为队列名
	if exchange == "" && key == "" {
		key = topic
	}

	headers := amqp.Table{}
	for k, v := range message.Headers {
		headers[k] = v
	}
	for k, v := range options.Headers {
		headers[k] = v
	}

	amqpMsg := amqp.Publishing{
		DeliveryMode: q.config.DeliveryMode,
		ContentType:  "application/json",
		Body:         data,
		Headers:      headers,
	}
	if message.Timestamp > 0 {
		amqpMsg.Timestamp = UnixMilliToTime(message.Timestamp)
	}

	if err := q.channel.PublishWithContext(ctx, exchange, key, false, false, amqpMsg); err != nil {
		return queue.NewQueueError("publish", topic, err)
	}

	return nil
}

// BatchPublish 批量发布消息到指定主题。
// RabbitMQ 不原生支持批量发布，逐条发送。
func (q *Queue[T]) BatchPublish(ctx context.Context, topic string, messages []*queue.Message[T], opts ...queue.Option) error {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.closed {
		return queue.NewQueueError("batch_publish", topic, queue.ErrQueueClosed)
	}

	if topic == "" {
		return queue.NewQueueError("batch_publish", "", queue.ErrTopicRequired)
	}

	if len(messages) == 0 {
		return nil
	}

	for _, message := range messages {
		if err := q.Publish(ctx, topic, message, opts...); err != nil {
			return fmt.Errorf("batch_publish failed at message: %w", err)
		}
	}

	return nil
}

// Subscribe 订阅指定主题的消息。
func (q *Queue[T]) Subscribe(ctx context.Context, topic string, handler queue.Handler[T], opts ...queue.Option) (queue.Subscription, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.closed {
		return nil, queue.NewQueueError("subscribe", topic, queue.ErrQueueClosed)
	}

	if topic == "" {
		return nil, queue.NewQueueError("subscribe", "", queue.ErrTopicRequired)
	}

	options := queue.MergeOptions(opts...)
	codec := q.getCodec(options)

	// 确保交换机存在
	exchange := q.config.Exchange
	if exchange != "" {
		if err := q.declareExchange(exchange, q.config.ExchangeType); err != nil {
			return nil, queue.NewQueueError("subscribe", topic, err)
		}
	}

	// 声明队列
	queueName := q.config.QueuePrefix + topic
	if options.Group != "" {
		queueName = q.config.QueuePrefix + options.Group + "." + topic
	}

	_, err := q.channel.QueueDeclare(
		queueName,
		q.config.Durable,
		q.config.AutoDelete,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, queue.NewQueueError("subscribe", topic, err)
	}

	// 绑定队列到交换机
	if exchange != "" {
		bindingKey := topic
		if err := q.channel.QueueBind(queueName, bindingKey, exchange, false, nil); err != nil {
			return nil, queue.NewQueueError("subscribe", topic, err)
		}
	}

	// 开始消费
	deliveries, err := q.channel.Consume(queueName, "", !options.AutoAck, false, false, false, nil)
	if err != nil {
		return nil, queue.NewQueueError("subscribe", topic, err)
	}

	subCtx, cancel := context.WithCancel(ctx)

	sub := &rabbitSubscription[T]{
		topic:      topic,
		queueName:  queueName,
		deliveries: deliveries,
		channel:    q.channel,
		handler:    handler,
		codec:      codec,
		autoAck:    options.AutoAck,
		closed:     false,
		paused:     false,
		cancelCtx:  cancel,
	}

	// 启动消费 goroutine
	go sub.consume(subCtx)

	// 注册订阅
	q.subMu.Lock()
	q.subscriptions[queueName] = sub
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
	q.subscriptions = make(map[string]*rabbitSubscription[T])
	q.subMu.Unlock()

	if q.channel != nil {
		if err := q.channel.Close(); err != nil {
			return queue.NewQueueError("close", "", err)
		}
	}

	if q.conn != nil {
		if err := q.conn.Close(); err != nil {
			return queue.NewQueueError("close", "", err)
		}
	}

	return nil
}

// rabbitSubscription 实现 queue.Subscription 接口。
type rabbitSubscription[T any] struct {
	topic      string
	queueName  string
	deliveries <-chan amqp.Delivery
	channel    *amqp.Channel
	handler    queue.Handler[T]
	codec      queue.Codec
	autoAck    bool

	mu        sync.Mutex
	paused    bool
	closed    bool
	cancelCtx context.CancelFunc
}

// consume 在后台 goroutine 中运行消费循环。
func (s *rabbitSubscription[T]) consume(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case delivery, ok := <-s.deliveries:
			if !ok {
				return
			}

			// 检查暂停状态
			s.mu.Lock()
			paused := s.paused
			s.mu.Unlock()

			if paused {
				// 暂停时不处理，Nack 并重新入队
				_ = delivery.Nack(false, true)
				continue
			}

			// 解码消息
			var value T
			if err := s.codec.Decode(delivery.Body, &value); err != nil {
				// 解码失败，拒绝消息且不重新入队
				_ = delivery.Nack(false, false)
				continue
			}

			// 构建消息头
			headers := make(map[string]string)
			for k, v := range delivery.Headers {
				if str, ok := v.(string); ok {
					headers[k] = str
				}
			}

			message := &queue.Message[T]{
				Topic:     s.topic,
				Key:       delivery.RoutingKey,
				Value:     value,
				Headers:   headers,
				Timestamp: delivery.Timestamp.UnixMilli(),
			}

			// 调用用户 Handler
			if err := s.handler(ctx, message); err != nil {
				// NACK
				_ = delivery.Nack(false, true)
			} else {
				// ACK（仅手动确认模式）
				if !s.autoAck {
					_ = delivery.Ack(false)
				}
			}
		}
	}
}

// Unsubscribe 取消订阅。
func (s *rabbitSubscription[T]) Unsubscribe() error {
	return s.close()
}

// Pause 暂停消费。
func (s *rabbitSubscription[T]) Pause() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return queue.NewQueueError("pause", s.topic, queue.ErrSubscriptionClosed)
	}

	s.paused = true
	return nil
}

// Resume 恢复消费。
func (s *rabbitSubscription[T]) Resume() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return queue.NewQueueError("resume", s.topic, queue.ErrSubscriptionClosed)
	}

	s.paused = false
	return nil
}

// close 内部关闭方法。
func (s *rabbitSubscription[T]) close() error {
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

// UnixMilliToTime 将 Unix 毫秒时间戳转换为 time.Time。
func UnixMilliToTime(ms int64) time.Time {
	return time.Unix(ms/1000, (ms%1000)*1e6)
}
