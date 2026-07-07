## Context

`lab/app/daghttp/` 已是自包含应用：`config.go` / `serve.go` / `handler.go` / `units.go` + `fixtures/graphs/echo.yaml`，基于 `pkg/dag/invocation/app.StringApp` + `pkg/rpc/server/http` + `pkg/rpc/server/dagbridge`，端口 8086。它演示「请求 = 实例、同步排空到终态、终态 payload 作响应体」范式：handler 调 `StringApp.InvokeString`（内部 Start + Drain + Snapshot 一体），由 `dagbridge.ResponseForTerminalResult` 把终态映射为 HTTP 响应。

DAG 引擎同时支持另一类生命周期——`WAIT` 节点让实例停在 `INSTANCE_STATUS_WAITING`，由外部 `Engine.DeliverSignal` 投递 `SignalDelivery` 推进。这套能力目前只在 `lab-dag` CLI（`lab/cmd/lab-dag` 的 `signal` 子命令 + `lab/internal/dag/runtime.go` 的 `DeliverSignal`）与单测里出现，缺少一个自包含 HTTP 应用演示完整的异步编排闭环。

关键约束（来自代码调研）：

- **StringApp 仅暴露同步 `InvokeString`**。`pkg/dag/invocation/app/doc.go` 明确：「复杂生命周期、异步 Signal/Saga 等场景应继续使用底层 API」。`StringApp` 嵌入 `*Runtime`，故可通过 `sa.Runtime.Memory().Engine()` 取得 `*dagmemory.Engine` 调用 `StartInstance` / `DeliverSignal` / `DrainInstance` / `GetInstance`。
- **`InvokeString` 在 WAITING 时直接返回**（不阻塞、不报错）：`pkg/dag/memory/drain.go` 的 `drainDone` 把 WAITING 与四个终态并列视作「排空完成」；`IsWaiting()` 显式暴露给调用方。
- **现成 fixture 可参考**：`lab/internal/fixtures/graphs/approval_branch.yaml`（入口即 `wait` 节点，`signal: approval`，approval → success，always → failure）。
- **`dagbridge.ResponseForTerminalResult` 是同步语义**：WAITING → 500（"同步调用上下文不应进入 WAITING"）。异步应用需自写映射纯函数。
- **signal payload 默认合并到实例 snapshot**（`pkg/dag/entity/signal.go`），但本变更 Non-goals 不演示 payload 合并，仅用 signal name 推进。
- **`lab-dag` CLI 的 `signal` 子命令**：构造 `SignalDelivery{EntityId, SignalName, DeliveryId="cli-delivery"}`，调 `engine.DeliverSignal`；推进后状态变 RUNNING，需再触发 `DrainInstance` 才能继续往后跑。

## Goals / Non-Goals

**Goals:**

- 新建 `lab/app/dagsignal/` 自包含应用，演示「HTTP 请求 → 实例停在 WAITING → 另一端点 signal 推进到终态」的异步编排范式
- 端到端闭环可手动验证：`POST /start` 拿 entityID → `POST /signal/{entityID}` 推进 → `GET /instances/{entityID}` 查状态
- 应用结构严格对齐 `daghttp`（`config.go` / `serve.go` / `handler.go` + fixture + `cmd/lab-dag-signal/main.go`），使两者可被直接对比
- 在本 `design.md` 中专章对比两个应用，归纳可抽取的公共逻辑与场景特化逻辑，为后续 follow-up 抽取共享脚手架提供基线
- 复用既有 `pkg/` 公共能力（StringApp 装配实体类型 / unit / 图加载、`rpc/server/http`、`lab/internal/web/api`），不引入新外部依赖

**Non-Goals:**

- 不在 `pkg/` 新增异步 DAG 门面（`StartString` / `SignalString`）；继续走 `Memory().Engine()` 底层 API
- 不抽取共享脚手架代码（仅归纳分析，落地由 follow-up 变更承担）
- 不修改 `dagbridge`、`daghttp`、`lab-dag`、`lab/internal/dag/`、`LabConfig` 跨域结构
- 不演示 signal payload 合并、deadline 超时分支、JOIN/saga 等其它 DAG 能力
- 不引入 gRPC / WebSocket / 持久化 / 认证限流

## Decisions

### D1: 应用命名 `dagsignal`、binary `lab-dag-signal`、端口 8087

**决策**: 命名与 `daghttp`（domain + 传输/能力）对齐：`dagsignal` 直指其演示的核心 DAG 能力（signal 推进）；binary `lab-dag-signal` 与 `lab-dag-http` 平行；端口 8087 紧接 8086。

