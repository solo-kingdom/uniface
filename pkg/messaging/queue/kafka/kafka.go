package kafka

import (
	"context"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/solo-kingdom/uniface/pkg/messaging/queue"
)

// Queue 实现 queue.Queue[T] 接口，使用 Kafka 作为消息代理。
type Queue[T any] struct {
	producer sarama.SyncProducer
	client   sarama.Client
	config   *Config
	codec    queue.Codec
	mu       sync.RWMutex
	closed   bool

	// 跟踪活跃的消费者组和订阅
	subscriptions map[string]*kafkaSubscription[T]
	subMu         sync.Mutex
}

// New 创建新的 Kafka 队列实例。
func New[T any](opts ...Option) (*Queue[T], error) {
	cfg := NewConfig(opts...)
	saramaConfig := buildSaramaConfig(cfg)

	client, err := sarama.NewClient(cfg.Brokers, saramaConfig)
	if err != nil {
		return nil, queue.NewQueueError("connect", "", err)
	}

	producer, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		_ = client.Close()
		return nil, queue.NewQueueError("connect", "", err)
	}

	codec := &queue.JSONCodec{}

	return &Queue[T]{
		producer:      producer,
		client:        client,
		config:        cfg,
		codec:         codec,
		subscriptions: make(map[string]*kafkaSubscription[T]),
	}, nil
}

// buildSaramaConfig 从 Config 构建 sarama.Config。
func buildSaramaConfig(cfg *Config) *sarama.Config {
	sc := sarama.NewConfig()
	sc.ClientID = cfg.ClientID
	sc.Producer.RequiredAcks = sarama.RequiredAcks(cfg.ProducerRequiredAcks)
	sc.Producer.Timeout = cfg.ProducerTimeout
	sc.Producer.Return.Successes = true
	sc.Producer.Return.Errors = true
	sc.Consumer.Offsets.Initial = cfg.ConsumerOffsetsInitial
	sc.Consumer.MaxProcessingTime = cfg.ConsumerMaxProcessingTime
	sc.ChannelBufferSize = cfg.ChannelBufferSize

	if cfg.Version != "" {
		if v, err := sarama.ParseKafkaVersion(cfg.Version); err == nil {
			sc.Version = v
		}
	}

	if cfg.TLS {
		sc.Net.TLS.Enable = true
	}

	switch cfg.AuthType {
	case "sasl_plain":
		sc.Net.SASL.Enable = true
		sc.Net.SASL.User = cfg.AuthUser
		sc.Net.SASL.Password = cfg.AuthPassword
	case "sasl_scram":
		sc.Net.SASL.Enable = true
		sc.Net.SASL.User = cfg.AuthUser
		sc.Net.SASL.Password = cfg.AuthPassword
		sc.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
			return &XDGSCRAMClient{}
		}
	}

	return sc
}

// getCodec 从选项中获取编解码器，如果未指定则使用默认的。
func (q *Queue[T]) getCodec(opts *queue.Options) queue.Codec {
	if opts != nil && opts.Codec != nil {
		return opts.Codec
	}
	return q.codec
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

	key := message.Key
	if options.Key != "" {
		key = options.Key
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(data),
	}

	if key != "" {
		msg.Key = sarama.StringEncoder(key)
	}

	if message.Timestamp > 0 {
		msg.Timestamp = time.UnixMilli(message.Timestamp)
	}

	// 附加 Headers
	if message.Headers != nil || options.Headers != nil {
		var headers []sarama.RecordHeader
		for k, v := range message.Headers {
			headers = append(headers, sarama.RecordHeader{
				Key:   []byte(k),
				Value: []byte(v),
			})
		}
		for k, v := range options.Headers {
			headers = append(headers, sarama.RecordHeader{
				Key:   []byte(k),
				Value: []byte(v),
			})
		}
		msg.Headers = headers
	}

	select {
	case <-ctx.Done():
		return queue.NewQueueError("publish", topic, ctx.Err())
	default:
	}

	_, _, err = q.producer.SendMessage(msg)
	if err != nil {
		return queue.NewQueueError("publish", topic, err)
	}

	return nil
}

