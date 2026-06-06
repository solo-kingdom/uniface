// Package dag 提供实体实例 DAG 执行引擎抽象。
package dag

import (
	"context"
	"time"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
)

// ComputeUnit 计算单元，业务实现 Execute。
type ComputeUnit interface {
	Execute(ctx context.Context, snapshot *dagv1.EntitySnapshot) (*dagv1.EntityMutation, error)
}

// Compensator 补偿单元，业务实现 Compensate。
type Compensator interface {
	Compensate(ctx context.Context, comp *dagv1.CompensationContext) error
}

// GraphResolver 根据当前快照与节点定义解析下一跳。
type GraphResolver interface {
	Resolve(ctx context.Context, graph *dagv1.GraphSpec, nodeID string, snapshot *dagv1.EntitySnapshot) (string, error)
}

// Registry 类型、图与计算单元注册表。
type Registry interface {
	RegisterEntityType(reg *dagv1.EntityTypeRegistration) error
	ResolveType(key *dagv1.EntityTypeKey) (*dagv1.EntityTypeRegistration, error)
	RegisterGraph(spec *dagv1.GraphSpec) error
	GetGraph(version *dagv1.GraphVersion) (*dagv1.GraphSpec, error)
	RegisterComputeUnit(def *dagv1.ComputeUnitDef) error
	GetComputeUnit(unitID string) (*dagv1.ComputeUnitDef, error)
	RegisterComputeUnitImpl(unitID string, unit ComputeUnit) error
	GetComputeUnitImpl(unitID string) (ComputeUnit, error)
	RegisterCompensator(unitID string, comp Compensator) error
	GetCompensator(unitID string) (Compensator, error)
	Close() error
}

// LineStore 实例线持久化存储。
type LineStore interface {
	CreateInstance(ctx context.Context, req *dagv1.StartInstanceRequest, entryNodeID string) (*dagv1.EntityInstance, error)
	GetInstance(ctx context.Context, ref *dagv1.EntityRef) (*dagv1.EntityInstance, error)
	GetSnapshot(ctx context.Context, ref *dagv1.EntityRef) (*dagv1.EntitySnapshot, error)
	CreateExecution(ctx context.Context, record *dagv1.ExecutionRecord) (*dagv1.ExecutionRecord, error)
	GetExecution(ctx context.Context, idempotencyKey string) (*dagv1.ExecutionRecord, error)
	CommitHop(ctx context.Context, commit *dagv1.HopCommit) error
	ListJournal(ctx context.Context, ref *dagv1.EntityRef) ([]*dagv1.LineJournalEntry, error)
	GetSagaState(ctx context.Context, ref *dagv1.EntityRef) (*dagv1.SagaState, error)
	RecordSignalDelivery(ctx context.Context, entityID, signalName, deliveryID string) (bool, error)
	ListRunnableInstances(ctx context.Context) ([]*dagv1.EntityRef, error)
	ListWaitingTimeouts(ctx context.Context, now time.Time) ([]*WaitingInstance, error)
	Close() error
}

// WaitingInstance 等待中的实例及其超时配置。
type WaitingInstance struct {
	Ref                   *dagv1.EntityRef
	Deadline              time.Time
	OnTimeoutTargetNodeID string
	SignalName            string
}

// Engine DAG 执行引擎。
type Engine interface {
	StartInstance(ctx context.Context, req *dagv1.StartInstanceRequest, opts ...Option) (*dagv1.EntityInstance, error)
	GetInstance(ctx context.Context, ref *dagv1.EntityRef) (*dagv1.EntityInstance, error)
	CancelInstance(ctx context.Context, ref *dagv1.EntityRef) error
	DeliverSignal(ctx context.Context, delivery *dagv1.SignalDelivery) error
	RunOnce(ctx context.Context) error
	Close() error
}

// Scheduler 调度就绪实例 hop。
type Scheduler interface {
	Tick(ctx context.Context) error
	Close() error
}
