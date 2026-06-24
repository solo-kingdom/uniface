package server

import (
	"context"
	"sync"
	"time"
)

// defaultShutdownTimeout 是 ctx 触发关闭时的排空宽限时间。
const defaultShutdownTimeout = 10 * time.Second

// Options 是 Server 的配置。
type Options struct {
	// Addr 是监听地址（如 ":8086"）。默认 ":8080"。
	Addr string

	// Transport 是具体传输实现。Start 前必须设置，否则返回 ErrNoTransport。
	Transport Transport

	// Middlewares 是中间件链，按注册顺序由外向内包裹 handler。
	Middlewares []Middleware

	// ReadTimeout 是读超时（透传给具体传输，如 HTTP 的 http.Server.ReadTimeout）。
	// 0 表示不限。
	ReadTimeout time.Duration
}

// Option 修改 Options。
type Option func(*Options)

// Apply 依次应用给定选项并返回 Options 自身。
func (o *Options) Apply(opts ...Option) *Options {
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// DefaultOptions 返回默认选项。
func DefaultOptions() *Options {
	return &Options{
		Addr:        ":8080",
		Transport:   nil,
		Middlewares: nil,
		ReadTimeout: 0,
	}
}

// MergeOptions 合并多个选项，后设置的覆盖先设置的；未设置的字段取默认值。
func MergeOptions(opts ...Option) *Options {
	return DefaultOptions().Apply(opts...)
}

// WithAddr 设置监听地址。
func WithAddr(addr string) Option {
	return func(o *Options) {
		o.Addr = addr
	}
}

// WithTransport 设置传输实现。
func WithTransport(t Transport) Option {
	return func(o *Options) {
		o.Transport = t
	}
}

// WithMiddleware 追加一个或多个中间件。多次调用按调用顺序累积。
//
// 示例:
//
//	srv := server.New(
//	    server.WithMiddleware(loggingMiddleware),
//	    server.WithMiddleware(recoverMiddleware),
//	)
//	// 请求处理顺序: logging → recover → handler → recover → logging
func WithMiddleware(mw ...Middleware) Option {
	return func(o *Options) {
		o.Middlewares = append(o.Middlewares, mw...)
	}
}

// WithReadTimeout 设置读超时。
func WithReadTimeout(d time.Duration) Option {
	return func(o *Options) {
		o.ReadTimeout = d
	}
}

// BaseServer 是 Server 接口的基础实现：持有路由表 + Options，把生命周期
// （Start/Shutdown/Close）委托给 Transport。具体实现可通过 New 直接使用。
//
// Start 时会把已注册的 (Route, Handler) 快照（应用了中间件链）传给 Transport.Serve。
type BaseServer struct {
	opts *Options

	mu     sync.RWMutex
	routes map[Route]Handler // 路由表（注册顺序由 order 保留）
	order  []Route
	closed bool
}

// New 创建 BaseServer 并应用给定选项。
func New(opts ...Option) *BaseServer {
	return &BaseServer{
		opts:   MergeOptions(opts...),
		routes: map[Route]Handler{},
	}
}

// Options 返回当前配置（只读使用，不要修改）。
func (s *BaseServer) Options() *Options { return s.opts }

// Handle 注册一条路由与处理器。
//   - handler 为 nil 返回 ErrInvalidHandler；
//   - 服务已 Close 返回 ErrServerClosed；
//   - 重复注册同一路由返回 ErrRouteExists。
func (s *BaseServer) Handle(route Route, handler Handler) error {
	if handler == nil {
		return NewServerError("Handle", route.Method+" "+route.Path, ErrInvalidHandler)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return NewServerError("Handle", route.Method+" "+route.Path, ErrServerClosed)
	}
	if _, exists := s.routes[route]; exists {
		return NewServerError("Handle", route.Method+" "+route.Path, ErrRouteExists)
	}
	s.routes[route] = handler
	s.order = append(s.order, route)
	return nil
}

// Start 阻塞启动服务：将路由快照交由 Transport 物化监听。
// ctx 取消或 Shutdown 调用后返回。
func (s *BaseServer) Start(ctx context.Context) error {
	s.mu.RLock()
	transport := s.opts.Transport
	if transport == nil {
		s.mu.RUnlock()
		return NewServerError("Start", "", ErrNoTransport)
	}
	routes := s.buildRoutesLocked()
	addr := s.opts.Addr
	opts := s.opts
	s.mu.RUnlock()

	errCh := make(chan error, 1)
	go func() { errCh <- transport.Serve(addr, routes, opts) }()

	select {
	case <-ctx.Done():
		// ctx 取消：触发优雅排空后等待 Serve 返回。
		sctx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
		defer cancel()
		_ = transport.Shutdown(sctx)
		return <-errCh
	case err := <-errCh:
		return err
	}
}

// Shutdown 优雅关闭，委托 Transport 处理。
func (s *BaseServer) Shutdown(ctx context.Context) error {
	s.mu.RLock()
	transport := s.opts.Transport
	s.mu.RUnlock()
	if transport == nil {
		return nil
	}
	return transport.Shutdown(ctx)
}

// Close 立即释放资源并标记服务已关闭；后续 Handle 返回 ErrServerClosed。
func (s *BaseServer) Close() error {
	s.mu.Lock()
	s.closed = true
	transport := s.opts.Transport
	s.mu.Unlock()
	if transport == nil {
		return nil
	}
	return transport.Close()
}

// buildRoutesLocked 构造应用了中间件链的路由快照。调用者须持读锁。
func (s *BaseServer) buildRoutesLocked() []RouteHandler {
	out := make([]RouteHandler, 0, len(s.order))
	for _, r := range s.order {
		out = append(out, RouteHandler{Route: r, Handler: s.chainLocked(s.routes[r])})
	}
	return out
}

// chainLocked 以洋葱模型包裹 handler：opts.Middlewares=[M1,M2] → M1(M2(h))。
// 调用者须持读锁。
func (s *BaseServer) chainLocked(h Handler) Handler {
	for i := len(s.opts.Middlewares) - 1; i >= 0; i-- {
		h = s.opts.Middlewares[i](h)
	}
	return h
}

// 确保接口实现。
var _ Server = (*BaseServer)(nil)
