package daghttp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/lab/internal/web/api"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"github.com/solo-kingdom/uniface/pkg/dag/graph"
	"github.com/solo-kingdom/uniface/pkg/dag/memory"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"gopkg.in/yaml.v3"
)

const (
	labEntityType = "lab.Generic"
	labSchema     = "v1"
	labPayloadURL = "type.googleapis.com/google.protobuf.StringValue"
	labTypeKey    = "lab.Generic"
)

// Runtime 封装 daghttp 域专用的 memory DAG 引擎（与 lab/internal/dag 完全隔离）。
type Runtime struct {
	reg      *memory.Registry
	store    *memory.LineStore
	engine   *memory.Engine
	fixtures string
	rec      *api.OpRecorder
	mu       sync.RWMutex
	loaded   map[string]string
}

// NewRuntime 创建 daghttp 运行时并注册 hello/echo ComputeUnit。
func NewRuntime(fixturesDir string) (*Runtime, error) {
	reg := memory.NewRegistry()
	store := memory.NewLineStore()
	rt := &Runtime{
		reg:      reg,
		store:    store,
		fixtures: fixturesDir,
		rec:      api.NewOpRecorder(50),
		loaded:   map[string]string{},
	}
	eng := memory.NewEngine(reg, store)
	rt.engine = eng

	typeKey := &dagv1.EntityTypeKey{EntityType: labTypeKey, PayloadSchemaVersion: labSchema}
	if err := reg.RegisterEntityType(&dagv1.EntityTypeRegistration{
		TypeKey:        typeKey,
		PayloadTypeUrl: labPayloadURL,
	}); err != nil {
		return nil, err
	}

	for _, u := range defaultUnits(typeKey) {
		if err := reg.RegisterComputeUnit(u.def); err != nil {
			return nil, err
		}
		if err := reg.RegisterComputeUnitImpl(u.def.UnitId, u.impl); err != nil {
			return nil, err
		}
	}

	return rt, nil
}

type unitBundle struct {
	def  *dagv1.ComputeUnitDef
	impl dag.ComputeUnit
}

func defaultUnits(typeKey *dagv1.EntityTypeKey) []unitBundle {
	return []unitBundle{
		{
			def: &dagv1.ComputeUnitDef{
				UnitId:          "lab.hello",
				InputTypeKey:    typeKey,
				OutputTypeKeys:  []*dagv1.EntityTypeKey{typeKey},
				SideEffectClass: dagv1.SideEffectClass_SIDE_EFFECT_NONE,
			},
			impl: &helloUnit{},
		},
		{
			def: &dagv1.ComputeUnitDef{
				UnitId:          "lab.echo",
				InputTypeKey:    typeKey,
				OutputTypeKeys:  []*dagv1.EntityTypeKey{typeKey},
				SideEffectClass: dagv1.SideEffectClass_SIDE_EFFECT_NONE,
			},
			impl: &echoUnit{},
		},
	}
}

// LoadGraphFile 从 YAML 文件加载并注册图（仅支持 compute/terminal 节点）。
func (rt *Runtime) LoadGraphFile(path string) (*dagv1.GraphSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	spec, err := parseGraphYAML(data)
	if err != nil {
		return nil, err
	}
	if err := graph.ValidateGraphSpec(spec); err != nil {
		return nil, err
	}
	if err := rt.reg.RegisterGraph(spec); err != nil {
		return nil, err
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

// Drain 排空实例至终态或 WAITING。
func (rt *Runtime) Drain(ctx context.Context, entityID string, opts ...dag.Option) (*dagv1.EntityInstance, error) {
	inst, err := rt.engine.DrainInstance(ctx, &dagv1.EntityRef{EntityId: entityID}, opts...)
	rt.rec.Record("drain", entityID, err == nil, err)
	return inst, err
}

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

// Close 关闭运行时。
func (rt *Runtime) Close() error {
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
	Kind        string                `yaml:"kind"`
	Unit        string                `yaml:"unit"`
	Outcome     string                `yaml:"outcome"`
	Transitions []graphTransitionYAML `yaml:"transitions"`
}

type graphTransitionYAML struct {
	Target string `yaml:"target"`
}

func parseGraphYAML(data []byte) (*dagv1.GraphSpec, error) {
	var doc graphYAML
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if doc.GraphID == "" || doc.Entry == "" {
		return nil, fmt.Errorf("graph_id and entry are required")
	}
	version := doc.Version
	if version == "" {
		version = "v1"
	}

	nodes := map[string]*dagv1.NodeDef{}
	for id, n := range doc.Nodes {
		node := &dagv1.NodeDef{NodeId: id}
		switch strings.ToLower(n.Kind) {
		case "compute":
			node.Kind = dagv1.NodeKind_NODE_KIND_COMPUTE
			if n.Unit == "" {
				return nil, fmt.Errorf("compute node %q missing unit", id)
			}
			node.UnitId = n.Unit
		case "terminal":
			node.Kind = dagv1.NodeKind_NODE_KIND_TERMINAL
			node.TerminalOutcome = parseOutcome(n.Outcome)
		default:
			return nil, fmt.Errorf("unsupported node kind %q on %q", n.Kind, id)
		}
		for _, tr := range n.Transitions {
			node.Transitions = append(node.Transitions, &dagv1.Transition{
				TargetNodeId: tr.Target,
				Condition:    &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}},
			})
		}
		nodes[id] = node
	}
	return &dagv1.GraphSpec{
		Version:     &dagv1.GraphVersion{GraphId: doc.GraphID, Version: version},
		EntryNodeId: doc.Entry,
		Nodes:       nodes,
	}, nil
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
