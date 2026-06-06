package memory

import (
	"context"
	"fmt"
	"time"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"github.com/solo-kingdom/uniface/pkg/dag/entity"
	"github.com/solo-kingdom/uniface/pkg/dag/graph"
	"github.com/solo-kingdom/uniface/pkg/dag/runtime"
)

// Engine 内存 DAG 引擎。
type Engine struct {
	reg      *Registry
	store    *LineStore
	resolver dag.GraphResolver
	opts     *dag.Options
	closed   bool
}

// NewEngine 创建内存引擎。
func NewEngine(reg *Registry, store *LineStore, opts ...dag.Option) *Engine {
	o := dag.MergeOptions(opts...)
	return &Engine{
		reg:      reg,
		store:    store,
		resolver: graph.NewResolver(),
		opts:     o,
	}
}

func (e *Engine) StartInstance(ctx context.Context, req *dagv1.StartInstanceRequest, opts ...dag.Option) (*dagv1.EntityInstance, error) {
	if e.closed {
		return nil, dag.ErrStoreClosed
	}
	if err := entity.ValidateTypeKey(req.TypeKey); err != nil {
		return nil, err
	}
	reg, err := e.reg.ResolveType(req.TypeKey)
	if err != nil {
		return nil, err
	}
	if err := entity.ValidatePayloadTypeURL(reg, req.InitialPayload); err != nil {
		return nil, err
	}
	spec, err := e.reg.GetGraph(req.GraphVersion)
	if err != nil {
		return nil, err
	}
	inst, err := e.store.CreateInstance(ctx, req, spec.EntryNodeId)
	if err != nil {
		return nil, err
	}
	return inst, nil
}

func (e *Engine) GetInstance(ctx context.Context, ref *dagv1.EntityRef) (*dagv1.EntityInstance, error) {
	return e.store.GetInstance(ctx, ref)
}

func (e *Engine) CancelInstance(ctx context.Context, ref *dagv1.EntityRef) error {
	return e.store.UpdateInstanceStatus(ref, dagv1.InstanceStatus_INSTANCE_STATUS_CANCELLED)
}

func (e *Engine) DeliverSignal(ctx context.Context, delivery *dagv1.SignalDelivery) error {
	if delivery == nil || delivery.EntityId == "" {
		return dag.ErrInstanceNotFound
	}
	ref := &dagv1.EntityRef{EntityId: delivery.EntityId}
	inst, err := e.store.GetInstance(ctx, ref)
	if err != nil {
		return err
	}
	if inst.Status != dagv1.InstanceStatus_INSTANCE_STATUS_WAITING {
		return nil
	}
	waiting, err := e.store.GetWaiting(ref)
	if err != nil {
		return err
	}
	if !signalNameAccepted(waiting, delivery.SignalName) {
		return dag.ErrSignalMismatch
	}
	newDelivery, err := e.store.RecordSignalDelivery(ctx, delivery.EntityId, delivery.SignalName, delivery.DeliveryId)
	if err != nil {
		return err
	}
	if !newDelivery {
		return nil
	}
	spec, err := e.reg.GetGraph(inst.GraphVersion)
	if err != nil {
		return err
	}
	node, ok := spec.Nodes[inst.CurrentNodeId]
	if !ok {
		return dag.ErrInvalidGraph
	}
	snap, err := e.store.GetSnapshot(ctx, ref)
	if err != nil {
		return err
	}
	nextNode, err := e.resolver.Resolve(ctx, spec, inst.CurrentNodeId, snap)
	if err != nil {
		return err
	}
	if nextNode == inst.CurrentNodeId && len(node.Transitions) > 0 {
		nextNode = node.Transitions[0].TargetNodeId
	}
	idem := runtime.SignalIdempotencyKey(delivery.EntityId, delivery.SignalName, delivery.DeliveryId)
	return e.store.CommitHop(ctx, &dagv1.HopCommit{
		Ref:            ref,
		NodeId:         inst.CurrentNodeId,
		InputSequence:  inst.Sequence,
		IdempotencyKey: idem,
		NextNodeId:     nextNode,
		NextStatus:     dagv1.InstanceStatus_INSTANCE_STATUS_RUNNING,
		JournalKind:    dagv1.JournalKind_JOURNAL_KIND_SIGNAL_RECEIVED,
		SignalName:     delivery.SignalName,
		DeliveryId:     delivery.DeliveryId,
	})
}

