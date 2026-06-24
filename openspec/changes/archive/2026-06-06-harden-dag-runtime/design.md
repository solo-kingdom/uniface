## Context

`pkg/dag/memory` 已实现 DAG 引擎 MVP，集成测试覆盖黄金路径与 Saga 补偿。探索阶段发现实现与 `openspec/specs/dag-*` 契约存在多处脱节：补偿栈在 `Compensate` 前 pop、图热更新未接线、并发调度缺 per-entity 保护，以及校验与超时等细节缺口。本次在单进程内存实现内加固，不改变 Proto 契约。

## Goals / Non-Goals

**Goals:**

- 补偿、幂等、路由、信号、校验形成可靠闭环，崩溃后可安全重试
- `GRAPH_PIN_ON_NODE` 在 Scheduler 与 `DeliverSignal` 路径生效
- 同 `entity_id` 的 hop 在 Engine 层串行，避免双次 `Execute`
- 补齐 spec 已声明但未实现的校验与 journal 语义

**Non-Goals:**

- 分布式 Worker、KV 持久化、CEL、`SIDE_EFFECT_EXTERNAL`
- `GRAPH_PIN_EXPLICIT`、MIGRATE 节点
- 修改 `api/dag/v1/*.proto`

## Decisions

### D1: 补偿 pop 移入 CommitHop

**决策**: `processCompensation` 只读栈顶帧 → `Compensate` → `CommitHop(COMPENSATION_COMMITTED)`；pop 在 `LineStore.CommitHop` 内与 journal 同事务。

**理由**: 崩溃点在 Compensate 与 CommitHop 之间时，栈帧仍在，可重试 Compensate；与 hop exactly-once 模型一致。

**替代**: 独立 `PopSagaFrame` 在 Compensate 后调用——放弃，非原子。

### D2: forward_snapshot 压栈时写入

**决策**: `commitCompute` push `CompensationFrame` 时填充 `forward_snapshot = outSnap.Payload`；补偿优先使用该快照。

**理由**: 补偿应基于正向 hop 输出，而非可能已变更的当前快照。

### D3: Registry latest + ResolveGraphForInstance

**决策**: `RegisterGraph` 更新 `latestGraphs[graph_id]`；新增 `ResolveGraphForInstance(inst)` 封装 pin 策略；`GRAPH_PIN_ON_NODE` 使用 `graph.ResolveGraphVersion(inst, latest)`。

**理由**: 单一入口，Scheduler / DeliverSignal 共用，避免遗漏。

**替代**: 每次手动 GetLatest——易散落。

### D4: per-entity 锁（sync.Map + *sync.Mutex）

**决策**: `Engine` 持有 `entityLocks`，`RunOnce`/`DeliverSignal`/`processTimeout` 对目标 `entity_id` 加锁。

**理由**: MVP 最小改动即可防止同实例并发 hop；二期可换分片调度。

### D5: 校验在 CommitHop 前执行

**决策**: `entity.ValidateOutputType`、`entity.ValidateSchemaCompatible` 在 `commitCompute` 写库前调用；失败返回 `ErrTypeMismatch` / `ErrIncompatibleSchema`。

**理由**: spec 已声明，实现补齐；早失败避免脏 journal。

### D6: LineStore 辅助方法

**决策**: 新增 `UpdateExecutionAttempt`、`AdvanceInstanceNode`；`CommitHop` 幂等命中时 reconcile saga pop。

**理由**: `advanceAfterCommit` 与重试计数需持久化，不可只改 clone。

## Risks / Trade-offs

| 风险 | 缓解 |
|------|------|
| PIN_ON_NODE 后节点被删 | `ResolveGraphForInstance` 返回 `ErrInvalidGraph`，实例 FAILED |
| per-entity 锁粒度粗 | 仅锁单实例，不同实例仍可并行 |
| latest = 最后注册版本 | 文档说明；二期可加显式 publish |
| accepted_signals 与 signal_name 皆空 | 拒绝任意信号（返回 mismatch） |

## Migration Plan

纯实现加固，无数据迁移。步骤：

1. 按 tasks.md 顺序修改 `pkg/dag/memory/*` 与 `entity/`
2. `make test` 全绿
3. 归档 change 后 delta spec 合并至 `openspec/specs/dag-*/`

回滚：还原 `pkg/dag` 改动，不影响其他模块。

## Open Questions

- 单元级 `RetryPolicy` 是否本期接入，或继续用全局 `DefaultRetryPolicy`？（建议本期仅持久化 attempt，单元级 policy 可 follow-up）
