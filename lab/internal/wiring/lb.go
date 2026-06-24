package wiring

import (
	"fmt"
	"strings"

	"github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer"
	"github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer/implementations/consistenthash"
	"github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer/implementations/random"
	"github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer/implementations/roundrobin"
	"github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer/implementations/weighted"
)

// NewBalancer 根据配置创建负载均衡器。
func NewBalancer(cfg LBConfig) (loadbalancer.Balancer[interface{}], string, error) {
	algo := strings.ToLower(strings.TrimSpace(cfg.Algo))
	if algo == "" {
		algo = "roundrobin"
	}

	switch algo {
	case "roundrobin", "rr":
		return roundrobin.New[interface{}](), algo, nil
	case "random":
		return random.New[interface{}](), algo, nil
	case "weighted":
		return weighted.New[interface{}](), algo, nil
	case "consistenthash", "hash":
		return consistenthash.New[interface{}](150, nil), algo, nil
	default:
		return nil, algo, fmt.Errorf("unsupported lb algo: %s", algo)
	}
}
