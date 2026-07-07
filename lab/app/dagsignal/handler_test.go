package dagsignal

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/solo-kingdom/uniface/pkg/dag/invocation/app"
	rpcserver "github.com/solo-kingdom/uniface/pkg/rpc/server"
)

// setupService 构造一个加载了 approval fixture 的 StringApp 与 Service。
// dagsignal 无 COMPUTE unit（演示焦点为 WAIT + signal），故不注册任何 unit。
func setupService(t *testing.T, graphID string) (*app.StringApp, *Service) {
	t.Helper()
	sa, err := app.NewStringApp(
		app.WithGraphDir(filepath.Join("fixtures", "graphs")),
		app.WithLoaderDefaults("lab.Generic", "v1"),
	)
	if err != nil {
		t.Fatalf("NewStringApp: %v", err)
	}
	if _, err := sa.LoadGraphID("approval"); err != nil {
		_ = sa.Close()
		t.Fatalf("LoadGraphID(approval): %v", err)
	}
	return sa, NewService(sa, graphID)
}

// instanceBody 是响应 body JSON 的解析结构。
type instanceBody struct {
	EntityID string `json:"entity_id"`
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
}

// parseBody 解析响应体为 instanceBody。
func parseBody(t *testing.T, b []byte) instanceBody {
	t.Helper()
	var body instanceBody
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatalf("unmarshal body %q: %v", b, err)
	}
	return body
}

