package dag

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/lab/internal/web/api"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"github.com/solo-kingdom/uniface/pkg/dag/graph"
	"github.com/solo-kingdom/uniface/pkg/dag/memory"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"gopkg.in/yaml.v3"
)

const (
	labEntityType  = "lab.Generic"
	labSchema      = "v1"
	labPayloadURL  = "type.googleapis.com/google.protobuf.StringValue"
	labTypeKey     = "lab.Generic"
)

// Runtime 封装 memory DAG 引擎与注册表。
type Runtime struct {
	reg       *memory.Registry
	store     *memory.LineStore
	engine    *memory.Engine
	fixtures  string
	rec       *api.OpRecorder
	mu        sync.RWMutex
	loaded    map[string]string
	mockSrv   *httptest.Server
	resolver  dag.HttpClientResolver
}

// RuntimeOption 修改 Runtime 配置。
type RuntimeOption func(*Runtime)

// WithHTTPResolver 注入声明式 HttpUnit 使用的服务实例解析器。
// nil（默认）表示仅支持 HttpUnit 的 url 直连模式。
func WithHTTPResolver(r dag.HttpClientResolver) RuntimeOption {
	return func(rt *Runtime) { rt.resolver = r }
}

// NewRuntime 创建 DAG 运行时并注册通用 ComputeUnit。
func NewRuntime(fixturesDir string, opts ...RuntimeOption) (*Runtime, error) {
	reg := memory.NewRegistry()
	store := memory.NewLineStore()
	rt := &Runtime{
		reg:      reg,
		store:    store,
		fixtures: fixturesDir,
		rec:      api.NewOpRecorder(50),
		loaded:   map[string]string{},
	}
	for _, opt := range opts {
		opt(rt)
	}
	eng := memory.NewEngine(reg, store, dag.WithHttpClientResolver(rt.resolver))
	rt.engine = eng

	typeKey := &dagv1.EntityTypeKey{EntityType: labTypeKey, PayloadSchemaVersion: labSchema}
	if err := reg.RegisterEntityType(&dagv1.EntityTypeRegistration{
		TypeKey:        typeKey,
		PayloadTypeUrl: labPayloadURL,
	}); err != nil {
		return nil, err
	}

	units := defaultUnits(typeKey)
	for _, u := range units {
		if err := reg.RegisterComputeUnit(u.def); err != nil {
			return nil, err
		}
		if u.impl != nil {
			if err := reg.RegisterComputeUnitImpl(u.def.UnitId, u.impl); err != nil {
				return nil, err
			}
		}
		if u.comp != nil {
			if err := reg.RegisterCompensator(u.def.UnitId, u.comp); err != nil {
				return nil, err
			}
		}
	}

	return rt, nil
}

// StartMockHTTPServer 启动一个内置 mock HTTP 服务（http_call fixture 的目标服务）。
// 该服务将收到的 StringValue payload 回写为 {"value":"processed:<input>"}，
// 使 HttpUnit 默认 MODE_AUTO → update 的黄金路径可端到端演示。
// 重复调用会先关闭旧实例。addr 例如 "127.0.0.1:18099"。
func (rt *Runtime) StartMockHTTPServer(addr string) error {
	if rt.mockSrv != nil {
		rt.mockSrv.Close()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		// 输入为 google.protobuf.StringValue 的 protojson 表示。
		// 注意：protojson 对 wrapper 类型展开为裸标量（"hello"），故用 protojson 解包。
		input := extractStringValue(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// 回写裸标量字符串（StringValue 的 protojson 形态），HttpUnit MODE_AUTO 反序列化为 StringValue。
		out, _ := protojson.Marshal(wrapperspb.String("processed:" + input))
		_, _ = w.Write(out)
	})
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	srv := httptest.NewUnstartedServer(mux)
	srv.Listener = ln
	srv.Start()
	rt.mockSrv = srv
	return nil
}

// extractStringValue 从 StringValue 的 protojson 表示（裸标量 "x" 或对象 {"value":"x"}）提取字符串。
func extractStringValue(body []byte) string {
	var sv wrapperspb.StringValue
	if err := protojson.Unmarshal(body, &sv); err == nil {
		return sv.GetValue()
	}
	// 容错回退。
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err == nil {
		if v, ok := m["value"].(string); ok {
			return v
		}
	}
	return string(body)
}

