# RPC Server 设计

传输无关的统一服务抽象，对应代码 `pkg/rpc/server/`（HTTP 实现位于 `pkg/rpc/server/http/`）。

> 注：代码用 `loadbalancer`，文档用 `load-balancer`（历史命名约定）；本路径直接镜像
> `pkg/rpc/server/`，无歧义。

## 动机

uniface 的能力接口（KV/Config/LB/Queue/DAG）已成型，但「对外暴露服务」一层没有统一
抽象：每个 lab CLI 各自手写 `net/http` + chi 样板，处理器与传输强耦合，无法跨传输复用。
本设计在 `pkg/rpc/server` 引入面向接口的统一服务抽象，以标准库 `net/http` 提供首个传输
实现。

## 约束

- 根模块零外部依赖（当前仅 `google.golang.org/protobuf`）。
- 遵循既有接口优先布局（`interface.go` / `options.go` / `errors.go`）与函数式 Options 模式。
- 不预先抽象无用例的能力（gRPC 传输留待首个真实场景）。

## 关键决策

### 1. 传输无关的请求/响应信封，而非传输原生处理器

`Handler` 签名为 `func(ctx context.Context, req *Request) (*Response, error)`，
`Request`/`Response` 为最小信封（`Method`、`Path`、`Header`、`Body []byte`、`StatusCode`）。

- HTTP 可 1:1 映射（method/path/header/body/status）；未来 gRPC 可将 `Path` 映射为
  fully-qualified method、`Body` 为 protobuf 字节。同一 handler 无需改写即可跨传输复用。
- 否决「传输原生（HTTP 直接收 `http.HandlerFunc`）」：处理器与传输强耦合，无法复用。
- 否决「泛型 `Server[Req, Res any]`」：过度设计；字节信封更简单且对 HTTP/gRPC 均自然。

### 2. Server 与 Transport 职责分离

- `Server`：持有路由表 + `Options` + 一个 `Transport`，负责生命周期
  （`Start`/`Shutdown`/`Close`）与 `Handle(route, handler)` 注册。
- `Transport`：把 `(Route, Handler)` 集合物化为具体传输监听。HTTP 传输用
  `net/http.ServeMux` + `http.Server`。
- `NewHTTPServer(addr string, opts ...Option) Server` 为便捷构造
  （内部 `New(WithAddr(addr), WithTransport(httpTransport))`）。

新增 gRPC 传输只需实现 `Transport`，不触动 handler 与路由代码。

### 3. HTTP 实现留在根模块（仅用标准库）

`pkg/rpc/server/http` 仅依赖标准库 `net/http`，不引入 chi 等第三方库，故可放在根模块内、
不破坏零外部依赖。lab 子模块仍可在自己的 UI 里用 chi（互不影响）。

否决「把 HTTP 实现做成独立 Go 子模块」：标准库非外部依赖，无需拆模块；拆分反而增加
`go.mod` 维护负担。

### 4. 中间件为 `func(Handler) Handler` 链

统一中间件签名 `Middleware func(Handler) Handler`，HTTP/gRPC 均可表达（日志、recover、
超时）。按 `WithMiddleware` 注册顺序由外向内组成调用链（洋葱模型）。本期不内置实现，
仅定义签名与 `WithMiddleware` option。

### 5. 生命周期与优雅关闭

- `Start(ctx)` 阻塞启动：将路由快照交由 `Transport.Serve` 物化监听。
  `ctx` 取消时触发 `Transport.Shutdown` 排空后返回；`Shutdown` 调用亦使 `Start` 返回。
- `Shutdown(ctx)` 优雅关闭：停止接受新连接，处理完进行中请求后返回。
- `Close()` 立即释放资源并标记关闭；后续 `Handle` 返回 `ErrServerClosed`。

## 风险与权衡

- **信封可能漏字段**：未来 gRPC 可能需要 trailers/streaming。→ 信封当前覆盖 HTTP 全字段；
  streaming 列为 Non-goal，届时按真实用例扩展。
- **过度抽象风险**：`Transport` 抽象目前仅 HTTP 一个实现。→ 抽象面最小化（信封 + 生命周期），
  且需求明确要求多传输兼容，属有据可依的抽象。

## 迁移

纯新增变更，无数据/接口迁移。回滚策略：删除 `pkg/rpc/server/`，还原 Makefile/README
增量即可，不影响现有 lab CLI 与 `pkg/dag`。
