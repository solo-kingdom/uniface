## Why

`pkg/dag/memory` MVP 黄金路径已跑通，但探索发现多处契约与实现脱节：补偿在 `CommitHop` 前 pop 栈导致崩溃后帧丢失、`GRAPH_PIN_ON_NODE` 未接线、并发 `RunOnce` 可双次 `Execute`，以及信号校验、超时路由、重试计数等半实现缺口。若不加固，引擎无法在单进程多 goroutine 或崩溃重试场景下形成可靠闭环。

## What Changes

- 补偿流程改为「先 Compensate、后 CommitHop 原子 pop」；压栈时写入 `forward_snapshot`
- Registry 增加 `latestGraphs` 与 `ResolveGraphForInstance`，接线 `GRAPH_PIN_ON_NODE`
- Engine 增加 per-entity 锁，保证同实例 hop 串行
- 实现 `accepted_signals` 校验、`output_type_keys` 与 `compatible_inputs` 提交前校验
- 修复 `advanceAfterCommit`、`processTimeouts`、`ExecutionRecord.Attempt` 持久化
- spawn hop 使用 `JOURNAL_KIND_SPAWNED`；补偿幂等 reconcile 与 journal 一致

## Capabilities

### New Capabilities

（无——本次为既有 DAG 能力的实现加固。）

### Modified Capabilities

- `dag-saga`: 明确 pop 仅在 `CommitHop` 内；`forward_snapshot` 必填；幂等 reconcile 规则
- `dag-runtime`: per-entity 串行；重试 attempt 持久化；`advanceAfterCommit` 落库语义
- `dag-graph`: `GRAPH_PIN_ON_NODE` 解析最新图版本；Registry latest 追踪
- `dag-signal`: `accepted_signals` 完整支持；超时 hop 使用正确 node/sequence
- `dag-entity`: `output_type_keys` 与 `compatible_inputs` 提交校验；spawn journal kind

## Impact

- `pkg/dag/memory/engine.go`、`linestore.go`、`registry.go`
- `pkg/dag/entity/entity.go`、`pkg/dag/interface.go`、`pkg/dag/graph/graph.go`
- 增量测试：`integration_test.go` 及单元测试
- 无 Proto 破坏性变更；无新外部依赖

## Non-goals

- 分布式 Worker、分片调度、`SIDE_EFFECT_EXTERNAL`、CEL 路由
- `GRAPH_PIN_EXPLICIT`、MIGRATE 节点、实例内 schema 静默升级
- KV/Queue 持久化后端替换
