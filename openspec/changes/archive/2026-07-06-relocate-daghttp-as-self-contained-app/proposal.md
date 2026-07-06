## Why

`daghttp` 作为验证「HTTP→DAG→响应」编排范式的应用，其代码当前散落在两处：`lab/internal/daghttp/`（handler/service）与 `lab/internal/wiring/daghttp.go`（装配、`lab.hello`/`lab.echo` unit、默认值）。`lab/internal/` 语义上是「跨域共享基础设施」的位置，把具体应用塞进去既让 daghttp 失去独立应用的外观，也让外部读者难以一眼识别整个应用边界。本次以「封装程度验收」为切入点，把 daghttp 重整为自包含应用，整体迁移到 `lab/app/daghttp/`，使「除了 `pkg/` 基础包外的 daghttp 相关代码均落在该路径下」。

## What Changes

- 新建 `lab/app/daghttp/` 作为 daghttp 自包含应用根，纳入 handler/service、StringApp 装配、`lab.hello`/`lab.echo` 计算单元、echo fixture、`DAGConfig` schema、以及生命周期入口
- 移动 `lab/internal/daghttp/handler.go`、`handler_test.go`、`fixtures/graphs/echo.yaml` → `lab/app/daghttp/`
- 将 `lab/internal/wiring/daghttp.go` 中 daghttp 专属代码（`NewDAGHTTP`、`registerLabUnits`、`helloFunc`、`echoFunc`）整体迁入 `lab/app/daghttp/` 包内
- 新增 `lab/app/daghttp/serve.go`：导出 `LoadConfig() (*Config, error)` 与 `Serve(ctx, addr, cfg) error`，封装 StringApp 构建 + fixture 加载 + unit 注册 + `*app.StringApp.Close` 兜底 + `rpc.Server` 注册 + 启停
- 精简 `lab/cmd/lab-dag-http/main.go`：仅保留 flag 解析、信号 `ctx`、调用 `daghttp.LoadConfig` + `daghttp.Serve`
- `lab/internal/wiring/daghttp.go` 迁空后删除；`lab/internal/wiring/config.go` 中 `DAGConfig` 类型迁入 `lab/app/daghttp/config.go`，跨域共享 `LabConfig` 仍保留
- 更新 `openspec/specs/uniface-lab/spec.md` 的 DAG HTTP 章节：daghttp 实现位置由 `lab/internal/daghttp` 改为 `lab/app/daghttp`

## Capabilities

### New Capabilities

（无新增 capability）

### Modified Capabilities

- `uniface-lab`: 修订「DAG HTTP 服务验证 CLI」与「DAG HTTP 按域生命周期」两项 Requirement —— daghttp 实现位置由 `lab/internal/daghttp` 改为 `lab/app/daghttp`；明确 daghttp 是自包含应用，handler/装配/unit/fixture/配置 schema/生命周期入口均落在该路径下，与 `pkg/` 基础包形成清晰边界

## Impact

- **新增**：`lab/app/daghttp/`（`config.go`、`serve.go`、`units.go`、`handler.go`、`handler_test.go`、`fixtures/graphs/echo.yaml`）
- **移动**：`lab/internal/daghttp/handler.go` 等三个文件 + `lab/internal/wiring/daghttp.go` 的 daghttp 专属内容
- **修改**：`lab/cmd/lab-dag-http/main.go`（-25 行/+10 行）；`openspec/specs/uniface-lab/spec.md`（路径引用更新）
- **删除**：`lab/internal/wiring/daghttp.go`（迁完后为空壳）
- **依赖**：仅路径重排，导入语句同步更新；无 proto/公共 API 变更；无新增外部依赖
- **破坏性**：仅 lab 模块内部包路径变更，不影响根模块 `pkg/` 与外部使用者

## Non-goals

- 不修改 `pkg/` 下任何公共能力（`StringApp`、`dagbridge`、`EntityIDGen`、`pkg/rpc/server` 等保持原状）
- 不调整 `lab-kv`、`lab-config`、`lab-lb`、`lab-queue`、`lab-dag`、`lab-ui` 的位置与装配
- 不重构 `lab/internal/wiring/config.go` 中跨域共享的 `LabConfig`/`KVConfig`/`LBConfig`/`QueueConfig`/`ServicesConfig`
- 不引入新的传输（gRPC/WebSocket）、持久化后端、认证/限流中间件
- 不拆分 daghttp 的接口边界（本次仅目录迁移与装配入口封装，不引入新的包或抽象层）
- 不改 `lab/Makefile`、根 `Makefile`、`docker-compose.yml` 中按域目标的命名与依赖
