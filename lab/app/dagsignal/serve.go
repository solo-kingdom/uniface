package dagsignal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	rpchttp "github.com/solo-kingdom/uniface/pkg/rpc/server/http"
	"github.com/solo-kingdom/uniface/pkg/dag/invocation/app"
)

// LoadConfig 解析 LAB_CONFIG 或 configs/default.yaml 中的 `dagsignal` 段，
// 应用 LAB_DAGSIGNAL_STORE / LAB_DAGSIGNAL_FIXTURES_DIR 环境变量覆写，并回退到
// dagsignal 自身默认值（FixturesDir 缺省时使用 DefaultFixturesDir）。
//
// 注意：dagsignal 自治其配置 schema，使用独立的 `dagsignal` 段与 `LAB_DAGSIGNAL_*`
// env 前缀，不依赖 LabConfig 跨域聚合，也不与 `dag`/`daghttp` 段冲突。
func LoadConfig() (*Config, error) {
	path, err := resolveConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	// 仅解析 `dagsignal` 段。
	raw := struct {
		DAGSignal Config `yaml:"dagsignal"`
	}{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	cfg := &raw.DAGSignal

	if cfg.FixturesDir == "" {
		cfg.FixturesDir = DefaultFixturesDir
	}
	if cfg.Store == "" {
		cfg.Store = "memory"
	}
	if v := os.Getenv("LAB_DAGSIGNAL_STORE"); v != "" {
		cfg.Store = v
	}
	if v := os.Getenv("LAB_DAGSIGNAL_FIXTURES_DIR"); v != "" {
		cfg.FixturesDir = v
	}
	return cfg, nil
}

// resolveConfigPath 解析 LAB_CONFIG 或候选默认配置路径。
func resolveConfigPath() (string, error) {
	if path := os.Getenv("LAB_CONFIG"); path != "" {
		return path, nil
	}
	candidates := []string{
		"configs/default.yaml",
		filepath.Join("lab", "configs", "default.yaml"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("config file not found; set LAB_CONFIG or create configs/default.yaml")
}

// Serve 构造 dagsignal StringApp 运行时、加载 approval fixture，注册 /start、
// /signal/{entityID}、/instances/{entityID} 与 /api/status 路由到统一 rpc.Server
// 抽象，并在 addr 上阻塞监听至 ctx 取消。
//
// 返回前会确保 *app.StringApp.Close 释放底层资源（即使 Register / Start 失败
// 也通过 defer rt.Close() 兜底）。
func Serve(ctx context.Context, addr string, cfg *Config) error {
	rt, err := buildRuntime(cfg)
	if err != nil {
		return err
	}
	defer rt.Close()

	svc := NewService(rt, defaultGraphID)
	srv := rpchttp.NewHTTPServer(addr)
	if err := svc.Register(srv); err != nil {
		return fmt.Errorf("register routes: %w", err)
	}

	fmt.Printf("lab-dag-signal listening on %s (POST /start)\n", addr)
	return srv.Start(ctx)
}

// buildRuntime 根据 cfg 构建 StringApp 并加载 approval fixture。
//
// dagsignal 演示焦点为 WAIT + signal 路由，故不注册任何 COMPUTE unit
// （与 daghttp 的 registerUnits 不同）。异步路径走底层 Engine API，参见
// pkg/dag/invocation/app/doc.go。
func buildRuntime(cfg *Config) (*app.StringApp, error) {
	store := strings.ToLower(strings.TrimSpace(cfg.Store))
	if store == "" {
		store = "memory"
	}

	switch store {
	case "memory":
		if cfg.FixturesDir == "" {
			return nil, fmt.Errorf("dagsignal: cfg.FixturesDir is required; see dagsignal.DefaultFixturesDir")
		}
		sa, err := app.NewStringApp(
			app.WithGraphDir(cfg.FixturesDir),
			app.WithLoaderDefaults("lab.Generic", "v1"),
		)
		if err != nil {
			return nil, err
		}
		// dagsignal 不注册 COMPUTE unit（演示焦点为 WAIT + signal）。
		if _, err := sa.LoadGraphID(defaultGraphID); err != nil {
			_ = sa.Close()
			return nil, fmt.Errorf("load approval fixture: %w", err)
		}
		return sa, nil
	default:
		return nil, fmt.Errorf("unsupported dagsignal store: %s", store)
	}
}
