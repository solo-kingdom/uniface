package lb

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/solo-kingdom/uniface/lab/internal/web/api"
	"github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer"
)

// Handler 封装负载均衡操作。
type Handler struct {
	bal    loadbalancer.Balancer[interface{}]
	algo   string
	rec    *api.OpRecorder
	mu     sync.RWMutex
	health bool
	errMsg string
	lastSim map[string]int
}

// NewHandler 创建 LB 处理器。
func NewHandler(bal loadbalancer.Balancer[interface{}], algo string) *Handler {
	return &Handler{
		bal:    bal,
		algo:   algo,
		rec:    api.NewOpRecorder(50),
		health: bal != nil,
	}
}

// Add 注册实例。
func (h *Handler) Add(ctx context.Context, inst *loadbalancer.Instance) error {
	err := h.bal.Add(ctx, inst)
	h.rec.Record("add", inst.ID, err == nil, err)
	h.setHealth(err)
	return err
}

// Remove 移除实例。
func (h *Handler) Remove(ctx context.Context, id string) error {
	err := h.bal.Remove(ctx, id)
	h.rec.Record("remove", id, err == nil, err)
	h.setHealth(err)
	return err
}

// Select 选择实例。
func (h *Handler) Select(ctx context.Context, key string) (*loadbalancer.Instance, error) {
	var opts []loadbalancer.Option
	if key != "" {
		opts = append(opts, loadbalancer.WithKey(key))
	}
	inst, err := h.bal.Select(ctx, opts...)
	h.rec.Record("select", key, err == nil, err)
	h.setHealth(err)
	return inst, err
}

// Simulate 模拟选择分布。
func (h *Handler) Simulate(ctx context.Context, n int, keyPrefix string) map[string]int {
	counts := map[string]int{}
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("%s-%d", keyPrefix, i)
		inst, err := h.Select(ctx, key)
		if err != nil {
			continue
		}
		counts[inst.ID]++
	}
	h.mu.Lock()
	h.lastSim = counts
	h.mu.Unlock()
	h.rec.Record("simulate", fmt.Sprintf("n=%d", n), true, nil)
	return counts
}

// GetAll 返回全部实例。
func (h *Handler) GetAll(ctx context.Context) ([]*loadbalancer.Instance, error) {
	return h.bal.GetAll(ctx)
}

// Algo 返回算法名称。
func (h *Handler) Algo() string { return h.algo }

// Balancer 返回底层均衡器。
func (h *Handler) Balancer() loadbalancer.Balancer[interface{}] { return h.bal }

// Status 返回域状态。
func (h *Handler) Status() api.Status {
	h.mu.RLock()
	defer h.mu.RUnlock()
	extra := map[string]any{}
	if h.lastSim != nil {
		extra["simulation"] = h.lastSim
	}
	instances, _ := h.bal.GetAll(context.Background())
	extra["instances"] = instances
	return api.Status{
		Domain:      "lb",
		Impl:        h.algo,
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

// Close 关闭均衡器。
func (h *Handler) Close() error {
	if h.bal == nil {
		return nil
	}
	return h.bal.Close()
}

// MockClientFactory 返回 mock 客户端工厂。
func MockClientFactory() loadbalancer.Option {
	return loadbalancer.WithClientFactory(func(inst *loadbalancer.Instance) (interface{}, error) {
		return map[string]string{"id": inst.ID, "addr": fmt.Sprintf("%s:%d", inst.Address, inst.Port)}, nil
	})
}