type unitBundle struct {
	def  *dagv1.ComputeUnitDef
	impl dag.ComputeUnit
	comp dag.Compensator
}

func defaultUnits(typeKey *dagv1.EntityTypeKey) []unitBundle {
	return []unitBundle{
		{
			def: &dagv1.ComputeUnitDef{
				UnitId:          "lab.echo",
				InputTypeKey:    typeKey,
				OutputTypeKeys:  []*dagv1.EntityTypeKey{typeKey},
				SideEffectClass: dagv1.SideEffectClass_SIDE_EFFECT_NONE,
			},
			impl: &echoUnit{},
		},
		{
			def: &dagv1.ComputeUnitDef{
				UnitId:          "lab.fail_once",
				InputTypeKey:    typeKey,
				OutputTypeKeys:  []*dagv1.EntityTypeKey{typeKey},
				SideEffectClass: dagv1.SideEffectClass_SIDE_EFFECT_IDEMPOTENT,
			},
			impl: &failOnceUnit{},
		},
		{
			def: &dagv1.ComputeUnitDef{
				UnitId:          "lab.rollback",
				InputTypeKey:    typeKey,
				SideEffectClass: dagv1.SideEffectClass_SIDE_EFFECT_IDEMPOTENT,
			},
			comp: &rollbackCompensator{},
		},
	}
}

// LoadGraphFile 从 YAML 文件加载并注册图。
func (rt *Runtime) LoadGraphFile(path string) (*dagv1.GraphSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	spec, unitDefs, err := parseGraphYAML(data)
	if err != nil {
		return nil, err
	}
	if err := graph.ValidateGraphSpec(spec); err != nil {
		return nil, err
	}
	if err := rt.reg.RegisterGraph(spec); err != nil {
		return nil, err
	}
	for _, def := range unitDefs {
		if err := rt.reg.RegisterComputeUnit(def); err != nil {
			return nil, err
		}
	}
	rt.mu.Lock()
	rt.loaded[spec.Version.GraphId] = path
	rt.mu.Unlock()
	rt.rec.Record("graph load", spec.Version.GraphId, true, nil)
	return spec, nil
}

// LoadFixture 按图 ID 从 fixtures 目录加载。
func (rt *Runtime) LoadFixture(graphID string) (*dagv1.GraphSpec, error) {
	path := filepath.Join(rt.fixtures, graphID+".yaml")
	return rt.LoadGraphFile(path)
}

// Start 启动实例。
func (rt *Runtime) Start(ctx context.Context, graphID, entityID, payload string) (*dagv1.EntityInstance, error) {
	ref := &dagv1.EntityRef{EntityId: entityID}
	typeKey := &dagv1.EntityTypeKey{EntityType: labTypeKey, PayloadSchemaVersion: labSchema}
	var initial *anypb.Any
	if payload != "" {
		initial, _ = anypb.New(wrapperspb.String(payload))
	}

	inst, err := rt.engine.StartInstance(ctx, &dagv1.StartInstanceRequest{
		Ref:            ref,
		TypeKey:        typeKey,
		InitialPayload: initial,
		GraphVersion:   &dagv1.GraphVersion{GraphId: graphID, Version: "v1"},
		GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_START,
	})
	if err != nil {
		rt.rec.Record("start", entityID, false, err)
		return nil, err
	}
	rt.rec.Record("start", entityID, true, nil)
	return inst, nil
}

// GetInstance 查询实例。
func (rt *Runtime) GetInstance(ctx context.Context, entityID string) (*dagv1.EntityInstance, error) {
	return rt.engine.GetInstance(ctx, &dagv1.EntityRef{EntityId: entityID})
}

// DeliverSignal 注入信号。
func (rt *Runtime) DeliverSignal(ctx context.Context, entityID, signal, deliveryID string) error {
	err := rt.engine.DeliverSignal(ctx, &dagv1.SignalDelivery{
		EntityId: entityID, SignalName: signal, DeliveryId: deliveryID,
	})
	rt.rec.Record("signal", fmt.Sprintf("%s:%s", entityID, signal), err == nil, err)
	return err
}

