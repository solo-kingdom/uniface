## Context

`pkg/dag/memory` MVP 已闭环 exactly-once hop、Saga 补偿、四种 NodeKind。探索阶段确认复杂系统瓶颈不在 NodeKind 数量，而在：

1. `DeliverSignal` 忽略 `payload`，无法信号驱动分支
2. `JoinBarrier` 静态预声明，无法承载运行时决定 N 的动态 fan-out/fan-in
3. 条件路由无兜底时实例 silent stuck；单元级 `RetryPolicy` 与补偿 retry 未接线

本次在 `api/dag/v1` 做**向后兼容**的 proto 扩展，在 `pkg/dag/memory` 内实现，不引入分布式组件。

## Goals / Non-Goals

**Goals:**

- 信号 payload 可参与路由，`Condition` 支持 `signal_predicate`
- `JoinSpec` 支持 `dynamic_barriers`（correlation 前缀 + 期望完成数）
- 图静态校验覆盖 COMPUTE/WAIT 必填项、兜底边、环检测
- `ErrNoTransition` 将实例置 `FAILED`；单元级与补偿 retry 接线
- 补偿完成后同一 tick 连续处理直至栈空或路由 TERMINAL
- 多 spawn journal 记录全部子实例

**Non-Goals:**

- CEL、`SIDE_EFFECT_EXTERNAL`、Outbox、新 NodeKind、分布式 Worker、KV 持久化

## Decisions

### D1: 信号 payload 合并策略

**决策**: `DeliverSignal` 成功去重后，若 `SignalDelivery.payload` 非空，在 `CommitHop(SIGNAL_RECEIVED)` 前将 payload merge 入 snapshot：

- 默认策略 `MERGE_OVERWRITE`：同 type_url 的 protobuf 字段级 merge（`proto.Merge`）；type_url 不同则包装为 `SignalEnvelope{signal_name, payload}` 写入专用 wrapper message（`dag.v1.SignalPayload`）
- WAIT 节点 `WaitNodeConfig.merge_signal_payload = true`（默认 true）控制是否合并

**理由**: 路由仍基于 `EntitySnapshot`，无需改 `GraphResolver` 签名；下一 hop 的 `FieldPredicate` 可直接读合并后字段。

**替代**: 独立 `SignalContext` 并行传递——放弃，Resolver 接口需改，调用链扩散。

### D2: Condition.signal_predicate

**决策**: 在 `graph.proto` 的 `Condition.oneof` 新增 `SignalPredicate`：

```protobuf
message SignalPredicate {
  string signal_name = 1;           // 可选，空则匹配任意已接收信号
  FieldPredicate payload_predicate = 2; // 对合并后 snapshot 求值
}
```

求值时机：`DeliverSignal` 路由时，若当前 hop 为 SIGNAL_RECEIVED，优先评估 `signal_predicate`；COMPUTE 节点正常评估 `field_predicate`。

**理由**: 复用现有 `FieldPredicate` 求值器，不引入 CEL。

### D3: JoinSpec.dynamic_barriers

**决策**: 新增 `DynamicJoinBarrier`：

```protobuf
message DynamicJoinBarrier {
  string correlation_prefix = 1;  // 匹配 parent 下 correlation_id 前缀
  int32 expected_count = 2;       // 期望完成子实例数，0 表示全部匹配
  JoinPolicy policy = 3;          // JOIN_ALL_SUCCESS / JOIN_ANY_SUCCESS
}
```

`JoinSpec` 保留静态 `barriers`，新增 `repeated DynamicJoinBarrier dynamic_barriers`。检查逻辑：

1. `ListChildrenByCorrelationPrefix(parent, prefix)` 返回子实例列表
2. `expected_count > 0` 时要求 `len(children) >= expected_count` 且完成数按 policy 判断
3. `expected_count == 0` 时以 snapshot 中 `spawned_count` 或 journal `SPAWNED` 条目计数为期望数（取最近一次 spawn hop 的 spawned 列表长度）

**理由**: 覆盖「运行时 spawn N 个、join 等 N 个完成」主路径；静态 barrier 保持兼容。

