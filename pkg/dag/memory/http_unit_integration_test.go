package memory

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"github.com/solo-kingdom/uniface/pkg/dag/testpb"
	"google.golang.org/protobuf/types/known/anypb"
)

// alwaysCond 复用：always 路由条件。
var alwaysCond = &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}}

// httpTestEnv 组装一个最小可运行 HttpUnit 图环境。
type httpTestEnv struct {
	reg    *Registry
	store  *LineStore
	engine *Engine
	server *httptest.Server
}

func newHTTPTestEnv(t *testing.T, handler http.Handler, resolver dag.HttpClientResolver) *httpTestEnv {
	t.Helper()
	srv := httptest.NewServer(handler)
	reg := NewRegistry()
	store := NewLineStore()
	var opts []dag.Option
	if resolver != nil {
		opts = append(opts, dag.WithHttpClientResolver(resolver))
	}
	eng := NewEngine(reg, store, opts...)

	tk := &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema}
	if err := reg.RegisterEntityType(&dagv1.EntityTypeRegistration{
		TypeKey:        tk,
		PayloadTypeUrl: orderTypeURL,
	}); err != nil {
		t.Fatal(err)
	}
	// 子图（spawn 场景需要）。
	if err := reg.RegisterGraph(childTerminalGraph()); err != nil {
		t.Fatal(err)
	}
	return &httpTestEnv{reg: reg, store: store, engine: eng, server: srv}
}

// registerHTTPUnit 注册一个 HttpUnit def + 单节点图（charge → term_success）。
func (e *httpTestEnv) registerHTTPUnit(t *testing.T, unitID string, cfg *dagv1.HttpUnit, useResolver bool) {
	t.Helper()
	tk := &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema}
	def := &dagv1.ComputeUnitDef{
		UnitId:          unitID,
		InputTypeKey:    tk,
		OutputTypeKeys:  []*dagv1.EntityTypeKey{tk},
		SideEffectClass: dagv1.SideEffectClass_SIDE_EFFECT_IDEMPOTENT,
		Implementation:  &dagv1.ComputeUnitDef_Http{Http: cfg},
	}
	if err := e.reg.RegisterComputeUnit(def); err != nil {
		t.Fatal(err)
	}
	spec := &dagv1.GraphSpec{
		Version:     &dagv1.GraphVersion{GraphId: "http-it", Version: "v1"},
		EntryNodeId: "charge",
		Nodes: map[string]*dagv1.NodeDef{
			"charge": {
				NodeId: "charge", Kind: dagv1.NodeKind_NODE_KIND_COMPUTE, UnitId: unitID,
				Transitions: []*dagv1.Transition{{TargetNodeId: "term_success", Condition: alwaysCond}},
			},
			"term_success": {
				NodeId: "term_success", Kind: dagv1.NodeKind_NODE_KIND_TERMINAL,
				TerminalOutcome: dagv1.TerminalOutcome_TERMINAL_OUTCOME_SUCCESS,
			},
		},
	}
	if err := e.reg.RegisterGraph(spec); err != nil {
		t.Fatal(err)
	}
}

func (e *httpTestEnv) start(t *testing.T) *dagv1.EntityInstance {
	t.Helper()
	tk := &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema}
	inst, err := e.engine.StartInstance(context.Background(), &dagv1.StartInstanceRequest{
		Ref:            &dagv1.EntityRef{EntityId: "order-1"},
		TypeKey:        tk,
		InitialPayload: orderAny(&testpb.Order{OrderId: "o1", Amount: 100, Status: "new", Approved: true}),
		GraphVersion:   &dagv1.GraphVersion{GraphId: "http-it", Version: "v1"},
		GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_START,
	})
	if err != nil {
		t.Fatal(err)
	}
	return inst
}

// ---- 7.1 HttpUnit 2xx → update 黄金路径 ----

