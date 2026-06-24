package memory

import (
	"context"
	"sync/atomic"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"github.com/solo-kingdom/uniface/pkg/dag/entity"
	"github.com/solo-kingdom/uniface/pkg/dag/testpb"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	orderTypeURL = "type.googleapis.com/dag.testpb.Order"
	orderType    = "order.Order"
	orderSchema  = "v1"
	graphID      = "order-fulfillment"
	graphVersion = "v1"
)

func orderAny(order *testpb.Order) *anypb.Any {
	a, _ := anypb.New(order)
	return a
}

func setupGoldenPath(t interface {
	Helper()
	Fatal(...any)
}) (*Registry, *LineStore, *Engine, *chargeUnit, *refundCompensator) {
	return setupWithSpawn(t, &spawnUnit{})
}

func setupWithSpawn(t interface {
	Helper()
	Fatal(...any)
}, spawn *spawnUnit) (*Registry, *LineStore, *Engine, *chargeUnit, *refundCompensator) {
	t.Helper()
	reg := NewRegistry()
	store := NewLineStore()
	eng := NewEngine(reg, store)
	charge := &chargeUnit{}
	refund := &refundCompensator{}

	typeKey := &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema}
	if err := reg.RegisterEntityType(&dagv1.EntityTypeRegistration{
		TypeKey:        typeKey,
		PayloadTypeUrl: orderTypeURL,
	}); err != nil {
		t.Fatal(err)
	}

	graph := goldenGraphSpec()
	if err := reg.RegisterGraph(graph); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterGraph(childTerminalGraph()); err != nil {
		t.Fatal(err)
	}

	units := []struct {
		def  *dagv1.ComputeUnitDef
		impl dag.ComputeUnit
	}{
		{
			def: &dagv1.ComputeUnitDef{
				UnitId:          "order.validate",
				InputTypeKey:    typeKey,
				OutputTypeKeys:  []*dagv1.EntityTypeKey{typeKey},
				SideEffectClass: dagv1.SideEffectClass_SIDE_EFFECT_NONE,
			},
			impl: &validateUnit{},
		},
		{
			def: &dagv1.ComputeUnitDef{
				UnitId:          "order.charge",
				InputTypeKey:    typeKey,
				OutputTypeKeys:  []*dagv1.EntityTypeKey{typeKey},
				SideEffectClass: dagv1.SideEffectClass_SIDE_EFFECT_IDEMPOTENT,
			},
			impl: charge,
		},
		{
			def: &dagv1.ComputeUnitDef{
				UnitId:          "order.spawn_payments",
				InputTypeKey:    typeKey,
				OutputTypeKeys:  []*dagv1.EntityTypeKey{typeKey},
				SideEffectClass: dagv1.SideEffectClass_SIDE_EFFECT_NONE,
			},
			impl: spawn,
		},
	}
	for _, u := range units {
		if err := reg.RegisterComputeUnit(u.def); err != nil {
			t.Fatal(err)
		}
		if u.impl != nil {
			if err := reg.RegisterComputeUnitImpl(u.def.UnitId, u.impl); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := reg.RegisterCompensator("order.refund", refund); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterComputeUnit(&dagv1.ComputeUnitDef{
		UnitId:          "order.refund",
		InputTypeKey:    typeKey,
		SideEffectClass: dagv1.SideEffectClass_SIDE_EFFECT_IDEMPOTENT,
	}); err != nil {
		t.Fatal(err)
	}
	return reg, store, eng, charge, refund
}

func goldenGraphSpec() *dagv1.GraphSpec {
	always := &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}}
	return &dagv1.GraphSpec{
		Version:     &dagv1.GraphVersion{GraphId: graphID, Version: graphVersion},
		EntryNodeId: "validate",
		Nodes: map[string]*dagv1.NodeDef{
			"validate": {
				NodeId: "validate", Kind: dagv1.NodeKind_NODE_KIND_COMPUTE, UnitId: "order.validate",
				Transitions: []*dagv1.Transition{{TargetNodeId: "wait_approval", Condition: always, Priority: 0}},
			},
			"wait_approval": {
				NodeId: "wait_approval", Kind: dagv1.NodeKind_NODE_KIND_WAIT,
				WaitConfig:  &dagv1.WaitNodeConfig{SignalName: "manual_approval"},
				Transitions: []*dagv1.Transition{{TargetNodeId: "charge", Condition: always, Priority: 0}},
			},
			"charge": {
				NodeId: "charge", Kind: dagv1.NodeKind_NODE_KIND_COMPUTE, UnitId: "order.charge",
				CompensatorUnitId: "order.refund",
				Transitions:       []*dagv1.Transition{{TargetNodeId: "spawn_payments", Condition: always, Priority: 0}},
			},
			"spawn_payments": {
				NodeId: "spawn_payments", Kind: dagv1.NodeKind_NODE_KIND_COMPUTE, UnitId: "order.spawn_payments",
				Transitions: []*dagv1.Transition{{TargetNodeId: "join", Condition: always, Priority: 0}},
			},
			"join": {
				NodeId: "join", Kind: dagv1.NodeKind_NODE_KIND_JOIN,
				JoinSpec: &dagv1.JoinSpec{
					Policy: dagv1.JoinPolicy_JOIN_ALL_SUCCESS,
					Barriers: []*dagv1.JoinBarrier{
						{Target: &dagv1.JoinBarrier_ChildEntityId{ChildEntityId: "payment-1"}},
						{Target: &dagv1.JoinBarrier_ChildEntityId{ChildEntityId: "payment-2"}},
					},
				},
				Transitions: []*dagv1.Transition{{TargetNodeId: "term_success", Condition: always, Priority: 0}},
			},
			"term_success": {
				NodeId: "term_success", Kind: dagv1.NodeKind_NODE_KIND_TERMINAL,
				TerminalOutcome: dagv1.TerminalOutcome_TERMINAL_OUTCOME_SUCCESS,
			},
			"term_failure": {
				NodeId: "term_failure", Kind: dagv1.NodeKind_NODE_KIND_TERMINAL,
				TerminalOutcome: dagv1.TerminalOutcome_TERMINAL_OUTCOME_FAILURE,
			},
		},
	}
}

