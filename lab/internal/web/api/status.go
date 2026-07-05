package api

import (
	"errors"
	"fmt"
	"sync"
	"time"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
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

// ResultSentinel 是类型化操作结果的最小接口：可派生 ok 字段与状态。
//
// 任何"同步 DAG 调用的结果"都应实现该接口（*app.StringCallResult / *app.CallResult
// 隐式满足）；*app.StringCallResult 还额外实现 ResultSentinelWithErr。
//
// Status 返回 dagv1.InstanceStatus 枚举值，recorder 通过 enum.String() 转为可读名。
type ResultSentinel interface {
	// IsCompleted 报告操作是否成功（OK 终态）。
	IsCompleted() bool
	// Status 返回操作当前状态枚举。
	Status() dagv1.InstanceStatus
}

// ResultSentinelWithErr 在 ResultSentinel 基础上额外暴露 Err() —— 让 recorder
// 优先取业务错误信息而非仅 "status=<Status>" 占位。
//
// 不要求所有实现方都满足；recorder 在 type assertion 失败时回退到 "status=<Status>" 形式。
type ResultSentinelWithErr interface {
	ResultSentinel
	Err() error
}

// OpRecorder 记录最近操作（环形缓冲）。
type OpRecorder struct {
	mu  sync.Mutex
	ops []Operation
	max int
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

// RecordResult 追加一条操作记录，ok 与 err 字段从 ResultSentinel 派生。
//
//   - res == nil → ok=false, err="nil result"
//   - res.IsCompleted() == true → ok=true, err=""
//   - res.IsCompleted() == false 且 res 满足 ResultSentinelWithErr + Err() != nil
//     → ok=false, err=res.Err().Error()
//   - 其它 → ok=false, err="status=<res.Status()>"
//
// Detail 透传调用方提供的字符串。
func (r *OpRecorder) RecordResult(op, detail string, res ResultSentinel) {
	if res == nil {
		r.Record(op, detail, false, errors.New("nil result"))
		return
	}
	ok := res.IsCompleted()
	if ok {
		r.Record(op, detail, true, nil)
		return
	}
	var err error
	if e, hasErr := res.(ResultSentinelWithErr); hasErr && e.Err() != nil {
		err = e.Err()
	} else {
		err = fmt.Errorf("status=%s", res.Status().String())
	}
	r.Record(op, detail, false, err)
}

// Snapshot 返回最近操作副本。
func (r *OpRecorder) Snapshot() []Operation {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Operation, len(r.ops))
	copy(out, r.ops)
	return out
}
