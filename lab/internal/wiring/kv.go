package wiring

import (
	"fmt"
	"strings"

	"github.com/solo-kingdom/uniface/pkg/storage/kv"
	"github.com/solo-kingdom/uniface/pkg/storage/kv/boltdb"
	"github.com/solo-kingdom/uniface/pkg/storage/kv/redis"
)

// NewKV 根据配置创建 KV 存储实现。
func NewKV(cfg KVConfig) (kv.Storage, string, error) {
	impl := strings.ToLower(strings.TrimSpace(cfg.Impl))
	if impl == "" {
		impl = "redis"
	}

	switch impl {
	case "redis":
		store, err := redis.New(redis.WithAddr(cfg.Addr))
		if err != nil {
			return nil, impl, err
		}
		return store, impl, nil
	case "boltdb", "bolt":
		path := cfg.Path
		if path == "" {
			path = "/tmp/uniface-lab/kv.bolt"
		}
		store, err := boltdb.New(boltdb.WithPath(path))
		if err != nil {
			return nil, impl, err
		}
		return store, impl, nil
	case "aerospike":
		return nil, impl, fmt.Errorf("aerospike kv adapter does not fully implement kv.Storage in current pkg; use redis or boltdb")
	default:
		return nil, impl, fmt.Errorf("unsupported kv impl: %s", impl)
	}
}

func splitHostPort(addr, defaultPort string) (string, string) {
	if addr == "" {
		return "localhost", defaultPort
	}
	if strings.Contains(addr, ":") {
		parts := strings.SplitN(addr, ":", 2)
		return parts[0], parts[1]
	}
	return addr, defaultPort
}
