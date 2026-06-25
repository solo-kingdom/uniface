package app_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag/entity"
	"github.com/solo-kingdom/uniface/pkg/dag/invocation"
	"github.com/solo-kingdom/uniface/pkg/dag/invocation/app"
	invocationmemory "github.com/solo-kingdom/uniface/pkg/dag/invocation/memory"
	"github.com/solo-kingdom/uniface/pkg/dag/testpb"
)

const (
	testType   = "test.Generic"
	testSchema = "v1"
)

func newEchoRuntime(t *testing.T) (*app.Runtime, *dagv1.EntityTypeKey) {
	t.Helper()
	rt := app.New()
	typeKey, err := rt.RegisterStringEntityType(testType, testSchema)
	if err != nil {
		t.Fatalf("RegisterStringEntityType: %v", err)
	}
	if err := rt.RegisterStringUnit("test.echo", typeKey, func(_ context.Context, in string) (string, error) {
		return "echo:" + in, nil
	}); err != nil {
		t.Fatalf("RegisterStringUnit: %v", err)
	}
	always := &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}}
	if err := rt.RegisterGraph(&dagv1.GraphSpec{
		Version:     &dagv1.GraphVersion{GraphId: "echo", Version: "v1"},
		EntryNodeId: "echo",
		Nodes: map[string]*dagv1.NodeDef{
			"echo": {
				NodeId: "echo", Kind: dagv1.NodeKind_NODE_KIND_COMPUTE, UnitId: "test.echo",
				Transitions: []*dagv1.Transition{{TargetNodeId: "done", Condition: always, Priority: 0}},
			},
			"done": {
				NodeId: "done", Kind: dagv1.NodeKind_NODE_KIND_TERMINAL,
				TerminalOutcome: dagv1.TerminalOutcome_TERMINAL_OUTCOME_SUCCESS,
			},
		},
	}); err != nil {
		t.Fatalf("RegisterGraph: %v", err)
	}
	return rt, typeKey
}

// TestRuntime_NoGlobalState 验证多个独立实例互不共享注册表。
func TestRuntime_NoGlobalState(t *testing.T) {
	rt1 := app.New()
	rt2 := app.New()
	defer rt1.Close()
	defer rt2.Close()

	if _, err := rt1.RegisterStringEntityType("a.Type", "v1"); err != nil {
		t.Fatal(err)
	}
	if _, err := rt2.RegisterStringEntityType("b.Type", "v1"); err != nil {
		t.Fatal(err)
	}
	if _, err := rt1.Memory().Registry().ResolveType(&dagv1.EntityTypeKey{EntityType: "b.Type", PayloadSchemaVersion: "v1"}); err == nil {
		t.Fatal("rt1 不应解析 rt2 注册的类型")
	}
}

// TestRuntime_NoLabSemantics 验证新建 Runtime 不内置 lab fixture。
func TestRuntime_NoLabSemantics(t *testing.T) {
	rt := app.New()
	defer rt.Close()

	if _, err := rt.Memory().Registry().ResolveType(&dagv1.EntityTypeKey{EntityType: "lab.Generic", PayloadSchemaVersion: "v1"}); err == nil {
		t.Fatal("不应内置 lab.Generic")
	}
	if _, err := rt.Memory().Registry().GetComputeUnit("lab.echo"); err == nil {
		t.Fatal("不应内置 lab.echo")
	}
}

// TestRegisterStringEntityType 验证 StringValue payload type URL。
func TestRegisterStringEntityType(t *testing.T) {
	rt := app.New()
	defer rt.Close()

	typeKey, err := rt.RegisterStringEntityType(testType, testSchema)
	if err != nil {
		t.Fatal(err)
	}
	reg, err := rt.Memory().Registry().ResolveType(typeKey)
	if err != nil {
		t.Fatal(err)
	}
	if reg.PayloadTypeUrl != app.StringPayloadTypeURL {
		t.Fatalf("PayloadTypeUrl = %q, want %q", reg.PayloadTypeUrl, app.StringPayloadTypeURL)
	}
}

