package memory

import (
	"context"
	"errors"
	"testing"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"github.com/solo-kingdom/uniface/pkg/dag/entity"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	echoGraphID      = "echo"
	echoGraphVersion = "v1"
	echoType         = "lab.Generic"
	echoSchema       = "v1"
	echoPayloadURL   = "type.googleapis.com/google.protobuf.StringValue"
)

func setupEchoDrain(t *testing.T) (*Registry, *Engine) {
	t.Helper()
	reg := NewRegistry()
	store := NewLineStore()
	eng := NewEngine(reg, store)

	typeKey := &dagv1.EntityTypeKey{EntityType: echoType, PayloadSchemaVersion: echoSchema}
	if err := reg.RegisterEntityType(&dagv1.EntityTypeRegistration{
		TypeKey:        typeKey,
		PayloadTypeUrl: echoPayloadURL,
	}); err != nil {
		t.Fatal(err)
	}
	always := &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}}
	if err := reg.RegisterGraph(&dagv1.GraphSpec{
		Version:     &dagv1.GraphVersion{GraphId: echoGraphID, Version: echoGraphVersion},
		EntryNodeId: "echo",
		Nodes: map[string]*dagv1.NodeDef{
			"echo": {
				NodeId: "echo", Kind: dagv1.NodeKind_NODE_KIND_COMPUTE, UnitId: "lab.echo",
				Transitions: []*dagv1.Transition{{TargetNodeId: "done", Condition: always, Priority: 0}},
			},
			"done": {
				NodeId: "done", Kind: dagv1.NodeKind_NODE_KIND_TERMINAL,
				TerminalOutcome: dagv1.TerminalOutcome_TERMINAL_OUTCOME_SUCCESS,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterComputeUnit(&dagv1.ComputeUnitDef{
		UnitId:          "lab.echo",
		InputTypeKey:    typeKey,
		OutputTypeKeys:  []*dagv1.EntityTypeKey{typeKey},
		SideEffectClass: dagv1.SideEffectClass_SIDE_EFFECT_NONE,
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterComputeUnitImpl("lab.echo", &drainEchoUnit{}); err != nil {
		t.Fatal(err)
	}
	return reg, eng
}

type drainEchoUnit struct{}

func (u *drainEchoUnit) Execute(_ context.Context, snapshot *dagv1.EntitySnapshot) (*dagv1.EntityMutation, error) {
	var input string
	if snapshot.Payload != nil {
		var sv wrapperspb.StringValue
		if err := snapshot.Payload.UnmarshalTo(&sv); err == nil {
			input = sv.GetValue()
		}
	}
	out, _ := anypb.New(wrapperspb.String("echo:" + input))
	return &dagv1.EntityMutation{
		Intent: &dagv1.EntityMutation_Update{
			Update: entity.NewSnapshot(snapshot.Ref, snapshot.TypeKey, snapshot.Sequence+1, out),
		},
	}, nil
}

func startEchoInstance(t *testing.T, eng *Engine, entityID, payload string) *dagv1.EntityRef {
	t.Helper()
	ref := &dagv1.EntityRef{EntityId: entityID}
	var initial *anypb.Any
	if payload != "" {
		initial, _ = anypb.New(wrapperspb.String(payload))
	}
	if _, err := eng.StartInstance(context.Background(), &dagv1.StartInstanceRequest{
		Ref:            ref,
		TypeKey:        &dagv1.EntityTypeKey{EntityType: echoType, PayloadSchemaVersion: echoSchema},
		InitialPayload: initial,
		GraphVersion:   &dagv1.GraphVersion{GraphId: echoGraphID, Version: echoGraphVersion},
		GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_START,
	}); err != nil {
		t.Fatalf("StartInstance: %v", err)
	}
	return ref
}

func TestDrainInstance_EchoCompleted(t *testing.T) {
	_, eng := setupEchoDrain(t)
	ctx := context.Background()
	ref := startEchoInstance(t, eng, "e-1", "hello")

	inst, err := eng.DrainInstance(ctx, ref)
	if err != nil {
		t.Fatalf("DrainInstance: %v", err)
	}
	if inst.Status != dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED {
		t.Fatalf("status = %v, want COMPLETED", inst.Status)
	}
}

func TestDrainInstance_WaitingReturnsEarly(t *testing.T) {
	_, _, eng, _, _ := setupGoldenPath(t)
	ctx := context.Background()
	ref := &dagv1.EntityRef{EntityId: "order-wait"}
	startOrder(t, eng, ref)

	for i := 0; i < 20; i++ {
		if err := eng.RunOnce(ctx); err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		inst, _ := eng.GetInstance(ctx, ref)
		if inst.Status == dagv1.InstanceStatus_INSTANCE_STATUS_WAITING {
			break
		}
	}

	before, _ := eng.GetInstance(ctx, ref)
	if before.Status != dagv1.InstanceStatus_INSTANCE_STATUS_WAITING {
		t.Fatalf("precondition: want WAITING, got %v", before.Status)
	}

	inst, err := eng.DrainInstance(ctx, ref)
	if err != nil {
		t.Fatalf("DrainInstance: %v", err)
	}
	if inst.Status != dagv1.InstanceStatus_INSTANCE_STATUS_WAITING {
		t.Fatalf("status = %v, want WAITING", inst.Status)
	}
}

func TestDrainInstance_Exceeded(t *testing.T) {
	reg := NewRegistry()
	store := NewLineStore()
	eng := NewEngine(reg, store)

	typeKey := &dagv1.EntityTypeKey{EntityType: echoType, PayloadSchemaVersion: echoSchema}
	if err := reg.RegisterEntityType(&dagv1.EntityTypeRegistration{
		TypeKey:        typeKey,
		PayloadTypeUrl: echoPayloadURL,
	}); err != nil {
		t.Fatal(err)
	}
	always := &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}}
	if err := reg.RegisterGraph(&dagv1.GraphSpec{
		Version:     &dagv1.GraphVersion{GraphId: "two-step", Version: "v1"},
		EntryNodeId: "step1",
		Nodes: map[string]*dagv1.NodeDef{
			"step1": {
				NodeId: "step1", Kind: dagv1.NodeKind_NODE_KIND_COMPUTE, UnitId: "lab.noop",
				Transitions: []*dagv1.Transition{{TargetNodeId: "step2", Condition: always, Priority: 0}},
			},
			"step2": {
				NodeId: "step2", Kind: dagv1.NodeKind_NODE_KIND_COMPUTE, UnitId: "lab.noop",
				Transitions: []*dagv1.Transition{{TargetNodeId: "done", Condition: always, Priority: 0}},
			},
			"done": {
				NodeId: "done", Kind: dagv1.NodeKind_NODE_KIND_TERMINAL,
				TerminalOutcome: dagv1.TerminalOutcome_TERMINAL_OUTCOME_SUCCESS,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterComputeUnit(&dagv1.ComputeUnitDef{
		UnitId:          "lab.noop",
		InputTypeKey:    typeKey,
		OutputTypeKeys:  []*dagv1.EntityTypeKey{typeKey},
		SideEffectClass: dagv1.SideEffectClass_SIDE_EFFECT_NONE,
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterComputeUnitImpl("lab.noop", &drainNoopUnit{}); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	ref := &dagv1.EntityRef{EntityId: "e-exceed"}
	if _, err := eng.StartInstance(ctx, &dagv1.StartInstanceRequest{
		Ref:            ref,
		TypeKey:        typeKey,
		InitialPayload: mustStringAny("x"),
		GraphVersion:   &dagv1.GraphVersion{GraphId: "two-step", Version: "v1"},
		GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_START,
	}); err != nil {
		t.Fatalf("StartInstance: %v", err)
	}

	inst, err := eng.DrainInstance(ctx, ref, dag.WithDrainMaxHops(1))
	if err == nil {
		t.Fatal("expected ErrDrainExceeded")
	}
	if !errors.Is(err, dag.ErrDrainExceeded) {
		t.Fatalf("errors.Is ErrDrainExceeded = false, got %v", err)
	}
	if inst == nil {
		t.Fatal("expected instance snapshot on exceed")
	}
	if inst.Status == dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED {
		t.Fatal("should not have completed within hop limit")
	}
}

type drainNoopUnit struct{}

func (u *drainNoopUnit) Execute(_ context.Context, snapshot *dagv1.EntitySnapshot) (*dagv1.EntityMutation, error) {
	out, _ := anypb.New(wrapperspb.String("noop"))
	return &dagv1.EntityMutation{
		Intent: &dagv1.EntityMutation_Update{
			Update: entity.NewSnapshot(snapshot.Ref, snapshot.TypeKey, snapshot.Sequence+1, out),
		},
	}, nil
}

func mustStringAny(s string) *anypb.Any {
	a, _ := anypb.New(wrapperspb.String(s))
	return a
}

func TestDrainInstance_ContextCanceled(t *testing.T) {
	_, eng := setupEchoDrain(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ref := startEchoInstance(t, eng, "e-cancel", "x")

	inst, err := eng.DrainInstance(ctx, ref)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if inst == nil {
		t.Fatal("expected instance snapshot on cancel")
	}
}
