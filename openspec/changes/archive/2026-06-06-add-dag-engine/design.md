## Context

Uniface 遵循接口优先、实现可热切换的架构模式。现有领域（KV、Config、Queue、LoadBalancer）均为「契约 + 多实现子模块」结构。本次新增的 DAG 执行引擎延续该模式，但语义上更接近工作流编排：以**实体实例为线**，计算单元为节点，图规格为数据。

探索阶段已确认的核心建模：

- 线 = `EntityInstance`（`entity_id` 全局唯一，append-only `LineJournal`）
- 类型身份 = `EntityTypeKey{entity_type, payload_schema_version}` 强制二元组
- 图结束 = 显式 `NODE_KIND_TERMINAL`，禁止空 target
- 子实例 = `SpawnSpec.graph` 必填，不继承父图
- exactly-once = `ExecutionRecord` 幂等键 + `CommitHop` 事务提交
- 一期包含 WaitSignal、Saga、Join

### 与现有组件关系

| 组件 | MVP | 二期 |
|------|-----|------|
| `pkg/storage/kv` | 不用 | LineStore / Journal 后端 |
| `pkg/messaging/queue` | 不用 | Worker 任务分发 |
| `pkg/rpc/.../shard` | 不用 | 按 `entity_id` 分片调度 |
| `pkg/storage/config` | 不用 | GraphSpec / Registry 热更新 |

## Goals / Non-Goals

**Goals:**

- 定义 `pkg/dag/` 核心接口与 Protobuf 契约 `api/dag/v1/`
- 实现单进程内存 MVP（Engine + Scheduler + MemoryLineStore）
- 框架级 exactly-once：`idempotency_key = hash(entity_id, node_id, input_sequence)`
- 数据驱动 `GraphResolver`（FieldPredicate + NodeKind 分支）
- 黄金路径集成测试：Start → Validate → Wait → Charge（崩溃重试）→ Spawn → Join → Terminal，及 Saga 失败分支

**Non-Goals:**

- 分布式 Worker、CEL 路由、EXTERNAL 副作用 Outbox
- 实例内 schema 静默升级（需专用 MIGRATE 节点）
- 可视化、多语言 SDK

## Decisions

### D1: 领域包路径

**决策**: 新建 `pkg/dag/` 与 `api/dag/v1/`。

```
pkg/dag/
  interface.go      # Engine, ComputeUnit, Compensator, GraphResolver, LineStore, Registry
  options.go
  errors.go
  entity/           # Go 类型与 proto 映射（非子模块）
  graph/
  runtime/
  memory/           # MVP 内存实现

api/dag/v1/
  common.proto, entity.proto, graph.proto, unit.proto
  registry.proto, saga.proto, runtime.proto
```

**理由**: DAG 编排是独立领域，不宜塞入 `pkg/rpc/` 或 `pkg/messaging/`。

### D2: 线 = 实例，Journal 为证据链

**决策**: `EntityInstance` 存元数据（status、sequence、current_node、graph_version）；每个 committed hop 写 `LineJournalEntry{KIND_NODE_COMMITTED}`。

**理由**: 与 Temporal Workflow History 同构，EOS 可基于 journal 重放恢复。

**替代方案**: 纯事件溯源（只存事件，快照投影）。放弃——MVP 复杂度更高，且业务更习惯 snapshot 输入。

### D3: Exactly-once 语义

**决策**:

```
对外 EOS = 同一 idempotency_key 至多一条 NODE_COMMITTED journal
idempotency_key = f(entity_id, node_id, input_sequence)
```

流程：

1. `CreateExecution` CAS：已有 COMMITTED → 直接返回
2. `Execute` 可多次调用（崩溃重试）
3. `CommitHop` 事务：写 COMMITTED journal + 更新 instance + saga push（原子）
4. 业务 `Execute`/`Compensate` 必须逻辑幂等；`SideEffectClass_IDEMPOTENT` 要求外部调用带幂等键

**替代方案**: 分布式 2PC。放弃——MVP 过重；一期仅 NONE + IDEMPOTENT。

### D4: 图节点四种 Kind

**决策**: `COMPUTE | WAIT | JOIN | TERMINAL`。

| Kind | 行为 |
|------|------|
| COMPUTE | 绑定 `ComputeUnitDef`，Execute → Mutation → Commit |
| WAIT | 进入 WAITING，等 `DeliverSignal` 或超时 |
| JOIN | 检查子实例 barriers，满足后 JOIN_COMMITTED |
| TERMINAL | 设置 `TerminalOutcome`，实例终态 |

**理由**: 显式 TERMINAL 避免空 target 歧义；WAIT/JOIN 独立 kind 简化调度器。

### D5: 类型注册与 FieldPredicate

**决策**: `EntityTypeRegistration` 主键为 `EntityTypeKey`；`FieldPredicate` 求值前必须 `proto.Unmarshal` payload。

**field_path 规则（MVP）**: 标量字段、一层 `repeated` 下标（如 `items[0].sku`）。

**理由**: 避免对 `Any` 做黑盒 JSON 路径；类型安全。

### D6: Saga 持久化栈

**决策**: `SagaState.stack` 与 `EntityInstance` 同事务持久化；正向 hop commit 时 push frame；失败时 LIFO pop 执行 `Compensator`。

补偿幂等键：`hash(entity_id, "comp", forward_sequence, compensator_unit_id)`。

### D7: MVP 调度模型

**决策**: 单进程 `Scheduler` 轮询就绪实例；`shard_key = entity_id` 接口预留，MVP 不并发同实例 hop。

**理由**: 先证明逻辑闭环，分布式为二期透明替换。

## Risks / Trade-offs

| 风险 | 缓解 |
|------|------|
| Execute 内副作用重复 | 文档强制幂等 + IDEMPOTENT 契约；EXTERNAL 一期不支持 |
| Join 轮询延迟 | MVP 轮询；二期改事件驱动（子实例终态回调） |
| GraphSpec 配置错误无出边 | Resolver 返回 `ErrNoTransition`；实例 → FAILED |
| proto 与 Go 类型双维护 | 代码生成 `protoc-gen-go`；映射层薄封装 |
| 单进程 MVP 无法验证分布式 EOS | 二期补 CAS 锁 + 分片测试；MVP 验证崩溃重试 |

## Migration Plan

纯新增，无迁移。发布顺序：

1. `api/dag/v1/*.proto` + `make build`
2. `pkg/dag/` 接口 + memory 实现
3. 集成测试黄金路径
4. 归档 change 后 `openspec/specs/dag-*/` 成为基线

回滚：删除 `pkg/dag/` 与 `api/dag/`，不影响现有模块。

## Open Questions

- `WAIT` 节点是否允许绑定 `unit_id`（一期采用纯配置型，不绑 unit）
- Join 子实例匹配用 `child_entity_id` 还是 `correlation_id`（一期两者均支持，`JoinBarrier` 显式声明）
