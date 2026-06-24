## ADDED Requirements

### Requirement: DAG HTTP 服务验证 CLI

系统 SHALL 提供独立 lab 模块 `lab-dag-http`（`lab/cmd/lab-dag-http`），对外仅暴露 `POST /echo` 端点，并通过统一 `pkg/rpc/server` 抽象启动（SHALL NOT 直接手写 `net/http` 样板）。每次 `/echo` 请求 SHALL 包装为一个 `EntityInstance`，经 `echo` 图（`lab.echo` compute → terminal）排空到终态后，将终态 payload 作为响应体返回；`COMPLETED` 映射 HTTP 200，`FAILED`/`COMPENSATED` 映射 HTTP 500 并附失败原因。该模块 SHALL 复用 `lab/internal/dag.Runtime`、`echo` fixture 与 `lab.echo` unit，SHALL NOT 修改 `lab-dag` 引擎验证台或其 HTTP API。

#### Scenario: echo 请求经 DAG 返回

- **WHEN** 执行 `curl -X POST http://localhost:8086/echo -d 'hello'`
- **THEN** 响应状态码为 200，响应体为 `echo:hello`（`lab.echo` unit 对输入加 `echo:` 前缀）

#### Scenario: 终态失败映射 5xx

- **WHEN** 一个请求经 DAG 排空后实例终态为 `FAILED` 或 `COMPENSATED`
- **THEN** 响应状态码为 500，响应体包含失败原因

#### Scenario: 通过统一 Server 抽象启动

- **WHEN** 执行 `lab-dag-http serve`
- **THEN** 服务经 `pkg/rpc/server` 的 `Server` 抽象启动并监听 8086，而非进程内直接调用 `http.ListenAndServe`

### Requirement: DAG HTTP 按域生命周期

系统 SHALL 将 `daghttp` 纳入 lab 域注册表（二进制 `lab-dag-http`，默认端口 `8086`，无 compose 中间件依赖），并提供按域目标 `lab-build-dag-http`、`lab-up-dag-http`、`lab-down-dag-http`，行为与既有域目标一致。

#### Scenario: 按域启动 DAG HTTP

- **WHEN** 执行 `make lab-up-dag-http`
- **THEN** 仅构建并启动 `lab-dag-http serve` 进程，不启动其他域进程，也不启动任何 compose 中间件

#### Scenario: 按域关停

- **WHEN** 已执行 `make lab-up-dag-http`，随后执行 `make lab-down-dag-http`
- **THEN** 仅停止 `lab-dag-http` 进程，不影响其他运行中的 lab 域进程

#### Scenario: 多域组合包含 DAG HTTP

- **WHEN** 执行 `make lab-up LAB_MODULES=dag,daghttp`
- **THEN** 同时启动 `lab-dag` 与 `lab-dag-http` 两个进程，且不启动额外 compose 服务
