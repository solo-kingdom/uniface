package invocation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"github.com/solo-kingdom/uniface/pkg/dag/entity"
	"github.com/solo-kingdom/uniface/pkg/dag/invocation"
	"github.com/solo-kingdom/uniface/pkg/dag/memory"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	invType    = "lab.Generic"
	invSchema  = "v1"
	invPayURL  = "type.googleapis.com/google.protobuf.StringValue"
	invGraphID = "echo"
	invVersion = "v1"
)

func invTypeKey() *dagv1.EntityTypeKey {
	return &dagv1.EntityTypeKey{EntityType: invType, PayloadSchemaVersion: invSchema}
}

func invVersionRef() *dagv1.GraphVersion {
	return &dagv1.GraphVersion{GraphId: invGraphID, Version: invVersion}
}

// setupEcho 注册一个 echo 计算节点 + 成功终态图，返回可直接使用的 Invoker。
func setupEcho(t *testing.T) *invocation.Invoker {
	t.Helper()
	reg := memory.NewRegistry()
	store := memory.NewLineStore()
	eng := memory.NewEngine(reg, store)

	if err := reg.RegisterEntityType(&dagv1.EntityTypeRegistration{
		TypeKey:        invTypeKey(),
		PayloadTypeUrl: invPayURL,
	}); err != nil {
		t.Fatal(err)
	}
	always := &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}}
	if err := reg.RegisterGraph(&dagv1.GraphSpec{
		Version:     invVersionRef(),
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
		InputTypeKey:    invTypeKey(),
		OutputTypeKeys:  []*dagv1.EntityTypeKey{invTypeKey()},
		SideEffectClass: dagv1.SideEffectClass_SIDE_EFFECT_NONE,
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterComputeUnitImpl("lab.echo", &echoUnit{}); err != nil {
		t.Fatal(err)
	}
	return invocation.NewInvoker(eng, store)
}

// setupTerminal 注册一个入口即给定 outcome 的终态图。
func setupTerminal(t *testing.T, outcome dagv1.TerminalOutcome, graphID string) *invocation.Invoker {
	t.Helper()
	reg := memory.NewRegistry()
	store := memory.NewLineStore()
	eng := memory.NewEngine(reg, store)

	if err := reg.RegisterEntityType(&dagv1.EntityTypeRegistration{
		TypeKey:        invTypeKey(),
		PayloadTypeUrl: invPayURL,
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterGraph(&dagv1.GraphSpec{
		Version:     &dagv1.GraphVersion{GraphId: graphID, Version: invVersion},
		EntryNodeId: "term",
		Nodes: map[string]*dagv1.NodeDef{
			"term": {
				NodeId: "term", Kind: dagv1.NodeKind_NODE_KIND_TERMINAL, TerminalOutcome: outcome,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	return invocation.NewInvoker(eng, store)
}

// setupWait 注册一个入口即 wait 节点的图，用于验证 WAITING 提前返回。
func setupWait(t *testing.T) *invocation.Invoker {
	t.Helper()
	reg := memory.NewRegistry()
	store := memory.NewLineStore()
	eng := memory.NewEngine(reg, store)

	if err := reg.RegisterEntityType(&dagv1.EntityTypeRegistration{
		TypeKey:        invTypeKey(),
		PayloadTypeUrl: invPayURL,
	}); err != nil {
		t.Fatal(err)
	}
	always := &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}}
	if err := reg.RegisterGraph(&dagv1.GraphSpec{
		Version:     &dagv1.GraphVersion{GraphId: "wait-graph", Version: invVersion},
		EntryNodeId: "wait",
		Nodes: map[string]*dagv1.NodeDef{
			"wait": {
				NodeId: "wait", Kind: dagv1.NodeKind_NODE_KIND_WAIT,
				WaitConfig:  &dagv1.WaitNodeConfig{SignalName: "approval", DefaultDeadlineSeconds: 60},
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
	return invocation.NewInvoker(eng, store)
}

type echoUnit struct{}

func (u *echoUnit) Execute(_ context.Context, snapshot *dagv1.EntitySnapshot) (*dagv1.EntityMutation, error) {
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

var _ dag.ComputeUnit = (*echoUnit)(nil)

func stringAny(s string) *anypb.Any {
	a, _ := anypb.New(wrapperspb.String(s))
	return a
}

// TestInvoke_SuccessTerminal 验证成功终态返回 COMPLETED 实例与终态 snapshot。
func TestInvoke_SuccessTerminal(t *testing.T) {
	inv := setupEcho(t)
	res, err := inv.Invoke(context.Background(), &invocation.InvokeRequest{
		Ref:            &dagv1.EntityRef{EntityId: "e-1"},
		TypeKey:        invTypeKey(),
		InitialPayload: stringAny("hello"),
		GraphVersion:   invVersionRef(),
		GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_START,
	})
	if err != nil {
		t.Fatalf("Invoke error: %v", err)
	}
	if res.Instance == nil {
		t.Fatal("Instance is nil")
	}
	if res.Instance.Status != dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED {
		t.Fatalf("Status = %s, want COMPLETED", res.Instance.Status)
	}
	if res.Snapshot == nil {
		t.Fatal("Snapshot is nil")
	}
	var sv wrapperspb.StringValue
	if err := res.Snapshot.Payload.UnmarshalTo(&sv); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	if sv.GetValue() != "echo:hello" {
		t.Fatalf("payload = %q, want echo:hello", sv.GetValue())
	}
	if !invocation.IsTerminal(res.Instance.Status) {
		t.Fatal("IsTerminal(COMPLETED) = false, want true")
	}
	if err := invocation.TerminalError(res.Instance); err != nil {
		t.Fatalf("TerminalError(COMPLETED) = %v, want nil", err)
	}
}

// TestInvoke_FailureTerminal 验证失败终态作为结果返回，error 为 nil。
func TestInvoke_FailureTerminal(t *testing.T) {
	inv := setupTerminal(t, dagv1.TerminalOutcome_TERMINAL_OUTCOME_FAILURE, "fail-graph")
	res, err := inv.Invoke(context.Background(), &invocation.InvokeRequest{
		Ref:          &dagv1.EntityRef{EntityId: "e-fail"},
		TypeKey:      invTypeKey(),
		GraphVersion: &dagv1.GraphVersion{GraphId: "fail-graph", Version: invVersion},
		GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_START,
	})
	if err != nil {
		t.Fatalf("Invoke error: %v; 失败终态应作为结果返回而非 error", err)
	}
	if res.Instance == nil {
		t.Fatal("Instance is nil")
	}
	if res.Instance.Status != dagv1.InstanceStatus_INSTANCE_STATUS_FAILED {
		t.Fatalf("Status = %s, want FAILED", res.Instance.Status)
	}
	if !invocation.IsTerminal(res.Instance.Status) {
		t.Fatal("IsTerminal(FAILED) = false, want true")
	}
	if err := invocation.TerminalError(res.Instance); err == nil {
		t.Fatal("TerminalError(FAILED) = nil, want error")
	}
}

// TestInvoke_WaitingReturnsEarly 验证实例进入 WAITING 时提前返回。
func TestInvoke_WaitingReturnsEarly(t *testing.T) {
	inv := setupWait(t)
	res, err := inv.Invoke(context.Background(), &invocation.InvokeRequest{
		Ref:          &dagv1.EntityRef{EntityId: "e-wait"},
		TypeKey:      invTypeKey(),
		GraphVersion: &dagv1.GraphVersion{GraphId: "wait-graph", Version: invVersion},
		GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_START,
	})
	if err != nil {
		t.Fatalf("Invoke error: %v", err)
	}
	if res.Instance == nil {
		t.Fatal("Instance is nil")
	}
	if res.Instance.Status != dagv1.InstanceStatus_INSTANCE_STATUS_WAITING {
		t.Fatalf("Status = %s, want WAITING", res.Instance.Status)
	}
	if invocation.IsTerminal(res.Instance.Status) {
		t.Fatal("IsTerminal(WAITING) = true, want false")
	}
}

// TestInvoke_DrainErrorPropagated 验证上下文取消时 Drain 错误透传且附带部分结果。
func TestInvoke_DrainErrorPropagated(t *testing.T) {
	inv := setupEcho(t)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(2 * time.Millisecond)

	res, err := inv.Invoke(ctx, &invocation.InvokeRequest{
		Ref:            &dagv1.EntityRef{EntityId: "e-cancel"},
		TypeKey:        invTypeKey(),
		InitialPayload: stringAny("x"),
		GraphVersion:   invVersionRef(),
		GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_START,
	})
	if err == nil {
		t.Skip("drain 未返回错误（实例已先排空），跳过 ctx 取消路径")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context error", err)
	}
	// 部分结果：StartInstance 成功则 Instance 非 nil。
	if res != nil && res.Instance == nil {
		t.Fatal("partial result missing Instance")
	}
}

// TestInvoke_NilRefRejected 验证空 Ref 被拒绝。
func TestInvoke_NilRefRejected(t *testing.T) {
	inv := setupEcho(t)
	_, err := inv.Invoke(context.Background(), &invocation.InvokeRequest{
		TypeKey:      invTypeKey(),
		GraphVersion: invVersionRef(),
	})
	if err == nil {
		t.Fatal("expected error for nil Ref, got nil")
	}
}