**理由**:
- 「`dag<能力>`」前缀让两个应用在 `lab/app/` 下天然成对，外部读者一眼识别「这是 DAG 能力验证应用之一」
- 端口连续排布（8085 dag → 8086 daghttp → 8087 dagsignal）保持心智模型一致
- 既有 archive `add-rpc-server-dag-http-lab` 已确立 `dag-http` 命名风格，`dag-signal` 是其自然延伸

**替代**:
- `dagwait` —— 放弃，"wait" 描述节点类型而非用户操作；用户视角是「发 signal」
- `dagasync` —— 放弃，"async" 是传输语义而非 DAG 能力；与 `daghttp`（同步 HTTP）对比时反差不如 `dagsignal` 直接
- `approval` —— 放弃，绑死业务语义（审批），违背 lab 中立原则

### D2: 复用 `StringApp` 装配，经 `Memory().Engine()` 走异步 API

**决策**: `buildRuntime` 与 `daghttp` 几乎一致——`app.NewStringApp(WithGraphDir(...), WithLoaderDefaults("lab.Generic","v1"))` → 注册单元 → `LoadGraphID("approval")`。但 handler 不调 `InvokeString`，而是经 `sa.Runtime.Memory().Engine()` 取底层 `*dagmemory.Engine`，直接调用：

```go
eng := sa.Runtime.Memory().Engine()
eng.StartInstance(ctx, &StartRequest{Ref, TypeKey, GraphVersion, InitialPayload, GraphPinPolicy}, dag.WithStartEntityID(...))
eng.DrainInstance(ctx, ref)                              // 推进到 WAITING
eng.DeliverSignal(ctx, &dagv1.SignalDelivery{EntityId, SignalName, DeliveryId})
eng.DrainInstance(ctx, ref)                              // 推进到终态
inst, _ := eng.GetInstance(ctx, ref)                     // 查状态
```

`Service` 持有 `engine dag.Engine` 与 `typeKey *dagv1.EntityTypeKey`（从 `sa.TypeKey()` 取），不再持有 `*app.StringApp` 句柄（装配完即只用 Engine）。

**理由**:
- `StringApp` 已封装「实体类型注册 + unit 注册 + 图加载 + EntityTypeKey」的样板，复用可让 `buildRuntime` 与 `daghttp` 几乎逐行对齐——这是「公共逻辑归纳」的前提
- `doc.go` 明确异步应走底层 API，本决策与之对齐
- 经 `Memory().Engine()` 取 `dag.Engine` 接口（而非 `*dagmemory.Engine` 具体类型）保持面向接口，便于未来替换实现

**替代**:
- 在 `pkg/dag/invocation/app` 新增 `StartString` / `SignalString` 门面 —— Non-goals 已排除（避免过早抽象，且本变更是 lab 验证台）
- 让 `Service` 直接持有 `*app.StringApp` —— 放弃，handler 不需要 StringApp 的同步入口，持有它会让依赖关系误导读者

### D3: fixture `approval.yaml` —— entry 即 wait 节点

**决策**: 新增 `lab/app/dagsignal/fixtures/graphs/approval.yaml`，结构与 `lab/internal/fixtures/graphs/approval_branch.yaml` 对齐但内聚到 dagsignal 自治：

```yaml
graph_id: approval
version: v1
entity_type: lab.Generic
schema_version: v1
entry: wait
nodes:
  wait:
    kind: wait
    signal: approval
    deadline_seconds: 3600
    on_timeout: failure
    transitions:
      - target: success
        condition:
          signal:
            name: approval
      - target: failure
        condition:
          always: true
  success:
    kind: terminal
    outcome: success
  failure:
    kind: terminal
    outcome: failure
```

本应用不注册任何 COMPUTE unit（`registerUnits` 为空操作或省略），因为演示焦点是 `WAIT` + `signal` 路由，而非 COMPUTE。

**理由**:
- 入口即 wait 节点 → `StartInstance` + 首次 `DrainInstance` 后实例立刻停在 WAITING，端到端流程最短
- `condition: signal: name: approval` 与 `condition: always: true` 兜底，演示 signal 名匹配路由
- 自带 fixture（不引用 `lab/internal/fixtures/`）保持 dagsignal 自包含，与 `daghttp` 自带 `echo.yaml` 对齐

**替代**:
- 复用 `lab/internal/fixtures/graphs/approval_branch.yaml` —— 放弃，破坏自包含原则（daghttp 自带 echo.yaml 已确立模式）
- 在 wait 之前加一个 COMPUTE 节点（如 `validate`）—— 放弃，增加 unit 注册样板，模糊演示焦点

