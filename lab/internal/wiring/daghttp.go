package wiring

import (
	"context"
	"fmt"
	"strings"

	"github.com/solo-kingdom/uniface/lab/internal/daghttp"
	"github.com/solo-kingdom/uniface/pkg/dag/invocation/app"
)

// NewDAGHTTP 构造 StringApp 运行时、加载 echo fixture，并装配 HTTP 服务。
// graphID 为空时 Service 默认使用 "echo"。
//
// 必填：cfg.FixturesDir；为空时返回 error，提示 lab/internal/daghttp.DefaultFixturesDir。
// 返回 *app.StringApp 是直接暴露给调用方的"类型化 Runtime 门面"。
func NewDAGHTTP(cfg DAGConfig, graphID string) (*app.StringApp, *daghttp.Service, error) {
	store := strings.ToLower(strings.TrimSpace(cfg.Store))
	if store == "" {
		store = "memory"
	}

	switch store {
	case "memory":
		if cfg.FixturesDir == "" {
			return nil, nil, fmt.Errorf("daghttp: cfg.FixturesDir is required; see lab/internal/daghttp.DefaultFixturesDir")
		}
		sa, err := app.NewStringApp(
			app.WithGraphDir(cfg.FixturesDir),
			app.WithLoaderDefaults("lab.Generic", "v1"),
		)
		if err != nil {
			return nil, nil, err
		}
		if err := registerLabUnits(sa); err != nil {
			_ = sa.Close()
			return nil, nil, err
		}
		if _, err := sa.LoadGraphID("echo"); err != nil {
			_ = sa.Close()
			return nil, nil, fmt.Errorf("load echo fixture: %w", err)
		}
		svc := daghttp.NewService(sa, graphID)
		return sa, svc, nil
	default:
		return nil, nil, fmt.Errorf("unsupported daghttp store: %s", store)
	}
}

// registerLabUnits 注册 daghttp 域专用单元。失败由 StringApp.RegisterUnit 自动 close。
func registerLabUnits(sa *app.StringApp) error {
	if err := sa.RegisterUnit("lab.hello", helloFunc); err != nil {
		return fmt.Errorf("register lab.hello: %w", err)
	}
	if err := sa.RegisterUnit("lab.echo", echoFunc); err != nil {
		return fmt.Errorf("register lab.echo: %w", err)
	}
	return nil
}

func helloFunc(_ context.Context, msg string) (string, error) {
	return "hello, " + msg, nil
}

func echoFunc(_ context.Context, msg string) (string, error) {
	return "echo:" + msg, nil
}
