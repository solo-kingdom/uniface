package daghttp

import (
	"context"
	"fmt"

	"github.com/solo-kingdom/uniface/pkg/dag/invocation/app"
)

// registerUnits 注册 daghttp 域专用单元。失败由 StringApp.RegisterUnit 自动 close 底层 Runtime。
func registerUnits(sa *app.StringApp) error {
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
