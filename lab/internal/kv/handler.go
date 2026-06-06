package kv

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/solo-kingdom/uniface/lab/internal/web/api"
	"github.com/solo-kingdom/uniface/pkg/storage/kv"
)

// Handler 封装 KV 存储操作。
type Handler struct {
	store  kv.Storage
	impl   string
	rec    *api.OpRecorder
	mu     sync.RWMutex
	health bool
	errMsg string
	lastConformance *ConformanceResult
}

// NewHandler 创建 KV 处理器。
func NewHandler(store kv.Storage, impl string) *Handler {
	return &Handler{
		store:  store,
		impl:   impl,
		rec:    api.NewOpRecorder(50),
		health: store != nil,
	}
}

// Set 写入键值。
func (h *Handler) Set(ctx context.Context, key string, value any) error {
	err := h.store.Set(ctx, key, value)
	h.rec.Record("set", fmt.Sprintf("%s=%v", key, value), err == nil, err)
	h.setHealth(err)
	return err
}

// Get 读取键值。
func (h *Handler) Get(ctx context.Context, key string) (any, error) {
	var out any
	err := h.store.Get(ctx, key, &out)
	h.rec.Record("get", key, err == nil, err)
	h.setHealth(err)
	return out, err
}

// Delete 删除键。
func (h *Handler) Delete(ctx context.Context, key string) error {
	err := h.store.Delete(ctx, key)
	h.rec.Record("delete", key, err == nil, err)
	h.setHealth(err)
	return err
}

// Exists 检查键是否存在。
func (h *Handler) Exists(ctx context.Context, key string) (bool, error) {
	ok, err := h.store.Exists(ctx, key)
	h.rec.Record("exists", key, err == nil, err)
	h.setHealth(err)
	return ok, err
}

// List 列出所有键。
func (h *Handler) List(ctx context.Context) ([]string, error) {
	keys, err := h.store.List(ctx)
	h.rec.Record("list", fmt.Sprintf("%d keys", len(keys)), err == nil, err)
	h.setHealth(err)
	return keys, err
}

// Store 返回底层存储。
func (h *Handler) Store() kv.Storage { return h.store }

// Impl 返回当前实现名称。
func (h *Handler) Impl() string { return h.impl }

// SetConformanceResult 保存最近一次 conformance 结果。
func (h *Handler) SetConformanceResult(result *ConformanceResult) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastConformance = result
}

// ConformanceResult conformance 运行结果。
type ConformanceResult struct {
	Passed int      `json:"passed"`
	Failed int      `json:"failed"`
	Errors []string `json:"errors,omitempty"`
}

// Status 返回域状态。
func (h *Handler) Status() api.Status {
	h.mu.RLock()
	defer h.mu.RUnlock()
	extra := map[string]any{}
	if h.lastConformance != nil {
		extra["conformance"] = h.lastConformance
	}
	return api.Status{
		Domain:      "kv",
		Impl:        h.impl,
		Healthy:     h.health,
		Error:       h.errMsg,
		RecentOps:   h.rec.Snapshot(),
		Extra:       extra,
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

// FormatValue 格式化值为 JSON 字符串。
func FormatValue(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}
