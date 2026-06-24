package wiring

import (
	"fmt"

	daglab "github.com/solo-kingdom/uniface/lab/internal/dag"
	"github.com/solo-kingdom/uniface/lab/internal/daghttp"
)

// NewDAGHTTP 构造 DAG 运行时、加载 echo fixture，并装配 daghttp 服务。
// graphID 为空时 Service 默认使用 "echo"。
func NewDAGHTTP(cfg DAGConfig, graphID string) (*daglab.Runtime, *daghttp.Service, error) {
	rt, _, err := NewDAG(cfg)
	if err != nil {
		return nil, nil, err
	}
	if _, err := rt.LoadFixture("echo"); err != nil {
		_ = rt.Close()
		return nil, nil, fmt.Errorf("load echo fixture: %w", err)
	}
	svc := daghttp.NewService(rt, graphID)
	return rt, svc, nil
}