// Journal 查询 journal。
func (rt *Runtime) Journal(ctx context.Context, entityID string) ([]*dagv1.LineJournalEntry, error) {
	return rt.store.ListJournal(ctx, &dagv1.EntityRef{EntityId: entityID})
}

// SagaState 查询 saga 栈。
func (rt *Runtime) SagaState(ctx context.Context, entityID string) (*dagv1.SagaState, error) {
	return rt.store.GetSagaState(ctx, &dagv1.EntityRef{EntityId: entityID})
}

// RunOnce 执行一次调度。
func (rt *Runtime) RunOnce(ctx context.Context) error {
	err := rt.engine.RunOnce(ctx)
	if err != nil {
		rt.rec.Record("run-once", "", false, err)
		return err
	}
	rt.rec.Record("run-once", "", true, nil)
	return nil
}

// Engine 返回底层引擎。
func (rt *Runtime) Engine() *memory.Engine { return rt.engine }

// Store 返回 line store。
func (rt *Runtime) Store() *memory.LineStore { return rt.store }

// LoadedGraphs 返回已加载图 ID。
func (rt *Runtime) LoadedGraphs() map[string]string {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	out := make(map[string]string, len(rt.loaded))
	for k, v := range rt.loaded {
		out[k] = v
	}
	return out
}

// Status 返回域状态。
func (rt *Runtime) Status() api.Status {
	return api.Status{
		Domain:      "dag",
		Impl:        "memory",
		Healthy:     true,
		RecentOps:   rt.rec.Snapshot(),
		Extra: map[string]any{
			"loaded_graphs": rt.LoadedGraphs(),
		},
		CollectedAt: time.Now(),
	}
}

// Close 关闭运行时。
func (rt *Runtime) Close() error {
	if rt.mockSrv != nil {
		rt.mockSrv.Close()
	}
	return rt.engine.Close()
}

type graphYAML struct {
	GraphID       string                   `yaml:"graph_id"`
	Version       string                   `yaml:"version"`
	EntityType    string                   `yaml:"entity_type"`
	SchemaVersion string                   `yaml:"schema_version"`
	Entry         string                   `yaml:"entry"`
	Nodes         map[string]graphNodeYAML `yaml:"nodes"`
}

type graphNodeYAML struct {
	Kind            string                `yaml:"kind"`
	Unit            interface{}           `yaml:"unit"` // string（旧式 unit_id）或对象（含 http 子结构）
	UnitID          string                `yaml:"unit_id"`
	Compensator     string                `yaml:"compensator"`
	Signal          string                `yaml:"signal"`
	Outcome         string                `yaml:"outcome"`
	DynamicPrefix   string                `yaml:"dynamic_prefix"`
	JoinPolicy      string                `yaml:"join_policy"`
	DeadlineSeconds int64                 `yaml:"deadline_seconds"`
	OnTimeout       string                `yaml:"on_timeout"`
	FailOnChildFail bool                  `yaml:"fail_parent_on_child_failure"`
	RetryPolicy     *retryPolicyYAML      `yaml:"retry_policy"`
	Transitions     []graphTransitionYAML `yaml:"transitions"`
}

type retryPolicyYAML struct {
	MaxAttempts    int32  `yaml:"max_attempts"`
	InitialBackoff string `yaml:"initial_backoff"`
	MaxBackoff     string `yaml:"max_backoff"`
}

type graphTransitionYAML struct {
	Target    string            `yaml:"target"`
	Priority  int32             `yaml:"priority"`
	Condition *conditionYAML    `yaml:"condition"`
	Always    bool              `yaml:"always"` // 兼容简写：always: true
	Field     *fieldPredicateYAML `yaml:"field"`
	Signal    *signalPredicateYAML `yaml:"signal"`
}

type conditionYAML struct {
	Always  *bool                  `yaml:"always"`
	Field   *fieldPredicateYAML    `yaml:"field"`
	Signal  *signalPredicateYAML   `yaml:"signal"`
}

type fieldPredicateYAML struct {
	Path  string `yaml:"path"`
	Op    string `yaml:"op"`
	Value string `yaml:"value"`
}

