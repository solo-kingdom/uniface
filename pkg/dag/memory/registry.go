package memory

import (
	"fmt"
	"sync"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"github.com/solo-kingdom/uniface/pkg/dag/entity"
	"github.com/solo-kingdom/uniface/pkg/dag/graph"
	"github.com/solo-kingdom/uniface/pkg/dag/units"
)

// Registry 内存注册表实现。
type Registry struct {
	mu           sync.RWMutex
	types        map[string]*dagv1.EntityTypeRegistration
	graphs       map[string]*dagv1.GraphSpec
	latestGraphs map[string]*dagv1.GraphVersion
	units        map[string]*dagv1.ComputeUnitDef
	unitImpls    map[string]dag.ComputeUnit
	compImpls    map[string]dag.Compensator
	// declarative 记录 unit_id 是否已构造声明式适配器（缓存实例）。
	declarative map[string]dag.ComputeUnit
	resolver    dag.HttpClientResolver
	closed      bool
}

// NewRegistry 创建内存注册表。
func NewRegistry() *Registry {
	return &Registry{
		types:        make(map[string]*dagv1.EntityTypeRegistration),
		graphs:       make(map[string]*dagv1.GraphSpec),
		latestGraphs: make(map[string]*dagv1.GraphVersion),
		units:        make(map[string]*dagv1.ComputeUnitDef),
		unitImpls:    make(map[string]dag.ComputeUnit),
		compImpls:    make(map[string]dag.Compensator),
		declarative:  make(map[string]dag.ComputeUnit),
	}
}

// SetHttpClientResolver 注入声明式 HttpUnit 使用的服务实例解析器。
// 由 Engine 在构造时透传；nil 表示仅支持 url 直连。
func (r *Registry) SetHttpClientResolver(resolver dag.HttpClientResolver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resolver = resolver
}

func typeKeyID(key *dagv1.EntityTypeKey) string {
	return key.EntityType + "@" + key.PayloadSchemaVersion
}

func graphKey(v *dagv1.GraphVersion) string {
	return v.GraphId + "@" + v.Version
}

func (r *Registry) RegisterEntityType(reg *dagv1.EntityTypeRegistration) error {
	if err := entity.ValidateTypeKey(reg.GetTypeKey()); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return dag.ErrStoreClosed
	}
	r.types[typeKeyID(reg.TypeKey)] = reg
	return nil
}

func (r *Registry) ResolveType(key *dagv1.EntityTypeKey) (*dagv1.EntityTypeRegistration, error) {
	if err := entity.ValidateTypeKey(key); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	reg, ok := r.types[typeKeyID(key)]
	if !ok {
		return nil, dag.ErrInvalidEntityType
	}
	return reg, nil
}

func (r *Registry) RegisterGraph(spec *dagv1.GraphSpec) error {
	if err := graph.ValidateGraphSpec(spec); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return dag.ErrStoreClosed
	}
	r.graphs[graphKey(spec.Version)] = spec
	r.latestGraphs[spec.Version.GraphId] = &dagv1.GraphVersion{
		GraphId: spec.Version.GraphId,
		Version: spec.Version.Version,
	}
	return nil
}

