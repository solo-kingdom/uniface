## Why

DAG 引擎 MVP 已验证黄金路径，但复杂系统（动态并行、信号驱动分支、长生命周期审批）在三处撞墙：信号 payload 不参与路由、JOIN barrier 只能静态预声明、运行时 `ErrNoTransition` 导致实例 silent stuck。本次在保持四种 NodeKind 骨架不变的前提下，补强表达力与运行时可靠性，使单进程实现能承载中等复杂度生产场景。

## What Changes

- **信号路由**：`DeliverSignal` 可将 `SignalDelivery.payload` 合并入 snapshot（可配置策略），`Condition` 新增 `signal_predicate` 支持信号名 + payload 字段分支
- **动态 JOIN**：`JoinSpec` 新增 `dynamic_barriers`（correlation 前缀 + 期望完成数 + `JoinPolicy`），替代运行时未知 N 时预声明 N 个 barrier 的 hack
- **图校验加固**：`ValidateGraphSpec` 校验 COMPUTE 必填 `unit_id`、WAIT 必填信号配置、条件路由图要求兜底边、环检测
- **运行时可靠性**：`ErrNoTransition` 将实例置 `FAILED`；`ComputeUnitDef.retry_policy` 与补偿 retry 接线；补偿完成后继续 pop 直至 `COMPENSATED` 或路由到 TERMINAL
- **Journal 改进**：多 spawn 时 journal 记录全部 `spawned_ref`

## Capabilities

### New Capabilities

（无新增 capability 包，能力归入现有 dag-* spec）

### Modified Capabilities

- `dag-graph`：新增 `signal_predicate` Condition、`dynamic_barriers` JoinSpec、静态校验规则扩展
- `dag-signal`：信号 payload 合并 snapshot、信号条件路由
- `dag-runtime`：`ErrNoTransition` → FAILED、单元级 RetryPolicy、多 spawn journal
- `dag-saga`：补偿 retry、补偿完成后连续 pop / 路由 TERMINAL

## Impact

- **Proto**：`api/dag/v1/graph.proto`（Condition、JoinSpec）、`api/dag/v1/entity.proto`（可选 `SignalMergePolicy`）
- **实现**：`pkg/dag/graph/`、`pkg/dag/memory/engine.go`、`pkg/dag/memory/linestore.go`
- **测试**：扩展 `integration_test.go`、新增动态 JOIN 与信号分支场景
- **Lab**：`lab/internal/dag/` fixture 可演示动态拆单 + 审批分支

## Non-goals

- CEL 表达式路由、`SIDE_EFFECT_EXTERNAL`、Outbox
- 新 NodeKind（MIGRATE、FORK、SUBGRAPH）
- 分布式 Worker、KV 持久化、`GRAPH_PIN_EXPLICIT`
- 修改 `prompts/` 目录
