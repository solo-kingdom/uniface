package invocation

import (
	"context"
	"errors"
	"fmt"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"google.golang.org/protobuf/types/known/anypb"
)

// InvokeRequest 描述一次请求式 DAG 调用输入。
//
// 字段语义对齐 dagv1.StartInstanceRequest，调用方需提供有效的 Ref、TypeKey
// 与 GraphVersion；InitialPayload 为 nil 时表示空 payload。
type InvokeRequest struct {
	Ref            *dagv1.EntityRef
	TypeKey        *dagv1.EntityTypeKey
	InitialPayload *anypb.Any
	GraphVersion   *dagv1.GraphVersion
	GraphPinPolicy dagv1.GraphPinPolicy
	CorrelationId  string
	Parent         *dagv1.EntityRef
	// Options 透传给 StartInstance 与 DrainInstance（hop 上限、resolver 等）。
	Options []dag.Option
}

// InvokeResult 描述一次请求式 DAG 调用结果。
//
// Instance 永远非 nil（只要 Start 成功）；Snapshot 在可读取时返回，否则为 nil。
// 调用方依据 Instance.Status 决定终态语义：COMPLETED/FAILED/COMPENSATED/CANCELLED
// 为终态，WAITING 表示实例仍在等待外部信号。
type InvokeResult struct {
	Instance *dagv1.EntityInstance
	Snapshot *dagv1.EntitySnapshot
}

// Invoker 组合 dag.Engine 与 dag.LineStore，封装请求式调用路径。
//
// Invoke 执行 StartInstance -> DrainInstance -> GetSnapshot：
//   - 成功排空到终态或 WAITING 时返回对应 Instance 与可读取的 Snapshot，error 为 nil；
//   - DrainInstance 返回 error（上下文取消、hop 上限耗尽、底层错误）时透传该 error，
//     并在可读取当前实例/snapshot 时附带部分结果。
//
// Invoker 不替代 dag.Engine，也不改变 Engine 既有生命周期接口语义。
type Invoker struct {
	engine dag.Engine
	store  dag.LineStore
}

// NewInvoker 创建 Invoker。engine 与 store 不可为 nil。
func NewInvoker(engine dag.Engine, store dag.LineStore) *Invoker {
	if engine == nil {
		panic("invocation: engine must not be nil")
	}
	if store == nil {
		panic("invocation: store must not be nil")
	}
	return &Invoker{engine: engine, store: store}
}

// Engine 返回底层引擎。
func (inv *Invoker) Engine() dag.Engine { return inv.engine }

// Store 返回底层 LineStore。
func (inv *Invoker) Store() dag.LineStore { return inv.store }

// Invoke 执行一次请求式 DAG 调用。
func (inv *Invoker) Invoke(ctx context.Context, req *InvokeRequest) (*InvokeResult, error) {
	if req == nil || req.Ref == nil || req.Ref.EntityId == "" {
		return nil, errors.New("invocation: InvokeRequest.Ref is required")
	}

	startReq := &dagv1.StartInstanceRequest{
		Ref:            req.Ref,
		TypeKey:        req.TypeKey,
		InitialPayload: req.InitialPayload,
		GraphVersion:   req.GraphVersion,
		GraphPinPolicy: req.GraphPinPolicy,
		CorrelationId:  req.CorrelationId,
		Parent:         req.Parent,
	}
	inst, err := inv.engine.StartInstance(ctx, startReq, req.Options...)
	if err != nil {
		return nil, err
	}

	drained, err := inv.engine.DrainInstance(ctx, req.Ref, req.Options...)
	if drained != nil {
		inst = drained
	}
	result := &InvokeResult{Instance: inst}
	if inst != nil {
		if snap, snapErr := inv.store.GetSnapshot(ctx, req.Ref); snapErr == nil {
			result.Snapshot = snap
		}
	}
	return result, err
}

// IsTerminal 报告实例状态是否为终态（COMPLETED/FAILED/COMPENSATED/CANCELLED）。
func IsTerminal(status dagv1.InstanceStatus) bool {
	switch status {
	case dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED,
		dagv1.InstanceStatus_INSTANCE_STATUS_FAILED,
		dagv1.InstanceStatus_INSTANCE_STATUS_COMPENSATED,
		dagv1.InstanceStatus_INSTANCE_STATUS_CANCELLED:
		return true
	default:
		return false
	}
}

// TerminalError 在实例为失败终态时返回错误，便于调用方把终态映射为业务错误。
//
// 成功终态（COMPLETED）与非终态（WAITING/RUNNING 等）返回 nil。
func TerminalError(inst *dagv1.EntityInstance) error {
	if inst == nil {
		return fmt.Errorf("invocation: nil instance")
	}
	switch inst.Status {
	case dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED:
		return nil
	case dagv1.InstanceStatus_INSTANCE_STATUS_CANCELLED:
		return fmt.Errorf("invocation: instance %q cancelled", inst.Ref.GetEntityId())
	case dagv1.InstanceStatus_INSTANCE_STATUS_FAILED:
		return fmt.Errorf("invocation: instance %q failed", inst.Ref.GetEntityId())
	case dagv1.InstanceStatus_INSTANCE_STATUS_COMPENSATED:
		return fmt.Errorf("invocation: instance %q compensated", inst.Ref.GetEntityId())
	default:
		return nil
	}
}
