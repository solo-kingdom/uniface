// Package loader 提供声明式 Graph Loader，将外部 YAML 文档解析为
// dagv1.GraphSpec 与可选内联 ComputeUnitDef。
//
// Loader 覆盖已有 DAG proto 能表达的图结构：compute、terminal、wait、join 节点，
// transition condition（always/field/signal），retry_policy 与内联 HttpUnit。
// Loader 不引入 proto 没有表达的新业务语义。
//
// Loader 不绑定 fixture 文件定位：文件读取与定位由调用方负责，
// Loader 仅接受已读取的字节切片。
package loader

import (
	"fmt"
	"strings"
	"time"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"google.golang.org/protobuf/types/known/durationpb"
	"gopkg.in/yaml.v3"
)

// Result 描述一次图加载的结果。
type Result struct {
	Spec     *dagv1.GraphSpec
	UnitDefs []*dagv1.ComputeUnitDef
}

// Options 控制 Loader 行为。
type Options struct {
	// DefaultEntityType 当文档未声明 entity_type 时的默认值。
	DefaultEntityType string
	// DefaultSchemaVersion 当文档未声明 schema_version 时的默认值。
	DefaultSchemaVersion string
}

// LoadYAML 将 YAML 文档解析为 GraphSpec 与内联 ComputeUnitDef。
//
// 文档必须声明 graph_id 与 entry。entity_type/schema_version 缺省时使用 Options 默认值；
// 默认值为空且文档未声明时返回错误（Loader 不内置任何业务默认）。
// 返回的 GraphSpec 可通过 graph.ValidateGraphSpec（调用方负责）。
func LoadYAML(data []byte, opts *Options) (*Result, error) {
	var doc graphYAML
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("loader: unmarshal yaml: %w", err)
	}
	if doc.GraphID == "" || doc.Entry == "" {
		return nil, fmt.Errorf("loader: graph_id and entry are required")
	}
	version := doc.Version
	if version == "" {
		version = "v1"
	}
	typeKey := &dagv1.EntityTypeKey{EntityType: doc.EntityType, PayloadSchemaVersion: doc.SchemaVersion}
	if typeKey.EntityType == "" {
		typeKey.EntityType = opts.defaultEntityType()
	}
	if typeKey.PayloadSchemaVersion == "" {
		typeKey.PayloadSchemaVersion = opts.defaultSchemaVersion()
	}
	if typeKey.EntityType == "" || typeKey.PayloadSchemaVersion == "" {
		return nil, fmt.Errorf("loader: entity_type and schema_version are required (set via document or Options)")
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
				return nil, err
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
				SignalName:             n.Signal,
				DefaultDeadlineSeconds: n.DeadlineSeconds,
				OnTimeoutTargetNodeId:  n.OnTimeout,
			}
		case "join":
			node.Kind = dagv1.NodeKind_NODE_KIND_JOIN
			policy := dagv1.JoinPolicy_JOIN_ALL_SUCCESS
			if strings.EqualFold(n.JoinPolicy, "any_success") {
				policy = dagv1.JoinPolicy_JOIN_ANY_SUCCESS
			}
			node.JoinSpec = &dagv1.JoinSpec{
				Policy:                policy,
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
			return nil, fmt.Errorf("loader: unsupported node kind %q on %q", n.Kind, id)
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
	return &Result{Spec: spec, UnitDefs: unitDefs}, nil
}

func (o *Options) defaultEntityType() string {
	if o == nil {
		return ""
	}
	return o.DefaultEntityType
}

func (o *Options) defaultSchemaVersion() string {
	if o == nil {
		return ""
	}
	return o.DefaultSchemaVersion
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
	Kind            string              `yaml:"kind"`
	Unit            interface{}         `yaml:"unit"` // string（unit_id）或对象（含 http 子结构）
	UnitID          string              `yaml:"unit_id"`
	Compensator     string              `yaml:"compensator"`
	Signal          string              `yaml:"signal"`
	Outcome         string              `yaml:"outcome"`
	DynamicPrefix   string              `yaml:"dynamic_prefix"`
	JoinPolicy      string              `yaml:"join_policy"`
	DeadlineSeconds int64               `yaml:"deadline_seconds"`
	OnTimeout       string              `yaml:"on_timeout"`
	FailOnChildFail bool                `yaml:"fail_parent_on_child_failure"`
	RetryPolicy     *retryPolicyYAML    `yaml:"retry_policy"`
	Transitions     []transitionYAML    `yaml:"transitions"`
}

type retryPolicyYAML struct {
	MaxAttempts    int32  `yaml:"max_attempts"`
	InitialBackoff string `yaml:"initial_backoff"`
	MaxBackoff     string `yaml:"max_backoff"`
}

type transitionYAML struct {
	Target    string              `yaml:"target"`
	Priority  int32               `yaml:"priority"`
	Condition *conditionYAML      `yaml:"condition"`
	Always    bool                `yaml:"always"`
	Field     *fieldPredicateYAML `yaml:"field"`
	Signal    *signalPredicateYAML `yaml:"signal"`
}

type conditionYAML struct {
	Always *bool                `yaml:"always"`
	Field  *fieldPredicateYAML  `yaml:"field"`
	Signal *signalPredicateYAML `yaml:"signal"`
}

type fieldPredicateYAML struct {
	Path  string `yaml:"path"`
	Op    string `yaml:"op"`
	Value string `yaml:"value"`
}

type signalPredicateYAML struct {
	Name             string              `yaml:"name"`
	PayloadPredicate *fieldPredicateYAML `yaml:"payload_predicate"`
}

// resolveUnitReference 解析 unit 字段：字符串（旧式）或对象（含 http 子结构 → HttpUnit def）。
// 返回 (unitID, 内联 ComputeUnitDef 或 nil, error)。
func resolveUnitReference(graphID, nodeID string, n *graphNodeYAML, typeKey *dagv1.EntityTypeKey) (string, *dagv1.ComputeUnitDef, error) {
	switch v := n.Unit.(type) {
	case nil:
		if n.UnitID != "" {
			return n.UnitID, nil, nil
		}
		return "", nil, fmt.Errorf("loader: compute node %q missing unit", nodeID)
	case string:
		return v, nil, nil
	case map[string]interface{}:
		httpCfg, err := parseInlineHttpUnit(v)
		if err != nil {
			return "", nil, fmt.Errorf("loader: node %q: %w", nodeID, err)
		}
		if httpCfg == nil {
			return "", nil, fmt.Errorf("loader: node %q: unit object must contain http config", nodeID)
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
		return "", nil, fmt.Errorf("loader: node %q: unsupported unit type %T", nodeID, n.Unit)
	}
}

// buildCondition 构造 dagv1.Condition，缺省为 always。
func buildCondition(tr transitionYAML) *dagv1.Condition {
	if tr.Condition != nil {
		if c := condFromConditionYAML(tr.Condition); c != nil {
			return c
		}
	}
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
			Mode:           parseResponseMode(cfg.Response.Mode),
			PayloadTypeUrl: cfg.Response.PayloadTypeURL,
			PayloadField:   cfg.Response.PayloadField,
			OnSuccess:      parseOutcome(cfg.Response.OnSuccess),
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
