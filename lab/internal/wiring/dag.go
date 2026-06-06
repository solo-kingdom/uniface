package wiring

import (
	"fmt"
	"strings"

	"github.com/solo-kingdom/uniface/lab/internal/dag"
)

// NewDAG 根据配置创建 DAG 运行时。
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
		rt, err := dag.NewRuntime(fixtures)
		if err != nil {
			return nil, store, err
		}
		return rt, store, nil
	default:
		return nil, store, fmt.Errorf("unsupported dag store: %s", store)
	}
}