**替代**: 纯 snapshot 驱动（业务写 child ID 列表）——放弃，JOIN 节点不应依赖业务维护 ID 列表。

### D4: 图静态校验扩展

**决策**: `ValidateGraphSpec` 新增：

| 检查项 | 规则 |
|--------|------|
| COMPUTE | `unit_id` 非空 |
| WAIT | `wait_config.signal_name` 非空或 `accepted_signals` 非空 |
| 条件路由 | 非 TERMINAL/WAIT/JOIN 节点至少一条 `always=true` 兜底边 |
| 环检测 | DFS 检测环；有环则失败（WAIT 超时边不计入环） |

注册时 `Registry.RegisterGraph` 可选校验 `unit_id` 已注册（warn 级，不阻断，避免启动顺序耦合）。

### D5: ErrNoTransition 处理

**决策**: `processCompute` / `advanceAfterCommit` / `processJoin` 收到 `ErrNoTransition` 时：

1. 写 `NODE_COMMITTED` journal（kind 不变，标记失败原因 metadata 放 `HopCommit` 新字段 `failure_reason`）
2. 实例 `status` → `FAILED`，`current_node_id` 保持

**理由**: 复杂图配置错误应快速失败，避免 Scheduler 无限重试。

### D6: RetryPolicy 接线

**决策**:

- `processCompute`：`maxAttempts` 取 `unitDef.RetryPolicy.MaxAttempts`，为 0 时 fallback `opts.DefaultRetryPolicy`
- `processCompensation`：同样读取 compensator 对应 unit 的 `RetryPolicy`；失败时 `UpdateExecutionAttempt` 等价字段（复用 `ExecutionRecord` 或新增 `CompensationRecord.attempt`）
- backoff：MVP 仅持久化 attempt 计数，不 sleep（与现实现一致）

### D7: 补偿完成后连续处理

**决策**: `processCompensation` 在 `CommitHop(COMPENSATION_COMMITTED)` 成功后：

- 若栈非空，**同一次 `processInstance` 调用**继续处理下一帧（循环，非递归超 100 帧则返回 error）
- 栈空后：`status` → `COMPENSATED`；若 `current_node_id` 对应节点有 transitions，尝试 `ResolveTransitions` 路由到 TERMINAL

**理由**: 对齐 `dag-saga` spec「补偿后进入失败终止节点」场景；减少额外 `RunOnce` 轮次。

### D8: 多 spawn journal

**决策**: `LineJournalEntry` 新增 `repeated EntityRef spawned_refs`；保留 `spawned_ref` 兼容（= `spawned_refs[0]`）。`CommitHop` 写入全部 spawned。

## Risks / Trade-offs

| 风险 | 缓解 |
|------|------|
| signal merge 污染 snapshot schema | 默认 merge；type_url 不匹配时用 `SignalPayload` wrapper，业务可选用 `merge_signal_payload=false` |
| dynamic join 计数与 spawn 不一致 | 以 journal `SPAWNED` 条目为权威期望数；spawn 失败则不写 journal |
| 环检测误报 WAIT 超时边 | 超时边单独处理，不参与主 transition DFS |
| proto 字段新增 | 全部 additive，旧客户端忽略新字段 |
| 补偿连续循环阻塞 | 上限 100 帧；超出返回 error |

## Migration Plan

1. 扩展 proto → `make proto`
2. 实现 graph 校验 + resolver 扩展
3. 实现 engine/linestore 信号 merge、dynamic join、retry、补偿循环
4. 扩展集成测试：审批分支、动态拆单 join
5. 归档后 delta spec 合并至 `openspec/specs/dag-*/`

回滚：还原 proto 与 `pkg/dag` 改动；已持久化实例若用了新字段，内存 MVP 无迁移负担。

## Open Questions

- `SignalPayload` wrapper 是否单独 proto message，还是复用 `google.protobuf.Struct`？（建议专用 message，类型更明确）
- `dynamic_barriers` 的 `expected_count=0` 是否一期实现，或要求业务显式写入 snapshot？（建议一期实现，依赖 journal SPAWNED 计数）