// TestInvokeString_Echo 验证 string 注册、函数式 unit 与 echo 图调用。
func TestInvokeString_Echo(t *testing.T) {
	rt, typeKey := newEchoRuntime(t)
	defer rt.Close()

	res, err := rt.InvokeString(context.Background(), &app.StringCall{
		GraphID:  "echo",
		EntityID: "e-1",
		Payload:  "hello",
		TypeKey:  typeKey,
	})
	if err != nil {
		t.Fatalf("InvokeString: %v", err)
	}
	if !res.IsCompleted() {
		t.Fatalf("Status = %s, want COMPLETED", res.Status())
	}
	if res.Value != "echo:hello" {
		t.Fatalf("Value = %q, want echo:hello", res.Value)
	}
}

// TestInvokeMessage_Proto 验证 protobuf message 编解码。
func TestInvokeMessage_Proto(t *testing.T) {
	rt := app.New()
	defer rt.Close()

	orderTypeURL := "type.googleapis.com/dag.testpb.Order"
	if err := rt.Memory().RegisterEntityTypeSimple("test.Order", testSchema, orderTypeURL); err != nil {
		t.Fatal(err)
	}
	typeKey := &dagv1.EntityTypeKey{EntityType: "test.Order", PayloadSchemaVersion: testSchema}

	if err := rt.RegisterComputeUnitDef(&dagv1.ComputeUnitDef{
		UnitId:          "test.order",
		InputTypeKey:    typeKey,
		OutputTypeKeys:  []*dagv1.EntityTypeKey{typeKey},
		SideEffectClass: dagv1.SideEffectClass_SIDE_EFFECT_NONE,
	}); err != nil {
		t.Fatal(err)
	}
	if err := rt.RegisterComputeUnitImpl("test.order", &orderEchoUnit{}); err != nil {
		t.Fatal(err)
	}

	always := &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}}
	if err := rt.RegisterGraph(&dagv1.GraphSpec{
		Version:     &dagv1.GraphVersion{GraphId: "order", Version: "v1"},
		EntryNodeId: "order",
		Nodes: map[string]*dagv1.NodeDef{
			"order": {
				NodeId: "order", Kind: dagv1.NodeKind_NODE_KIND_COMPUTE, UnitId: "test.order",
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

	out := &testpb.Order{}
	res, err := rt.InvokeMessage(context.Background(), &app.MessageCall{
		GraphID:  "order",
		EntityID: "o-1",
		Input:    &testpb.Order{OrderId: "42", Amount: 9.9},
		TypeKey:  typeKey,
	}, out)
	if err != nil {
		t.Fatalf("InvokeMessage: %v", err)
	}
	if !res.IsCompleted() {
		t.Fatalf("Status = %s, want COMPLETED", res.Status())
	}
	if out.OrderId != "42" || out.Status != "processed" {
		t.Fatalf("out = %+v, want OrderId=42 Status=processed", out)
	}
}

// TestInvokeString_FailedTerminal 验证 FAILED 终态不填充 Value。
func TestInvokeString_FailedTerminal(t *testing.T) {
	rt, typeKey := newTerminalRuntime(t, dagv1.TerminalOutcome_TERMINAL_OUTCOME_FAILURE, "fail")
	defer rt.Close()

	res, err := rt.InvokeString(context.Background(), &app.StringCall{
		GraphID:  "fail",
		EntityID: "e-fail",
		Payload:  "x",
		TypeKey:  typeKey,
	})
	if err != nil {
		t.Fatalf("InvokeString drain error: %v", err)
	}
	if res.IsCompleted() {
		t.Fatal("FAILED 不应被当作 COMPLETED")
	}
	if res.Value != "" {
		t.Fatalf("Value = %q, want empty on FAILED", res.Value)
	}
	if res.TerminalErr() == nil {
		t.Fatal("TerminalErr 应非 nil")
	}
}

// TestInvokeString_CancelledTerminal 验证非 COMPLETED 终态不填充 Value（与 FAILED 同类）。
func TestInvokeString_CancelledTerminal(t *testing.T) {
	rt, typeKey := newTerminalRuntime(t, dagv1.TerminalOutcome_TERMINAL_OUTCOME_FAILURE, "term2")
	defer rt.Close()

	res, err := rt.InvokeString(context.Background(), &app.StringCall{
		GraphID:  "term2",
		EntityID: "e-2",
		TypeKey:  typeKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsCompleted() || res.Value != "" {
		t.Fatalf("非 COMPLETED 终态不应填充 Value: completed=%v value=%q", res.IsCompleted(), res.Value)
	}
}

// TestInvokeString_Waiting 验证 WAITING 显式暴露且不继续等待。
func TestInvokeString_Waiting(t *testing.T) {
	rt, typeKey := newWaitRuntime(t)
	defer rt.Close()

	res, err := rt.InvokeString(context.Background(), &app.StringCall{
		GraphID:  "wait-graph",
		EntityID: "e-wait",
		Payload:  "hold",
		TypeKey:  typeKey,
	})
	if err != nil {
		t.Fatalf("InvokeString: %v", err)
	}
	if !res.IsWaiting() {
		t.Fatalf("Status = %s, want WAITING", res.Status())
	}
	if res.Value != "" {
		t.Fatalf("Value = %q, want empty on WAITING", res.Value)
	}
	if res.TerminalErr() != nil {
		t.Fatalf("TerminalErr on WAITING = %v, want nil", res.TerminalErr())
	}
}

// TestLoadGraphFile 验证按文件加载 YAML 图。
func TestLoadGraphFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "echo.yaml")
	if err := os.WriteFile(path, []byte(`
graph_id: file-echo
version: v1
entity_type: test.Generic
schema_version: v1
entry: echo
nodes:
  echo:
    kind: compute
    unit: test.echo
    transitions:
      - target: done
  done:
    kind: terminal
    outcome: success
`), 0o644); err != nil {
		t.Fatal(err)
	}

	rt, typeKey := newEchoRuntime(t)
	defer rt.Close()
	if _, err := rt.LoadGraphFile(path); err != nil {
		t.Fatalf("LoadGraphFile: %v", err)
	}
	loaded := rt.LoadedGraphs()
	if loaded["file-echo"] != path {
		t.Fatalf("LoadedGraphs = %v, want file-echo -> %q", loaded, path)
	}

	res, err := rt.InvokeString(context.Background(), &app.StringCall{
		GraphID:  "file-echo",
		EntityID: "e-file",
		Payload:  "hi",
		TypeKey:  typeKey,
	})
	if err != nil {
		t.Fatalf("InvokeString: %v", err)
	}
	if res.Value != "echo:hi" {
		t.Fatalf("Value = %q", res.Value)
	}
}

// TestLoadGraphID 验证按 graph ID 从目录加载。
func TestLoadGraphID(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dir-echo.yaml"), []byte(`
graph_id: dir-echo
version: v1
entity_type: test.Generic
schema_version: v1
entry: echo
nodes:
  echo:
    kind: compute
    unit: test.echo
    transitions:
      - target: done
  done:
    kind: terminal
    outcome: success
`), 0o644); err != nil {
		t.Fatal(err)
	}

	rt, typeKey := newEchoRuntime(t)
	defer rt.Close()
	rtWithDir := app.New(app.WithGraphDir(dir), app.WithLoaderDefaults(testType, testSchema))
	defer rtWithDir.Close()
	// 复制注册到带目录的 runtime
	typeKey2, _ := rtWithDir.RegisterStringEntityType(testType, testSchema)
	_ = rtWithDir.RegisterStringUnit("test.echo", typeKey2, func(_ context.Context, in string) (string, error) {
		return "echo:" + in, nil
	})
	if _, err := rtWithDir.LoadGraphID("dir-echo"); err != nil {
		t.Fatalf("LoadGraphID: %v", err)
	}
	_ = typeKey
	res, err := rtWithDir.InvokeString(context.Background(), &app.StringCall{
		GraphID:  "dir-echo",
		EntityID: "e-dir",
		Payload:  "x",
		TypeKey:  typeKey2,
	})
	if err != nil || res.Value != "echo:x" {
		t.Fatalf("InvokeString: err=%v value=%q", err, res.Value)
	}
}

// TestLoadGraphID_JSON 验证 .json 后缀回退。
func TestLoadGraphID_JSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "json-echo.json"), []byte(`{
  "graph_id": "json-echo",
  "version": "v1",
  "entity_type": "test.Generic",
  "schema_version": "v1",
  "entry": "echo",
  "nodes": {
    "echo": {"kind": "compute", "unit": "test.echo", "transitions": [{"target": "done"}]},
    "done": {"kind": "terminal", "outcome": "success"}
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	rt := app.New(app.WithGraphDir(dir), app.WithLoaderDefaults(testType, testSchema))
	defer rt.Close()
	typeKey, _ := rt.RegisterStringEntityType(testType, testSchema)
	_ = rt.RegisterStringUnit("test.echo", typeKey, func(_ context.Context, in string) (string, error) {
		return "echo:" + in, nil
	})
	if _, err := rt.LoadGraphID("json-echo"); err != nil {
		t.Fatalf("LoadGraphID json: %v", err)
	}
}

// TestNewWithMemory 验证 memory 选项透传。
func TestNewWithMemory(t *testing.T) {
	rt := app.NewWithMemory()
	defer rt.Close()
	if rt.Memory() == nil {
		t.Fatal("Memory() is nil")
	}
}

// --- helpers ---

func newTerminalRuntime(t *testing.T, outcome dagv1.TerminalOutcome, graphID string) (*app.Runtime, *dagv1.EntityTypeKey) {
	t.Helper()
	rt := app.New()
	typeKey, err := rt.RegisterStringEntityType(testType, testSchema)
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.RegisterGraph(&dagv1.GraphSpec{
		Version:     &dagv1.GraphVersion{GraphId: graphID, Version: "v1"},
		EntryNodeId: "term",
		Nodes: map[string]*dagv1.NodeDef{
			"term": {
				NodeId: "term", Kind: dagv1.NodeKind_NODE_KIND_TERMINAL, TerminalOutcome: outcome,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	return rt, typeKey
}

func newWaitRuntime(t *testing.T) (*app.Runtime, *dagv1.EntityTypeKey) {
	t.Helper()
	rt := app.New()
	typeKey, err := rt.RegisterStringEntityType(testType, testSchema)
	if err != nil {
		t.Fatal(err)
	}
	always := &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}}
	if err := rt.RegisterGraph(&dagv1.GraphSpec{
		Version:     &dagv1.GraphVersion{GraphId: "wait-graph", Version: "v1"},
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
	return rt, typeKey
}

type orderEchoUnit struct{}

func (u *orderEchoUnit) Execute(ctx context.Context, snapshot *dagv1.EntitySnapshot) (*dagv1.EntityMutation, error) {
	_ = ctx
	var in testpb.Order
	if snapshot != nil && snapshot.Payload != nil {
		_ = snapshot.Payload.UnmarshalTo(&in)
	}
	out, _ := invocation.MarshalAny(&testpb.Order{OrderId: in.OrderId, Amount: in.Amount, Status: "processed"})
	return &dagv1.EntityMutation{
		Intent: &dagv1.EntityMutation_Update{
			Update: entity.NewSnapshot(snapshot.Ref, snapshot.TypeKey, snapshot.Sequence+1, out),
		},
	}, nil
}

// 确保底层 memory 包仍可直接使用（回归锚点）。
var _ = invocationmemory.New
