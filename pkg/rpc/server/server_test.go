package server

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestMergeOptionsDefaults 验证未显式设置的字段取默认值，显式设置的被覆盖。
func TestMergeOptionsDefaults(t *testing.T) {
	opts := MergeOptions(WithAddr(":8086"), WithReadTimeout(5*time.Second))

	if opts.Addr != ":8086" {
		t.Fatalf("Addr = %q, want %q", opts.Addr, ":8086")
	}
	if opts.ReadTimeout != 5*time.Second {
		t.Fatalf("ReadTimeout = %v, want %v", opts.ReadTimeout, 5*time.Second)
	}
	if opts.Transport != nil {
		t.Fatalf("Transport = %v, want nil", opts.Transport)
	}
	if len(opts.Middlewares) != 0 {
		t.Fatalf("Middlewares len = %d, want 0", len(opts.Middlewares))
	}
}

// TestDefaultOptions 验证默认监听地址。
func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	if opts.Addr != ":8080" {
		t.Fatalf("default Addr = %q, want :8080", opts.Addr)
	}
}

// TestWithMiddlewareAccumulate 验证多次 WithMiddleware 按调用顺序累积。
func TestWithMiddlewareAccumulate(t *testing.T) {
	opts := MergeOptions(
		WithMiddleware(func(h Handler) Handler { return h }),
		WithMiddleware(func(h Handler) Handler { return h }),
	)
	if len(opts.Middlewares) != 2 {
		t.Fatalf("Middlewares len = %d, want 2", len(opts.Middlewares))
	}
}

// TestMiddlewareChainOrder 验证洋葱模型：M1 先于 M2 执行前半、M2 先于 M1 执行后半。
func TestMiddlewareChainOrder(t *testing.T) {
	var trace []string
	mk := func(name string) Middleware {
		return func(next Handler) Handler {
			return func(ctx context.Context, req *Request) (*Response, error) {
				trace = append(trace, name+":pre")
				resp, err := next(ctx, req)
				trace = append(trace, name+":post")
				return resp, err
			}
		}
	}

	srv := New(WithMiddleware(mk("M1")), WithMiddleware(mk("M2")))
	base := srv // *BaseServer
	wrapped := base.chainLocked(func(ctx context.Context, req *Request) (*Response, error) {
		trace = append(trace, "handler")
		return &Response{StatusCode: 200}, nil
	})

	resp, err := wrapped(context.Background(), &Request{Method: "POST", Path: "/echo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("StatusCode = %d, want 200", resp.StatusCode)
	}

	want := []string{"M1:pre", "M2:pre", "handler", "M2:post", "M1:post"}
	if len(trace) != len(want) {
		t.Fatalf("trace = %v, want %v", trace, want)
	}
	for i, w := range want {
		if trace[i] != w {
			t.Fatalf("trace[%d] = %q, want %q (full: %v)", i, trace[i], w, trace)
		}
	}
}

// TestMiddlewareChainEmpty 验证无中间件时 handler 原样返回。
func TestMiddlewareChainEmpty(t *testing.T) {
	srv := New()
	called := false
	h := srv.chainLocked(func(ctx context.Context, req *Request) (*Response, error) {
		called = true
		return &Response{StatusCode: 201}, nil
	})
	resp, err := h(context.Background(), &Request{})
	if err != nil || !called || resp.StatusCode != 201 {
		t.Fatalf("chain broken: resp=%+v called=%v err=%v", resp, called, err)
	}
}

// TestHandleDuplicateRoute 验证重复注册报 ErrRouteExists 且支持 errors.Is/As。
func TestHandleDuplicateRoute(t *testing.T) {
	srv := New()
	route := Route{Method: "POST", Path: "/echo"}
	stub := func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{StatusCode: 200}, nil
	}

	if err := srv.Handle(route, stub); err != nil {
		t.Fatalf("first Handle: %v", err)
	}
	err := srv.Handle(route, stub)
	if !errors.Is(err, ErrRouteExists) {
		t.Fatalf("expected ErrRouteExists, got %v", err)
	}
	var se *ServerError
	if !errors.As(err, &se) {
		t.Fatalf("expected *ServerError, got %T", err)
	}
	if se.Op != "Handle" {
		t.Fatalf("Op = %q, want Handle", se.Op)
	}
	if se.Key != "POST /echo" {
		t.Fatalf("Key = %q, want POST /echo", se.Key)
	}
}

// TestHandleInvalidHandler 验证 nil handler 报错。
func TestHandleInvalidHandler(t *testing.T) {
	srv := New()
	err := srv.Handle(Route{Method: "GET", Path: "/x"}, nil)
	if !errors.Is(err, ErrInvalidHandler) {
		t.Fatalf("expected ErrInvalidHandler, got %v", err)
	}
}

// TestHandleAfterClose 验证 Close 后注册报 ErrServerClosed。
func TestHandleAfterClose(t *testing.T) {
	srv := New()
	if err := srv.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	stub := func(ctx context.Context, req *Request) (*Response, error) { return nil, nil }
	err := srv.Handle(Route{Method: "GET", Path: "/x"}, stub)
	if !errors.Is(err, ErrServerClosed) {
		t.Fatalf("expected ErrServerClosed, got %v", err)
	}
}

// TestStartNoTransport 验证未配置 Transport 时 Start 报 ErrNoTransport。
func TestStartNoTransport(t *testing.T) {
	srv := New()
	err := srv.Start(context.Background())
	if !errors.Is(err, ErrNoTransport) {
		t.Fatalf("expected ErrNoTransport, got %v", err)
	}
}

// TestHandlePreservesOrder 验证路由按注册顺序保留（供 buildRoutesLocked 快照使用）。
func TestHandlePreservesOrder(t *testing.T) {
	srv := New()
	stub := func(ctx context.Context, req *Request) (*Response, error) { return nil, nil }
	routes := []Route{
		{Method: "GET", Path: "/a"},
		{Method: "POST", Path: "/b"},
		{Method: "GET", Path: "/c"},
	}
	for _, r := range routes {
		if err := srv.Handle(r, stub); err != nil {
			t.Fatalf("Handle %v: %v", r, err)
		}
	}
	srv.mu.RLock()
	got := srv.buildRoutesLocked()
	srv.mu.RUnlock()
	if len(got) != len(routes) {
		t.Fatalf("routes len = %d, want %d", len(got), len(routes))
	}
	for i, r := range routes {
		if got[i].Route != r {
			t.Fatalf("got[%d].Route = %v, want %v", i, got[i].Route, r)
		}
	}
}