func (r *Registry) GetLatestGraphVersion(graphID string) (*dagv1.GraphVersion, error) {
	if graphID == "" {
		return nil, dag.ErrInvalidGraph
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	latest, ok := r.latestGraphs[graphID]
	if !ok {
		return nil, dag.ErrInvalidGraph
	}
	out := *latest
	return &out, nil
}

func (r *Registry) ResolveGraphForInstance(inst *dagv1.EntityInstance) (*dagv1.GraphSpec, error) {
	if inst == nil || inst.GraphVersion == nil || inst.GraphVersion.GraphId == "" {
		return nil, dag.ErrInvalidGraph
	}
	latest, err := r.GetLatestGraphVersion(inst.GraphVersion.GraphId)
	if err != nil {
		return nil, err
	}
	version := graph.ResolveGraphVersion(inst, latest)
	return r.GetGraph(version)
}

func (r *Registry) GetGraph(version *dagv1.GraphVersion) (*dagv1.GraphSpec, error) {
	if version == nil || version.GraphId == "" || version.Version == "" {
		return nil, dag.ErrInvalidGraph
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	spec, ok := r.graphs[graphKey(version)]
	if !ok {
		return nil, dag.ErrInvalidGraph
	}
	return spec, nil
}

func (r *Registry) RegisterComputeUnit(def *dagv1.ComputeUnitDef) error {
	if def == nil || def.UnitId == "" {
		return fmt.Errorf("%w: missing unit id", dag.ErrInvalidGraph)
	}
	if err := entity.ValidateTypeKey(def.InputTypeKey); err != nil {
		return err
	}
	if def.SideEffectClass == dagv1.SideEffectClass_SIDE_EFFECT_EXTERNAL {
		return dag.ErrUnsupportedSideEffect
	}
	// 声明式 implementation 配置静态校验（HttpUnit service/url 等）。
	if err := graph.ValidateComputeUnitDef(def); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return dag.ErrStoreClosed
	}
	// 互斥校验：implementation 非空时禁止同 unit_id 的 Go 注册，反之亦然。
	if def.GetImplementation() != nil {
		if _, exists := r.unitImpls[def.UnitId]; exists {
			return fmt.Errorf("%w: unit %q has Go impl, cannot register declarative implementation", dag.ErrInvalidGraph, def.UnitId)
		}
	}
	r.units[def.UnitId] = def
	// 声明式配置变更后清除缓存的适配器实例，下次 GetComputeUnitImpl 重新构造。
	delete(r.declarative, def.UnitId)
	return nil
}

func (r *Registry) GetComputeUnit(unitID string) (*dagv1.ComputeUnitDef, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.units[unitID]
	if !ok {
		return nil, fmt.Errorf("unit %q not found", unitID)
	}
	return def, nil
}

func (r *Registry) RegisterComputeUnitImpl(unitID string, unit dag.ComputeUnit) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return dag.ErrStoreClosed
	}
	// 互斥校验：已注册含 implementation 的 def 时禁止 Go 注册。
	if def, exists := r.units[unitID]; exists && def.GetImplementation() != nil {
		return fmt.Errorf("%w: unit %q has declarative implementation, cannot register Go impl", dag.ErrInvalidGraph, unitID)
	}
	r.unitImpls[unitID] = unit
	return nil
}

func (r *Registry) GetComputeUnitImpl(unitID string) (dag.ComputeUnit, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// 1. 声明式优先：implementation 非空时构造/复用适配器。
	if def, exists := r.units[unitID]; exists && def.GetImplementation() != nil {
		if cached, ok := r.declarative[unitID]; ok {
			return cached, nil
		}
		impl, err := r.buildDeclarativeUnit(def)
		if err != nil {
			return nil, err
		}
		r.declarative[unitID] = impl
		return impl, nil
	}
	// 2. 回退进程内 Go 注册。
	u, ok := r.unitImpls[unitID]
	if !ok {
		return nil, fmt.Errorf("unit impl %q not found", unitID)
	}
	return u, nil
}

// buildDeclarativeUnit 按 implementation oneof 类型构造声明式 ComputeUnit 适配器。
// 持锁调用（由 GetComputeUnitImpl 在写锁内调用）。
func (r *Registry) buildDeclarativeUnit(def *dagv1.ComputeUnitDef) (dag.ComputeUnit, error) {
	switch impl := def.GetImplementation().(type) {
	case *dagv1.ComputeUnitDef_Http:
		return units.NewHttpUnit(impl.Http, r.resolver), nil
	default:
		return nil, fmt.Errorf("unit %q has unsupported declarative implementation %T", def.UnitId, def.GetImplementation())
	}
}

func (r *Registry) RegisterCompensator(unitID string, comp dag.Compensator) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return dag.ErrStoreClosed
	}
	r.compImpls[unitID] = comp
	return nil
}

func (r *Registry) GetCompensator(unitID string) (dag.Compensator, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.compImpls[unitID]
	if !ok {
		return nil, fmt.Errorf("compensator %q not found", unitID)
	}
	return c, nil
}

func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return nil
}
