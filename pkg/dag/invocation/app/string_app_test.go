package app_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/solo-kingdom/uniface/pkg/dag/invocation/app"
)

// TestStringApp_ConstructAndRegister 验证 NewStringApp + 多次 RegisterUnit 成功，
// 且 StringApp 内部 Runtime 已注册 StringValue 实体类型与对应 ComputeUnit。
func TestStringApp_ConstructAndRegister(t *testing.T) {
	sa, err := app.NewStringApp()
	if err != nil {
		t.Fatalf("NewStringApp: %v", err)
	}
	defer sa.Close()

	hello := func(_ context.Context, in string) (string, error) { return "hello, " + in, nil }
	echo := func(_ context.Context, in string) (string, error) { return "echo:" + in, nil }

	if err := sa.RegisterUnit("lab.hello", hello); err != nil {
		t.Fatalf("RegisterUnit(lab.hello): %v", err)
	}
	if err := sa.RegisterUnit("lab.echo", echo); err != nil {
		t.Fatalf("RegisterUnit(lab.echo): %v", err)
	}

	if _, err := sa.Memory().Registry().ResolveType(sa.TypeKey()); err != nil {
		t.Fatalf("ResolveType(typeKey): %v", err)
	}
	if _, err := sa.Memory().Registry().GetComputeUnit("lab.hello"); err != nil {
		t.Fatalf("GetComputeUnit(lab.hello): %v", err)
	}
	if _, err := sa.Memory().Registry().GetComputeUnit("lab.echo"); err != nil {
		t.Fatalf("GetComputeUnit(lab.echo): %v", err)
	}
}

// TestStringApp_RegisterFailureAutoClose 验证 RegisterUnit 失败时 StringApp
// 已自动关闭底层 Runtime（后续 InvokeString 反映"已关闭"状态而非 panic）。
//
// 触发方式：底层 registry 拒绝空 unit_id（返回 ErrInvalidGraph 风格错误），
// StringApp.RegisterUnit 据此触发自动 Close。
func TestStringApp_RegisterFailureAutoClose(t *testing.T) {
	sa, err := app.NewStringApp()
	if err != nil {
		t.Fatalf("NewStringApp: %v", err)
	}

	// 先注册一个成功单元确认 StringApp 健康。
	if err := sa.RegisterUnit("lab.ok", func(_ context.Context, in string) (string, error) {
		return in, nil
	}); err != nil {
		t.Fatalf("baseline RegisterUnit: %v", err)
	}

	// 用空 unit_id 触发底层 registry 校验错误。
	err = sa.RegisterUnit("", func(_ context.Context, in string) (string, error) {
		return in, nil
	})
	if err == nil {
		t.Fatal("expected empty-unitID error, got nil")
	}

	// StringApp 应已自动关闭底层 Runtime —— 后续 InvokeString 应反映已关闭状态。
	if _, err := sa.InvokeString(context.Background(), "any", "e1", "x"); err == nil {
		t.Fatal("InvokeString on closed StringApp should error")
	}
}

// TestStringApp_InvokeString_HidesTypeKey 验证调用方不需要提供 TypeKey；
// 内部使用 StringApp 绑定的 typeKey 构造 StringCall。
func TestStringApp_InvokeString_HidesTypeKey(t *testing.T) {
	dir := t.TempDir()
	sa, err := app.NewStringApp(app.WithGraphDir(dir), app.WithLoaderDefaults("app.String", "v1"))
	if err != nil {
		t.Fatalf("NewStringApp: %v", err)
	}
	defer sa.Close()

	echo := func(_ context.Context, in string) (string, error) { return "echo:" + in, nil }
	if err := sa.RegisterUnit("lab.echo", echo); err != nil {
		t.Fatalf("RegisterUnit: %v", err)
	}

	const yaml = `graph_id: echo
version: v1
entity_type: app.String
schema_version: v1
entry: echo
nodes:
  echo:
    kind: compute
    unit: lab.echo
    transitions:
      - target: done
  done:
    kind: terminal
    outcome: success
`
	if err := os.WriteFile(filepath.Join(dir, "echo.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write echo yaml: %v", err)
	}
	if _, err := sa.LoadGraphID("echo"); err != nil {
		t.Fatalf("LoadGraphID: %v", err)
	}

	res, err := sa.InvokeString(context.Background(), "echo", "e-1", "hello")
	if err != nil {
		t.Fatalf("InvokeString: %v", err)
	}
	if !res.IsCompleted() {
		t.Fatalf("Status = %s, want COMPLETED", res.Status())
	}
	if res.Value != "echo:hello" {
		t.Fatalf("Value = %q, want echo:hello", res.Value)
	}
}

// TestStringApp_NoLabUnitsByDefault 验证 NewStringApp 不内置任何 lab.* 业务单元。
func TestStringApp_NoLabUnitsByDefault(t *testing.T) {
	sa, err := app.NewStringApp()
	if err != nil {
		t.Fatalf("NewStringApp: %v", err)
	}
	defer sa.Close()

	for _, unitID := range []string{"lab.hello", "lab.echo", "lab.anything"} {
		if _, err := sa.Memory().Registry().GetComputeUnit(unitID); err == nil {
			t.Fatalf("不应内置单元 %q", unitID)
		}
	}
	// StringApp 的默认 entityType 也不是 lab.* —— 验证类型为 app.String。
	if sa.TypeKey().EntityType != "app.String" {
		t.Fatalf("default EntityType = %q, want app.String", sa.TypeKey().EntityType)
	}
}
