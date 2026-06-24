package daghttp

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"github.com/solo-kingdom/uniface/pkg/dag/graph"
	"github.com/solo-kingdom/uniface/pkg/dag/invocation"
	invocationmemory "github.com/solo-kingdom/uniface/pkg/dag/invocation/memory"
	"github.com/solo-kingdom/uniface/pkg/dag/invocation/loader"
)

const (
	labSchema     = "v1"
	labPayloadURL = "type.googleapis.com/google.protobuf.StringValue"
	labTypeKey    = "lab.Generic"
)

// Runtime 封装 daghttp 域专用的 DAG 运行时（与 lab/internal/dag 完全隔离）。
//
// Runtime 基于公共 invocationmemory.Runtime 装配，注册自身 hello/echo ComputeUnit，
// 通过公共 loader 解析 YAML 图，通过公共 Invoker 执行请求式调用。
type Runtime struct {
	rt       *invocationmemory.Runtime
	fixtures string
	mu       sync.RWMutex
	loaded   map[string]string
}

// NewRuntime 创建 daghttp 运行时并注册 hello/echo ComputeUnit。
func NewRuntime(fixturesDir string) (*Runtime, error) {
	rt := invocationmemory.New()
	typeKey := &dagv1.EntityTypeKey{EntityType: labTypeKey, PayloadSchemaVersion: labSchema}
	if err := rt.RegisterEntityTypeSimple(labTypeKey, labSchema, labPayloadURL); err != nil {
		_ = rt.Close()
		return nil, err
	}
	for _, u := range defaultUnits(typeKey) {
		if err := rt.RegisterComputeUnitFull(u.def, u.impl); err != nil {
			_ = rt.Close()
			return nil, err
		}
	}
	return &Runtime{
		rt:       rt,
		fixtures: fixturesDir,
		loaded:   map[string]string{},
	}, nil
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

// LoadGraphFile 从 YAML 文件加载并注册图，使用公共 loader 解析。
func (rt *Runtime) LoadGraphFile(path string) (*dagv1.GraphSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	res, err := loader.LoadYAML(data, &loader.Options{
		DefaultEntityType:    labTypeKey,
		DefaultSchemaVersion: labSchema,
	})
	if err != nil {
		return nil, err
	}
	if err := graph.ValidateGraphSpec(res.Spec); err != nil {
		return nil, err
	}
	if err := rt.rt.RegisterGraph(res.Spec); err != nil {
		return nil, err
	}
	for _, def := range res.UnitDefs {
		if err := rt.rt.RegisterComputeUnitDef(def); err != nil {
			return nil, err
		}
	}
	rt.mu.Lock()
	rt.loaded[res.Spec.Version.GraphId] = path
	rt.mu.Unlock()
	return res.Spec, nil
}

// LoadFixture 按图 ID 从 fixtures 目录加载。
func (rt *Runtime) LoadFixture(graphID string) (*dagv1.GraphSpec, error) {
	path := filepath.Join(rt.fixtures, graphID+".yaml")
	return rt.LoadGraphFile(path)
}

// Invoke 请求式调用：一次性 Start+Drain+Snapshot（使用公共 Invoker）。
// payload 为空时表示空请求体。
func (rt *Runtime) Invoke(ctx context.Context, graphID, entityID, payload string) (*invocation.InvokeResult, error) {
	req := &invocation.InvokeRequest{
		Ref:            &dagv1.EntityRef{EntityId: entityID},
		TypeKey:        &dagv1.EntityTypeKey{EntityType: labTypeKey, PayloadSchemaVersion: labSchema},
		GraphVersion:   &dagv1.GraphVersion{GraphId: graphID, Version: "v1"},
		GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_START,
	}
	if payload != "" {
		var err error
		if req.InitialPayload, err = invocation.MarshalString(payload); err != nil {
			return nil, err
		}
	}
	return rt.rt.Invoker().Invoke(ctx, req)
}

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
	return rt.rt.Close()
}
