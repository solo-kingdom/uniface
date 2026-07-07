## Why

`lab/app/daghttp/` 已验证「HTTP 请求 → DAG 同步排空 → 终态 payload 作响应」的编排范式，但它只覆盖最简单的黄金路径：入口即 COMPUTE 链、`InvokeString` 一次性 Start+Drain+Snapshot、`COMPLETED → 200`。DAG 引擎的另一类核心能力——`WAIT` 节点 + 外部 `signal` 推进的异步生命周期——目前只在 `lab-dag` CLI 与单测里出现，没有一个自包含的 HTTP 应用演示「请求触发后停在 WAITING、另一端点 signal 推进到终态」的请求编排范式。缺少第二个具体应用，也让「哪些是可抽取的公共应用脚手架、哪些是场景特化逻辑」无法被对比归纳。

## What Changes

- 新建 `lab/app/dagsignal/` 自包含应用：参考 `daghttp` 结构，演示 `WAIT` + `signal` 异步编排
- 新增 fixture `lab/app/dagsignal/fixtures/graphs/approval.yaml`：入口即 `wait` 节点（`signal: approval`），`approval` signal → success 终态，其它 → failure 终态（参考既有 `lab/internal/fixtures/graphs/approval_branch.yaml`）
- 新增 `lab/cmd/lab-dag-signal/main.go`：仅 flag + 信号 ctx + `dagsignal.LoadConfig()` + `dagsignal.Serve(ctx, addr, cfg)`
- 新增 HTTP 端点（端口 `8087`）：
  - `POST /start`：`StartInstance` + `DrainInstance` 推进到 WAITING → `202 Accepted` + `{entity_id, status:"WAITING"}`
  - `POST /signal/{entityID}`：构造 `SignalDelivery` + `DeliverSignal` + `DrainInstance` → `200`（含终态）或 `202`（仍 WAITING）
  - `GET /instances/{entityID}`：透传 `GetInstance` 状态
  - `GET /api/status`：域状态（复用 `lab/internal/web/api`）
- `lab/Makefile`：将 `dagsignal` 加入 `MODULES`，注册 `lab-dag-signal` / 端口 `8087` / 无 compose profile，自动获得 `lab-build-dag-signal` / `lab-up-dag-signal` / `lab-down-dag-signal` 目标
- `lab/configs/default.yaml`：新增 `dagsignal` 配置段
- `lab/README.md`：新增「DAG Signal HTTP 服务」章节，并在 CLI 表与域注册表补充 `lab-dag-signal` / 8087
- `design.md` 中专章对比 `daghttp` 与 `dagsignal`，归纳可抽取的公共逻辑（应用骨架、StringApp 装配、entityID 生成、OpRecorder 集成、终态→HTTP 映射策略），为后续 follow-up 抽取共享脚手架提供基线

## Capabilities

### New Capabilities

（无新增 capability）

### Modified Capabilities

- `uniface-lab`: 新增「DAG Signal HTTP 验证 CLI」与「DAG Signal 按域生命周期」两项 Requirement —— 引入 `lab-dag-signal`（端口 8087）演示 `WAIT` + `signal` 异步编排，按域目标命名与既有 `daghttp` 域一致

## Impact

- **新增**：`lab/app/dagsignal/`（`config.go` / `serve.go` / `handler.go` / `handler_test.go` / `serve_test.go` / `fixtures/graphs/approval.yaml`）、`lab/cmd/lab-dag-signal/main.go`
- **修改**：`lab/Makefile`（`MODULES` 追加 `dagsignal`）、`lab/configs/default.yaml`、`lab/README.md`、`openspec/specs/uniface-lab/spec.md`
- **依赖**：复用 `pkg/dag/invocation/app.StringApp` 装配实体类型/unit/图加载，经 `sa.Runtime.Memory().Engine()` 访问 `StartInstance` / `DeliverSignal` / `DrainInstance` / `GetInstance`；复用 `pkg/rpc/server/http` 与 `lab/internal/web/api`；无新增外部依赖
- **破坏性**：无；仅追加新 lab 域，不改既有域与 `pkg/` 公共 API

## Non-goals

- 不在 `pkg/` 下新增异步 DAG 门面（`StartString` / `SignalString` 之类）；异步路径继续走底层 `Engine` API
- 不抽取 `daghttp` 与 `dagsignal` 的共享脚手架（`design.md` 仅做归纳分析，落地留给后续 follow-up 变更）
- 不修改 `pkg/rpc/server/dagbridge`（其同步 `ResponseForTerminalResult` 把 WAITING 映射为 500 的语义保持不变；dagsignal 自带异步映射纯函数）
- 不调整 `daghttp`、`lab-dag`、`lab/internal/dag/`、其它 lab CLI 与 `LabConfig` 跨域结构
- 不引入 gRPC / WebSocket / 持久化后端 / 认证限流；不演示 signal payload 合并（仅 signal name 推进）
