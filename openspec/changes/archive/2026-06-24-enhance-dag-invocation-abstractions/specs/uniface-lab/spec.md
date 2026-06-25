## MODIFIED Requirements

### Requirement: DAG HTTP 服务验证 CLI

系统 SHALL 提供独立 lab 模块 `lab-dag-http`（`lab/cmd/lab-dag-http`），对外仅暴露 `POST /echo` 端点，并通过统一 `pkg/rpc/server` 抽象启动（SHALL NOT 直接手写 `net/http` 样板）。每次 `/echo` 请求 SHALL 包装为一个独立 `EntityInstance`，经 `lab-dag-http` 自有 echo 图排空到终态后，将终态 payload 作为响应体返回；`COMPLETED` 映射 HTTP 200，`FAILED`/`COMPENSATED` 映射 HTTP 500 并附失败原因。

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
