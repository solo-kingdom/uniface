## Why

Uniface 已提供 KV 存储、配置管理、消息队列、负载均衡等基础设施抽象，但缺少**工作流 / DAG 编排**领域的统一接口。业务系统普遍需要将领域实体（Protobuf 定义）沿数据驱动图流转，并保证 exactly-once 执行、等待外部信号、失败补偿等可靠性语义，而 Temporal、Airflow 等方案与 Uniface 的接口优先、实现可热切换原则不一致。

需要新增以**实体实例为线、计算单元为节点**的分布式 DAG 执行引擎框架：业务实现 ComputeUnit，框架提供调度、持久化、Journal、Saga 与类型注册，并复用现有 KV/Queue/Shard 等积木。

## What Changes

- 新增顶层领域 `pkg/dag/`，遵循接口优先模式（interface.go、options.go、errors.go）
- 新增 Protobuf 契约 `api/dag/v1/`（entity、graph、unit、registry、saga、runtime）
- 实现核心接口：`Engine`、`ComputeUnit`、`Compensator`、`GraphResolver`、`LineStore`、`Registry`
- 实现内存 MVP 运行时（单进程 Scheduler + Memory LineStore），跑通黄金路径集成测试
- 支持实体类型二元组 `EntityTypeKey(entity_type, payload_schema_version)` 强制校验
- 支持数据驱动 `GraphSpec`（COMPUTE / WAIT / JOIN / TERMINAL 节点）
- 支持框架级 exactly-once（ExecutionRecord 幂等键 + CommitHop 事务语义）
- 支持 `WaitSignal` / `DeliverSignal` 一期闭环
- 支持 Saga 补偿（持久化补偿栈 + Compensator）
- 支持 Fan-out（Spawn）与 Fan-in（Join）

## Capabilities

### New Capabilities

- `dag-entity`: 实体实例线模型（EntityRef、EntityInstance、EntitySnapshot、EntityMutation、SpawnSpec）
- `dag-graph`: 数据驱动图规格与路由（GraphSpec、NodeKind、Transition、GraphResolver、Terminal/Wait/Join）
- `dag-runtime`: 执行引擎运行时（Engine、Scheduler、ExecutionRecord、LineJournal、exactly-once CommitHop）
- `dag-saga`: Saga 补偿（SagaState、CompensationFrame、Compensator、补偿 Journal）
- `dag-signal`: 等待与外部信号（WaitSignal、SignalDelivery、WAITING 状态机、delivery_id 去重）

### Modified Capabilities

（无——纯新增，不修改现有 KV、Config、Queue、LoadBalancer 规格）

## Non-goals

- 不实现 CEL 表达式路由（一期仅 FieldPredicate）
- 不实现 `SideEffectClass_EXTERNAL` 的 Outbox/2PC（一期仅 NONE + IDEMPOTENT）
- 不实现分布式 Worker 池与 Queue 传输（二期；MVP 为单进程内存实现）
- 不实现多语言 ComputeUnit SDK
- 不实现图 mid-flight 自动迁移（`GRAPH_PIN_EXPLICIT` 仅预留）
- 不实现可视化 UI 或 DAG 编辑器

## Impact

- **新增领域**：`pkg/dag/` 与 `api/dag/v1/`，和 `pkg/storage/`、`pkg/messaging/`、`pkg/rpc/` 并列
- **根模块**：接口与内存 MVP 在根 `go.mod`；后续存储后端可为独立子模块
- **依赖**：MVP 零外部依赖；二期可选依赖 `pkg/storage/kv`、`pkg/messaging/queue`、`pkg/rpc/governance/loadbalancer/shard`
- **不涉及现有代码变更**：纯新增
