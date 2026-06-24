## Why

DAG 引擎内核（COMPUTE / WAIT / JOIN / TERMINAL、Saga 补偿、信号路由）已闭环，但 `ComputeUnit` 当前只能通过进程内 Go 接口注册——业务方必须为每个节点写 Go 代码并重新部署，使 DAG 难以作为"独立引擎产品"对外交付。为承载复杂在线业务 pipeline 与实时数据处理 pipeline，节点必须能以**配置驱动**方式调用远程服务。

## What Changes

- **声明式 ComputeUnit 实现**：`ComputeUnitDef` 新增 `implementation` oneof，与进程内 Go 注册互斥；引擎按声明构造 unit 实例
- **内置 HttpUnit**：覆盖 HTTP/REST 服务调用。配置 `service`（走 Balancer 解析）或 `url`（直连）、`method`、`path`、`headers`、`request_body`、`response` 映射、`timeout`、`retry_on`
- **payload 表达 Level 0/1**：默认整包传（`snapshot.payload` 整个 `Any`）；显式字段路径（`snapshot.payload.order`）复用现有 `resolveFieldPath`
- **response → mutation 映射**：默认 HTTP 2xx → `update`（response 整体作为新 payload）；支持显式 `payload_field` 投影与 `on_success: update|complete|spawn|fail`
- **HTTP 错误分类**：`retry_on.status_codes`（默认 502/503/504）触发 `RetryPolicy`；4xx 默认映射为 `fail`（不重试，按图配置触发补偿）
- **Balancer 集成**：`HttpUnit.service` 通过 `pkg/rpc/governance/loadbalancer` 解析实例，避免硬编码地址
- **YAML schema 升级**：`lab/internal/dag` 解析器支持 `condition`（field/signal）、`priority`、`retry_policy`、`unit.http` 配置块，让现有引擎能力在 YAML 层可表达

## Capabilities

### New Capabilities

- `dag-units`: 声明式 ComputeUnit 实现契约——HttpUnit 配置形态、request body 构造、response → mutation 映射、HTTP 错误分类与重试、Balancer 服务发现集成

### Modified Capabilities

- `dag-runtime`: `ComputeUnitDef` schema 扩展（`implementation` oneof）；Engine unit 解析逻辑（声明式优先于进程内注册）；`SideEffectClass` 与 HttpUnit 副作用语义对齐（`SIDE_EFFECT_IDEMPOTENT` 默认，HTTP unit 应使用幂等键或显式 `NONE`）

## Impact

- **Proto**：`api/dag/v1/unit.proto`（`ComputeUnitDef.implementation` oneof、`HttpUnit` message、`ResponseMapping`、`RetryClassification`、`BodyTemplate`）
- **代码**：
  - 新增 `pkg/dag/units/`（内置 HttpUnit 实现）
  - `pkg/dag/memory/registry.go`（声明式 unit 解析与 Balancer 注入）
  - `pkg/dag/memory/engine.go`（unit 解析路径分支）
  - `pkg/dag/graph/`（body 字段路径求值复用 `resolveFieldPath`）
- **Lab**：`lab/internal/dag/runtime.go` YAML schema 升级；`lab/internal/fixtures/graphs/` 新增 HTTP unit 示例 fixture
- **依赖**：`pkg/dag` 引入对 `pkg/rpc/governance/loadbalancer` 的可选依赖（通过接口注入，根模块零依赖原则不破）

## Non-goals

- `GrpcUnit` / `RpcUnit` 统一抽象（零用例不预先抽象，待首个 gRPC 必须场景评估）
- CEL 表达式、payload Level 2 模板引擎
- 持久化 LineStore（保持内存 MVP）
- 跨进程 spawn、callback 机制、分布式 worker
- 调度后台化、实例列表查询 API（可在后续小迭代中加）
- 修改 `prompts/` 目录
