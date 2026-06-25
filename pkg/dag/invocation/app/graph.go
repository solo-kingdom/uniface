package app

import (
	"fmt"
	"os"
	"path/filepath"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag/graph"
	"github.com/solo-kingdom/uniface/pkg/dag/invocation/loader"
)

// LoadGraphFile 从文件加载 YAML 或 JSON 图文档，校验并注册 GraphSpec 与内联 ComputeUnitDef。
//
// 文件定位由调用方负责；公共 loader 仅解析字节内容。
func (r *Runtime) LoadGraphFile(path string) (*dagv1.GraphSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return r.loadGraphBytes(path, data)
}

// LoadGraphID 按上层约定从 graphDir 定位并加载图：优先 <graphID>.yaml，其次 <graphID>.json。
//
// graphDir 未配置时返回错误；公共 loader 包本身不绑定目录或文件名约定。
func (r *Runtime) LoadGraphID(graphID string) (*dagv1.GraphSpec, error) {
	if r.graphDir == "" {
		return nil, fmt.Errorf("app: graph directory not configured")
	}
	path, err := resolveGraphPath(r.graphDir, graphID)
	if err != nil {
		return nil, err
	}
	return r.LoadGraphFile(path)
}

// LoadGraphFromDir 从指定目录按 graph ID 约定加载图（不修改 Runtime 的 graphDir）。
func (r *Runtime) LoadGraphFromDir(graphID, dir string) (*dagv1.GraphSpec, error) {
	path, err := resolveGraphPath(dir, graphID)
	if err != nil {
		return nil, err
	}
	return r.LoadGraphFile(path)
}

func (r *Runtime) loadGraphBytes(sourcePath string, data []byte) (*dagv1.GraphSpec, error) {
	res, err := loader.LoadYAML(data, &r.loadOpts)
	if err != nil {
		return nil, err
	}
	if err := graph.ValidateGraphSpec(res.Spec); err != nil {
		return nil, err
	}
	if err := r.rt.RegisterGraph(res.Spec); err != nil {
		return nil, err
	}
	for _, def := range res.UnitDefs {
		if err := r.rt.RegisterComputeUnitDef(def); err != nil {
			return nil, err
		}
	}
	r.recordLoaded(res.Spec.Version.GraphId, sourcePath)
	return res.Spec, nil
}

func resolveGraphPath(dir, graphID string) (string, error) {
	for _, ext := range []string{".yaml", ".json"} {
		path := filepath.Join(dir, graphID+ext)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("app: graph %q not found in %q (.yaml or .json)", graphID, dir)
}
