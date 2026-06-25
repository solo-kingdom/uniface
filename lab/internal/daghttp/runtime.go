package daghttp

import (
	"context"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag/invocation/app"
)

const (
	labSchema  = "v1"
	labTypeKey = "lab.Generic"
)

// Runtime 封装 daghttp 域专用的 DAG 运行时（与 lab/internal/dag 完全隔离）。
//
// Runtime 基于公共 invocation/app 轻量封装装配，注册自身 hello/echo ComputeUnit，
// 通过 app 图加载约定解析 YAML 图，通过 InvokeString 执行请求式调用。
type Runtime struct {
	app     *app.Runtime
	typeKey *dagv1.EntityTypeKey
}

// NewRuntime 创建 daghttp 运行时并注册 hello/echo ComputeUnit。
func NewRuntime(fixturesDir string) (*Runtime, error) {
	ar := app.New(
		app.WithGraphDir(fixturesDir),
		app.WithLoaderDefaults(labTypeKey, labSchema),
	)
	typeKey, err := ar.RegisterStringEntityType(labTypeKey, labSchema)
	if err != nil {
		_ = ar.Close()
		return nil, err
	}
	if err := ar.RegisterStringUnit("lab.hello", typeKey, helloFunc); err != nil {
		_ = ar.Close()
		return nil, err
	}
	if err := ar.RegisterStringUnit("lab.echo", typeKey, echoFunc); err != nil {
		_ = ar.Close()
		return nil, err
	}
	return &Runtime{app: ar, typeKey: typeKey}, nil
}

func helloFunc(_ context.Context, msg string) (string, error) {
	return "hello, " + msg, nil
}

func echoFunc(_ context.Context, msg string) (string, error) {
	return "echo:" + msg, nil
}

// LoadGraphFile 从 YAML 文件加载并注册图。
func (rt *Runtime) LoadGraphFile(path string) (*dagv1.GraphSpec, error) {
	return rt.app.LoadGraphFile(path)
}

// LoadFixture 按图 ID 从 fixtures 目录加载。
func (rt *Runtime) LoadFixture(graphID string) (*dagv1.GraphSpec, error) {
	return rt.app.LoadGraphID(graphID)
}

// Invoke 请求式调用 string payload 图实例。
func (rt *Runtime) Invoke(ctx context.Context, graphID, entityID, payload string) (*app.StringCallResult, error) {
	return rt.app.InvokeString(ctx, &app.StringCall{
		GraphID:  graphID,
		EntityID: entityID,
		Payload:  payload,
		TypeKey:  rt.typeKey,
	})
}

// LoadedGraphs 返回已加载图 ID。
func (rt *Runtime) LoadedGraphs() map[string]string {
	return rt.app.LoadedGraphs()
}

// Close 关闭运行时。
func (rt *Runtime) Close() error {
	return rt.app.Close()
}
