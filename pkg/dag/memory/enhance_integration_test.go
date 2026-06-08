package memory

import (
	"context"
	"sync/atomic"
	"testing"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"github.com/solo-kingdom/uniface/pkg/dag/graph"
	"github.com/solo-kingdom/uniface/pkg/dag/testpb"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestSignalApprovalBranch(t *testing.T) {
	_, _, eng, _, _ := setupSignalBranchGraph(t)
	ctx := context.Background()
	ref := &dagv1.EntityRef{EntityId: "order-approval"}
	startInstance(t, eng, ref, "signal-branch", "v1")

	runUntilWaiting(t, eng, ref)
	approved, _ := anypb.New(&testpb.Order{Approved: true})
	if err := eng.DeliverSignal(ctx, &dagv1.SignalDelivery{
		EntityId: ref.EntityId, SignalName: "approval", DeliveryId: "D1", Payload: approved,
	}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		_ = eng.RunOnce(ctx)
		inst, _ := eng.GetInstance(ctx, ref)
		if inst.CurrentNodeId == "term_success" && inst.Status == dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED {
			return
		}
	}
	t.Fatal("expected approval branch to term_success")
}

func TestSignalMergeDisabled(t *testing.T) {
	reg := NewRegistry()
	store := NewLineStore()
	eng := NewEngine(reg, store)
	typeKey := &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema}
	_ = reg.RegisterEntityType(&dagv1.EntityTypeRegistration{TypeKey: typeKey, PayloadTypeUrl: orderTypeURL})

	falseVal := false
	always := &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}}
	approvedPred := &dagv1.SignalPredicate{
		SignalName:       "approval",
		PayloadPredicate: &dagv1.FieldPredicate{FieldPath: "Approved", Op: dagv1.CompareOp_COMPARE_OP_EQ, Value: "true"},
	}
	spec := &dagv1.GraphSpec{
		Version:     &dagv1.GraphVersion{GraphId: "no-merge", Version: "v1"},
		EntryNodeId: "wait",
		Nodes: map[string]*dagv1.NodeDef{
			"wait": {
				NodeId: "wait", Kind: dagv1.NodeKind_NODE_KIND_WAIT,
				WaitConfig: &dagv1.WaitNodeConfig{SignalName: "approval", MergeSignalPayload: &falseVal},
				Transitions: []*dagv1.Transition{
					{TargetNodeId: "term_success", Condition: &dagv1.Condition{Kind: &dagv1.Condition_SignalPredicate{SignalPredicate: approvedPred}}, Priority: 10},
					{TargetNodeId: "term_failure", Condition: always, Priority: 0},
				},
			},
			"term_success": {NodeId: "term_success", Kind: dagv1.NodeKind_NODE_KIND_TERMINAL, TerminalOutcome: dagv1.TerminalOutcome_TERMINAL_OUTCOME_SUCCESS},
			"term_failure": {NodeId: "term_failure", Kind: dagv1.NodeKind_NODE_KIND_TERMINAL, TerminalOutcome: dagv1.TerminalOutcome_TERMINAL_OUTCOME_FAILURE},
		},
	}
	if err := reg.RegisterGraph(spec); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	ref := &dagv1.EntityRef{EntityId: "order-no-merge"}
	startInstance(t, eng, ref, "no-merge", "v1")
	runUntilWaiting(t, eng, ref)
	seqBefore, _ := eng.GetInstance(ctx, ref)
	approved, _ := anypb.New(&testpb.Order{Approved: true})
	if err := eng.DeliverSignal(ctx, &dagv1.SignalDelivery{
		EntityId: ref.EntityId, SignalName: "approval", DeliveryId: "D1", Payload: approved,
	}); err != nil {
		t.Fatal(err)
	}
	seqAfter, _ := eng.GetInstance(ctx, ref)
	if seqBefore.Sequence != seqAfter.Sequence {
		t.Fatalf("merge disabled should not increment sequence: before=%d after=%d", seqBefore.Sequence, seqAfter.Sequence)
	}
	inst, _ := eng.GetInstance(ctx, ref)
	if inst.CurrentNodeId != "term_failure" {
		t.Fatalf("without merge payload predicate should miss, got node %q", inst.CurrentNodeId)
	}
	_ = store
}