type validateUnit struct{}

func (u *validateUnit) Execute(ctx context.Context, snapshot *dagv1.EntitySnapshot) (*dagv1.EntityMutation, error) {
	order := &testpb.Order{OrderId: "o-1", Amount: 100, Status: "validated", Approved: true}
	return &dagv1.EntityMutation{Intent: &dagv1.EntityMutation_Update{Update: entity.NewSnapshot(snapshot.Ref, snapshot.TypeKey, snapshot.Sequence+1, orderAny(order))}}, nil
}

type chargeUnit struct {
	crashOnce atomic.Bool
	fail      atomic.Bool
	execCount atomic.Int32
}

func (u *chargeUnit) Execute(ctx context.Context, snapshot *dagv1.EntitySnapshot) (*dagv1.EntityMutation, error) {
	u.execCount.Add(1)
	if u.crashOnce.CompareAndSwap(false, true) {
		return nil, context.Canceled
	}
	if u.fail.Load() {
		return &dagv1.EntityMutation{Intent: &dagv1.EntityMutation_Fail{Fail: &dagv1.FailIntent{
			Reason:              "charge failed",
			TriggerCompensation: true,
		}}}, nil
	}
	order := &testpb.Order{OrderId: "o-1", Amount: 100, Status: "charged", Approved: true}
	return &dagv1.EntityMutation{Intent: &dagv1.EntityMutation_Update{Update: entity.NewSnapshot(snapshot.Ref, snapshot.TypeKey, snapshot.Sequence+1, orderAny(order))}}, nil
}

type spawnUnit struct {
	failWithCompensation bool
}

func (u *spawnUnit) Execute(ctx context.Context, snapshot *dagv1.EntitySnapshot) (*dagv1.EntityMutation, error) {
	if u.failWithCompensation {
		return &dagv1.EntityMutation{Intent: &dagv1.EntityMutation_Fail{Fail: &dagv1.FailIntent{
			Reason:              "spawn failed",
			TriggerCompensation: true,
		}}}, nil
	}
	gv := &dagv1.GraphVersion{GraphId: "child-terminal", Version: "v1"}
	childType := &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema}
	return &dagv1.EntityMutation{Intent: &dagv1.EntityMutation_Spawn{Spawn: &dagv1.SpawnList{Specs: []*dagv1.SpawnSpec{
		{Ref: &dagv1.EntityRef{EntityId: "payment-1"}, TypeKey: childType, InitialPayload: orderAny(&testpb.Order{OrderId: "p-1"}), Graph: gv},
		{Ref: &dagv1.EntityRef{EntityId: "payment-2"}, TypeKey: childType, InitialPayload: orderAny(&testpb.Order{OrderId: "p-2"}), Graph: gv},
	}}}}, nil
}

type refundCompensator struct {
	count atomic.Int32
}

func (c *refundCompensator) Compensate(ctx context.Context, comp *dagv1.CompensationContext) error {
	c.count.Add(1)
	return nil
}

func childTerminalGraph() *dagv1.GraphSpec {
	return &dagv1.GraphSpec{
		Version:     &dagv1.GraphVersion{GraphId: "child-terminal", Version: "v1"},
		EntryNodeId: "term_success",
		Nodes: map[string]*dagv1.NodeDef{
			"term_success": {
				NodeId: "term_success", Kind: dagv1.NodeKind_NODE_KIND_TERMINAL,
				TerminalOutcome: dagv1.TerminalOutcome_TERMINAL_OUTCOME_SUCCESS,
			},
		},
	}
}