func signalNameAccepted(waiting *dag.WaitingInstance, name string) bool {
	if waiting == nil || waiting.SignalName == "" {
		return name != ""
	}
	return waiting.SignalName == name
}

func (e *Engine) RunOnce(ctx context.Context) error {
	sched := NewScheduler(e.reg, e.store, e.resolver, e.opts)
	if err := sched.processTimeouts(ctx); err != nil {
		return err
	}
	return sched.Tick(ctx)
}

func (e *Engine) Close() error {
	e.closed = true
	return nil
}

// Scheduler hop 调度器。
type Scheduler struct {
	reg      *Registry
	store    *LineStore
	resolver dag.GraphResolver
	opts     *dag.Options
}

func NewScheduler(reg *Registry, store *LineStore, resolver dag.GraphResolver, opts *dag.Options) *Scheduler {
	if opts == nil {
		opts = dag.DefaultOptions()
	}
	return &Scheduler{reg: reg, store: store, resolver: resolver, opts: opts}
}

func (s *Scheduler) Tick(ctx context.Context) error {
	refs, err := s.store.ListRunnableInstances(ctx)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if err := s.processInstance(ctx, ref); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) Close() error {
	return nil
}

func (s *Scheduler) processTimeouts(ctx context.Context) error {
	timeouts, err := s.store.ListWaitingTimeouts(ctx, time.Now())
	if err != nil {
		return err
	}
	for _, w := range timeouts {
		if w.OnTimeoutTargetNodeID == "" {
			continue
		}
		inst, err := s.store.GetInstance(ctx, w.Ref)
		if err != nil {
			return err
		}
		spec, err := s.reg.GetGraph(inst.GraphVersion)
		if err != nil {
			return err
		}
		node, ok := spec.Nodes[w.OnTimeoutTargetNodeID]
		if !ok {
			continue
		}
		status, err := terminalStatusForNode(node)
		if err != nil {
			return err
		}
		idem := runtime.HopIdempotencyKey(w.Ref.EntityId, "timeout", 0)
		if err := s.store.CommitHop(ctx, &dagv1.HopCommit{
			Ref:            w.Ref,
			NodeId:         w.Ref.EntityId,
			InputSequence:  0,
			IdempotencyKey: idem,
			NextNodeId:     w.OnTimeoutTargetNodeID,
			NextStatus:     status,
			JournalKind:    dagv1.JournalKind_JOURNAL_KIND_SIGNAL_RECEIVED,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) processInstance(ctx context.Context, ref *dagv1.EntityRef) error {
	inst, err := s.store.GetInstance(ctx, ref)
	if err != nil {
		return err
	}
	if inst.Status == dagv1.InstanceStatus_INSTANCE_STATUS_CANCELLED {
		return nil
	}
	if inst.Status == dagv1.InstanceStatus_INSTANCE_STATUS_COMPENSATING {
		return s.processCompensation(ctx, inst)
	}
	spec, err := s.reg.GetGraph(inst.GraphVersion)
	if err != nil {
		return err
	}
	node, ok := spec.Nodes[inst.CurrentNodeId]
	if !ok {
		return fmt.Errorf("%w: node %q", dag.ErrInvalidGraph, inst.CurrentNodeId)
	}
	switch node.Kind {
	case dagv1.NodeKind_NODE_KIND_TERMINAL:
		return s.commitTerminal(ctx, inst, node)
	case dagv1.NodeKind_NODE_KIND_WAIT:
		return s.enterWait(ctx, inst, node, nil)
	case dagv1.NodeKind_NODE_KIND_JOIN:
		return s.processJoin(ctx, inst, spec, node)
	default:
		return s.processCompute(ctx, inst, spec, node)
	}
}

func (s *Scheduler) processCompute(ctx context.Context, inst *dagv1.EntityInstance, spec *dagv1.GraphSpec, node *dagv1.NodeDef) error {
	snap, err := s.store.GetSnapshot(ctx, inst.Ref)
	if err != nil {
		return err
	}
	unitDef, err := s.reg.GetComputeUnit(node.UnitId)
	if err != nil {
		return err
	}
	if !entity.TypeKeyEqual(snap.TypeKey, unitDef.InputTypeKey) {
		return dag.ErrTypeMismatch
	}
	idem := runtime.HopIdempotencyKey(inst.Ref.EntityId, node.NodeId, inst.Sequence)
	existing, err := s.store.GetExecution(ctx, idem)
	if err != nil {
		return err
	}
	if existing != nil && existing.Status == dagv1.ExecutionStatus_EXECUTION_STATUS_COMMITTED {
		return s.advanceAfterCommit(ctx, inst, spec, node, snap)
	}
	record := &dagv1.ExecutionRecord{
		IdempotencyKey: idem,
		EntityId:       inst.Ref.EntityId,
		NodeId:         node.NodeId,
		InputSequence:  inst.Sequence,
		Status:         dagv1.ExecutionStatus_EXECUTION_STATUS_RUNNING,
	}
	record, err = s.store.CreateExecution(ctx, record)
	if err != nil {
		return err
	}
	if record.Status == dagv1.ExecutionStatus_EXECUTION_STATUS_COMMITTED {
		return s.advanceAfterCommit(ctx, inst, spec, node, snap)
	}
	unit, err := s.reg.GetComputeUnitImpl(node.UnitId)
	if err != nil {
		return err
	}
	mutation, err := unit.Execute(ctx, snap)
	if err != nil {
		return s.handleExecuteError(ctx, inst, node, record, err)
	}
	return s.applyMutation(ctx, inst, spec, node, snap, mutation, idem)
}

func (s *Scheduler) handleExecuteError(ctx context.Context, inst *dagv1.EntityInstance, node *dagv1.NodeDef,
	record *dagv1.ExecutionRecord, execErr error) error {
	maxAttempts := s.opts.DefaultRetryPolicy.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	if record.Attempt >= int32(maxAttempts) {
		return execErr
	}
	record.Attempt++
	return nil
}

func (s *Scheduler) applyMutation(ctx context.Context, inst *dagv1.EntityInstance, spec *dagv1.GraphSpec, node *dagv1.NodeDef,
	snap *dagv1.EntitySnapshot, mutation *dagv1.EntityMutation, idem string) error {
	if mutation == nil {
		return fmt.Errorf("nil mutation")
	}
	switch m := mutation.Intent.(type) {
	case *dagv1.EntityMutation_Wait:
		return s.enterWait(ctx, inst, node, m.Wait)
	case *dagv1.EntityMutation_Fail:
		return s.handleFail(ctx, inst, spec, node, m.Fail)
	case *dagv1.EntityMutation_Spawn:
		return s.commitCompute(ctx, inst, spec, node, snap, mutation, idem)
	case *dagv1.EntityMutation_Update:
		return s.commitComputeWithSnapshot(ctx, inst, spec, node, m.Update, mutation, idem)
	case *dagv1.EntityMutation_Complete:
		return s.commitTerminalOutcome(ctx, inst, spec, node, m.Complete, idem)
	default:
		return s.commitCompute(ctx, inst, spec, node, snap, mutation, idem)
	}
}

func (s *Scheduler) commitCompute(ctx context.Context, inst *dagv1.EntityInstance, spec *dagv1.GraphSpec, node *dagv1.NodeDef,
	outSnap *dagv1.EntitySnapshot, mutation *dagv1.EntityMutation, idem string) error {
	if outSnap == nil {
		if u, ok := mutation.Intent.(*dagv1.EntityMutation_Update); ok {
			outSnap = u.Update
		} else {
			cur, err := s.store.GetSnapshot(ctx, inst.Ref)
			if err != nil {
				return err
			}
			outSnap = cur
		}
	}
	if outSnap != nil {
		outSnap = entity.CloneSnapshot(outSnap)
		outSnap.Sequence = inst.Sequence + 1
	}
	nextNode, err := s.resolver.Resolve(ctx, spec, node.NodeId, outSnap)
	if err != nil {
		return err
	}
	nextStatus := dagv1.InstanceStatus_INSTANCE_STATUS_RUNNING
	if nextNode != "" {
		if n, ok := spec.Nodes[nextNode]; ok && n.Kind == dagv1.NodeKind_NODE_KIND_TERMINAL {
			st, err := terminalStatusForNode(n)
			if err != nil {
				return err
			}
			nextStatus = st
		}
	}
	var sagaDelta *dagv1.SagaState
	if node.CompensatorUnitId != "" && node.Kind == dagv1.NodeKind_NODE_KIND_COMPUTE {
		frame := &dagv1.CompensationFrame{
			NodeId:            node.NodeId,
			UnitId:            node.UnitId,
			CompensatorUnitId: node.CompensatorUnitId,
			ForwardSequence:   inst.Sequence + 1,
		}
		sagaDelta = &dagv1.SagaState{Stack: []*dagv1.CompensationFrame{frame}}
	}
	var spawned []*dagv1.EntityRef
	if sp, ok := mutation.Intent.(*dagv1.EntityMutation_Spawn); ok {
		for _, spec := range sp.Spawn.Specs {
			if err := entity.ValidateSpawnSpec(spec); err != nil {
				return err
			}
			childReq := &dagv1.StartInstanceRequest{
				Ref:            spec.Ref,
				TypeKey:        spec.TypeKey,
				InitialPayload: spec.InitialPayload,
				GraphVersion:   spec.Graph,
				GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_START,
				CorrelationId:  spec.CorrelationId,
				Parent:         inst.Ref,
			}
			childGraph, err := s.reg.GetGraph(spec.Graph)
			if err != nil {
				return err
			}
			if _, err := s.store.CreateInstance(ctx, childReq, childGraph.EntryNodeId); err != nil {
				return err
			}
			spawned = append(spawned, spec.Ref)
		}
	}
	if err := s.store.CommitHop(ctx, &dagv1.HopCommit{
		Ref:            inst.Ref,
		NodeId:         node.NodeId,
		InputSequence:  inst.Sequence,
		IdempotencyKey: idem,
		OutputSnapshot: outSnap,
		NextNodeId:     nextNode,
		NextStatus:     nextStatus,
		SagaDelta:      sagaDelta,
		Spawned:        spawned,
		JournalKind:    dagv1.JournalKind_JOURNAL_KIND_NODE_COMMITTED,
	}); err != nil {
		return err
	}
	updated, _ := s.store.GetInstance(ctx, inst.Ref)
	if updated.Status == dagv1.InstanceStatus_INSTANCE_STATUS_RUNNING && nextNode != "" {
		if n, ok := spec.Nodes[nextNode]; ok {
			switch n.Kind {
			case dagv1.NodeKind_NODE_KIND_WAIT:
				return s.enterWait(ctx, updated, n, waitSignalFromNode(n))
			}
		}
	}
	return nil
}

func (s *Scheduler) enterWait(ctx context.Context, inst *dagv1.EntityInstance, node *dagv1.NodeDef, wait *dagv1.WaitSignal) error {
	signalName := ""
	var deadline time.Time
	var onTimeout string
	var accepted []string
	if wait != nil {
		signalName = wait.SignalName
		accepted = wait.AcceptedSignals
		onTimeout = wait.OnTimeoutTargetNodeId
		if wait.Deadline != nil {
			deadline = wait.Deadline.AsTime()
		}
	}
	if node != nil && node.WaitConfig != nil {
		if signalName == "" {
			signalName = node.WaitConfig.SignalName
		}
		accepted = node.WaitConfig.AcceptedSignals
		onTimeout = node.WaitConfig.OnTimeoutTargetNodeId
		if node.WaitConfig.DefaultDeadlineSeconds > 0 && deadline.IsZero() {
			deadline = time.Now().Add(time.Duration(node.WaitConfig.DefaultDeadlineSeconds) * time.Second)
		}
	}
	w := &dag.WaitingInstance{
		Ref:                   inst.Ref,
		Deadline:              deadline,
		OnTimeoutTargetNodeID: onTimeout,
		SignalName:            signalName,
	}
	_ = accepted
	return s.store.SetWaiting(inst.Ref, node.NodeId, w)
}

func (s *Scheduler) processJoin(ctx context.Context, inst *dagv1.EntityInstance, spec *dagv1.GraphSpec, node *dagv1.NodeDef) error {
	if node.JoinSpec == nil {
		return dag.ErrInvalidGraph
	}
	ok, failChild, err := s.checkJoinBarriers(ctx, inst, node.JoinSpec)
	if err != nil {
		return err
	}
	if failChild && node.JoinSpec.FailParentOnChildFailure {
		return s.store.UpdateInstanceStatus(inst.Ref, dagv1.InstanceStatus_INSTANCE_STATUS_FAILED)
	}
	if !ok {
		return nil
	}
	snap, err := s.store.GetSnapshot(ctx, inst.Ref)
	if err != nil {
		return err
	}
	nextNode, err := graph.ResolveTransitions(node, snap)
	if err != nil {
		return err
	}
	idem := runtime.HopIdempotencyKey(inst.Ref.EntityId, node.NodeId, inst.Sequence)
	return s.store.CommitHop(ctx, &dagv1.HopCommit{
		Ref:            inst.Ref,
		NodeId:         node.NodeId,
		InputSequence:  inst.Sequence,
		IdempotencyKey: idem,
		OutputSnapshot: snap,
		NextNodeId:     nextNode,
		NextStatus:     dagv1.InstanceStatus_INSTANCE_STATUS_RUNNING,
		JournalKind:    dagv1.JournalKind_JOURNAL_KIND_JOIN_COMMITTED,
	})
}

func (s *Scheduler) checkJoinBarriers(ctx context.Context, inst *dagv1.EntityInstance, join *dagv1.JoinSpec) (ready bool, childFailed bool, err error) {
	completed := 0
	for _, b := range join.Barriers {
		childID := ""
		switch t := b.Target.(type) {
		case *dagv1.JoinBarrier_ChildEntityId:
			childID = t.ChildEntityId
		case *dagv1.JoinBarrier_CorrelationId:
			childID, err = s.findChildByCorrelation(ctx, inst.Ref, t.CorrelationId)
			if err != nil {
				return false, false, err
			}
		}
		if childID == "" {
			return false, false, nil
		}
		child, err := s.store.GetInstance(ctx, &dagv1.EntityRef{EntityId: childID})
		if err != nil {
			return false, false, nil
		}
		switch child.Status {
		case dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED:
			completed++
		case dagv1.InstanceStatus_INSTANCE_STATUS_FAILED, dagv1.InstanceStatus_INSTANCE_STATUS_COMPENSATED:
			if join.FailParentOnChildFailure {
				return false, true, nil
			}
		default:
			return false, false, nil
		}
	}
	switch join.Policy {
	case dagv1.JoinPolicy_JOIN_ANY_SUCCESS:
		return completed > 0, false, nil
	default:
		return completed == len(join.Barriers), false, nil
	}
}

func (s *Scheduler) findChildByCorrelation(ctx context.Context, parent *dagv1.EntityRef, correlationID string) (string, error) {
	ref, err := s.store.FindChildByCorrelation(parent, correlationID)
	if err != nil {
		return "", err
	}
	return ref.EntityId, nil
}

func (s *Scheduler) handleFail(ctx context.Context, inst *dagv1.EntityInstance, spec *dagv1.GraphSpec, node *dagv1.NodeDef, fail *dagv1.FailIntent) error {
	if fail != nil && fail.TriggerCompensation {
		return s.store.UpdateInstanceStatus(inst.Ref, dagv1.InstanceStatus_INSTANCE_STATUS_COMPENSATING)
	}
	return s.store.UpdateInstanceStatus(inst.Ref, dagv1.InstanceStatus_INSTANCE_STATUS_FAILED)
}

func (s *Scheduler) processCompensation(ctx context.Context, inst *dagv1.EntityInstance) error {
	saga, err := s.store.GetSagaState(ctx, inst.Ref)
	if err != nil {
		return err
	}
	if len(saga.Stack) == 0 {
		return s.store.UpdateInstanceStatus(inst.Ref, dagv1.InstanceStatus_INSTANCE_STATUS_COMPENSATED)
	}
	frame := saga.Stack[len(saga.Stack)-1]
	idem := runtime.CompensationIdempotencyKey(inst.Ref.EntityId, frame.ForwardSequence, frame.CompensatorUnitId)
	journal, _ := s.store.ListJournal(ctx, inst.Ref)
	for _, j := range journal {
		if j.IdempotencyKey == idem {
			if _, err := s.store.PopSagaFrame(inst.Ref); err != nil {
				return err
			}
			return nil
		}
	}
	if _, err := s.store.PopSagaFrame(inst.Ref); err != nil {
		return err
	}
	comp, err := s.reg.GetCompensator(frame.CompensatorUnitId)
	if err != nil {
		return err
	}
	snap, err := s.store.GetSnapshot(ctx, inst.Ref)
	if err != nil {
		return err
	}
	compCtx := &dagv1.CompensationContext{
		EntityId:        inst.Ref.EntityId,
		NodeId:          frame.NodeId,
		ForwardSequence: frame.ForwardSequence,
		Snapshot:        snap.Payload,
	}
	if err := comp.Compensate(ctx, compCtx); err != nil {
		return err
	}
	return s.store.CommitHop(ctx, &dagv1.HopCommit{
		Ref:            inst.Ref,
		NodeId:         frame.NodeId,
		InputSequence:  frame.ForwardSequence,
		IdempotencyKey: idem,
		NextNodeId:     inst.CurrentNodeId,
		NextStatus:     dagv1.InstanceStatus_INSTANCE_STATUS_COMPENSATING,
		JournalKind:    dagv1.JournalKind_JOURNAL_KIND_COMPENSATION_COMMITTED,
	})
}

func (s *Scheduler) commitTerminal(ctx context.Context, inst *dagv1.EntityInstance, node *dagv1.NodeDef) error {
	status, err := terminalStatusForNode(node)
	if err != nil {
		return err
	}
	idem := runtime.HopIdempotencyKey(inst.Ref.EntityId, node.NodeId, inst.Sequence)
	return s.store.CommitHop(ctx, &dagv1.HopCommit{
		Ref:            inst.Ref,
		NodeId:         node.NodeId,
		InputSequence:  inst.Sequence,
		IdempotencyKey: idem,
		NextNodeId:     node.NodeId,
		NextStatus:     status,
		JournalKind:    dagv1.JournalKind_JOURNAL_KIND_NODE_COMMITTED,
	})
}

func (s *Scheduler) commitTerminalOutcome(ctx context.Context, inst *dagv1.EntityInstance, spec *dagv1.GraphSpec, node *dagv1.NodeDef,
	outcome dagv1.TerminalOutcome, idem string) error {
	status := dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED
	if outcome == dagv1.TerminalOutcome_TERMINAL_OUTCOME_FAILURE {
		status = dagv1.InstanceStatus_INSTANCE_STATUS_FAILED
	}
	return s.store.CommitHop(ctx, &dagv1.HopCommit{
		Ref:            inst.Ref,
		NodeId:         node.NodeId,
		InputSequence:  inst.Sequence,
		IdempotencyKey: idem,
		NextNodeId:     node.NodeId,
		NextStatus:     status,
		JournalKind:    dagv1.JournalKind_JOURNAL_KIND_NODE_COMMITTED,
	})
}

func (s *Scheduler) advanceAfterCommit(ctx context.Context, inst *dagv1.EntityInstance, spec *dagv1.GraphSpec, node *dagv1.NodeDef, snap *dagv1.EntitySnapshot) error {
	nextNode, err := s.resolver.Resolve(ctx, spec, node.NodeId, snap)
	if err != nil {
		return err
	}
	updated, err := s.store.GetInstance(ctx, inst.Ref)
	if err != nil {
		return err
	}
	updated.CurrentNodeId = nextNode
	if n, ok := spec.Nodes[nextNode]; ok && n.Kind == dagv1.NodeKind_NODE_KIND_TERMINAL {
		return s.commitTerminal(ctx, updated, n)
	}
	return nil
}

func terminalStatusForNode(node *dagv1.NodeDef) (dagv1.InstanceStatus, error) {
	if node == nil {
		return dagv1.InstanceStatus_INSTANCE_STATUS_UNSPECIFIED, dag.ErrInvalidGraph
	}
	switch node.TerminalOutcome {
	case dagv1.TerminalOutcome_TERMINAL_OUTCOME_SUCCESS:
		return dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED, nil
	case dagv1.TerminalOutcome_TERMINAL_OUTCOME_FAILURE:
		return dagv1.InstanceStatus_INSTANCE_STATUS_FAILED, nil
	default:
		return dagv1.InstanceStatus_INSTANCE_STATUS_FAILED, nil
	}
}

func waitSignalFromNode(n *dagv1.NodeDef) *dagv1.WaitSignal {
	if n == nil || n.WaitConfig == nil {
		return nil
	}
	return &dagv1.WaitSignal{
		SignalName:            n.WaitConfig.SignalName,
		AcceptedSignals:       n.WaitConfig.AcceptedSignals,
		OnTimeoutTargetNodeId: n.WaitConfig.OnTimeoutTargetNodeId,
	}
}

func (s *Scheduler) commitComputeWithSnapshot(ctx context.Context, inst *dagv1.EntityInstance, spec *dagv1.GraphSpec, node *dagv1.NodeDef,
	outSnap *dagv1.EntitySnapshot, mutation *dagv1.EntityMutation, idem string) error {
	return s.commitCompute(ctx, inst, spec, node, outSnap, mutation, idem)
}
