## 1. 统一 Server 抽象（pkg/rpc/server）

- [x] 1.1 新建 `pkg/rpc/server/interface.go`：定义 `Request`（Method/Path/Header/Body）、`Response`（StatusCode/Header/Body）、`Handler`（`func(ctx, *Request) (*Response, error)`）、`Route`（Method/Path）、`Middleware`（`func(Handler) Handler`）、`Transport`（物化路由表 + 生命周期）、`Server`（`Handle`/`Start`/`Shutdown`/`Close`）公开接口
- [x] 1.2 新建 `pkg/rpc/server/options.go`：函数式 Options（`Options`、`Option`、`DefaultOptions`、`MergeOptions`、`WithAddr`、`WithTransport`、`WithMiddleware`、`WithReadTimeout`），含 `BaseServer` 基础实现（路由表 + Options + 委托 Transport 的 `Start`/`Shutdown`/`Close`）
- [x] 1.3 新建 `pkg/rpc/server/errors.go`：sentinel errors（`ErrRouteExists`、`ErrServerClosed` 等）与含 `Op` 字段的自定义错误类型，支持 `errors.Is/As`
- [x] 1.4 编写 `pkg/rpc/server` 单测：Options 合并默认值、中间件洋葱模型顺序、`Handle` 重复注册报错（`go test ./pkg/rpc/server/...`，仅标准库依赖）

## 2. HTTP 传输实现（pkg/rpc/server/http）

- [x] 2.1 新建 `pkg/rpc/server/http/transport.go`：基于标准库 `net/http` 的 `Transport` 实现，将 `(*Request,*Response)` 信封与 `http.ResponseWriter`/`*http.Request` 双向适配（method/path/header/body/status），用 `http.ServeMux` 挂载路由
- [x] 2.2 提供 `NewHTTPServer(addr string, opts ...Option) Server` 便捷构造，等价 `New(WithAddr(addr), WithTransport(httpTransport))`
- [x] 2.3 编写 `pkg/rpc/server/http` 单测：用 `httptest` 验证请求→信封→handler→响应映射、优雅关闭（`Shutdown` 处理完进行中请求）
- [x] 2.4 验证根模块零外部依赖：`go list -m all` 确认未引入第三方模块（HTTP 实现仅用标准库）

## 3. lab-dag-http 请求适配（lab/internal/daghttp）

- [x] 3.1 新建 `lab/internal/daghttp/handler.go`：实现 echo `Handler`——读 `Request.Body` → `StringValue` payload → 生成唯一 entityID → `dag.Runtime.Start("echo", entityID, payload)`
- [x] 3.2 实现排空循环：`RunOnce` 直到 `GetInstance().Status` 为终态（`COMPLETED`/`FAILED`/`COMPENSATED`）或达上限（防死循环）；读终态 payload（StringValue）→ `Response.Body`，`COMPLETED`→200，否则→500 附失败原因
- [x] 3.3 新建 `lab/internal/wiring/daghttp.go`（或扩展 `dag.go`）：工厂函数构造 `dag.Runtime`、加载 `echo` fixture、装配 `daghttp` handler
- [x] 3.4 编写 `lab/internal/daghttp` 单测：`echo:hello` 黄金路径 + 失败终态 5xx（复用 `lab/internal/dag` 现有 `runtime_test.go` 模式）

## 4. lab-dag-http CLI 与服务装配

- [x] 4.1 新建 `lab/cmd/lab-dag-http/main.go`：`serve` 子命令经 `pkg/rpc/server` 的 `NewHTTPServer` 启动，注册 `POST /echo` → `daghttp` handler，默认 `:8086`，支持 `-addr` 覆盖；`SIGINT/SIGTERM` 触发 `Shutdown`
- [x] 4.2 在 `lab/internal/web/api` 复用 `Status`/`OpRecorder` 为 `daghttp` 暴露 `GET /api/status`（如需要，最小化复用）
- [x] 4.3 手动验证：`cd lab && go build ./cmd/lab-dag-http`，启动后 `curl -X POST :8086/echo -d hello` 返回 `echo:hello`

## 5. 构建脚本与按域目标

- [x] 5.1 在 `lab/Makefile` 域注册表新增 `daghttp`（BIN=`lab-dag-http`，PROFILES 空，PORT=8086），依赖现有模板自动生成 `build-daghttp`/`up-daghttp`/`down-daghttp`；把 `daghttp` 加入 `MODULES`
- [x] 5.2 在根 `Makefile` 增加 `lab-build-dag-http`/`lab-up-dag-http`/`lab-down-dag-http` 转发目标，与既有 `lab-up-dag` 等格式一致
- [x] 5.3 验证按域启停：`make lab-up-dag-http` 仅启动 `lab-dag-http`，`make lab-down-dag-http` 仅停止它，`make lab-up LAB_MODULES=dag,daghttp` 同时启动两者且不启 compose

## 6. 文档

- [x] 6.1 新建 `pkg/rpc/server/README.md`：架构、核心概念、`NewHTTPServer` 快速示例、多传输扩展指引（gRPC 为后续）
- [x] 6.2 在 `docs/` 镜像 `pkg/rpc/server/` 路径补设计文档（注意 loadbalancer/load-balancer 命名约定）
- [x] 6.3 更新 `lab/README.md`：新增 `lab-dag-http` 行（端口 8086，`/echo`）、按域用法示例
- [x] 6.4 更新 `CLAUDE.md`：核心接口增加 `rpc.Server` 条目，lab 命令表增加 `lab-dag-http`
