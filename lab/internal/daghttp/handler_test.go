package daghttp

import (
	"context"
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rpcserver "github.com/solo-kingdom/uniface/pkg/rpc/server"
)

// setupService 构造一个加载了 echo fixture 的 Runtime 与 Service。
func setupService(t *testing.T, graphID string) (*Runtime, *Service) {
	t.Helper()
	rt, err := NewRuntime(filepath.Join("fixtures", "graphs"))
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
func setupFailingService(t *testing.T) (*Runtime, *Service) {
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
	rt, err := NewRuntime(dir)
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	if _, err := rt.LoadFixture("failterm"); err != nil {
		t.Fatalf("LoadFixture(failterm): %v", err)
	}
	return rt, NewService(rt, "failterm")
}

// TestEcho_GoldenPath 验证 hello → echo 两节点黄金路径。
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
	if string(resp.Body) != "echo:hello, hello" {
		t.Fatalf("Body = %q, want echo:hello, hello", resp.Body)
	}
}

// TestEcho_EmptyBody 验证空请求体也能完成（hello 节点仍加前缀）。
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
	if string(resp.Body) != "echo:hello, " {
		t.Fatalf("Body = %q, want echo:hello, ", resp.Body)
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

// TestHandler_UsesAppFacade 确认 handler 源码不直接构造 InvokeRequest 或手写 anypb 编解码。
func TestHandler_UsesAppFacade(t *testing.T) {
	t.Helper()
	src, err := os.ReadFile("handler.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	if strings.Contains(body, "InvokeRequest") {
		t.Fatal("handler.go 不应直接构造 invocation.InvokeRequest")
	}
	if strings.Contains(body, "anypb.") || strings.Contains(body, "MarshalString") || strings.Contains(body, "UnmarshalString") {
		t.Fatal("handler.go 不应手写 anypb/StringValue 编解码")
	}
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "handler.go", body, parser.AllErrors); err != nil {
		t.Fatalf("parse handler.go: %v", err)
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