func TestHttpUnitIntegration_2xxUpdateGoldenPath(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"orderId":"o1","amount":100,"status":"charged","approved":true}`))
	})
	env := newHTTPTestEnv(t, handler, nil)
	defer env.server.Close()
	env.registerHTTPUnit(t, "order.http", &dagv1.HttpUnit{Url: env.server.URL, Method: "POST"}, false)
	env.start(t)

	if err := env.engine.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	inst, _ := env.store.GetInstance(context.Background(), &dagv1.EntityRef{EntityId: "order-1"})
	if inst.Status != dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED {
		t.Fatalf("expected COMPLETED, got %v", inst.Status)
	}
	snap, _ := env.store.GetSnapshot(context.Background(), &dagv1.EntityRef{EntityId: "order-1"})
	if snap.Payload.GetTypeUrl() != orderTypeURL {
		t.Fatalf("expected updated payload type %q, got %q", orderTypeURL, snap.Payload.GetTypeUrl())
	}
}

// ---- 7.2 HttpUnit 4xx → mutation.fail → FAILED ----

func TestHttpUnitIntegration_4xxFail(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	env := newHTTPTestEnv(t, handler, nil)
	defer env.server.Close()
	env.registerHTTPUnit(t, "order.http", &dagv1.HttpUnit{Url: env.server.URL}, false)
	env.start(t)

	if err := env.engine.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	inst, _ := env.store.GetInstance(context.Background(), &dagv1.EntityRef{EntityId: "order-1"})
	// fail mutation (trigger_compensation=false) → FAILED。
	if inst.Status != dagv1.InstanceStatus_INSTANCE_STATUS_FAILED {
		t.Fatalf("expected FAILED after 4xx fail, got %v", inst.Status)
	}
}

// ---- 7.3 HttpUnit 5xx → retry → 第 N 次成功；retry 耗尽 → fail ----

func TestHttpUnitIntegration_5xxRetryThenSuccess(t *testing.T) {
	var calls atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"orderId":"o1","amount":1,"status":"ok","approved":true}`))
	})
	env := newHTTPTestEnv(t, handler, nil)
	defer env.server.Close()
	env.registerHTTPUnit(t, "order.http", &dagv1.HttpUnit{
		Url: env.server.URL,
		RetryOn: &dagv1.RetryClassification{RetryStatusCodes: []int32{503}},
	}, false)
	// 设定 unit retry policy max_attempts=3。
	env.reg.units["order.http"].RetryPolicy = &dagv1.RetryPolicy{MaxAttempts: 3}
	env.start(t)

	// 第一次：503 → 可重试错误，attempt++。
	if err := env.engine.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce (attempt 1): %v", err)
	}
	// 第二次：200 → 成功。
	if err := env.engine.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce (attempt 2): %v", err)
	}
	inst, _ := env.store.GetInstance(context.Background(), &dagv1.EntityRef{EntityId: "order-1"})
	if inst.Status != dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED {
		t.Fatalf("expected COMPLETED after retry, got %v", inst.Status)
	}
	if calls.Load() != 2 {
		t.Fatalf("expected 2 HTTP calls, got %d", calls.Load())
	}
}

func TestHttpUnitIntegration_5xxRetryExhaustedFails(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	env := newHTTPTestEnv(t, handler, nil)
	defer env.server.Close()
	env.registerHTTPUnit(t, "order.http", &dagv1.HttpUnit{
		Url: env.server.URL,
		RetryOn: &dagv1.RetryClassification{RetryStatusCodes: []int32{503}},
	}, false)
	env.reg.units["order.http"].RetryPolicy = &dagv1.RetryPolicy{MaxAttempts: 2}
	env.start(t)

	// 前 maxAttempts-1 次重试返回 nil（re-queue），最后一次返回错误。
	var lastErr error
	for i := 0; i < 3; i++ {
		lastErr = env.engine.RunOnce(context.Background())
	}
	if lastErr == nil {
		t.Fatal("expected error after retry exhausted")
	}
}

// ---- 7.4 HttpClientResolver 解析 service → base URL → path ----