type signalPredicateYAML struct {
	Name             string               `yaml:"name"`
	PayloadPredicate *fieldPredicateYAML  `yaml:"payload_predicate"`
}

// parseGraphYAML 解析 YAML 为 GraphSpec + 内联 ComputeUnitDef（HttpUnit 等）。
func parseGraphYAML(data []byte) (*dagv1.GraphSpec, []*dagv1.ComputeUnitDef, error) {
	var doc graphYAML
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, nil, err
	}
	if doc.GraphID == "" || doc.Entry == "" {
		return nil, nil, fmt.Errorf("graph_id and entry are required")
	}
	version := doc.Version
	if version == "" {
		version = "v1"
	}
	typeKey := &dagv1.EntityTypeKey{EntityType: doc.EntityType, PayloadSchemaVersion: doc.SchemaVersion}
	if typeKey.EntityType == "" {
		typeKey.EntityType = labTypeKey
	}
	if typeKey.PayloadSchemaVersion == "" {
		typeKey.PayloadSchemaVersion = labSchema
	}

	nodes := map[string]*dagv1.NodeDef{}
	var unitDefs []*dagv1.ComputeUnitDef
	for id, n := range doc.Nodes {
		node := &dagv1.NodeDef{NodeId: id}
		switch strings.ToLower(n.Kind) {
		case "compute":
			node.Kind = dagv1.NodeKind_NODE_KIND_COMPUTE
			unitID, def, err := resolveUnitReference(doc.GraphID, id, &n, typeKey)
			if err != nil {
				return nil, nil, err
			}
			node.UnitId = unitID
			if def != nil {
				unitDefs = append(unitDefs, def)
			}
			if n.Compensator != "" {
				node.CompensatorUnitId = n.Compensator
			}
		case "wait":
			node.Kind = dagv1.NodeKind_NODE_KIND_WAIT
			node.WaitConfig = &dagv1.WaitNodeConfig{
				SignalName:               n.Signal,
				DefaultDeadlineSeconds:   n.DeadlineSeconds,
				OnTimeoutTargetNodeId:    n.OnTimeout,
			}
		case "join":
			node.Kind = dagv1.NodeKind_NODE_KIND_JOIN
			policy := dagv1.JoinPolicy_JOIN_ALL_SUCCESS
			if strings.EqualFold(n.JoinPolicy, "any_success") {
				policy = dagv1.JoinPolicy_JOIN_ANY_SUCCESS
			}
			node.JoinSpec = &dagv1.JoinSpec{
				Policy:                  policy,
				FailParentOnChildFailure: n.FailOnChildFail,
			}
			if n.DynamicPrefix != "" {
				node.JoinSpec.DynamicBarriers = []*dagv1.DynamicJoinBarrier{{
					CorrelationPrefix: n.DynamicPrefix,
					Policy:            policy,
				}}
			}
		case "terminal":
			node.Kind = dagv1.NodeKind_NODE_KIND_TERMINAL
			node.TerminalOutcome = parseOutcome(n.Outcome)
		default:
			return nil, nil, fmt.Errorf("unsupported node kind %q on %q", n.Kind, id)
		}
		for _, tr := range n.Transitions {
			cond := buildCondition(tr)
			node.Transitions = append(node.Transitions, &dagv1.Transition{
				TargetNodeId: tr.Target,
				Condition:    cond,
				Priority:     tr.Priority,
			})
		}
		nodes[id] = node
	}
	spec := &dagv1.GraphSpec{
		Version:     &dagv1.GraphVersion{GraphId: doc.GraphID, Version: version},
		EntryNodeId: doc.Entry,
		Nodes:       nodes,
	}
	return spec, unitDefs, nil
}