func TestDynamicJoinAllSuccess(t *testing.T) {
	reg, store, eng, _, _ := setupDynamicJoinGraph(t, dagv1.JoinPolicy_JOIN_ALL_SUCCESS)
	sched := NewScheduler(reg, store, graph.NewResolver(), dag.DefaultOptions())
	ctx := context.Background()
	ref := &dagv1.EntityRef{EntityId: "order-dj"}
	startInstance(t, eng, ref, "dynamic-join", "v1")

	for i := 0; i < 30; i++ {
		_ = eng.RunOnce(ctx)
		inst, _ := eng.GetInstance(ctx, ref)
		if inst.CurrentNodeId == "join" {
			break
		}
	}
	if err := store.UpdateInstanceStatus(&dagv1.EntityRef{EntityId: "pay-1"}, dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED); err != nil {
		t.Fatal(err)
	}
	if err := sched.processInstance(ctx, ref); err != nil {
		t.Fatal(err)
	}
	inst, _ := eng.GetInstance(ctx, ref)
	if inst.CurrentNodeId == "term_success" || inst.Status == dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED {
		t.Fatal("join should wait for all children")
	}
	for _, id := range []string{"pay-2", "pay-3"} {
		if err := store.UpdateInstanceStatus(&dagv1.EntityRef{EntityId: id}, dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 30; i++ {
		_ = eng.RunOnce(ctx)
		inst, _ = eng.GetInstance(ctx, ref)
		if inst.Status == dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED {
			return
		}
	}
	journal, _ := store.ListJournal(ctx, ref)
	for _, e := range journal {
		if e.Kind == dagv1.JournalKind_JOURNAL_KIND_JOIN_COMMITTED {
			return
		}
	}
	t.Fatal("expected join committed after all children complete")
}

func TestDynamicJoinAnySuccess(t *testing.T) {
	_, _, eng, _, _ := setupDynamicJoinGraph(t, dagv1.JoinPolicy_JOIN_ANY_SUCCESS)
	ctx := context.Background()
	ref := &dagv1.EntityRef{EntityId: "order-dj-any"}
	startInstance(t, eng, ref, "dynamic-join", "v1")

	for i := 0; i < 30; i++ {
		_ = eng.RunOnce(ctx)
		inst, _ := eng.GetInstance(ctx, ref)
		if inst.CurrentNodeId == "join" {
			break
		}
	}
	for i := 0; i < 30; i++ {
		_ = eng.RunOnce(ctx)
		inst, _ := eng.GetInstance(ctx, ref)
		if inst.Status == dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED {
			return
		}
	}
	t.Fatal("JOIN_ANY_SUCCESS should proceed after one child completes")
}

func TestErrNoTransitionFailsInstance(t *testing.T) {
	reg := NewRegistry()
	store := NewLineStore()
	sched := NewScheduler(reg, store, graph.NewResolver(), dag.DefaultOptions())
	typeKey := &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema}
	_ = reg.RegisterEntityType(&dagv1.EntityTypeRegistration{TypeKey: typeKey, PayloadTypeUrl: orderTypeURL})
	validate := &validateUnit{}
	_ = reg.RegisterComputeUnit(&dagv1.ComputeUnitDef{UnitId: "order.validate", InputTypeKey: typeKey, OutputTypeKeys: []*dagv1.EntityTypeKey{typeKey}})
	_ = reg.RegisterComputeUnitImpl("order.validate", validate)

	pred := &dagv1.FieldPredicate{FieldPath: "Amount", Op: dagv1.CompareOp_COMPARE_OP_GT, Value: "999999"}
	spec := &dagv1.GraphSpec{
		Version:     &dagv1.GraphVersion{GraphId: "runtime-no-route", Version: "v1"},
		EntryNodeId: "route",
		Nodes: map[string]*dagv1.NodeDef{
			"route": {
				NodeId: "route", Kind: dagv1.NodeKind_NODE_KIND_COMPUTE, UnitId: "order.validate",
				Transitions: []*dagv1.Transition{
					{TargetNodeId: "term_success", Condition: &dagv1.Condition{Kind: &dagv1.Condition_FieldPredicate{FieldPredicate: pred}}},
				},
			},
			"term_success": {NodeId: "term_success", Kind: dagv1.NodeKind_NODE_KIND_TERMINAL, TerminalOutcome: dagv1.TerminalOutcome_TERMINAL_OUTCOME_SUCCESS},
		},
	}
	ctx := context.Background()
	ref := &dagv1.EntityRef{EntityId: "no-route"}
	req := &dagv1.StartInstanceRequest{
		Ref: ref, TypeKey: typeKey, InitialPayload: orderAny(&testpb.Order{Amount: 1}),
		GraphVersion: spec.Version,
	}
	if _, err := store.CreateInstance(ctx, req, "route"); err != nil {
		t.Fatal(err)
	}
	inst, _ := store.GetInstance(ctx, ref)
	node := spec.Nodes["route"]
	if err := sched.processCompute(ctx, inst, spec, node); err != nil {
		t.Fatal(err)
	}
	updated, _ := store.GetInstance(ctx, ref)
	if updated.Status != dagv1.InstanceStatus_INSTANCE_STATUS_FAILED {
		t.Fatalf("expected FAILED, got %v", updated.Status)
	}
	journal, _ := store.ListJournal(ctx, ref)
	for _, e := range journal {
		if e.FailureReason != "" {
			return
		}
	}
	t.Fatal("expected failure_reason in journal")
}

func TestUnitRetryPolicy(t *testing.T) {
	_, store, sched, unit := setupRetryUnit(t, 5)
	ctx := context.Background()
	ref := &dagv1.EntityRef{EntityId: "retry-unit"}
	req := &dagv1.StartInstanceRequest{
		Ref: ref, TypeKey: &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema},
		GraphVersion: &dagv1.GraphVersion{GraphId: "retry", Version: "v1"},
	}
	if _, err := store.CreateInstance(ctx, req, "compute"); err != nil {
		t.Fatal(err)
	}
	inst, _ := store.GetInstance(ctx, ref)
	node := &dagv1.NodeDef{NodeId: "compute", Kind: dagv1.NodeKind_NODE_KIND_COMPUTE, UnitId: "order.retry"}
	spec := &dagv1.GraphSpec{
		EntryNodeId: "compute",
		Nodes: map[string]*dagv1.NodeDef{
			"compute":      node,
			"term_success": {NodeId: "term_success", Kind: dagv1.NodeKind_NODE_KIND_TERMINAL, TerminalOutcome: dagv1.TerminalOutcome_TERMINAL_OUTCOME_SUCCESS},
		},
	}
	var lastErr error
	for i := 0; i < 10; i++ {
		lastErr = sched.processCompute(ctx, inst, spec, node)
		if lastErr != nil {
			break
		}
		inst, _ = store.GetInstance(ctx, ref)
	}
	if lastErr == nil {
		t.Fatal("expected error after retries exhausted")
	}
	if unit.execCount.Load() < 5 {
		t.Fatalf("expected at least 5 attempts with unit max_attempts=5, got %d", unit.execCount.Load())
	}
}