type fakeHTTPResolver struct {
	client  *http.Client
	baseURL string
	err     error
}

func (f *fakeHTTPResolver) ResolveClient(ctx context.Context, service string) (*http.Client, string, error) {
	return f.client, f.baseURL, f.err
}

func TestHttpUnitIntegration_ServiceViaResolver(t *testing.T) {
	var hitPath string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"orderId":"o1","amount":1,"status":"ok","approved":true}`))
	})
	env := newHTTPTestEnv(t, handler, &fakeHTTPResolver{client: &http.Client{}, baseURL: ""})
	defer env.server.Close()
	// resolver 返回真实 mock server 作为 base URL。
	resolver := &fakeHTTPResolver{client: &http.Client{}, baseURL: env.server.URL}
	env.reg.SetHttpClientResolver(resolver)
	env.registerHTTPUnit(t, "order.http", &dagv1.HttpUnit{Service: "order-service", Path: "/charge"}, true)
	env.start(t)

	if err := env.engine.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if hitPath != "/charge" {
		t.Fatalf("expected resolver + path /charge, got %q", hitPath)
	}
}

// ---- 7.5 BodyTemplate Level 0 整包 / Level 1 字段路径 ----

func TestHttpUnitIntegration_BodyLevel0(t *testing.T) {
	var received []byte
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"orderId":"o1","amount":1,"status":"ok","approved":true}`))
	})
	env := newHTTPTestEnv(t, handler, nil)
	defer env.server.Close()
	env.registerHTTPUnit(t, "order.http", &dagv1.HttpUnit{Url: env.server.URL}, false)
	env.start(t)

	_ = env.engine.RunOnce(context.Background())
	if !strings.Contains(string(received), `"orderId":"o1"`) {
		t.Fatalf("Level 0 body should contain whole payload, got %s", received)
	}
}

// ---- 7.6 MODE_MUTATION response 直接 apply ----

func TestHttpUnitIntegration_ModeMutationComplete(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// EntityMutation{complete: TERMINAL_OUTCOME_SUCCESS=1} 的 protojson 表示。
		_, _ = w.Write([]byte(`{"complete":1}`))
	})
	env := newHTTPTestEnv(t, handler, nil)
	defer env.server.Close()
	env.registerHTTPUnit(t, "order.http", &dagv1.HttpUnit{
		Url: env.server.URL,
		Response: &dagv1.ResponseMapping{Mode: dagv1.ResponseMapping_MODE_MUTATION},
	}, false)
	env.start(t)

	if err := env.engine.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	inst, _ := env.store.GetInstance(context.Background(), &dagv1.EntityRef{EntityId: "order-1"})
	if inst.Status != dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED {
		t.Fatalf("MODE_MUTATION complete should yield COMPLETED, got %v", inst.Status)
	}
}

// ---- 7.7 未注入 resolver 且 service 非空 → Execute 错误 ----

func TestHttpUnitIntegration_MissingResolverErrors(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	env := newHTTPTestEnv(t, handler, nil)
	defer env.server.Close()
	// 不注入 resolver（env 用 nil 构造），HttpUnit service 非空。
	env.registerHTTPUnit(t, "order.http", &dagv1.HttpUnit{Service: "order-service"}, false)
	env.reg.units["order.http"].RetryPolicy = &dagv1.RetryPolicy{MaxAttempts: 1}
	env.start(t)

	// 配置错误每次 Execute 都失败，重试耗尽后 RunOnce 返回错误。
	var err error
	for i := 0; i < 3; i++ {
		err = env.engine.RunOnce(context.Background())
		if err != nil {
			break
		}
	}
	if err == nil {
		t.Fatal("expected error when service set but no resolver injected")
	}
	if !strings.Contains(err.Error(), "HttpClientResolver") {
		t.Fatalf("expected resolver error, got %v", err)
	}
}

// 确保 errors 包被引用（部分场景使用）。
var _ = errors.Is
var _ = anypb.New
