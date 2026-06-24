package memory_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"github.com/solo-kingdom/uniface/pkg/dag/entity"
	"github.com/solo-kingdom/uniface/pkg/dag/invocation"
	invocationmemory "github.com/solo-kingdom/uniface/pkg/dag/invocation/memory"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	rtType    = "order.Order"
	rtSchema  = "v1"
	rtPayURL  = "type.googleapis.com/google.protobuf.StringValue"
)

func rtTypeKey() *dagv1.EntityTypeKey {
	return &dagv1.EntityTypeKey{EntityType: rtType, PayloadSchemaVersion: rtSchema}
}

// TestRuntime_NoLabSemantics 验证新建 Runtime 不内置任何 lab fixture/unit。
func TestRuntime_NoLabSemantics(t *testing.T) {
	rt := invocationmemory.New()
	defer rt.Close()

	// 未注册任何实体类型 → 解析失败。
	if _, err := rt.Registry().ResolveType(rtTypeKey()); err == nil {
		t.Fatal("ResolveType 应失败：Runtime 不应内置 lab.Generic")
	}
	// 未注册任何图 → 获取失败。
	if _, err := rt.Registry().GetGraph(&dagv1.GraphVersion{GraphId: "lab.echo", Version: "v1"}); err == nil {
		t.Fatal("GetGraph 应失败：Runtime 不应内置 lab 图")
	}
	// 未注册任何 unit。
	if _, err := rt.Registry().GetComputeUnit("lab.echo"); err == nil {
		t.Fatal("GetComputeUnit(lab.echo) 应失败：Runtime 不应内置 lab unit")
	}
	if _, err := rt.Registry().GetComputeUnit("lab.hello"); err == nil {
		t.Fatal("GetComputeUnit(lab.hello) 应失败：Runtime 不应内置 lab unit")
	}
}

// echoComputeUnit 简单回显计算单元，用于验证 Runtime 可运行内存图。
type echoComputeUnit struct{}

func (u *echoComputeUnit) Execute(_ context.Context, snap *dagv1.EntitySnapshot) (*dagv1.EntityMutation, error) {
	in := ""
	if snap.Payload != nil {
		var sv wrapperspb.StringValue
		_ = snap.Payload.UnmarshalTo(&sv)
		in = sv.GetValue()
	}
	out, _ := anypb.New(wrapperspb.String("echo:" + in))
	return &dagv1.EntityMutation{
		Intent: &dagv1.EntityMutation_Update{
			Update: entity.NewSnapshot(snap.Ref, snap.TypeKey, snap.Sequence+1, out),
		},
	}, nil
}

var _ dag.ComputeUnit = (*echoComputeUnit)(nil)

// TestRuntime_RunnableMemoryGraph 验证注册后 Runtime 可启动并排空匹配图实例。
func TestRuntime_RunnableMemoryGraph(t *testing.T) {
	rt := invocationmemory.New()
	defer rt.Close()

	if err := rt.RegisterEntityTypeSimple(rtType, rtSchema, rtPayURL); err != nil {
		t.Fatal(err)
	}
	always := &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}}
	if err := rt.RegisterGraph(&dagv1.GraphSpec{
		Version:     &dagv1.GraphVersion{GraphId: "echo", Version: "v1"},
		EntryNodeId: "echo",
		Nodes: map[string]*dagv1.NodeDef{
			"echo": {
				NodeId: "echo", Kind: dagv1.NodeKind_NODE_KIND_COMPUTE, UnitId: "echo",
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
	if err := rt.RegisterComputeUnit("echo", rtTypeKey(), &echoComputeUnit{}); err != nil {
		t.Fatal(err)
	}

	payload, _ := anypb.New(wrapperspb.String("hello"))
	res, err := rt.Invoker().Invoke(context.Background(), &invocation.InvokeRequest{
		Ref:            &dagv1.EntityRef{EntityId: "e-1"},
		TypeKey:        rtTypeKey(),
		InitialPayload: payload,
		GraphVersion:   &dagv1.GraphVersion{GraphId: "echo", Version: "v1"},
		GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_START,
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if res.Instance.Status != dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED {
		t.Fatalf("Status = %s, want COMPLETED", res.Instance.Status)
	}
	got, _ := invocation.UnmarshalString(res.Snapshot)
	if got != "echo:hello" {
		t.Fatalf("payload = %q, want echo:hello", got)
	}
}

// staticResolver 固定返回同一 client 与 base URL，用于声明式 HttpUnit 测试。
type staticResolver struct {
	client  *http.Client
	baseURL string
}

func (r *staticResolver) ResolveClient(_ context.Context, service string) (*http.Client, string, error) {
	return r.client, r.baseURL, nil
}

// TestRuntime_HttpUnitWithResolver 验证注入 HttpClientResolver 后声明式 HttpUnit 可解析服务。
func TestRuntime_HttpUnitWithResolver(t *testing.T) {
	// mock HTTP 服务：把 StringValue payload 回写为 processed:<input>。
	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		in := ""
		var sv wrapperspb.StringValue
		if err := protojson.Unmarshal([]byte(r.URL.Query().Get("body")), &sv); err == nil {
			in = sv.GetValue()
		}
		_ = in
		// 直接回写裸标量（StringValue protojson 形态）。
		out, _ := protojson.Marshal(wrapperspb.String("processed:ok"))
		_, _ = w.Write(out)
	}))
	defer mockSrv.Close()

	resolver := &staticResolver{client: mockSrv.Client(), baseURL: mockSrv.URL}
	rt := invocationmemory.New(invocationmemory.WithHttpClientResolver(resolver))
	defer rt.Close()

	if err := rt.RegisterEntityTypeSimple(rtType, rtSchema, rtPayURL); err != nil {
		t.Fatal(err)
	}
	// 内联声明式 HttpUnit 定义。
	httpDef := &dagv1.ComputeUnitDef{
		UnitId:          "http.call",
		InputTypeKey:    rtTypeKey(),
		OutputTypeKeys:  []*dagv1.EntityTypeKey{rtTypeKey()},
		SideEffectClass: dagv1.SideEffectClass_SIDE_EFFECT_IDEMPOTENT,
		Implementation: &dagv1.ComputeUnitDef_Http{Http: &dagv1.HttpUnit{
			Service: "mock",
			Method:  http.MethodGet,
			Path:    "/",
			Headers: map[string]string{"Content-Type": "application/json"},
			Response: &dagv1.ResponseMapping{
				Mode:           dagv1.ResponseMapping_MODE_AUTO,
				PayloadTypeUrl: rtPayURL,
			},
		}},
	}
	if err := rt.RegisterComputeUnitDef(httpDef); err != nil {
		t.Fatal(err)
	}
	always := &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}}
	if err := rt.RegisterGraph(&dagv1.GraphSpec{
		Version:     &dagv1.GraphVersion{GraphId: "httpcall", Version: "v1"},
		EntryNodeId: "call",
		Nodes: map[string]*dagv1.NodeDef{
			"call": {
				NodeId: "call", Kind: dagv1.NodeKind_NODE_KIND_COMPUTE, UnitId: "http.call",
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
	// 验证声明式 HttpUnit 可被 Registry 解析（resolver 注入后不会因 service 非空报错）。
	if _, err := rt.Registry().GetComputeUnitImpl("http.call"); err != nil {
		t.Fatalf("GetComputeUnitImpl: %v", err)
	}
}
