// Package memory 提供标准内存 DAG Runtime 装配辅助。
//
// Runtime 封装 memory.Registry、memory.LineStore、memory.Engine 与 invocation.Invoker
// 的创建与持有，降低业务方手动组合底层组件的样板代码。
//
// Runtime 只提供通用装配能力，不内置任何 lab 语义（如 lab.Generic、lab.echo、
// lab.hello 等）。调用方需显式注册自身实体类型、图与计算单元。
package memory

import (
	"fmt"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"github.com/solo-kingdom/uniface/pkg/dag/invocation"
	dagmemory "github.com/solo-kingdom/uniface/pkg/dag/memory"
)

// Runtime 持有标准内存 DAG 运行时组件。
type Runtime struct {
	reg     *dagmemory.Registry
	store   *dagmemory.LineStore
	engine  *dagmemory.Engine
	invoker *invocation.Invoker
}

// Option 修改 Runtime 配置。
type Option func(*config)

type config struct {
	engineOpts []dag.Option
}

// WithEngineOptions 追加 memory.Engine 构造选项（如 HttpClientResolver、DrainMaxHops）。
func WithEngineOptions(opts ...dag.Option) Option {
	return func(c *config) {
		c.engineOpts = append(c.engineOpts, opts...)
	}
}

// WithHttpClientResolver 注入声明式 HttpUnit 使用的服务实例解析器。
// 透传给内部 Engine，使声明式 HttpUnit 在执行时可解析服务实例。
func WithHttpClientResolver(r dag.HttpClientResolver) Option {
	return func(c *config) {
		c.engineOpts = append(c.engineOpts, dag.WithHttpClientResolver(r))
	}
}

// New 创建标准内存 Runtime。
//
// 返回的 Runtime 不包含任何注册项，调用方需通过 RegisterEntityType、RegisterGraph、
// RegisterComputeUnit 等方法完成自身装配。
func New(opts ...Option) *Runtime {
	c := &config{}
	for _, opt := range opts {
		opt(c)
	}
	reg := dagmemory.NewRegistry()
	store := dagmemory.NewLineStore()
	engine := dagmemory.NewEngine(reg, store, c.engineOpts...)
	invoker := invocation.NewInvoker(engine, store)
	return &Runtime{
		reg:     reg,
		store:   store,
		engine:  engine,
		invoker: invoker,
	}
}

// Registry 返回底层注册表。
func (rt *Runtime) Registry() *dagmemory.Registry { return rt.reg }

// Store 返回底层 LineStore。
func (rt *Runtime) Store() *dagmemory.LineStore { return rt.store }

// Engine 返回底层引擎。
func (rt *Runtime) Engine() *dagmemory.Engine { return rt.engine }

// Invoker 返回与该 Runtime 绑定的请求式 Invoker。
func (rt *Runtime) Invoker() *invocation.Invoker { return rt.invoker }

// RegisterEntityType 注册实体类型。
func (rt *Runtime) RegisterEntityType(reg *dagv1.EntityTypeRegistration) error {
	return rt.reg.RegisterEntityType(reg)
}

// RegisterEntityTypeSimple 注册实体类型的便捷形式（仅类型名、schema 版本与 payload URL）。
func (rt *Runtime) RegisterEntityTypeSimple(entityType, schemaVersion, payloadTypeURL string) error {
	return rt.reg.RegisterEntityType(&dagv1.EntityTypeRegistration{
		TypeKey:        &dagv1.EntityTypeKey{EntityType: entityType, PayloadSchemaVersion: schemaVersion},
		PayloadTypeUrl: payloadTypeURL,
	})
}

// RegisterGraph 注册图规格。
func (rt *Runtime) RegisterGraph(spec *dagv1.GraphSpec) error {
	return rt.reg.RegisterGraph(spec)
}

// RegisterComputeUnitDef 注册计算单元定义（声明式 HttpUnit 等）。
func (rt *Runtime) RegisterComputeUnitDef(def *dagv1.ComputeUnitDef) error {
	return rt.reg.RegisterComputeUnit(def)
}

// RegisterComputeUnit 注册 Go 计算单元实现，并自动注册最小定义。
//
// 适用于无声明式 implementation 的纯进程内计算单元。
func (rt *Runtime) RegisterComputeUnit(unitID string, typeKey *dagv1.EntityTypeKey, unit dag.ComputeUnit) error {
	if err := rt.reg.RegisterComputeUnit(&dagv1.ComputeUnitDef{
		UnitId:          unitID,
		InputTypeKey:    typeKey,
		OutputTypeKeys:  []*dagv1.EntityTypeKey{typeKey},
		SideEffectClass: dagv1.SideEffectClass_SIDE_EFFECT_NONE,
	}); err != nil {
		return err
	}
	return rt.reg.RegisterComputeUnitImpl(unitID, unit)
}

// RegisterComputeUnitFull 注册含完整 ComputeUnitDef 的 Go 计算单元实现。
func (rt *Runtime) RegisterComputeUnitFull(def *dagv1.ComputeUnitDef, unit dag.ComputeUnit) error {
	if err := rt.reg.RegisterComputeUnit(def); err != nil {
		return err
	}
	return rt.reg.RegisterComputeUnitImpl(def.UnitId, unit)
}

// RegisterComputeUnitImpl 注册已有定义的计算单元实现。
func (rt *Runtime) RegisterComputeUnitImpl(unitID string, unit dag.ComputeUnit) error {
	return rt.reg.RegisterComputeUnitImpl(unitID, unit)
}

// RegisterCompensator 注册补偿器。
func (rt *Runtime) RegisterCompensator(unitID string, comp dag.Compensator) error {
	return rt.reg.RegisterCompensator(unitID, comp)
}

// ResolveGraphForInstance 解析实例绑定的图规格。
func (rt *Runtime) ResolveGraphForInstance(inst *dagv1.EntityInstance) (*dagv1.GraphSpec, error) {
	return rt.reg.ResolveGraphForInstance(inst)
}

// GetLatestGraphVersion 返回图 ID 的最新版本。
func (rt *Runtime) GetLatestGraphVersion(graphID string) (*dagv1.GraphVersion, error) {
	return rt.reg.GetLatestGraphVersion(graphID)
}

// String 返回 Runtime 的简要描述。
func (rt *Runtime) String() string {
	return fmt.Sprintf("memory.Runtime(loaded entity types via Registry)")
}

// Close 关闭运行时。
func (rt *Runtime) Close() error {
	return rt.engine.Close()
}
