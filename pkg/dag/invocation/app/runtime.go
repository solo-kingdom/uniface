package app

import (
	"fmt"
	"sync"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"github.com/solo-kingdom/uniface/pkg/dag/invocation/loader"
	invocationmemory "github.com/solo-kingdom/uniface/pkg/dag/invocation/memory"
)

// StringPayloadTypeURL 是 string payload 使用的 protobuf type URL。
const StringPayloadTypeURL = "type.googleapis.com/google.protobuf.StringValue"

// Runtime 请求式 DAG 应用封装，持有独立 memory.Runtime 与图加载上下文。
type Runtime struct {
	rt       *invocationmemory.Runtime
	graphDir string
	loadOpts loader.Options
	mu       sync.RWMutex
	loaded   map[string]string

	// stringEntityType / stringSchemaVersion 由 StringApp 专用 Option 设置。
	// 普通 *Runtime 不消费这两个字段；StringApp 在 NewStringApp 中读取后用于
	// RegisterStringEntityType。空值表示使用 StringApp 自己的默认值。
	stringEntityType    string
	stringSchemaVersion string
}

// Option 修改 Runtime 配置。
type Option func(*Runtime)

// WithGraphDir 设置按 graph ID 加载图时使用的目录。
func WithGraphDir(dir string) Option {
	return func(r *Runtime) {
		r.graphDir = dir
	}
}

// WithLoaderDefaults 设置图文档缺省 entity_type 与 schema_version。
func WithLoaderDefaults(entityType, schemaVersion string) Option {
	return func(r *Runtime) {
		r.loadOpts.DefaultEntityType = entityType
		r.loadOpts.DefaultSchemaVersion = schemaVersion
	}
}

// New 创建独立请求式 Runtime，不使用包级全局状态。
func New(opts ...Option) *Runtime {
	rt := &Runtime{
		rt:     invocationmemory.New(),
		loaded: map[string]string{},
	}
	for _, opt := range opts {
		opt(rt)
	}
	return rt
}

// NewWithMemory 创建 Runtime 并传入 memory.Runtime 构造选项。
func NewWithMemory(memOpts ...invocationmemory.Option) *Runtime {
	return &Runtime{
		rt:     invocationmemory.New(memOpts...),
		loaded: map[string]string{},
	}
}

// Memory 返回底层 memory.Runtime。
func (r *Runtime) Memory() *invocationmemory.Runtime { return r.rt }

// GraphDir 返回配置的图目录。
func (r *Runtime) GraphDir() string { return r.graphDir }

// LoadedGraphs 返回已加载图 ID 到文件路径的映射副本。
func (r *Runtime) LoadedGraphs() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]string, len(r.loaded))
	for k, v := range r.loaded {
		out[k] = v
	}
	return out
}

// recordLoaded 记录已加载图。
func (r *Runtime) recordLoaded(graphID, path string) {
	r.mu.Lock()
	r.loaded[graphID] = path
	r.mu.Unlock()
}

// Close 关闭运行时。
func (r *Runtime) Close() error {
	return r.rt.Close()
}

// String 返回简要描述。
func (r *Runtime) String() string {
	return fmt.Sprintf("app.Runtime(graphs=%d)", len(r.loaded))
}

// RegisterGraph 注册图规格（透传底层 Registry）。
func (r *Runtime) RegisterGraph(spec *dagv1.GraphSpec) error {
	return r.rt.RegisterGraph(spec)
}

// RegisterComputeUnitDef 注册计算单元定义（透传底层 Registry）。
func (r *Runtime) RegisterComputeUnitDef(def *dagv1.ComputeUnitDef) error {
	return r.rt.RegisterComputeUnitDef(def)
}

// RegisterComputeUnitImpl 注册已有定义的计算单元实现。
func (r *Runtime) RegisterComputeUnitImpl(unitID string, unit dag.ComputeUnit) error {
	return r.rt.RegisterComputeUnitImpl(unitID, unit)
}