// TestStart_ReturnsWaiting 验证 POST /start 返回 202 + WAITING + 非空 entity_id。
func TestStart_ReturnsWaiting(t *testing.T) {
	sa, svc := setupService(t, "approval")
	defer sa.Close()

	resp, err := svc.Start(context.Background(), &rpcserver.Request{
		Method: http.MethodPost,
		Path:   "/start",
		Body:   []byte("hello"),
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("StatusCode = %d, want 202; body=%q", resp.StatusCode, resp.Body)
	}
	body := parseBody(t, resp.Body)
	if body.EntityID == "" {
		t.Fatalf("entity_id empty; body=%q", resp.Body)
	}
	if body.Status != "INSTANCE_STATUS_WAITING" {
		t.Fatalf("Status = %q, want INSTANCE_STATUS_WAITING", body.Status)
	}
}

// TestSignal_PromotesToCompleted 验证 start → signal → 200 + COMPLETED。
func TestSignal_PromotesToCompleted(t *testing.T) {
	sa, svc := setupService(t, "approval")
	defer sa.Close()

	startResp, err := svc.Start(context.Background(), &rpcserver.Request{
		Method: http.MethodPost,
		Path:   "/start",
		Body:   []byte("hello"),
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	entityID := parseBody(t, startResp.Body).EntityID

	resp, err := svc.Signal(context.Background(), &rpcserver.Request{
		Method: http.MethodPost,
		Path:   "/signal/" + entityID,
	})
	if err != nil {
		t.Fatalf("Signal: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want 200; body=%q", resp.StatusCode, resp.Body)
	}
	body := parseBody(t, resp.Body)
	if body.Status != "INSTANCE_STATUS_COMPLETED" {
		t.Fatalf("Status = %q, want INSTANCE_STATUS_COMPLETED", body.Status)
	}
}

// TestSignal_NameMismatch_Returns400 验证 ?signal=unknown 返回 400。
func TestSignal_NameMismatch_Returns400(t *testing.T) {
	sa, svc := setupService(t, "approval")
	defer sa.Close()

	startResp, err := svc.Start(context.Background(), &rpcserver.Request{
		Method: http.MethodPost,
		Path:   "/start",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	entityID := parseBody(t, startResp.Body).EntityID

	resp, err := svc.Signal(context.Background(), &rpcserver.Request{
		Method: http.MethodPost,
		Path:   "/signal/" + entityID,
		Query:  map[string][]string{"signal": {"unknown"}},
	})
	if err != nil {
		t.Fatalf("Signal: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("StatusCode = %d, want 400; body=%q", resp.StatusCode, resp.Body)
	}
}

// TestSignal_UnknownEntity_Returns404 验证对不存在的实例 signal 返回 404。
func TestSignal_UnknownEntity_Returns404(t *testing.T) {
	sa, svc := setupService(t, "approval")
	defer sa.Close()

	resp, err := svc.Signal(context.Background(), &rpcserver.Request{
		Method: http.MethodPost,
		Path:   "/signal/nonexistent",
	})
	if err != nil {
		t.Fatalf("Signal: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("StatusCode = %d, want 404; body=%q", resp.StatusCode, resp.Body)
	}
}

// TestInstances_ReturnsStatus 验证 GET /instances/{entityID} 对已 start 的实例返回状态。
func TestInstances_ReturnsStatus(t *testing.T) {
	sa, svc := setupService(t, "approval")
	defer sa.Close()

	startResp, err := svc.Start(context.Background(), &rpcserver.Request{
		Method: http.MethodPost,
		Path:   "/start",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	entityID := parseBody(t, startResp.Body).EntityID

	resp, err := svc.Instances(context.Background(), &rpcserver.Request{
		Method: http.MethodGet,
		Path:   "/instances/" + entityID,
	})
	if err != nil {
		t.Fatalf("Instances: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("StatusCode = %d, want 202 (WAITING); body=%q", resp.StatusCode, resp.Body)
	}
	body := parseBody(t, resp.Body)
	if body.Status != "INSTANCE_STATUS_WAITING" {
		t.Fatalf("Status = %q, want INSTANCE_STATUS_WAITING", body.Status)
	}
}

// TestInstances_UnknownEntity_Returns404 验证查询不存在的实例返回 404。
func TestInstances_UnknownEntity_Returns404(t *testing.T) {
	sa, svc := setupService(t, "approval")
	defer sa.Close()

	resp, err := svc.Instances(context.Background(), &rpcserver.Request{
		Method: http.MethodGet,
		Path:   "/instances/nonexistent",
	})
	if err != nil {
		t.Fatalf("Instances: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("StatusCode = %d, want 404; body=%q", resp.StatusCode, resp.Body)
	}
}

// TestStatus_ContainsDomain 验证 /api/status 返回 JSON 且包含域名 dagsignal。
func TestStatus_ContainsDomain(t *testing.T) {
	sa, svc := setupService(t, "approval")
	defer sa.Close()

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
	if !strings.Contains(string(resp.Body), "dagsignal") {
		t.Fatalf("body = %q, want to contain dagsignal", resp.Body)
	}
}

// TestHandler_NotUsesInvokeString 确认 handler 源码不调 InvokeString 与
// dagbridge.ResponseForTerminalResult（异步应用的核心约束）。
//
// 使用 AST 扫描调用表达式（而非朴素字符串匹配），避免误伤解释性注释。
func TestHandler_NotUsesInvokeString(t *testing.T) {
	t.Helper()
	src, err := os.ReadFile("handler.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "handler.go", src, parser.AllErrors)
	if err != nil {
		t.Fatalf("parse handler.go: %v", err)
	}
	forbidden := map[string]string{
		// 异步应用 SHALL NOT 调用同步入口 InvokeString（见 spec）。
		"InvokeString": "StringApp.InvokeString（异步应用应走底层 Engine API）",
		// SHALL NOT 复用 dagbridge 同步映射（WAITING→500 语义冲突）。
		"ResponseForTerminalResult": "dagbridge.ResponseForTerminalResult",
	}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if why, hit := forbidden[sel.Sel.Name]; hit {
			t.Fatalf("handler.go 不应调用 %s（%s）", sel.Sel.Name, why)
		}
		return true
	})
	if strings.Contains(string(src), "atomic.Uint64") {
		t.Fatal("handler.go 不应自维护 entityID 计数器（应使用 app.EntityIDGen）")
	}
}

// TestRegister 验证路由注册（重复注册 /start 报错间接确认）。
func TestRegister(t *testing.T) {
	sa, svc := setupService(t, "approval")
	defer sa.Close()

	srv := rpcserver.New()
	if err := svc.Register(srv); err != nil {
		t.Fatalf("Register: %v", err)
	}
	// 重复注册 /start 应报错。
	if err := srv.Handle(rpcserver.Route{Method: http.MethodPost, Path: "/start"}, svc.Start); err == nil {
		t.Fatal("expected duplicate route error, got nil")
	}
}
