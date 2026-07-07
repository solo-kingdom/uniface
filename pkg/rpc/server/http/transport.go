// Package http 提供基于标准库 net/http 的 Transport 实现。
//
// 仅依赖标准库，不引入 chi 等第三方库，故可放在根模块内、保持零外部依赖。
// lab 子模块仍可在自己的 UI 里用 chi（互不影响）。
package http

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	rpcserver "github.com/solo-kingdom/uniface/pkg/rpc/server"
)

// Transport 基于 net/http 的传输实现：把 (*Request,*Response) 信封与
// http.ResponseWriter / *http.Request 双向适配，用 http.ServeMux 挂载路由。
type Transport struct {
	mu       sync.Mutex
	server   *http.Server
	listener net.Listener
	// owned 标记 listener 是否由本传输创建（Close 时需关闭）。
	owned bool
}

// TransportOption 配置 HTTP Transport。
type TransportOption func(*Transport)

// WithListener 注入预创建的监听器（测试或自定义 socket 场景）。
// 设置后 Serve 忽略 addr 参数，直接复用该 listener。
func WithListener(ln net.Listener) TransportOption {
	return func(t *Transport) { t.listener = ln }
}

// NewTransport 创建 HTTP Transport。
func NewTransport(opts ...TransportOption) *Transport {
	t := &Transport{}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// Serve 在 addr 上阻塞监听，把 routes 物化为 http.ServeMux 路由。
// opts.ReadTimeout 透传给 http.Server。Shutdown 或 Close 使其返回（优雅关闭返回 nil）。
func (t *Transport) Serve(addr string, routes []rpcserver.RouteHandler, opts *rpcserver.Options) error {
	mux := http.NewServeMux()
	for _, rh := range routes {
		mux.HandleFunc(patternFor(rh.Route), adapt(rh.Handler))
	}

	var readTimeout time.Duration
	if opts != nil {
		readTimeout = opts.ReadTimeout
	}
	srv := &http.Server{
		Addr:        addr,
		Handler:     mux,
		ReadTimeout: readTimeout,
	}

	t.mu.Lock()
	t.server = srv
	ln := t.listener
	t.mu.Unlock()

	if ln == nil {
		var err error
		ln, err = net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		t.mu.Lock()
		t.listener = ln
		t.owned = true
		t.mu.Unlock()
	}

	err := srv.Serve(ln)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown 优雅关闭：处理完进行中请求后返回。
func (t *Transport) Shutdown(ctx context.Context) error {
	t.mu.Lock()
	srv := t.server
	t.mu.Unlock()
	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}

// Close 立即释放监听资源。
func (t *Transport) Close() error {
	t.mu.Lock()
	srv := t.server
	ln := t.listener
	owned := t.owned
	t.server = nil
	t.listener = nil
	t.owned = false
	t.mu.Unlock()

	var err error
	if srv != nil {
		err = srv.Close()
	}
	if owned && ln != nil {
		_ = ln.Close()
	}
	return err
}

// patternFor 将 Route 转为 Go 1.22+ ServeMux 模式（"METHOD /path"）。
// Method 为空时仅按 path 匹配（兼容所有方法）。
func patternFor(r rpcserver.Route) string {
	if r.Method == "" {
		return r.Path
	}
	return r.Method + " " + r.Path
}

// adapt 将传输无关 Handler 适配为 http.HandlerFunc：
// 把 *http.Request 映射为 Request 信封调用 handler，再把 Response 映射回 HTTP 响应。
func adapt(h rpcserver.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()

		req := &rpcserver.Request{
			Method: r.Method,
			Path:   r.URL.Path,
			Header: r.Header,
			Query:  r.URL.Query(),
			Body:   body,
		}
		resp, err := h(r.Context(), req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeResponse(w, resp)
	}
}

// writeResponse 把 Response 信封原样写回 HTTP 响应。
// StatusCode 为 0 时按 200 处理。
func writeResponse(w http.ResponseWriter, resp *rpcserver.Response) {
	if resp == nil {
		resp = &rpcserver.Response{}
	}
	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	status := resp.StatusCode
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	if len(resp.Body) > 0 {
		_, _ = w.Write(resp.Body)
	}
}

// NewHTTPServer 是便捷构造：等价 server.New(WithAddr(addr), WithTransport(httpTransport))。
// 额外 opts 可覆盖传输或追加中间件等。
func NewHTTPServer(addr string, opts ...rpcserver.Option) rpcserver.Server {
	merged := append([]rpcserver.Option{
		rpcserver.WithAddr(addr),
		rpcserver.WithTransport(NewTransport()),
	}, opts...)
	return rpcserver.New(merged...)
}

// 确保接口实现。
var _ rpcserver.Transport = (*Transport)(nil)
