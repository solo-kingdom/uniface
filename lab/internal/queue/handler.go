package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/solo-kingdom/uniface/lab/internal/web/api"
	"github.com/solo-kingdom/uniface/pkg/messaging/queue"
)

// MessageRecord 最近消息记录。
type MessageRecord struct {
	Time  string         `json:"time"`
	Topic string         `json:"topic"`
	Body  map[string]any `json:"body"`
	Dir   string         `json:"dir"`
}

// Handler 封装消息队列操作。
type Handler struct {
	pub    queue.Publisher[map[string]any]
	sub    queue.Subscriber[map[string]any]
	closer func() error
	impl   string
	rec    *api.OpRecorder
	mu     sync.RWMutex
	health bool
	errMsg string
	msgs   []MessageRecord
}

// NewHandler 创建 Queue 处理器。
func NewHandler(pub queue.Publisher[map[string]any], sub queue.Subscriber[map[string]any], closer func() error, impl string) *Handler {
	return &Handler{
		pub:    pub,
		sub:    sub,
		closer: closer,
		impl:   impl,
		rec:    api.NewOpRecorder(50),
		health: pub != nil && sub != nil,
	}
}

// Publish 发布消息。
func (h *Handler) Publish(ctx context.Context, topic string, body map[string]any) error {
	msg := &queue.Message[map[string]any]{Topic: topic, Value: body, Timestamp: time.Now().UnixMilli()}
	err := h.pub.Publish(ctx, topic, msg)
	h.rec.Record("publish", topic, err == nil, err)
	h.addMsg("out", topic, body)
	h.setHealth(err)
	return err
}

// Subscribe 订阅主题（后台 goroutine）。
func (h *Handler) Subscribe(ctx context.Context, topic string) error {
	_, err := h.sub.Subscribe(ctx, topic, func(_ context.Context, msg *queue.Message[map[string]any]) error {
		h.addMsg("in", msg.Topic, msg.Value)
		h.rec.Record("subscribe", msg.Topic, true, nil)
		return nil
	})
	if err != nil {
		h.setHealth(err)
	}
	return err
}

func (h *Handler) addMsg(dir, topic string, body map[string]any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.msgs = append(h.msgs, MessageRecord{
		Time:  time.Now().Format(time.RFC3339),
		Topic: topic,
		Body:  body,
		Dir:   dir,
	})
	if len(h.msgs) > 50 {
		h.msgs = h.msgs[len(h.msgs)-50:]
	}
}

// Impl 返回实现名称。
func (h *Handler) Impl() string { return h.impl }

// Status 返回域状态。
func (h *Handler) Status() api.Status {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return api.Status{
		Domain:      "queue",
		Impl:        h.impl,
		Healthy:     h.health,
		Error:       h.errMsg,
		RecentOps:   h.rec.Snapshot(),
		Extra: map[string]any{
			"recent_messages": h.msgs,
		},
		CollectedAt: time.Now(),
	}
}

func (h *Handler) setHealth(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err != nil {
		h.health = false
		h.errMsg = err.Error()
		return
	}
	h.health = true
	h.errMsg = ""
}

// Close 关闭队列连接。
func (h *Handler) Close() error {
	if h.closer == nil {
		return nil
	}
	return h.closer()
}

// Messages 返回最近消息。
func (h *Handler) Messages() []MessageRecord {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]MessageRecord, len(h.msgs))
	copy(out, h.msgs)
	return out
}

// FormatBody 格式化消息体。
func FormatBody(body map[string]any) string {
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Sprintf("%v", body)
	}
	return string(b)
}
