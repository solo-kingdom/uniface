package api

import (
	"sync"
	"time"
)

// Operation 表示一次最近操作记录。
type Operation struct {
	Time   time.Time `json:"time"`
	Op     string    `json:"op"`
	Detail string    `json:"detail"`
	OK     bool      `json:"ok"`
	Error  string    `json:"error,omitempty"`
}

// Status 是各域 serve 模式统一的 /api/status 响应。
type Status struct {
	Domain      string         `json:"domain"`
	Impl        string         `json:"impl"`
	Healthy     bool           `json:"healthy"`
	Error       string         `json:"error,omitempty"`
	RecentOps   []Operation    `json:"recent_ops"`
	Extra       map[string]any `json:"extra,omitempty"`
	CollectedAt time.Time      `json:"collected_at"`
}

// OpRecorder 记录最近操作（环形缓冲）。
type OpRecorder struct {
	mu   sync.Mutex
	ops  []Operation
	max  int
}

// NewOpRecorder 创建操作记录器。
func NewOpRecorder(max int) *OpRecorder {
	if max <= 0 {
		max = 50
	}
	return &OpRecorder{max: max}
}

// Record 追加一条操作记录。
func (r *OpRecorder) Record(op, detail string, ok bool, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry := Operation{
		Time:   time.Now(),
		Op:     op,
		Detail: detail,
		OK:     ok,
	}
	if err != nil {
		entry.Error = err.Error()
	}
	r.ops = append(r.ops, entry)
	if len(r.ops) > r.max {
		r.ops = r.ops[len(r.ops)-r.max:]
	}
}

// Snapshot 返回最近操作副本。
func (r *OpRecorder) Snapshot() []Operation {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Operation, len(r.ops))
	copy(out, r.ops)
	return out
}