### D4: HTTP 语义 —— 自写异步映射纯函数

**决策**: 在 `lab/app/dagsignal/handler.go` 内定义包私有纯函数，把 `*dagv1.EntityInstance.Status` 映射到 `*rpcserver.Response`：

| 实例状态 | HTTP | Body |
|---|---|---|
| WAITING | 202 Accepted | `{"entity_id":"...","status":"WAITING"}` |
| COMPLETED | 200 OK | `{"entity_id":"...","status":"COMPLETED"}` |
| FAILED / COMPENSATED / CANCELLED | 500 | `{"entity_id":"...","status":"...","error":"..."}` |
| RUNNING（signal 后未排空到终态/WAITING） | 202 | `{"entity_id":"...","status":"RUNNING"}` |
| 实例不存在 | 404 | `{"error":"instance not found"}` |

不复用 `dagbridge.ResponseForTerminalResult`（其 WAITING → 500 同步语义与异步应用冲突）。

**理由**:
- 异步应用的「WAITING 是正常中间态」语义与 dagbridge 的「WAITING 是同步调用错误」语义根本不同，复用会误导
- 纯函数 + 状态枚举入参，无包级状态、无 I/O，符合 dagbridge 既定的「映射层为纯函数」风格
- JSON 体（而非裸字符串）便于返回 `entity_id` 供客户端后续 `POST /signal/{entityID}` 与 `GET /instances/{entityID}`

**替代**:
- 扩展 `dagbridge` 加 `ResponseForAsyncResult` —— 放弃，dagbridge doc 已明确定位为「同步 DAG 调用 → HTTP 响应」；扩展会模糊其边界，且本变更 Non-goals 排除修改 dagbridge
- 全部返回 200 + body 区分 —— 放弃，丢失 HTTP 语义，客户端必须解析 body 才能判断成败

### D5: 端点设计 —— start / signal / instances / status

**决策**: 四个端点，路径与 HTTP 动词对齐 REST 习惯：

- `POST /start` —— body 为 payload（可空）。生成 entityID，`StartInstance` + `DrainInstance` → 202 + `{entity_id, status}`（首帧必为 WAITING）。失败（启动/drain 错误）→ 500
- `POST /signal/{entityID}` —— body 可空，query `?signal=approval` 或默认 `approval`。`DeliverSignal` + `DrainInstance` → 200/202/500/404。signal 名不匹配 → 400（`ErrSignalMismatch`）
- `GET /instances/{entityID}` —— 透传 `GetInstance` → 200 + 状态 JSON；不存在 → 404
- `GET /api/status` —— 与 daghttp 一致，返回 `api.Status{Domain:"dagsignal", ...}`

`entityID` 经 `sa.NewEntityIDGen("signal")` 生成（与 daghttp 的 `"http"` 前缀平行）。

**理由**:
- start 与 signal 分离，直接映射「请求 = 启动」「后续请求 = signal」的两阶段异步语义
- `{entityID}` 路径参数让 curl 可直接拼装，无需解析 body
- `GET /instances/{entityID}` 提供只读查询，演示「实例状态可重复查询」的异步特性

**替代**:
- 单端点 `POST /echo-async` 返回 entityID 后由客户端轮询 —— 放弃，无法演示 signal 推进动作
- signal 用 body 而非 path —— 放弃，path 更利于 curl 与日志可读性

### D6: 配置 schema 自治 + env 覆写

**决策**: `lab/app/dagsignal/config.go` 定义 `Config { Store string; FixturesDir string }`（yaml `store` / `fixtures_dir`），`DefaultFixturesDir = "app/dagsignal/fixtures/graphs"`。`LoadConfig()` 解析 `LAB_CONFIG` 或 `configs/default.yaml` 中 `dagsignal` 段，应用 `LAB_DAGSIGNAL_STORE` / `LAB_DAGSIGNAL_FIXTURES_DIR` 覆写，`FixturesDir` 缺省回退 `DefaultFixturesDir`。`Store` 当前仅支持 `memory`。

**理由**:
- 与 `daghttp.Config` 逐字段对齐，使「配置加载是公共逻辑」可被归纳
- 独立 env 前缀（`LAB_DAGSIGNAL_*`）避免与 `LAB_DAG_*` 冲突
- `dagsignal` 段独立于 `dag` 段，应用自治其 schema（不依赖跨域 `LabConfig` 聚合）

**替代**:
- 复用 `dag` 段 —— 放弃，`dag` 段服务于 `lab-dag` CLI；混段会让配置语义模糊
- 引入 `LAB_DAGSIGNAL_GRAPH` 等更多字段 —— 放弃，YAGNI；本变更单图 `approval`

