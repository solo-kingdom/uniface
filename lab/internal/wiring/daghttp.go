package wiring

import (
	"fmt"
	"strings"

	"github.com/solo-kingdom/uniface/lab/internal/daghttp"
)

// NewDAGHTTP 构造 daghttp 运行时、加载 echo fixture，并装配 HTTP 服务。
// graphID 为空时 Service 默认使用 "echo"。
func NewDAGHTTP(cfg DAGConfig, graphID string) (*daghttp.Runtime, *daghttp.Service, error) {
	store := strings.ToLower(strings.TrimSpace(cfg.Store))
	if store == "" {
		store = "memory"
	}

	switch store {
	case "memory":
		fixtures := cfg.FixturesDir
		if fixtures == "" {
			fixtures = "internal/daghttp/fixtures/graphs"
		}
		rt, err := daghttp.NewRuntime(fixtures)
		if err != nil {
			return nil, nil, err
		}
		if _, err := rt.LoadFixture("echo"); err != nil {
			_ = rt.Close()
			return nil, nil, fmt.Errorf("load echo fixture: %w", err)
		}
		svc := daghttp.NewService(rt, graphID)
		return rt, svc, nil
	default:
		return nil, nil, fmt.Errorf("unsupported daghttp store: %s", store)
	}
}