func TestCompensationContinuousPop(t *testing.T) {
	reg, store, sched, _, refund := setupCompensationStack(t)
	ctx := context.Background()
	ref := &dagv1.EntityRef{EntityId: "comp-stack"}
	req := &dagv1.StartInstanceRequest{
		Ref:            ref,
		TypeKey:        &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema},
		GraphVersion:   &dagv1.GraphVersion{GraphId: "comp-graph", Version: "v1"},
		GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_START,
	}
	if _, err := store.CreateInstance(ctx, req, "charge"); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateInstanceStatus(ref, dagv1.InstanceStatus_INSTANCE_STATUS_COMPENSATING); err != nil {
		t.Fatal(err)
	}
	for _, frame := range []*dagv1.CompensationFrame{
		{NodeId: "validate", UnitId: "order.validate", CompensatorUnitId: "order.refund", ForwardSequence: 1},
		{NodeId: "charge", UnitId: "order.charge", CompensatorUnitId: "order.refund", ForwardSequence: 2},
	} {
		if err := store.PushSagaFrame(ref, frame); err != nil {
			t.Fatal(err)
		}
	}
	inst, _ := store.GetInstance(ctx, ref)
	if err := sched.processCompensation(ctx, inst); err != nil {
		t.Fatal(err)
	}
	if refund.count.Load() != 2 {
		t.Fatalf("expected 2 compensations in one tick, got %d", refund.count.Load())
	}
	saga, _ := store.GetSagaState(ctx, ref)
	if len(saga.Stack) != 0 {
		t.Fatalf("expected empty stack, got %d", len(saga.Stack))
	}
	updated, _ := store.GetInstance(ctx, ref)
	if updated.Status != dagv1.InstanceStatus_INSTANCE_STATUS_FAILED {
		t.Fatalf("expected FAILED after compensation terminal route, got %v", updated.Status)
	}
	if updated.CurrentNodeId != "term_failure" {
		t.Fatalf("expected term_failure node, got %q", updated.CurrentNodeId)
	}
	_ = reg
}