### D7: Makefile 域注册 —— 追加 `dagsignal`

**决策**: `lab/Makefile` 的 `MODULES` 追加 `dagsignal`，注册：

```makefile
MODULE_dagsignal_BIN := lab-dag-signal
MODULE_dagsignal_PROFILES :=
MODULE_dagsignal_PORT := 8087
```

自动获得 `lab-build-dag-signal` / `lab-up-dag-signal` / `lab-down-dag-signal`（既有 `module-targets` foreach 模板）。`lab/configs/default.yaml` 追加 `dagsignal:` 段。

**理由**:
- 既有 foreach 模板已支持「追加一行即获得三个目标」，零额外代码
- 无 compose profile（与 daghttp 一致），单域 `make lab-up-dag-signal` 前台阻塞、无外部中间件

## 公共逻辑归纳（daghttp ↔ dagsignal 对比）

本节是本变更的核心交付物之一：通过两个具体应用的对比，识别「可抽取的公共脚手架」与「场景特化逻辑」，为后续 follow-up 变更提供基线。

### 公共逻辑（抽取候选）

| 维度 | daghttp | dagsignal | 抽取方向 |
|---|---|---|---|
| 应用入口 | `LoadConfig() (*Config, error)` + `Serve(ctx, addr, cfg) error` | 同 | `appbase.Serve` 泛型化（Config 作为类型参数） |
| 配置加载 | 解析 `LAB_CONFIG`/`configs/default.yaml` 的 `<domain>` 段 + `LAB_<DOMAIN>_STORE`/`LAB_<DOMAIN>_FIXTURES_DIR` 覆写 + `FixturesDir` 回退 `DefaultFixturesDir` | 逐行对齐 | `appbase.LoadConfig(domain, DefaultFixturesDir)` |
| Config schema | `Config { Store, FixturesDir }` + yaml 标签 | 逐字段对齐 | 共享 `appbase.Config` 结构 |
| StringApp 装配 | `NewStringApp(WithGraphDir, WithLoaderDefaults("lab.Generic","v1"))` + 注册 unit + `LoadGraphID(defaultGraphID)` | 几乎逐行对齐（dagsignal 无 unit） | `appbase.BuildStringApp(cfg, units, defaultGraphID)` |
| entityID 生成 | `sa.NewEntityIDGen("http")` | `sa.NewEntityIDGen("signal")` | 仅前缀差异，已是公共 API |
| OpRecorder + api.Status | `api.NewOpRecorder(50)` + `StatusInfo() api.Status{Domain, Impl:"memory", Healthy:true, RecentOps, Extra, CollectedAt}` | 同 | `appbase.StatusProvider` 接口或 base struct |
| rpc.Server 装配 | `rpchttp.NewHTTPServer(addr)` + `svc.Register(srv)` + `srv.Start(ctx)` | 同 | `appbase.Serve` 内联 |
| main 骨架 | flag + signal ctx + LoadConfig + Serve | 同 | main 可缩到 ~30 行模板 |

### 场景特化逻辑（不可抽取）

| 维度 | daghttp | dagsignal | 为何特化 |
|---|---|---|---|
| 调用语义 | 同步 `InvokeString`（Start+Drain+Snapshot 一体） | 异步 `StartInstance` + 后续 `DeliverSignal` + `DrainInstance` 分步 | 这是两个应用的存在理由——演示不同 DAG 能力 |
| 终态→HTTP 映射 | `dagbridge.ResponseForTerminalResult`（COMPLETED→200，WAITING→500） | 自写异步映射（WAITING→202，COMPLETED→200，失败终态→500） | 同步/异步语义根本不同；映射层不可共享，但可共享「纯函数 + 状态枚举入参」的形态约定 |
| 请求编排 | 一个端点 `POST /echo`，请求=实例一次性排空 | 多端点（start/signal/instances），请求=start，后续请求=signal | 端点拓扑由编排范式决定 |
| fixture | COMPUTE 链（hello→echo→terminal） | entry=wait + signal 路由 | 图结构由演示场景决定 |
| unit 注册 | `lab.hello` / `lab.echo` | 无 | 是否需要 COMPUTE 取决于场景 |

### 抽取建议（留给 follow-up）

