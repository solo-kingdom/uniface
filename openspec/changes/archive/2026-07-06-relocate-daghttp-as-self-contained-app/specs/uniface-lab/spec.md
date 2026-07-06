## MODIFIED Requirements

### Requirement: DAG HTTP 服务验证 CLI

系统 SHALL 提供独立 lab 模块 `lab-dag-http`（`lab/cmd/lab-dag-http`），对外仅暴露 `POST /echo` 端点，并通过统一 `pkg/rpc/server` 抽象启动（SHALL NOT 直接手写 `net/http` 样板）。每次 `/echo` 请求 SHALL 包装为一个独立 `EntityInstance`，经 `lab-dag-http` 自有 echo 图排空到终态后，将终态 payload 作为响应体返回；`COMPLETED` 映射 HTTP 200，`FAILED`/`COMPENSATED` 映射 HTTP 500 并附失败原因。

`daghttp` SHALL 作为自包含应用整体落在 `lab/app/daghttp/` 目录下：handler/service、`StringApp` 装配函数、`lab.hello` / `lab.echo` 计算单元、echo fixture、`Config` schema（`Store` / `FixturesDir`）、生命周期入口（`Serve` / `LoadConfig`）均位于该路径下，不得散布到 `lab/internal/wiring/` 或其他跨域共享目录。`lab-dag-http` SHALL 通过 `lab/app/daghttp.Serve(ctx, addr, cfg)` 启动服务，main 包不再持有 StringApp 与装配细节。

`lab-dag-http` SHALL 与 `lab/internal/dag` 验证台完全隔离：不得复用 `lab/internal/dag.Runtime`、`lab/internal/dag` fixtures 或其 HTTP API。`lab-dag-http` SHALL 优先复用根模块公共 `pkg/dag/invocation` 请求式轻量封装装配其验证所需的 graph、entity type 与 compute units；当需要验证底层能力时 MAY 继续直接使用公共 Runtime/Invoker/Loader/Codec 抽象。

#### Scenario: echo 请求经 DAG 返回

- **WHEN** 执行 `curl -X POST http://localhost:8086/echo -d 'hello'`
- **THEN** 响应状态码为 200
- **AND** 响应体为 `lab-dag-http` 自有 echo 图终态 payload

#### Scenario: 终态失败映射 5xx

- **WHEN** 一个请求经 DAG 排空后实例终态为 `FAILED` 或 `COMPENSATED`
- **THEN** 响应状态码为 500，响应体包含失败原因

#### Scenario: 通过统一 Server 抽象启动

- **WHEN** 执行 `lab-dag-http serve`
- **THEN** 服务经 `pkg/rpc/server` 的 `Server` 抽象启动并监听 8086，而非进程内直接调用 `http.ListenAndServe`

#### Scenario: 与 lab-dag 运行时隔离

- **WHEN** 同时启动 `lab-dag serve` 与 `lab-dag-http serve`
- **THEN** 两个进程使用各自独立的 DAG runtime、fixtures 与 HTTP API
- **AND** `lab-dag-http` 不依赖 `lab/internal/dag.Runtime`

#### Scenario: 使用轻量请求式封装装配

- **WHEN** `lab-dag-http` 注册 echo 图、`lab.hello` 与 `lab.echo` 计算单元
- **THEN** 装配代码 SHALL 通过公共请求式 DAG 轻量封装完成常见注册、图加载和 string payload 调用
- **AND** 业务 handler 不需要直接构造 `invocation.InvokeRequest` 或手写 `anypb.Any` payload 编解码

#### Scenario: daghttp 实现全部落在 `lab/app/daghttp/`

- **WHEN** 查看 daghttp 应用的所有源代码（除 `lab/cmd/lab-dag-http/main.go` 外）
- **THEN** 所有 `.go` 文件与 fixture 均位于 `lab/app/daghttp/` 之下
- **AND** `lab/internal/wiring/` 中不再保留 daghttp 专属函数（`NewDAGHTTP` / `registerLabUnits` / `helloFunc` / `echoFunc`）
- **AND** `LabConfig.DAG` 字段类型为 `daghttp.Config`，DAGConfig 不再定义于 wiring 包

#### Scenario: main 仅持有 flag 与生命周期入口

- **WHEN** 阅读 `lab/cmd/lab-dag-http/main.go`
- **THEN** 文件 SHALL 仅包含 flag 解析、信号 `ctx`、`daghttp.LoadConfig()` 调用、`daghttp.Serve(ctx, addr, cfg)` 调用
- **AND** SHALL NOT 直接持有 `*app.StringApp`、`registerLabUnits`、`helloFunc`、`echoFunc`、`daghttp.NewService` 中的任意一个

### Requirement: DAG HTTP 按域生命周期

系统 SHALL 将 `daghttp` 纳入 lab 域注册表（二进制 `lab-dag-http`，默认端口 `8086`，无 compose 中间件依赖），并提供按域目标 `lab-build-dag-http`、`lab-up-dag-http`、`lab-down-dag-http`，行为与既有域目标一致。daghttp 域的可执行构建路径 SHALL 为 `lab/cmd/lab-dag-http`，其源码归属 SHALL 为 `lab/app/daghttp/`。

#### Scenario: 按域启动 DAG HTTP

- **WHEN** 执行 `make lab-up-dag-http`
- **THEN** 仅构建并启动 `lab-dag-http serve` 进程，不启动其他域进程，也不启动任何 compose 中间件

#### Scenario: 按域关停

- **WHEN** 已执行 `make lab-up-dag-http`，随后执行 `make lab-down-dag-http`
- **THEN** 仅停止 `lab-dag-http` 进程，不影响其他运行中的 lab 域进程

#### Scenario: 多域组合包含 DAG HTTP

- **WHEN** 执行 `make lab-up LAB_MODULES=dag,daghttp`
- **THEN** 同时启动 `lab-dag` 与 `lab-dag-http` 两个进程，且不启动额外 compose 服务