func setupSignalBranchGraph(t interface {
	Helper()
	Fatal(...any)
}) (*Registry, *LineStore, *Engine, *chargeUnit, *refundCompensator) {
	t.Helper()
	reg := NewRegistry()
	store := NewLineStore()
	eng := NewEngine(reg, store)
	typeKey := &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema}
	if err := reg.RegisterEntityType(&dagv1.EntityTypeRegistration{TypeKey: typeKey, PayloadTypeUrl: orderTypeURL}); err != nil {
		t.Fatal(err)
	}
	always := &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}}
	approvedPred := &dagv1.SignalPredicate{
		SignalName: "approval",
		PayloadPredicate: &dagv1.FieldPredicate{
			FieldPath: "Approved", Op: dagv1.CompareOp_COMPARE_OP_EQ, Value: "true",
		},
	}
	rejectedPred := &dagv1.SignalPredicate{
		SignalName: "approval",
		PayloadPredicate: &dagv1.FieldPredicate{
			FieldPath: "Approved", Op: dagv1.CompareOp_COMPARE_OP_EQ, Value: "false",
		},
	}
	spec := &dagv1.GraphSpec{
		Version:     &dagv1.GraphVersion{GraphId: "signal-branch", Version: "v1"},
		EntryNodeId: "validate",
		Nodes: map[string]*dagv1.NodeDef{
			"validate": {
				NodeId: "validate", Kind: dagv1.NodeKind_NODE_KIND_COMPUTE, UnitId: "order.validate",
				Transitions: []*dagv1.Transition{{TargetNodeId: "wait_approval", Condition: always}},
			},
			"wait_approval": {
				NodeId: "wait_approval", Kind: dagv1.NodeKind_NODE_KIND_WAIT,
				WaitConfig: &dagv1.WaitNodeConfig{SignalName: "approval"},
				Transitions: []*dagv1.Transition{
					{TargetNodeId: "term_success", Condition: &dagv1.Condition{Kind: &dagv1.Condition_SignalPredicate{SignalPredicate: approvedPred}}, Priority: 10},
					{TargetNodeId: "term_failure", Condition: &dagv1.Condition{Kind: &dagv1.Condition_SignalPredicate{SignalPredicate: rejectedPred}}, Priority: 5},
					{TargetNodeId: "term_failure", Condition: always, Priority: 0},
				},
			},
			"term_success": {NodeId: "term_success", Kind: dagv1.NodeKind_NODE_KIND_TERMINAL, TerminalOutcome: dagv1.TerminalOutcome_TERMINAL_OUTCOME_SUCCESS},
			"term_failure": {NodeId: "term_failure", Kind: dagv1.NodeKind_NODE_KIND_TERMINAL, TerminalOutcome: dagv1.TerminalOutcome_TERMINAL_OUTCOME_FAILURE},
		},
	}
	if err := reg.RegisterGraph(spec); err != nil {
		t.Fatal(err)
	}
	registerMinimalUnits(t, reg, typeKey)
	return reg, store, eng, nil, nil
}

