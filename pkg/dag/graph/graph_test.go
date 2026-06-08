package graph

import (
	"testing"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"github.com/solo-kingdom/uniface/pkg/dag/entity"
	"github.com/solo-kingdom/uniface/pkg/dag/testpb"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestValidateGraphSpec(t *testing.T) {
	always := &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}}
	spec := &dagv1.GraphSpec{
		Version:     &dagv1.GraphVersion{GraphId: "g", Version: "v1"},
		EntryNodeId: "start",
		Nodes: map[string]*dagv1.NodeDef{
			"start": {
				NodeId: "start", Kind: dagv1.NodeKind_NODE_KIND_COMPUTE, UnitId: "u",
				Transitions: []*dagv1.Transition{{TargetNodeId: "end", Condition: always}},
			},
			"end": {
				NodeId: "end", Kind: dagv1.NodeKind_NODE_KIND_TERMINAL,
				TerminalOutcome: dagv1.TerminalOutcome_TERMINAL_OUTCOME_SUCCESS,
			},
		},
	}
	if err := ValidateGraphSpec(spec); err != nil {
		t.Fatalf("valid graph rejected: %v", err)
	}
	spec.Nodes["start"].Transitions[0].TargetNodeId = ""
	if err := ValidateGraphSpec(spec); err == nil {
		t.Fatal("expected empty target error")
	}
}

func TestFieldPredicatePriority(t *testing.T) {
	typeKey := &dagv1.EntityTypeKey{EntityType: "order.Order", PayloadSchemaVersion: "v1"}
	order := &testpb.Order{Amount: 15000}
	snap := entity.NewSnapshot(&dagv1.EntityRef{EntityId: "o-1"}, typeKey, 0, mustAny(order))

	high := &dagv1.FieldPredicate{FieldPath: "Amount", Op: dagv1.CompareOp_COMPARE_OP_GT, Value: "10000"}
	low := &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}}
	spec := &dagv1.GraphSpec{
		EntryNodeId: "route",
		Nodes: map[string]*dagv1.NodeDef{
			"route": {
				NodeId: "route", Kind: dagv1.NodeKind_NODE_KIND_COMPUTE,
				Transitions: []*dagv1.Transition{
					{TargetNodeId: "high", Condition: &dagv1.Condition{Kind: &dagv1.Condition_FieldPredicate{FieldPredicate: high}}, Priority: 10},
					{TargetNodeId: "low", Condition: low, Priority: 0},
				},
			},
		},
	}
	r := NewResolver()
	next, err := r.Resolve(t.Context(), spec, "route", snap)
	if err != nil {
		t.Fatal(err)
	}
	if next != "high" {
		t.Fatalf("expected high, got %q", next)
	}
}

func TestFieldPredicateNoTransition(t *testing.T) {
	typeKey := &dagv1.EntityTypeKey{EntityType: "order.Order", PayloadSchemaVersion: "v1"}
	order := &testpb.Order{Amount: 1}
	snap := entity.NewSnapshot(&dagv1.EntityRef{EntityId: "o-1"}, typeKey, 0, mustAny(order))
	pred := &dagv1.FieldPredicate{FieldPath: "Amount", Op: dagv1.CompareOp_COMPARE_OP_GT, Value: "10000"}
	spec := &dagv1.GraphSpec{
		EntryNodeId: "route",
		Nodes: map[string]*dagv1.NodeDef{
			"route": {
				NodeId: "route", Kind: dagv1.NodeKind_NODE_KIND_COMPUTE,
				Transitions: []*dagv1.Transition{
					{TargetNodeId: "high", Condition: &dagv1.Condition{Kind: &dagv1.Condition_FieldPredicate{FieldPredicate: pred}}, Priority: 10},
				},
			},
		},
	}
	r := NewResolver()
	_, err := r.Resolve(t.Context(), spec, "route", snap)
	if err != dag.ErrNoTransition {
		t.Fatalf("expected ErrNoTransition, got %v", err)
	}
}

func mustAny(msg *testpb.Order) *anypb.Any {
	a, err := anypb.New(msg)
	if err != nil {
		panic(err)
	}
	return a
}

