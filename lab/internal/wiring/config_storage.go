package wiring

import (
	"fmt"
	"strings"

	"github.com/solo-kingdom/uniface/pkg/rpc/governance/config"
	"github.com/solo-kingdom/uniface/pkg/rpc/governance/config/consul"
)

// NewConfigStorage 根据配置创建配置存储实现。
func NewConfigStorage(cfg ConfigDomain) (config.Storage, string, error) {
	impl := strings.ToLower(strings.TrimSpace(cfg.Impl))
	if impl == "" {
		impl = "consul"
	}

	switch impl {
	case "consul":
		addr := cfg.Addr
		if addr == "" {
			addr = "127.0.0.1:8500"
		}
		prefix := cfg.Prefix
		if prefix == "" {
			prefix = "lab/"
		}
		store, err := consul.NewStorage(consul.WithAddress(addr), consul.WithKeyPrefix(prefix))
		if err != nil {
			return nil, impl, err
		}
		return store, impl, nil
	default:
		return nil, impl, fmt.Errorf("unsupported config impl: %s", impl)
	}
}