// BatchPublish 批量发布消息到指定主题。
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

	options := queue.MergeOptions(opts...)
	codec := q.getCodec(options)

	saramaMessages := make([]*sarama.ProducerMessage, 0, len(messages))
	for _, message := range messages {
		data, err := codec.Encode(message.Value)
		if err != nil {
			return queue.NewQueueError("batch_publish", topic, err)
		}

		key := message.Key
		if options.Key != "" {
			key = options.Key
		}

		msg := &sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.ByteEncoder(data),
		}

		if key != "" {
			msg.Key = sarama.StringEncoder(key)
		}

		if message.Timestamp > 0 {
			msg.Timestamp = time.UnixMilli(message.Timestamp)
		}

		saramaMessages = append(saramaMessages, msg)
	}

	select {
	case <-ctx.Done():
		return queue.NewQueueError("batch_publish", topic, ctx.Err())
	default:
	}

	err := q.producer.SendMessages(saramaMessages)
	if err != nil {
		return queue.NewQueueError("batch_publish", topic, err)
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
	groupID := options.Group
	if groupID == "" {
		groupID = q.config.GroupID
	}
	if groupID == "" {
		groupID = "uniface-default-group"
	}

	codec := q.getCodec(options)
	saramaConfig := buildSaramaConfig(q.config)

	consumerGroup, err := sarama.NewConsumerGroup(q.config.Brokers, groupID, saramaConfig)
	if err != nil {
		return nil, queue.NewQueueError("subscribe", topic, err)
	}

	sub := &kafkaSubscription[T]{
		topic:         topic,
		groupID:       groupID,
		consumerGroup: consumerGroup,
		handler:       handler,
		codec:         codec,
		autoAck:       options.AutoAck,
		maxRetries:    options.MaxRetries,
		queue:         q,
		paused:        false,
		closed:        false,
		cancelCtx:     nil,
	}

	// 启动消费 goroutine
	subCtx, cancel := context.WithCancel(ctx)
	sub.cancelCtx = cancel
	go sub.consume(subCtx)

	// 注册订阅
	q.subMu.Lock()
	q.subscriptions[topic+"@"+groupID] = sub
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
	q.subscriptions = make(map[string]*kafkaSubscription[T])
	q.subMu.Unlock()

	// 关闭生产者
	if q.producer != nil {
		if err := q.producer.Close(); err != nil {
			return queue.NewQueueError("close", "", err)
		}
	}

	// 关闭客户端
	if q.client != nil {
		if err := q.client.Close(); err != nil {
			return queue.NewQueueError("close", "", err)
		}
	}

	return nil
}

// kafkaSubscription 实现 queue.Subscription 接口。
type kafkaSubscription[T any] struct {
	topic         string
	groupID       string
	consumerGroup sarama.ConsumerGroup
	handler       queue.Handler[T]
	codec         queue.Codec
	autoAck       bool
	maxRetries    int
	queue         *Queue[T]

	mu        sync.Mutex
	paused    bool
	closed    bool
	cancelCtx context.CancelFunc
}

// consume 在后台 goroutine 中运行消费循环。
func (s *kafkaSubscription[T]) consume(ctx context.Context) {
	handler := &consumerGroupHandler[T]{
		subscription: s,
		codec:        s.codec,
		handler:      s.handler,
		autoAck:      s.autoAck,
	}

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
			// 暂停时等待恢复
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
				continue
			}
		}

		if err := s.consumerGroup.Consume(ctx, []string{s.topic}, handler); err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				// 消费错误，短暂等待后重试
				time.Sleep(time.Second)
			}
		}
	}
}

// Unsubscribe 取消订阅。
func (s *kafkaSubscription[T]) Unsubscribe() error {
	return s.close()
}

// Pause 暂停消费。
func (s *kafkaSubscription[T]) Pause() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return queue.NewQueueError("pause", s.topic, queue.ErrSubscriptionClosed)
	}

	s.paused = true
	return nil
}

// Resume 恢复消费。
func (s *kafkaSubscription[T]) Resume() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return queue.NewQueueError("resume", s.topic, queue.ErrSubscriptionClosed)
	}

	s.paused = false
	return nil
}

// close 内部关闭方法，不加锁。
func (s *kafkaSubscription[T]) close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true

	if s.cancelCtx != nil {
		s.cancelCtx()
	}

	if s.consumerGroup != nil {
		if err := s.consumerGroup.Close(); err != nil {
			return queue.NewQueueError("unsubscribe", s.topic, err)
		}
	}

	return nil
}

// consumerGroupHandler 实现 sarama.ConsumerGroupHandler 接口。
type consumerGroupHandler[T any] struct {
	subscription *kafkaSubscription[T]
	codec        queue.Codec
	handler      queue.Handler[T]
	autoAck      bool
}

func (h *consumerGroupHandler[T]) Setup(session sarama.ConsumerGroupSession) error {
	return nil
}

func (h *consumerGroupHandler[T]) Cleanup(session sarama.ConsumerGroupSession) error {
	return nil
}

func (h *consumerGroupHandler[T]) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		// 解码消息
		var value T
		if err := h.codec.Decode(msg.Value, &value); err != nil {
			// 解码失败，跳过此消息
			session.MarkMessage(msg, "")
			continue
		}

		// 构建消息头
		headers := make(map[string]string)
		for _, h := range msg.Headers {
			headers[string(h.Key)] = string(h.Value)
		}

		message := &queue.Message[T]{
			Topic:     msg.Topic,
			Key:       string(msg.Key),
			Value:     value,
			Headers:   headers,
			Timestamp: msg.Timestamp.UnixMilli(),
		}

		// 调用用户 Handler
		ctx := context.Background()
		if err := h.handler(ctx, message); err != nil {
			// NACK: Handler 返回错误
			// Kafka 不支持单条 NACK，这里标记消息以继续消费
			// 日志记录可由上层包装
		}

		// 标记消息已处理
		if h.autoAck {
			session.MarkMessage(msg, "")
		}
	}
	return nil
}

// XDGSCRAMClient 是 SASL/SCRAM 认证客户端的占位实现。
// 实际使用时需要引入 github.com/xdg-go/scram 包。
type XDGSCRAMClient struct{}

func (x *XDGSCRAMClient) Begin(userName, password, authzID string) error {
	return nil
}

func (x *XDGSCRAMClient) Step(challenge string) (string, error) {
	return "", nil
}

func (x *XDGSCRAMClient) Done() bool {
	return true
}