func setupDynamicJoinGraph(t interface {
	Helper()
	Fatal(...any)
}, policy dagv1.JoinPolicy) (*Registry, *LineStore, *Engine, *chargeUnit, *refundCompensator) {
	t.Helper()
	reg := NewRegistry()
	store := NewLineStore()
	eng := NewEngine(reg, store)
	typeKey := &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema}
	if err := reg.RegisterEntityType(&dagv1.EntityTypeRegistration{TypeKey: typeKey, PayloadTypeUrl: orderTypeURL}); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterGraph(childTerminalGraph()); err != nil {
		t.Fatal(err)
	}
	always := &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}}
	spec := &dagv1.GraphSpec{
		Version:     &dagv1.GraphVersion{GraphId: "dynamic-join", Version: "v1"},
		EntryNodeId: "spawn",
		Nodes: map[string]*dagv1.NodeDef{
			"spawn": {
				NodeId: "spawn", Kind: dagv1.NodeKind_NODE_KIND_COMPUTE, UnitId: "order.spawn_dynamic",
				Transitions: []*dagv1.Transition{{TargetNodeId: "join", Condition: always}},
			},
			"join": {
				NodeId: "join", Kind: dagv1.NodeKind_NODE_KIND_JOIN,
				JoinSpec: &dagv1.JoinSpec{
					DynamicBarriers: []*dagv1.DynamicJoinBarrier{{
						CorrelationPrefix: "pay-",
						ExpectedCount:     0,
						Policy:            policy,
					}},
				},
				Transitions: []*dagv1.Transition{{TargetNodeId: "term_success", Condition: always}},
			},
			"term_success": {NodeId: "term_success", Kind: dagv1.NodeKind_NODE_KIND_TERMINAL, TerminalOutcome: dagv1.TerminalOutcome_TERMINAL_OUTCOME_SUCCESS},
		},
	}
	if err := reg.RegisterGraph(spec); err != nil {
		t.Fatal(err)
	}
	registerMinimalUnits(t, reg, typeKey)
	if err := reg.RegisterComputeUnitImpl("order.spawn_dynamic", &dynamicSpawnUnit{}); err != nil {
		t.Fatal(err)
	}
	return reg, store, eng, nil, nil
}

func setupCompensationStack(t interface {
	Helper()
	Fatal(...any)
}) (*Registry, *LineStore, *Scheduler, *chargeUnit, *refundCompensator) {
	t.Helper()
	reg := NewRegistry()
	store := NewLineStore()
	refund := &refundCompensator{}
	typeKey := &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema}
	_ = reg.RegisterEntityType(&dagv1.EntityTypeRegistration{TypeKey: typeKey, PayloadTypeUrl: orderTypeURL})
	_ = reg.RegisterCompensator("order.refund", refund)
	_ = reg.RegisterComputeUnit(&dagv1.ComputeUnitDef{UnitId: "order.refund", InputTypeKey: typeKey})
	always := &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}}
	spec := &dagv1.GraphSpec{
		Version:     &dagv1.GraphVersion{GraphId: "comp-graph", Version: "v1"},
		EntryNodeId: "charge",
		Nodes: map[string]*dagv1.NodeDef{
			"charge": {
				NodeId: "charge", Kind: dagv1.NodeKind_NODE_KIND_COMPUTE, UnitId: "order.charge",
				Transitions: []*dagv1.Transition{{TargetNodeId: "term_failure", Condition: always}},
			},
			"term_failure": {
				NodeId: "term_failure", Kind: dagv1.NodeKind_NODE_KIND_TERMINAL,
				TerminalOutcome: dagv1.TerminalOutcome_TERMINAL_OUTCOME_FAILURE,
			},
		},
	}
	if err := reg.RegisterGraph(spec); err != nil {
		t.Fatal(err)
	}
	sched := NewScheduler(reg, store, graph.NewResolver(), dag.DefaultOptions())
	return reg, store, sched, nil, refund
}

type dynamicSpawnUnit struct{}

