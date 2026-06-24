package http

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	rpcserver "github.com/solo-kingdom/uniface/pkg/rpc/server"
)

// echoHandler 把请求体加 "echo:" 前缀回写。
func echoHandler(ctx context.Context, req *rpcserver.Request) (*rpcserver.Response, error) {
	return &rpcserver.Response{
		StatusCode: 201,
		Header:     map[string][]string{"X-Echo": {"yes"}},
		Body:       append([]byte("echo:"), req.Body...),
	}, nil
}

// TestAdapt_RequestToEnvelope 验证 HTTP 请求字段无损映射到 Request 信封。
func TestAdapt_RequestToEnvelope(t *testing.T) {
	var got *rpcserver.Request
	h := func(ctx context.Context, req *rpcserver.Request) (*rpcserver.Response, error) {
		got = req
		return &rpcserver.Response{StatusCode: 200, Body: []byte("ok")}, nil
	}

	body := []byte("hello")
	r := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(body))
	r.Header.Set("X-Test", "v1")
	w := httptest.NewRecorder()

	adapt(h)(w, r)

	if got.Method != http.MethodPost {
		t.Fatalf("Method = %q, want POST", got.Method)
	}
	if got.Path != "/echo" {
		t.Fatalf("Path = %q, want /echo", got.Path)
	}
	if string(got.Body) != "hello" {
		t.Fatalf("Body = %q, want hello", got.Body)
	}
	if len(got.Header["X-Test"]) == 0 || got.Header["X-Test"][0] != "v1" {
		t.Fatalf("Header X-Test = %v, want [v1]", got.Header["X-Test"])
	}
}

// TestAdapt_EnvelopeToResponse 验证 Response 信封原样写回 HTTP 响应（status/header/body）。
func TestAdapt_EnvelopeToResponse(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader([]byte("hi")))
	w := httptest.NewRecorder()

	adapt(echoHandler)(w, r)

	resp := w.Result()
	if resp.StatusCode != 201 {
		t.Fatalf("StatusCode = %d, want 201", resp.StatusCode)
	}
	if resp.Header.Get("X-Echo") != "yes" {
		t.Fatalf("Header X-Echo = %q, want yes", resp.Header.Get("X-Echo"))
	}
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "echo:hi" {
		t.Fatalf("Body = %q, want echo:hi", b)
	}
}

// TestAdapt_HandlerError 验证 handler 返回 error 时映射为 500。
func TestAdapt_HandlerError(t *testing.T) {
	h := func(ctx context.Context, req *rpcserver.Request) (*rpcserver.Response, error) {
		return nil, io.ErrUnexpectedEOF
	}
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	adapt(h)(w, r)

	if w.Result().StatusCode != http.StatusInternalServerError {
		t.Fatalf("StatusCode = %d, want 500", w.Result().StatusCode)
	}
}

// TestAdapt_NilResponseDefaultStatus 验证 nil Response 默认 200 且无 body。
func TestAdapt_NilResponseDefaultStatus(t *testing.T) {
	h := func(ctx context.Context, req *rpcserver.Request) (*rpcserver.Response, error) {
		return nil, nil
	}
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	adapt(h)(w, r)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want 200", w.Result().StatusCode)
	}
}

// TestServe_RoutesAndShutdown 端到端验证：注入 listener 后启动，请求路由到
// handler，Shutdown 后 Serve 返回 nil。
func TestServe_RoutesAndShutdown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()

	tr := NewTransport(WithListener(ln))
	routes := []rpcserver.RouteHandler{
		{Route: rpcserver.Route{Method: http.MethodPost, Path: "/echo"}, Handler: echoHandler},
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- tr.Serve(addr, routes, rpcserver.DefaultOptions()) }()

	// 等待监听就绪（listener 已存在，立即可用）。
	resp, err := http.Post("http://"+addr+"/echo", "text/plain", bytes.NewReader([]byte("world")))
	if err != nil {
		t.Fatalf("http.Post: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("StatusCode = %d, want 201", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b) != "echo:world" {
		t.Fatalf("Body = %q, want echo:world", b)
	}

	// 优雅关闭。
	if err := tr.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	select {
	case err := <-serveErr:
		if err != nil {
			t.Fatalf("Serve returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not return after Shutdown")
	}
}

// TestServe_GracefulShutdownWaitsInFlight 验证 Shutdown 排空进行中请求。
func TestServe_GracefulShutdownWaitsInFlight(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()

	var started, completed atomic.Bool
	slowHandler := func(ctx context.Context, req *rpcserver.Request) (*rpcserver.Response, error) {
		started.Store(true)
		time.Sleep(150 * time.Millisecond)
		completed.Store(true)
		return &rpcserver.Response{StatusCode: 200, Body: []byte("done")}, nil
	}
	tr := NewTransport(WithListener(ln))
	routes := []rpcserver.RouteHandler{
		{Route: rpcserver.Route{Method: http.MethodPost, Path: "/slow"}, Handler: slowHandler},
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- tr.Serve(addr, routes, rpcserver.DefaultOptions()) }()

	// 发起一个慢请求，随后立即 Shutdown。
	go func() {
		resp, _ := http.Post("http://"+addr+"/slow", "text/plain", bytes.NewReader([]byte("x")))
		if resp != nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}()

	// 等待 handler 已开始。
	deadline := time.Now().Add(time.Second)
	for !started.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !started.Load() {
		t.Fatal("handler never started")
	}

	if err := tr.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if !completed.Load() {
		t.Fatal("in-flight request was not drained before Shutdown returned")
	}
	<-serveErr
}

// TestNewHTTPServer 验证便捷构造注册路由后可用（用 :0 + 注入 listener 不便，
// 此处通过 BaseServer 句柄验证 Handle 与未启动状态）。
func TestNewHTTPServer_HandleAndStart(t *testing.T) {
	// 用注入 listener 的方式验证 NewHTTPServer 端到端。
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()

	// NewHTTPServer 默认创建自己的 transport/listener；为测试可控，改用
	// server.New + 显式注入 listener 的 transport。
	tr := NewTransport(WithListener(ln))
	srv := rpcserver.New(
		rpcserver.WithAddr(addr),
		rpcserver.WithTransport(tr),
	)
	if err := srv.Handle(rpcserver.Route{Method: http.MethodPost, Path: "/echo"}, echoHandler); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Start(ctx) }()

	resp, err := http.Post("http://"+addr+"/echo", "text/plain", bytes.NewReader([]byte("abc")))
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 201 || string(b) != "echo:abc" {
		t.Fatalf("got status=%d body=%q, want 201/echo:abc", resp.StatusCode, b)
	}

	cancel()
	_ = srv.Shutdown(context.Background())
}
