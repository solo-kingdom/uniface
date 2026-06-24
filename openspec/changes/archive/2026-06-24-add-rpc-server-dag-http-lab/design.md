## Context

uniface 的能力接口（KV/Config/LB/Queue/DAG）已成型，但「对外暴露服务」一层没有统一抽象：每个 lab CLI 各自手写 `net/http` + chi 样板（见 `lab/internal/web/server.go`），处理器与传输强耦合，无法跨传输复用。同时 DAG 引擎（`pkg/dag`，内存 MVP）只被 `lab-dag` 命令行作为「引擎验证台」驱动，缺少「一次请求 = 一次 DAG 编排」的端到端范式。

本变更在 `pkg/rpc/server` 引入面向接口的统一服务抽象，以标准库 `net/http` 提供首个传输实现，并用独立 lab 模块 `lab-dag-http` 验证「HTTP 请求经 DAG 排空到终态后返回」。

约束：
- 根模块零外部依赖（当前仅 `google.golang.org/protobuf`）。
- 遵循既有接口优先布局（`interface.go` / `options.go` / `errors.go`）与函数式 Options 模式。
- 不预先抽象无用例的能力（项目惯例：`enhance-dag-declarative-rpc` 明确不为零用例抽象 gRPC unit）。

## Goals / Non-Goals

**Goals:**
- 定义传输无关的统一 `Server` 抽象（生命周期 + 路由注册 + 优雅关闭），使同一 `Handler` 可在不同传输间复用。
- 提供基于标准库 `net/http` 的首个传输实现，不破坏根模块零依赖。
- 以 `lab-dag-http` 端到端演示「请求=实例、排空到终态、终态 payload 作为响应」。
- 复用现有 `pkg/dag` 引擎与 `lab/internal/dag.Runtime`、`echo` fixture、`lab.echo` unit，零重复实现。

**Non-Goals:**
- gRPC 传输实现（仅预留扩展点）。
- 修改 `lab-dag` 引擎验证台或 `pkg/dag` 内核。
- 异步/信号驱动的请求处理；并发限流、认证、可观测性中间件。
- 持久化 LineStore 与分布式 worker。

## Decisions

### 决策 1：传输无关的请求/响应信封，而非传输原生处理器

`Handler` 签名为 `func(ctx context.Context, req *Request) (*Response, error)`，`Request`/`Response` 为最小信封（`Method`、`Path`、`Header`、`Body []byte`、`StatusCode`）。

- **理由**：HTTP 可 1:1 映射（method/path/header/body/status）；未来 gRPC 可将 `Path` 映射为 fully-qualified method、`Body` 为 protobuf 字节。同一 handler 无需改写即可跨传输复用——这正是「统一封装」的目标。
- **备选 A**：传输原生（HTTP 实现直接收 `http.HandlerFunc`）。否决：处理器与传输强耦合，无法复用，违背变更目的。
- **备选 B**：泛型 `Server[Req, Res any]`。否决：过度设计；字节信封更简单且对 HTTP/gRPC 均自然。

### 决策 2：Server 与 Transport 职责分离

- `Server`（`pkg/rpc/server`）：持有路由表 + `Options` + 一个 `Transport`，负责生命周期（`Start`/`Shutdown`/`Close`）与 `Handle(route, handler)` 注册。
- `Transport`：把 `(Route, Handler)` 集合物化为具体传输监听。HTTP 传输用 `net/http.ServeMux` + `http.Server`。
- `NewHTTPServer(addr string, opts ...Option)` 为便捷构造（内部 `New(WithAddr(addr), WithTransport(http.NewTransport()))`）。

理由：路由注册语义统一在 `Server`，传输只管「上线材」。新增 gRPC 传输只需实现 `Transport`，不触动 handler 与路由代码。

### 决策 3：HTTP 实现留在根模块（仅用标准库）

`pkg/rpc/server/http` 仅依赖标准库 `net/http`，不引入 chi 等第三方库，故可放在根模块内、不破坏零外部依赖。lab 子模块仍可在自己的 UI 里用 chi（互不影响）。

- **备选**：把 HTTP 实现做成独立 Go 子模块（如 redis/aerospike）。否决：标准库非外部依赖，无需拆模块；拆分反而增加 `go.mod` 维护负担。

### 决策 4：中间件为 `func(Handler) Handler` 链

统一中间件签名 `Middleware func(Handler) Handler`，HTTP/gRPC 均可表达（日志、recover、超时）。本期不内置实现，仅定义签名与 `WithMiddleware` option，预留可观测性接入点。

### 决策 5：lab-dag-http 请求→实例适配

`lab/internal/daghttp` 提供适配器，复用 `lab/internal/dag.Runtime`：

1. 读取请求 `Body` → `google.protobuf.StringValue`。
2. 生成唯一 `entityID`（`http-<timestamp>-<rand>`）。
3. `Runtime.Start(graph="echo", entityID, payload)`。
4. 排空循环：`Runtime.RunOnce()` 直到 `GetInstance().Status` 为终态（`COMPLETED`/`FAILED`/`COMPENSATED`）或达上限（防死循环）。
5. 读终态 payload（StringValue）→ 响应 `Body`；`COMPLETED`→200，否则→500 并附失败原因。

复用现有 `echo.yaml`（`lab.echo` compute → terminal）与 `lab.echo` unit（输出 `echo:<input>`），无需新增 fixture。

### 决策 6：端口与按域目标

`lab-dag-http` 默认端口 `8086`，无 compose 中间件依赖。`lab/Makefile` 域注册表新增 `daghttp`（BIN=`lab-dag-http`，PROFILES 为空，PORT=8086），复用 lab-modular-targets 的模板生成 `lab-build-dag-http` 等目标。

## Risks / Trade-offs

- **信封可能漏字段**：未来 gRPC 可能需要 trailers/streaming。→ 缓解：信封当前覆盖 HTTP 全字段；streaming 列为 Non-goal，届时按真实用例扩展，不为零用例预抽象。
- **同步排空在内存引擎下的并发**：`memory.Engine.RunOnce` 为单进程实现，每请求独立实例、排空调用。→ 缓解：lab 范围单实例串行排空；在 README 注明 MVP 不含并发调度，生产化需持久化 store + worker（Non-goal）。
- **过度抽象风险**：`Transport` 抽象目前仅 HTTP 一个实现。→ 缓解：抽象面最小化（信封 + 生命周期），且用户需求明确要求多传输兼容，属有据可依的抽象而非臆测。
- **端口冲突**：8086 需未被占用。→ 缓解：CLI 支持 `-addr` 覆盖。

## Migration Plan

纯新增变更，无数据/接口迁移。回滚策略：删除 `pkg/rpc/server/`、`lab/cmd/lab-dag-http/`、`lab/internal/daghttp/`，还原 Makefile/README 增量即可，不影响现有 lab CLI 与 `pkg/dag`。

## Open Questions

无阻塞性问题。gRPC 传输的具体信封映射（trailers、streaming）留待首个真实 gRPC 场景的后续 change 评估。
