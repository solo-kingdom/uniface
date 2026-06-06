package config

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/solo-kingdom/uniface/lab/internal/web/api"
	cfg "github.com/solo-kingdom/uniface/pkg/rpc/governance/config"
)

// WatchEvent watch 事件。
type WatchEvent struct {
	Time  string `json:"time"`
	Key   string `json:"key"`
	Value any    `json:"value,omitempty"`
}

// Handler 封装配置存储操作。
type Handler struct {
	store  cfg.Storage
	impl   string
	rec    *api.OpRecorder
	mu     sync.RWMutex
	health bool
	errMsg string
	events []WatchEvent
}

// NewHandler 创建 Config 处理器。
func NewHandler(store cfg.Storage, impl string) *Handler {
	return &Handler{
		store:  store,
		impl:   impl,
		rec:    api.NewOpRecorder(50),
		health: store != nil,
		events: make([]WatchEvent, 0, 50),
	}
}

// Put 写入配置。
func (h *Handler) Put(ctx context.Context, key string, value any) error {
	err := h.store.Write(ctx, key, value)
	h.rec.Record("put", fmt.Sprintf("%s=%v", key, value), err == nil, err)
	h.setHealth(err)
	return err
}

// Get 读取配置。
func (h *Handler) Get(ctx context.Context, key string) (any, error) {
	var out any
	err := h.store.Read(ctx, key, &out)
	h.rec.Record("get", key, err == nil, err)
	h.setHealth(err)
	return out, err
}

// Delete 删除配置。
func (h *Handler) Delete(ctx context.Context, key string) error {
	err := h.store.Delete(ctx, key)
	h.rec.Record("delete", key, err == nil, err)
	h.setHealth(err)
	return err
}

// List 列出配置键。
func (h *Handler) List(ctx context.Context, prefix string) ([]string, error) {
	keys, err := h.store.List(ctx, prefix)
	h.rec.Record("list", prefix, err == nil, err)
	h.setHealth(err)
	return keys, err
}

// AddWatchEvent 追加 watch 事件。
func (h *Handler) AddWatchEvent(key string, value any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ev := WatchEvent{Time: time.Now().Format(time.RFC3339), Key: key, Value: value}
	h.events = append(h.events, ev)
	if len(h.events) > 50 {
		h.events = h.events[len(h.events)-50:]
	}
}
// WatchPrefix 监听前缀变更（阻塞，需在 goroutine 中调用）。
func (h *Handler) WatchPrefix(ctx context.Context, prefix string) error {
	return h.store.WatchPrefix(ctx, prefix, func(_ context.Context, key string, value interface{}) error {
		h.AddWatchEvent(key, value)
		h.rec.Record("watch", key, true, nil)
		return nil
	})
}

// Store 返回底层存储。
func (h *Handler) Store() cfg.Storage { return h.store }

// Impl 返回实现名称。
func (h *Handler) Impl() string { return h.impl }

// Status 返回域状态。
func (h *Handler) Status() api.Status {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return api.Status{
		Domain:      "config",
		Impl:        h.impl,
		Healthy:     h.health,
		Error:       h.errMsg,
		RecentOps:   h.rec.Snapshot(),
		Extra: map[string]any{
			"watch_events": h.events,
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

// Close 关闭存储。
func (h *Handler) Close() error {
	if h.store == nil {
		return nil
	}
	return h.store.Close()
}

// Events 返回 watch 事件副本。
func (h *Handler) Events() []WatchEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]WatchEvent, len(h.events))
	copy(out, h.events)
	return out
}
