package memory

import (
	"fmt"
	"sync"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"github.com/solo-kingdom/uniface/pkg/dag/entity"
	"github.com/solo-kingdom/uniface/pkg/dag/graph"
)

// Registry 内存注册表实现。
type Registry struct {
	mu        sync.RWMutex
	types     map[string]*dagv1.EntityTypeRegistration
	graphs    map[string]*dagv1.GraphSpec
	units     map[string]*dagv1.ComputeUnitDef
	unitImpls map[string]dag.ComputeUnit
	compImpls map[string]dag.Compensator
	closed    bool
}

func NewRegistry() *Registry {
	return &Registry{
		types:     make(map[string]*dagv1.EntityTypeRegistration),
		graphs:    make(map[string]*dagv1.GraphSpec),
		units:     make(map[string]*dagv1.ComputeUnitDef),
		unitImpls: make(map[string]dag.ComputeUnit),
		compImpls: make(map[string]dag.Compensator),
	}
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
	return nil
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
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return dag.ErrStoreClosed
	}
	r.units[def.UnitId] = def
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
	r.unitImpls[unitID] = unit
	return nil
}

func (r *Registry) GetComputeUnitImpl(unitID string) (dag.ComputeUnit, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.unitImpls[unitID]
	if !ok {
		return nil, fmt.Errorf("unit impl %q not found", unitID)
	}
	return u, nil
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