func (u *dynamicSpawnUnit) Execute(ctx context.Context, snapshot *dagv1.EntitySnapshot) (*dagv1.EntityMutation, error) {
	gv := &dagv1.GraphVersion{GraphId: "child-terminal", Version: "v1"}
	childType := snapshot.TypeKey
	ids := []string{"pay-1", "pay-2", "pay-3"}
	specs := make([]*dagv1.SpawnSpec, 0, len(ids))
	for _, id := range ids {
		specs = append(specs, &dagv1.SpawnSpec{
			Ref:            &dagv1.EntityRef{EntityId: id},
			TypeKey:        childType,
			InitialPayload: snapshot.Payload,
			Graph:          gv,
			CorrelationId:  id,
		})
	}
	return &dagv1.EntityMutation{Intent: &dagv1.EntityMutation_Spawn{Spawn: &dagv1.SpawnList{Specs: specs}}}, nil
}

type retryFailUnit struct {
	execCount atomic.Int32
}

func (u *retryFailUnit) Execute(ctx context.Context, snapshot *dagv1.EntitySnapshot) (*dagv1.EntityMutation, error) {
	u.execCount.Add(1)
	return nil, context.Canceled
}

func setupRetryUnit(t interface {
	Helper()
	Fatal(...any)
}, maxAttempts int32) (*Registry, *LineStore, *Scheduler, *retryFailUnit) {
	t.Helper()
	reg := NewRegistry()
	store := NewLineStore()
	typeKey := &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema}
	_ = reg.RegisterEntityType(&dagv1.EntityTypeRegistration{TypeKey: typeKey, PayloadTypeUrl: orderTypeURL})
	always := &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}}
	spec := &dagv1.GraphSpec{
		Version:     &dagv1.GraphVersion{GraphId: "retry", Version: "v1"},
		EntryNodeId: "compute",
		Nodes: map[string]*dagv1.NodeDef{
			"compute": {
				NodeId: "compute", Kind: dagv1.NodeKind_NODE_KIND_COMPUTE, UnitId: "order.retry",
				Transitions: []*dagv1.Transition{{TargetNodeId: "term_success", Condition: always}},
			},
			"term_success": {NodeId: "term_success", Kind: dagv1.NodeKind_NODE_KIND_TERMINAL, TerminalOutcome: dagv1.TerminalOutcome_TERMINAL_OUTCOME_SUCCESS},
		},
	}
	_ = reg.RegisterGraph(spec)
	unit := &retryFailUnit{}
	_ = reg.RegisterComputeUnit(&dagv1.ComputeUnitDef{
		UnitId: "order.retry", InputTypeKey: typeKey,
		RetryPolicy: &dagv1.RetryPolicy{MaxAttempts: maxAttempts},
	})
	_ = reg.RegisterComputeUnitImpl("order.retry", unit)
	sched := NewScheduler(reg, store, graph.NewResolver(), dag.MergeOptions(dag.WithMaxAttempts(3)))
	return reg, store, sched, unit
}

func registerMinimalUnits(t interface {
	Helper()
	Fatal(...any)
}, reg *Registry, typeKey *dagv1.EntityTypeKey) {
	t.Helper()
	if err := reg.RegisterComputeUnit(&dagv1.ComputeUnitDef{
		UnitId: "order.validate", InputTypeKey: typeKey, OutputTypeKeys: []*dagv1.EntityTypeKey{typeKey},
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterComputeUnitImpl("order.validate", &validateUnit{}); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterComputeUnit(&dagv1.ComputeUnitDef{
		UnitId: "order.spawn_dynamic", InputTypeKey: typeKey, OutputTypeKeys: []*dagv1.EntityTypeKey{typeKey},
	}); err != nil {
		t.Fatal(err)
	}
}

func startInstance(t *testing.T, eng *Engine, ref *dagv1.EntityRef, graphID, version string) {
	t.Helper()
	_, err := eng.StartInstance(context.Background(), &dagv1.StartInstanceRequest{
		Ref:            ref,
		TypeKey:        &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema},
		InitialPayload: orderAny(&testpb.Order{OrderId: "o-1"}),
		GraphVersion:   &dagv1.GraphVersion{GraphId: graphID, Version: version},
		GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_START,
	})
	if err != nil {
		t.Fatal(err)
	}
}
