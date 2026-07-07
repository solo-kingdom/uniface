// Package server 提供传输无关的统一 RPC 服务抽象。
//
// 本包定义统一服务契约：通过 Handle 注册传输无关的 Handler，由具体的
// Transport 实现（如 net/http）将路由集合物化为监听。同一组 Handler 可
// 在不同传输（HTTP、gRPC 等）间复用与热切换——这正是「统一封装」的目标。
//
// 根模块保持零外部依赖：interface.go 仅依赖标准库 context。
package server

import "context"

// Request 是传输无关的请求信封，可无损承载 HTTP 语义（method、path、
// header、body），并为未来 gRPC（path→fully-qualified method、body→protobuf
// 字节）预留映射空间。
type Request struct {
	// Method 是请求方法（HTTP 下为 GET/POST 等；gRPC 下可映射为空或固定值）。
	Method string
	// Path 是请求路径（HTTP 下为 URL path；gRPC 下可映射为 fully-qualified method）。
	Path string
	// Header 是请求头，键值对（HTTP 下为 map[string][]string 语义）。
	Header map[string][]string
	// Query 是 URL 查询参数（HTTP 下来自 r.URL.Query()；其它传输可留空）。
	// 添加本字段以支持依赖 query 参数的端点（如 dagsignal 的 ?signal= 覆盖），
	// 属向后兼容的增量字段：既有 handler 不读 Query 不受影响。
	Query map[string][]string
	// Body 是请求体原始字节。
	Body []byte
}

// Response 是传输无关的响应信封。
type Response struct {
	// StatusCode 是响应状态码（HTTP 下为 HTTP 状态码；0 表示 200）。
	StatusCode int
	// Header 是响应头。
	Header map[string][]string
	// Body 是响应体原始字节。
	Body []byte
}

// Handler 处理一个请求并返回响应。
// 实现可通过返回 error 表达处理失败，由传输层映射为 5xx。
type Handler func(ctx context.Context, req *Request) (*Response, error)

// Middleware 包裹 Handler。按 WithMiddleware 注册顺序由外向内组成调用链
// （洋葱模型）：先注册的中间件其前半逻辑先执行、后半逻辑后执行。
type Middleware func(Handler) Handler

// Route 标识一条路由（HTTP 方法 + 路径）。
type Route struct {
	Method string
	Path   string
}

// RouteHandler 是一条已注册路由及其处理器，由 Transport 物化为具体传输监听。
type RouteHandler struct {
	Route   Route
	Handler Handler
}

// Transport 把已注册的 (Route, Handler) 集合物化为具体传输监听。
// 新增传输（如 gRPC）只需实现该接口，不触动 Handler 与路由代码。
//
// 线程安全：所有实现必须是线程安全的。
type Transport interface {
	// Serve 在 addr 上阻塞监听，将到达的请求路由到对应 handler。
	// Shutdown 或 Close 会使其返回（优雅关闭返回 nil）。
	Serve(addr string, routes []RouteHandler, opts *Options) error

	// Shutdown 优雅关闭：停止接受新连接，处理完进行中请求后返回。
	Shutdown(ctx context.Context) error

	// Close 立即释放监听资源（不做排空）。
	Close() error
}

// Server 定义统一服务生命周期。
//
// 线程安全：所有实现必须是线程安全的。
// 资源管理：Close() 必须正确释放所有资源。
type Server interface {
	// Handle 注册一条路由与处理器。重复注册同一路由返回 ErrRouteExists。
	Handle(route Route, handler Handler) error

	// Start 阻塞启动服务，在配置地址上监听。
	// 当 ctx 取消或 Shutdown 被调用后返回。
	Start(ctx context.Context) error

	// Shutdown 优雅关闭：停止接受新连接，处理完进行中请求后返回。
	Shutdown(ctx context.Context) error

	// Close 立即释放所有资源。
	Close() error
}