// resolveUnitReference 解析 unit 字段：字符串（旧式）或对象（含 http 子结构 → HttpUnit def）。
// 返回 (unitID, 内联 ComputeUnitDef 或 nil, error)。
func resolveUnitReference(graphID, nodeID string, n *graphNodeYAML, typeKey *dagv1.EntityTypeKey) (string, *dagv1.ComputeUnitDef, error) {
	switch v := n.Unit.(type) {
	case nil:
		// unit 字段缺省：可能用显式 unit_id 字段。
		if n.UnitID != "" {
			return n.UnitID, nil, nil
		}
		return "", nil, fmt.Errorf("compute node %q missing unit", nodeID)
	case string:
		return v, nil, nil
	case map[string]interface{}:
		httpCfg, err := parseInlineHttpUnit(v)
		if err != nil {
			return "", nil, fmt.Errorf("node %q: %w", nodeID, err)
		}
		if httpCfg == nil {
			return "", nil, fmt.Errorf("node %q: unit object must contain http config", nodeID)
		}
		unitID := n.UnitID
		if unitID == "" {
			unitID = graphID + "." + nodeID
		}
		def := &dagv1.ComputeUnitDef{
			UnitId:          unitID,
			InputTypeKey:    typeKey,
			OutputTypeKeys:  []*dagv1.EntityTypeKey{typeKey},
			SideEffectClass: dagv1.SideEffectClass_SIDE_EFFECT_IDEMPOTENT,
			Implementation:  &dagv1.ComputeUnitDef_Http{Http: httpCfg},
		}
		if n.RetryPolicy != nil {
			def.RetryPolicy = buildRetryPolicy(n.RetryPolicy)
		}
		return unitID, def, nil
	default:
		return "", nil, fmt.Errorf("node %q: unsupported unit type %T", nodeID, n.Unit)
	}
}

// buildCondition 构造 dagv1.Condition，缺省为 always。
func buildCondition(tr graphTransitionYAML) *dagv1.Condition {
	if tr.Condition != nil {
		if c := condFromConditionYAML(tr.Condition); c != nil {
			return c
		}
	}
	// 兼容简写：顶层 always/field/signal。
	if tr.Field != nil {
		return &dagv1.Condition{Kind: &dagv1.Condition_FieldPredicate{FieldPredicate: fieldPredFromYAML(tr.Field)}}
	}
	if tr.Signal != nil {
		return &dagv1.Condition{Kind: &dagv1.Condition_SignalPredicate{SignalPredicate: signalPredFromYAML(tr.Signal)}}
	}
	if tr.Always {
		return &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}}
	}
	return &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}}
}

func condFromConditionYAML(c *conditionYAML) *dagv1.Condition {
	if c == nil {
		return nil
	}
	if c.Always != nil {
		return &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: *c.Always}}
	}
	if c.Field != nil {
		return &dagv1.Condition{Kind: &dagv1.Condition_FieldPredicate{FieldPredicate: fieldPredFromYAML(c.Field)}}
	}
	if c.Signal != nil {
		return &dagv1.Condition{Kind: &dagv1.Condition_SignalPredicate{SignalPredicate: signalPredFromYAML(c.Signal)}}
	}
	return nil
}

func fieldPredFromYAML(f *fieldPredicateYAML) *dagv1.FieldPredicate {
	if f == nil {
		return nil
	}
	return &dagv1.FieldPredicate{
		FieldPath: f.Path,
		Op:        parseCompareOp(f.Op),
		Value:     f.Value,
	}
}

func signalPredFromYAML(s *signalPredicateYAML) *dagv1.SignalPredicate {
	if s == nil {
		return nil
	}
	sp := &dagv1.SignalPredicate{SignalName: s.Name}
	if s.PayloadPredicate != nil {
		sp.PayloadPredicate = fieldPredFromYAML(s.PayloadPredicate)
	}
	return sp
}

func parseCompareOp(s string) dagv1.CompareOp {
	switch strings.ToLower(s) {
	case "eq", "==":
		return dagv1.CompareOp_COMPARE_OP_EQ
	case "ne", "!=":
		return dagv1.CompareOp_COMPARE_OP_NE
	case "gt", ">":
		return dagv1.CompareOp_COMPARE_OP_GT
	case "gte", ">=":
		return dagv1.CompareOp_COMPARE_OP_GTE
	case "lt", "<":
		return dagv1.CompareOp_COMPARE_OP_LT
	case "lte", "<=":
		return dagv1.CompareOp_COMPARE_OP_LTE
	default:
		return dagv1.CompareOp_COMPARE_OP_EQ
	}
}

