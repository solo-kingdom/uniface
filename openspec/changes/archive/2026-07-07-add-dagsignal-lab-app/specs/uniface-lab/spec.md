## ADDED Requirements

### Requirement: DAG Signal HTTP 验证 CLI

系统 SHALL 提供 `lab-dag-signal` 命令行工具（源码位于 `lab/app/dagsignal/`，CLI 入口 `lab/cmd/lab-dag-signal/main.go`），基于 `pkg/dag/invocation/app.StringApp` 装配实体类型与图加载，经 `sa.Runtime.Memory().Engine()` 访问底层 `StartInstance` / `DeliverSignal` / `DrainInstance` / `GetInstance`，演示「HTTP 请求 → 实例停在 WAITING → 另一端点 signal 推进到终态」的异步编排范式。工具 SHALL NOT 调用 `StringApp.InvokeString`（同步入口）；工具 SHALL NOT 修改 `pkg/rpc/server/dagbridge`（保持其同步 WAITING→500 语义）。

入口图 SHALL 为 `approval`：`entry` 指向 `wait` 节点，`signal: approval`；`approval` signal → success 终态，其它 → failure 终态（兜底 `condition: always`）。图 SHALL NOT 包含任何 COMPUTE 节点（演示焦点为 WAIT + signal 路由）。

HTTP 端点 SHALL 在默认端口 `8087` 暴露：
- `POST /start` —— 请求体作为 payload（可空），生成唯一 entityID，`StartInstance` + `DrainInstance` 推进；SHALL 返回 `202 Accepted` + JSON `{"entity_id":"...","status":"WAITING"}`。启动/drain 失败 SHALL 返回 `500`
- `POST /signal/{entityID}` —— `DeliverSignal`（signal 名默认 `approval`，可经 `?signal=` query 覆盖）+ `DrainInstance`；终态 → `200`，仍 WAITING → `202`，失败终态 → `500`，signal 名不匹配 → `400`，实例不存在 → `404`
- `GET /instances/{entityID}` —— 透传 `GetInstance`；存在 → `200` + 状态 JSON，不存在 → `404`
- `GET /api/status` —— 返回 `api.Status{Domain:"dagsignal", ...}`，与 daghttp `/api/status` 同构

#### Scenario: start 返回 WAITING 与 entityID

- **WHEN** 执行 `curl -X POST http://localhost:8087/start -d 'hello'`
- **THEN** 返回 `202 Accepted`
- **AND** 响应体为 JSON，包含非空 `entity_id` 与 `"status":"WAITING"`

#### Scenario: signal 推进到 COMPLETED

- **WHEN** 对 `POST /start` 返回的 `entity_id` 执行 `curl -X POST http://localhost:8087/signal/{entity_id}`
- **THEN** 返回 `200 OK`
- **AND** 响应体 JSON `"status"` 为 `"COMPLETED"`

#### Scenario: signal 名不匹配返回 400

- **WHEN** 执行 `curl -X POST "http://localhost:8087/signal/{entity_id}?signal=unknown"`
- **THEN** 返回 `400 Bad Request`
- **AND** 响应体包含 signal 不匹配信息

#### Scenario: signal 不存在的实例返回 404

- **WHEN** 执行 `curl -X POST http://localhost:8087/signal/nonexistent`
- **THEN** 返回 `404 Not Found`

#### Scenario: 查询实例状态

- **WHEN** 对已 start 的 `entity_id` 执行 `curl http://localhost:8087/instances/{entity_id}`
- **THEN** 返回 `200 OK`
- **AND** 响应体 JSON 包含 `"status"` 字段（WAITING 或 COMPLETED）

#### Scenario: 查询不存在的实例返回 404

- **WHEN** 执行 `curl http://localhost:8087/instances/nonexistent`
- **THEN** 返回 `404 Not Found`

#### Scenario: status 返回域信息

- **WHEN** 执行 `curl http://localhost:8087/api/status`
- **THEN** 返回 `200 OK` + `application/json`
- **AND** 响应体包含 `"domain":"dagsignal"`

#### Scenario: dagsignal 实现全部落在 `lab/app/dagsignal/`

- **WHEN** 查看 dagsignal 应用的所有源代码（除 `lab/cmd/lab-dag-signal/main.go` 外）
- **THEN** 所有 `.go` 文件与 fixture 均位于 `lab/app/dagsignal/` 之下
- **AND** SHALL NOT 引用 `lab/internal/wiring` 中专为 dagsignal 新增的专属函数
- **AND** SHALL NOT 调用 `StringApp.InvokeString` 或 `dagbridge.ResponseForTerminalResult`

#### Scenario: main 仅持有 flag 与生命周期入口

- **WHEN** 阅读 `lab/cmd/lab-dag-signal/main.go`
- **THEN** 文件 SHALL 仅包含 flag 解析、信号 `ctx`、`dagsignal.LoadConfig()` 调用、`dagsignal.Serve(ctx, addr, cfg)` 调用
- **AND** SHALL NOT 直接持有 `dag.Engine`、`*app.StringApp`、`dagsignal.NewService` 中的任意一个

#### Scenario: 配置自治与 env 覆写

- **WHEN** 设置 `LAB_CONFIG` 指向含 `dagsignal: {store: memory, fixtures_dir: /tmp/x}` 的 yaml，并设置 `LAB_DAGSIGNAL_FIXTURES_DIR=/tmp/y`
- **THEN** `LoadConfig()` 返回的 `FixturesDir` SHALL 为 `/tmp/y`（env 覆写 yaml）
- **AND** 未设置 env 且 yaml 无 `fixtures_dir` 时 SHALL 回退到 `DefaultFixturesDir`（`app/dagsignal/fixtures/graphs`）

### Requirement: DAG Signal 按域生命周期

系统 SHALL 将 `dagsignal` 纳入 lab 域注册表（二进制 `lab-dag-signal`，默认端口 `8087`，无 compose 中间件依赖），并提供按域目标 `lab-build-dag-signal`、`lab-up-dag-signal`、`lab-down-dag-signal`，行为与既有 `daghttp` 域目标一致。dagsignal 域的可执行构建路径 SHALL 为 `lab/cmd/lab-dag-signal`，其源码归属 SHALL 为 `lab/app/dagsignal/`。

#### Scenario: 按域启动 DAG Signal

- **WHEN** 执行 `make lab-up-dag-signal`
- **THEN** 仅构建并启动 `lab-dag-signal serve` 进程，不启动其他域进程，也不启动任何 compose 中间件

#### Scenario: 按域关停

- **WHEN** 已执行 `make lab-up-dag-signal`，随后执行 `make lab-down-dag-signal`
- **THEN** 仅停止 `lab-dag-signal` 进程，不影响其他运行中的 lab 域进程

#### Scenario: 多域组合包含 DAG Signal

- **WHEN** 执行 `make lab-up LAB_MODULES=daghttp,dagsignal`
- **THEN** 同时启动 `lab-dag-http` 与 `lab-dag-signal` 两个进程，且不启动额外 compose 服务

#### Scenario: 全量启动包含 DAG Signal

- **WHEN** 执行 `make lab-up`（即 `LAB_MODULES=all`）
- **THEN** 启动的进程集合 SHALL 包含 `lab-dag-signal`
- **AND** `lab/Makefile` 的 `MODULES` 列表 SHALL 包含 `dagsignal`