1. **`lab/app/base/`（或 `lab/internal/dagappbase/`）应用骨架** —— 封装 `LoadConfig` + `BuildStringApp` + `Serve(ctx, addr, cfg, register)` 模板，让 daghttp 与 dagsignal 的 `serve.go` 缩到 ~20 行
2. **终态映射策略接口** —— `ResponseMapper` 接口（`Map(*dagv1.EntityInstance) *rpcserver.Response`），daghttp 注入同步实现（包装 dagbridge），dagsignal 注入异步实现；映射层仍是各应用私有，但注册方式公共
3. **`api.Status` 构造 helper** —— `api.NewStatus(domain, impl, rec, extra)` 收敛 StatusInfo 样板

本变更不落地以上抽取，仅锁定基线；follow-up 变更在两个应用都稳定后再抽象，避免过早抽象（目前仅两个样本，模式可能随第三个应用调整）。

## Risks / Trade-offs

- **异步路径走 `Memory().Engine()` 绕过 StringApp 门面** → Mitigation: 在 `serve.go` 与 `handler.go` 注释明确「异步需走底层 Engine API，参见 `pkg/dag/invocation/app/doc.go`」；handler 测试覆盖 start→signal→终态全链路
- **`StartInstance` API 签名复杂（StartRequest / Options）** → Mitigation: 参考 `lab/internal/dag/runtime.go` 的 `Start` 封装与 `pkg/dag/memory/integration_test.go:80-124` 的端到端范例；如签名歧义大，先在 handler 内联最小调用，再决定是否封装
- **`DrainInstance` 后状态可能是 RUNNING（signal 后未到终态）而非 WAITING/终态** → Mitigation: 映射函数显式处理 RUNNING → 202；测试覆盖「signal 后仍 RUNNING」「signal 后 COMPLETED」两种路径
- **fixture `approval.yaml` 与 `lab/internal/fixtures/graphs/approval_branch.yaml` 内容近似重复** → Trade-off: 接受重复以保持 dagsignal 自包含；两份 fixture 语义独立（lab-dag 的演示多分支，dagsignal 的演示 HTTP 闭环）
- **`lab/README.md` 与 spec 章节膨胀** → Mitigation: 章节结构严格镜像 daghttp 既有章节，仅替换能力描述
- **未来抽取共享脚手架时 dagsignal 与 daghttp 都需重构** → Trade-off: 接受——这正是本变更「为 follow-up 提供基线」的目的；design.md 已锁定抽取方向

## Migration Plan

1. **骨架先行**：新建 `lab/app/dagsignal/` 空包 + `cmd/lab-dag-signal/main.go` 最小 usage，验证 `cd lab && go build ./...` 通过
2. **配置 + 装配**：写 `config.go` + `serve.go`（LoadConfig + buildRuntime + Serve），从 daghttp 复制并改 domain/graph/env 前缀
3. **fixture**：写 `fixtures/graphs/approval.yaml`
4. **handler**：写 `handler.go`（Service + 异步映射纯函数 + Start/Signal/Instances/Status 处理器 + Register）
5. **测试**：写 `handler_test.go`（start→WAITING、signal→COMPLETED、signal mismatch→400、unknown entity→404、status）+ `serve_test.go`（buildRuntime、LoadConfig env 覆写、unsupported store）
6. **CLI**：完成 `cmd/lab-dag-signal/main.go`（flag + signal ctx + LoadConfig + Serve）
7. **Makefile + 配置**：`MODULES` 追加 `dagsignal` + 三行注册；`configs/default.yaml` 加 `dagsignal:` 段
8. **文档**：`lab/README.md` 加「DAG Signal HTTP 服务」章节 + CLI 表 + 域注册表；`openspec/specs/uniface-lab/spec.md` 在 archive 时由本变更 specs delta merge
9. **回归**：`cd lab && go build ./... && go vet ./... && go test ./app/dagsignal/...`；`make lab-build-dag-signal`；`make lab-up-dag-signal` 后 `curl POST /start` → `curl POST /signal/{id}` → `curl GET /instances/{id}` 全链路验证
10. **归档**：`openspec validate add-dagsignal-lab-app --strict` 通过后 `openspec archive add-dagsignal-lab-app --yes`

回滚策略：单原子 PR；任一步失败 `git revert` 整体回滚。本变更不修改既有应用与 `pkg/`，回滚零副作用。

## Open Questions

- signal 默认名是否应从 `?signal=approval` query 取，还是固定 `approval`（fixture 仅一个 signal）？倾向固定 + query 覆盖，便于未来扩展多 signal fixture
- `POST /signal/{entityID}` 是否需要返回实例当前 payload？倾向不返回（保持状态查询走 `GET /instances/{id}` 单一职责）
- follow-up 抽取共享脚手架的时机？倾向等第三个 DAG 应用出现后再启动，避免两个样本过早抽象
