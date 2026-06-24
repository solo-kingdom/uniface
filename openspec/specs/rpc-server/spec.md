# rpc-server Specification

## Purpose
TBD - created by archiving change add-rpc-server-dag-http-lab. Update Purpose after archive.
## Requirements
### Requirement: 统一服务生命周期

系统 SHALL 在 `pkg/rpc/server` 提供传输无关的 `Server` 抽象，支持通过 `Handle(route Route, handler Handler)` 注册处理器、`Start(ctx)` 阻塞启动、`Shutdown(ctx)` 优雅关闭、`Close()` 释放资源。`Server` SHALL 委托 `Transport` 将已注册的 `(Route, Handler)` 集合物化为具体传输监听，使同一组处理器可在不同传输间复用。所有公开接口遵循 `interface.go` / `options.go` / `errors.go` 布局，根模块 SHALL 保持零外部依赖。

#### Scenario: 注册处理器并启动服务

- **WHEN** 创建 `Server`、注册一条路由并调用 `Start`
- **THEN** 服务在配置地址上监听，将该路由的请求交给对应 `Handler` 处理并返回其 `Response`

#### Scenario: 优雅关闭

- **WHEN** 已启动的服务收到 `Shutdown(ctx)`
- **THEN** 服务停止接受新连接，处理完进行中的请求后返回，并释放监听资源

#### Scenario: 同一处理器跨传输复用

- **WHEN** 同一 `Handler` 注册到 Server，且 Server 通过 `WithTransport` 切换为不同传输实现
- **THEN** 该处理器无需修改即可在新传输上被调用

### Requirement: 传输无关请求信封

系统 SHALL 定义 `Request`（含 `Method`、`Path`、`Header`、`Body []byte`）与 `Response`（含 `StatusCode`、`Header`、`Body []byte`）信封；`Handler` 签名 SHALL 为 `func(ctx context.Context, req *Request) (*Response, error)`。信封 SHALL 能无损承载 HTTP 语义（method、path、header、body、status code），并为未来 gRPC（path→method、body→protobuf 字节）预留映射空间。

#### Scenario: HTTP 请求映射为 Request 信封

- **WHEN** 一个 HTTP 请求到达 HTTP 传输
- **THEN** 其 method、path、header、body 被无损填入 `Request`，并以此调用注册的 `Handler`

#### Scenario: Response 信封映射为 HTTP 响应

- **WHEN** `Handler` 返回一个 `Response`
- **THEN** 其 `StatusCode`、`Header`、`Body` 被原样写回 HTTP 响应

### Requirement: HTTP 传输实现

系统 SHALL 在 `pkg/rpc/server/http` 提供基于标准库 `net/http` 的 `Transport` 实现，SHALL NOT 引入 chi 或其他第三方 HTTP 库（保持根模块零外部依赖）。SHALL 提供便捷构造 `NewHTTPServer(addr string, opts ...Option) Server`，内部等价于 `New(WithAddr(addr), WithTransport(httpTransport))`。

#### Scenario: 标准库监听

- **WHEN** 使用 `NewHTTPServer(":0", ...)` 注册路由后启动
- **THEN** 服务通过标准库 `net/http` 在指定地址监听并将请求路由到处理器

#### Scenario: 根模块零外部依赖

- **WHEN** 在根模块执行 `go list -m all`
- **THEN** `pkg/rpc/server` 及其 HTTP 实现不引入任何新增第三方模块依赖

### Requirement: 函数式 Options 与中间件

系统 SHALL 通过函数式 Options 配置 `Server`，至少提供 `WithAddr`、`WithTransport`、`WithMiddleware`、`WithReadTimeout`，并提供 `DefaultOptions()` 与 `MergeOptions(opts...)`。中间件签名 SHALL 为 `Middleware func(Handler) Handler`，并按 `WithMiddleware` 注册顺序由外向内组成调用链。

#### Scenario: Options 合并默认值

- **WHEN** 调用 `MergeOptions(WithAddr(":8086"), WithReadTimeout(5*time.Second))`
- **THEN** 未显式设置的字段取 `DefaultOptions()` 的值，显式设置的字段被覆盖

#### Scenario: 中间件链顺序

- **WHEN** 先后注册中间件 M1、M2（均包裹下游 handler），处理一个请求
- **THEN** M1 先于 M2 执行其前半逻辑、M2 先于 M1 执行其后半逻辑（洋葱模型），调用顺序与注册顺序一致

