package dag

import (
	"context"
	"fmt"
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
	"google.golang.org/protobuf/types/known/anypb"
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
	reg      *memory.Registry
	store    *memory.LineStore
	engine   *memory.Engine
	fixtures string
	rec      *api.OpRecorder
	mu       sync.RWMutex
	loaded   map[string]string
}

// NewRuntime 创建 DAG 运行时并注册通用 ComputeUnit。
func NewRuntime(fixturesDir string) (*Runtime, error) {
	reg := memory.NewRegistry()
	store := memory.NewLineStore()
	eng := memory.NewEngine(reg, store)

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

	return &Runtime{
		reg:      reg,
		store:    store,
		engine:   eng,
		fixtures: fixturesDir,
		rec:      api.NewOpRecorder(50),
		loaded:   map[string]string{},
	}, nil
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
	return rt.engine.Close()
}

type graphYAML struct {
	GraphID       string                       `yaml:"graph_id"`
	Version       string                       `yaml:"version"`
	EntityType    string                       `yaml:"entity_type"`
	SchemaVersion string                       `yaml:"schema_version"`
	Entry         string                       `yaml:"entry"`
	Nodes         map[string]graphNodeYAML     `yaml:"nodes"`
}

type graphNodeYAML struct {
	Kind          string              `yaml:"kind"`
	Unit          string              `yaml:"unit"`
	Compensator   string              `yaml:"compensator"`
	Signal        string              `yaml:"signal"`
	Outcome       string              `yaml:"outcome"`
	Transitions   []graphTransitionYAML `yaml:"transitions"`
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
	always := &dagv1.Condition{Kind: &dagv1.Condition_Always{Always: true}}
	nodes := map[string]*dagv1.NodeDef{}
	for id, n := range doc.Nodes {
		node := &dagv1.NodeDef{NodeId: id}
		switch strings.ToLower(n.Kind) {
		case "compute":
			node.Kind = dagv1.NodeKind_NODE_KIND_COMPUTE
			node.UnitId = n.Unit
			if n.Compensator != "" {
				node.CompensatorUnitId = n.Compensator
			}
		case "wait":
			node.Kind = dagv1.NodeKind_NODE_KIND_WAIT
			node.WaitConfig = &dagv1.WaitNodeConfig{SignalName: n.Signal}
		case "terminal":
			node.Kind = dagv1.NodeKind_NODE_KIND_TERMINAL
			node.TerminalOutcome = parseOutcome(n.Outcome)
		default:
			return nil, fmt.Errorf("unsupported node kind %q on %q", n.Kind, id)
		}
		for _, tr := range n.Transitions {
			node.Transitions = append(node.Transitions, &dagv1.Transition{
				TargetNodeId: tr.Target,
				Condition:    always,
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
	default:
		return dagv1.TerminalOutcome_TERMINAL_OUTCOME_SUCCESS
	}
}
