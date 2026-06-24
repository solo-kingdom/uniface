package daghttp

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	daglab "github.com/solo-kingdom/uniface/lab/internal/dag"
	rpcserver "github.com/solo-kingdom/uniface/pkg/rpc/server"
)

// setupService 构造一个加载了 echo fixture 的 Runtime 与 Service。
func setupService(t *testing.T, graphID string) (*daglab.Runtime, *Service) {
	t.Helper()
	rt, err := daglab.NewRuntime(filepath.Join("..", "dag", "..", "fixtures", "graphs"))
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	if _, err := rt.LoadFixture("echo"); err != nil {
		t.Fatalf("LoadFixture(echo): %v", err)
	}
	return rt, NewService(rt, graphID)
}

// setupFailingService 在临时目录构造一个入口即 failure terminal 的图，
// 用于确定性验证失败终态 → 5xx。
func setupFailingService(t *testing.T) (*daglab.Runtime, *Service) {
	t.Helper()
	dir := t.TempDir()
	// 入口节点为 failure terminal：实例一启动即 FAILED。
	if err := os.WriteFile(filepath.Join(dir, "failterm.yaml"), []byte(`
graph_id: failterm
version: v1
entity_type: lab.Generic
schema_version: v1
entry: fail
nodes:
  fail:
    kind: terminal
    outcome: failure
`), 0o644); err != nil {
		t.Fatalf("write failterm.yaml: %v", err)
	}
	rt, err := daglab.NewRuntime(dir)
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	if _, err := rt.LoadFixture("failterm"); err != nil {
		t.Fatalf("LoadFixture(failterm): %v", err)
	}
	return rt, NewService(rt, "failterm")
}

// TestEcho_GoldenPath 验证 echo:hello 黄金路径返回 200 与 echo:hello。
func TestEcho_GoldenPath(t *testing.T) {
	rt, svc := setupService(t, "echo")
	defer rt.Close()

	resp, err := svc.Echo(context.Background(), &rpcserver.Request{
		Method: http.MethodPost,
		Path:   "/echo",
		Body:   []byte("hello"),
	})
	if err != nil {
		t.Fatalf("Echo: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want 200; body=%q", resp.StatusCode, resp.Body)
	}
	if string(resp.Body) != "echo:hello" {
		t.Fatalf("Body = %q, want echo:hello", resp.Body)
	}
}

// TestEcho_EmptyBody 验证空请求体也能完成（echo:）。
func TestEcho_EmptyBody(t *testing.T) {
	rt, svc := setupService(t, "echo")
	defer rt.Close()

	resp, err := svc.Echo(context.Background(), &rpcserver.Request{
		Method: http.MethodPost,
		Path:   "/echo",
		Body:   nil,
	})
	if err != nil {
		t.Fatalf("Echo: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want 200; body=%q", resp.StatusCode, resp.Body)
	}
	if string(resp.Body) != "echo:" {
		t.Fatalf("Body = %q, want echo:", resp.Body)
	}
}

// TestEcho_FailureMaps5xx 验证失败终态（FAILED）映射为 500 并附原因。
func TestEcho_FailureMaps5xx(t *testing.T) {
	rt, svc := setupFailingService(t)
	defer rt.Close()

	resp, err := svc.Echo(context.Background(), &rpcserver.Request{
		Method: http.MethodPost,
		Path:   "/echo",
		Body:   []byte("boom"),
	})
	if err != nil {
		t.Fatalf("Echo: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("StatusCode = %d, want 500; body=%q", resp.StatusCode, resp.Body)
	}
	if !strings.Contains(string(resp.Body), "FAILED") {
		t.Fatalf("expected body to mention FAILED, got %q", resp.Body)
	}
}

// TestStatus 验证 /api/status 返回 JSON 且包含域名。
func TestStatus(t *testing.T) {
	rt, svc := setupService(t, "echo")
	defer rt.Close()

	resp, err := svc.Status(context.Background(), &rpcserver.Request{})
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if len(resp.Header["Content-Type"]) == 0 || resp.Header["Content-Type"][0] != "application/json" {
		t.Fatalf("Content-Type = %v, want application/json", resp.Header["Content-Type"])
	}
	if !strings.Contains(string(resp.Body), "daghttp") {
		t.Fatalf("body = %q, want to contain daghttp", resp.Body)
	}
}

// TestRegister 验证路由注册（重复注册 /echo 报错间接确认）。
func TestRegister(t *testing.T) {
	rt, svc := setupService(t, "echo")
	defer rt.Close()

	srv := rpcserver.New()
	if err := svc.Register(srv); err != nil {
		t.Fatalf("Register: %v", err)
	}
	// 重复注册 /echo 应报错。
	if err := srv.Handle(rpcserver.Route{Method: http.MethodPost, Path: "/echo"}, svc.Echo); err == nil {
		t.Fatal("expected duplicate route error, got nil")
	}
}