func TestSignalPredicateRouting(t *testing.T) {
	typeKey := &dagv1.EntityTypeKey{EntityType: "order.Order", PayloadSchemaVersion: "v1"}
	order := &testpb.Order{Approved: true}
	snap := entity.NewSnapshot(&dagv1.EntityRef{EntityId: "o-1"}, typeKey, 1, mustAny(order))
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
	always := &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}}
	spec := &dagv1.GraphSpec{
		EntryNodeId: "wait",
		Nodes: map[string]*dagv1.NodeDef{
			"wait": {
				NodeId: "wait", Kind: dagv1.NodeKind_NODE_KIND_WAIT,
				WaitConfig: &dagv1.WaitNodeConfig{SignalName: "approval"},
				Transitions: []*dagv1.Transition{
					{TargetNodeId: "success", Condition: &dagv1.Condition{Kind: &dagv1.Condition_SignalPredicate{SignalPredicate: approvedPred}}, Priority: 10},
					{TargetNodeId: "reject", Condition: &dagv1.Condition{Kind: &dagv1.Condition_SignalPredicate{SignalPredicate: rejectedPred}}, Priority: 5},
					{TargetNodeId: "fallback", Condition: always, Priority: 0},
				},
			},
		},
	}
	sigCtx := &SignalContext{SignalName: "approval"}
	next, err := ResolveTransitions(spec.Nodes["wait"], snap, sigCtx)
	if err != nil {
		t.Fatal(err)
	}
	if next != "success" {
		t.Fatalf("expected success, got %q", next)
	}
	sigCtx.SignalName = "rejection"
	next, err = ResolveTransitions(spec.Nodes["wait"], snap, sigCtx)
	if err != nil {
		t.Fatal(err)
	}
	if next != "fallback" {
		t.Fatalf("expected fallback for mismatched signal, got %q", next)
	}
}

func TestValidateGraphSpecComputeMissingUnitID(t *testing.T) {
	always := &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}}
	spec := &dagv1.GraphSpec{
		EntryNodeId: "start",
		Nodes: map[string]*dagv1.NodeDef{
			"start": {
				NodeId: "start", Kind: dagv1.NodeKind_NODE_KIND_COMPUTE,
				Transitions: []*dagv1.Transition{{TargetNodeId: "end", Condition: always}},
			},
			"end": {
				NodeId: "end", Kind: dagv1.NodeKind_NODE_KIND_TERMINAL,
				TerminalOutcome: dagv1.TerminalOutcome_TERMINAL_OUTCOME_SUCCESS,
			},
		},
	}
	if err := ValidateGraphSpec(spec); err == nil {
		t.Fatal("expected error for missing unit_id")
	}
}

func TestValidateGraphSpecMissingFallback(t *testing.T) {
	pred := &dagv1.FieldPredicate{FieldPath: "Amount", Op: dagv1.CompareOp_COMPARE_OP_GT, Value: "100"}
	spec := &dagv1.GraphSpec{
		EntryNodeId: "route",
		Nodes: map[string]*dagv1.NodeDef{
			"route": {
				NodeId: "route", Kind: dagv1.NodeKind_NODE_KIND_COMPUTE, UnitId: "u",
				Transitions: []*dagv1.Transition{
					{TargetNodeId: "high", Condition: &dagv1.Condition{Kind: &dagv1.Condition_FieldPredicate{FieldPredicate: pred}}},
				},
			},
			"high": {
				NodeId: "high", Kind: dagv1.NodeKind_NODE_KIND_TERMINAL,
				TerminalOutcome: dagv1.TerminalOutcome_TERMINAL_OUTCOME_SUCCESS,
			},
		},
	}
	if err := ValidateGraphSpec(spec); err == nil {
		t.Fatal("expected error for missing fallback transition")
	}
}

func TestValidateGraphSpecCycle(t *testing.T) {
	always := &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}}
	spec := &dagv1.GraphSpec{
		EntryNodeId: "a",
		Nodes: map[string]*dagv1.NodeDef{
			"a": {
				NodeId: "a", Kind: dagv1.NodeKind_NODE_KIND_COMPUTE, UnitId: "u",
				Transitions: []*dagv1.Transition{{TargetNodeId: "b", Condition: always}},
			},
			"b": {
				NodeId: "b", Kind: dagv1.NodeKind_NODE_KIND_COMPUTE, UnitId: "u",
				Transitions: []*dagv1.Transition{{TargetNodeId: "a", Condition: always}},
			},
		},
	}
	if err := ValidateGraphSpec(spec); err == nil {
		t.Fatal("expected cycle detection error")
	}
}