func buildRetryPolicy(rp *retryPolicyYAML) *dagv1.RetryPolicy {
	if rp == nil {
		return nil
	}
	out := &dagv1.RetryPolicy{MaxAttempts: rp.MaxAttempts}
	if rp.InitialBackoff != "" {
		if d, err := time.ParseDuration(rp.InitialBackoff); err == nil {
			out.InitialBackoff = durationpb.New(d)
		}
	}
	if rp.MaxBackoff != "" {
		if d, err := time.ParseDuration(rp.MaxBackoff); err == nil {
			out.MaxBackoff = durationpb.New(d)
		}
	}
	return out
}

// parseInlineHttpUnit 解析 unit.http 子结构为 *dagv1.HttpUnit。
func parseInlineHttpUnit(m map[string]interface{}) (*dagv1.HttpUnit, error) {
	rawHTTP, ok := m["http"]
	if !ok {
		return nil, nil
	}
	b, err := yaml.Marshal(rawHTTP)
	if err != nil {
		return nil, fmt.Errorf("marshal http config: %w", err)
	}
	var cfg httpUnitYAML
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal http config: %w", err)
	}
	hu := &dagv1.HttpUnit{
		Service: cfg.Service,
		Url:     cfg.URL,
		Method:  cfg.Method,
		Path:    cfg.Path,
		Headers: cfg.Headers,
	}
	if cfg.RequestBody != nil {
		hu.RequestBody = &dagv1.BodyTemplate{FieldPath: cfg.RequestBody.FieldPath}
	}
	if cfg.Response != nil {
		hu.Response = &dagv1.ResponseMapping{
			Mode:            parseResponseMode(cfg.Response.Mode),
			PayloadTypeUrl:  cfg.Response.PayloadTypeURL,
			PayloadField:    cfg.Response.PayloadField,
			OnSuccess:       parseOutcome(cfg.Response.OnSuccess),
		}
	}
	if cfg.Timeout != "" {
		if d, err := time.ParseDuration(cfg.Timeout); err == nil {
			hu.Timeout = durationpb.New(d)
		}
	}
	if cfg.RetryOn != nil {
		hu.RetryOn = &dagv1.RetryClassification{
			RetryStatusCodes: cfg.RetryOn.RetryStatusCodes,
			FailStatusCodes:  cfg.RetryOn.FailStatusCodes,
		}
	}
	return hu, nil
}

type httpUnitYAML struct {
	Service     string            `yaml:"service"`
	URL         string            `yaml:"url"`
	Method      string            `yaml:"method"`
	Path        string            `yaml:"path"`
	Headers     map[string]string `yaml:"headers"`
	RequestBody *bodyTemplateYAML `yaml:"request_body"`
	Response    *responseYAML     `yaml:"response"`
	Timeout     string            `yaml:"timeout"`
	RetryOn     *retryClassYAML   `yaml:"retry_on"`
}

type bodyTemplateYAML struct {
	FieldPath string `yaml:"field_path"`
}

type responseYAML struct {
	Mode           string `yaml:"mode"`
	PayloadTypeURL string `yaml:"payload_type_url"`
	PayloadField   string `yaml:"payload_field"`
	OnSuccess      string `yaml:"on_success"`
}

type retryClassYAML struct {
	RetryStatusCodes []int32 `yaml:"retry_status_codes"`
	FailStatusCodes  []int32 `yaml:"fail_status_codes"`
}

func parseResponseMode(s string) dagv1.ResponseMapping_Mode {
	switch strings.ToLower(s) {
	case "mutation":
		return dagv1.ResponseMapping_MODE_MUTATION
	case "auto", "":
		return dagv1.ResponseMapping_MODE_AUTO
	default:
		return dagv1.ResponseMapping_MODE_AUTO
	}
}

func parseOutcome(s string) dagv1.TerminalOutcome {
	switch strings.ToLower(s) {
	case "failure", "fail":
		return dagv1.TerminalOutcome_TERMINAL_OUTCOME_FAILURE
	case "success":
		return dagv1.TerminalOutcome_TERMINAL_OUTCOME_SUCCESS
	default:
		return dagv1.TerminalOutcome_TERMINAL_OUTCOME_SUCCESS
	}
}
