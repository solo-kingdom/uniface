package wiring

import (
	"fmt"
	"strings"

	"github.com/solo-kingdom/uniface/lab/internal/dag"
)

// NewDAG 根据配置创建 DAG 运行时。
// 当前 HttpUnit 通过 url 直连模式工作（resolver 注入为 nil）；
// 业务进程可按需通过 dag.WithHTTPResolver 注入 balanceradapter 包装的 Balancer 解析器。
func NewDAG(cfg DAGConfig) (*dag.Runtime, string, error) {
	store := strings.ToLower(strings.TrimSpace(cfg.Store))
	if store == "" {
		store = "memory"
	}

	switch store {
	case "memory":
		fixtures := cfg.FixturesDir
		if fixtures == "" {
			fixtures = "internal/fixtures/graphs"
		}
		// 暂注入 nil resolver：HttpUnit 仅支持 url 直连（http_call fixture 即用直连）。
		// 若需 service 路由，调用方在此注入 balanceradapter.New(balancer)。
		rt, err := dag.NewRuntime(fixtures)
		if err != nil {
			return nil, store, err
		}
		return rt, store, nil
	default:
		return nil, store, fmt.Errorf("unsupported dag store: %s", store)
	}
}
