# pkg/rpc/server

传输无关的统一 RPC 服务抽象。把「对外暴露服务」从各 CLI 的手写 `net/http` 样板中
抽离出来：通过 `Handle` 注册传输无关的 `Handler`，由具体 `Transport`（如 `net/http`）
把路由集合物化为监听。同一组 `Handler` 可在不同传输（HTTP、gRPC 等）间复用与热切换。

## 为什么需要它

uniface 已沉淀 KV/Config/LB/Queue/DAG 五类能力接口，但「对外暴露服务」一层缺少统一
抽象——处理器与传输强耦合，无法跨传输复用。本包以最小信封 + 生命周期抽象解决该问题。

## 核心概念

| 概念 | 说明 |
|------|------|
| `Request` / `Response` | 传输无关信封（Method/Path/Header/Body 与 StatusCode/Header/Body）。HTTP 可 1:1 映射；未来 gRPC 可将 `Path`→fully-qualified method、`Body`→protobuf 字节。 |
| `Handler` | `func(ctx, *Request) (*Response, error)`——业务处理器的统一签名。 |
| `Middleware` | `func(Handler) Handler`——按 `WithMiddleware` 注册顺序由外向内组成调用链（洋葱模型）。 |
| `Transport` | 把 `(Route, Handler)` 集合物化为具体传输监听（`Serve`/`Shutdown`/`Close`）。 |
| `Server` | 持有路由表 + `Options` + `Transport`，提供 `Handle`/`Start`/`Shutdown`/`Close` 生命周期。 |

文件布局遵循接口优先惯例：

```
pkg/rpc/server/
  interface.go   # Server / Transport / Handler / Request / Response / Route / Middleware
  options.go     # 函数式 Options + BaseServer 基础实现
  errors.go      # sentinel errors + ServerError
  http/          # 首个 Transport 实现（仅标准库 net/http，保持根模块零依赖）
```

## 快速开始

最小示例——用便捷构造 `NewHTTPServer` 暴露一个 echo 端点：

```go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"

    "github.com/solo-kingdom/uniface/pkg/rpc/server"
    rpchttp "github.com/solo-kingdom/uniface/pkg/rpc/server/http"
)

func main() {
    srv := rpchttp.NewHTTPServer(":8086")
    _ = srv.Handle(server.Route{Method: "POST", Path: "/echo"},
        func(ctx context.Context, req *server.Request) (*server.Response, error) {
            return &server.Response{
                StatusCode: 200,
                Body:       append([]byte("echo:"), req.Body...),
            }, nil
        })

    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()
    _ = srv.Start(ctx) // 阻塞，ctx 取消或 Shutdown 后返回
}
```

等价展开（便于切换传输）：

```go
srv := server.New(
    server.WithAddr(":8086"),
    server.WithTransport(rpchttp.NewTransport()),
)
```

## 中间件

中间件按 `WithMiddleware` 注册顺序由外向内包裹 handler（洋葱模型）：

```go
srv := server.New(
    server.WithTransport(tr),
    server.WithMiddleware(loggingMiddleware),   // 最外层
    server.WithMiddleware(recoverMiddleware),   // 内层
)
// 请求流: logging(pre) → recover(pre) → handler → recover(post) → logging(post)
```

## 多传输扩展（gRPC 为后续）

新增传输只需实现 `Transport` 接口，不触动 handler 与路由代码：

```go
type Transport interface {
    Serve(addr string, routes []RouteHandler, opts *Options) error
    Shutdown(ctx context.Context) error
    Close() error
}
```

未来 gRPC 传输可将 `Route.Path` 映射为 fully-qualified method、`Body` 映射为 protobuf
字节。本期不预抽象无 gRPC 用例的能力（streaming、trailers 等留待首个真实场景）。

## 零外部依赖

根模块保持零外部依赖：`interface.go` 仅依赖标准库 `context`；HTTP 实现仅用标准库
`net/http`，不引入 chi 等第三方库（lab 子模块仍可在自己的 UI 里用 chi，互不影响）。

## 端到端示例

`lab-dag-http`（`lab/cmd/lab-dag-http`）演示「HTTP 请求经 DAG 排空到终态后返回」：
每次 `POST /echo` 包装为一个 `EntityInstance`，经 echo 图（`lab.echo` → terminal）排空到
终态，终态 payload 作为响应体返回（`echo:<body>`）。
