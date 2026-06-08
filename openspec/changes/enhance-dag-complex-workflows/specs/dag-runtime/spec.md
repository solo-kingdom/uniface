## ADDED Requirements

### Requirement: ErrNoTransition 实例失败

当 Scheduler 在 COMPUTE、JOIN 或 advance 路径收到 `ErrNoTransition`，系统 SHALL 将实例 `status` 置为 `FAILED`，`current_node_id` 保持当前节点，并 SHALL 写入带 `failure_reason` 的 journal 条目。系统 SHALL NOT 无限重试该 hop。

#### Scenario: 条件路由全部未命中

- **WHEN** COMPUTE 节点所有 transition 条件均为 false
- **THEN** 实例 `status` 变为 `FAILED`，无新的 `NODE_COMMITTED` 成功推进

#### Scenario: 失败后不再调度

- **WHEN** 实例因 `ErrNoTransition` 变为 `FAILED`
- **THEN** 后续 `RunOnce` 不再对该实例调用 `Execute`

### Requirement: 单元级 RetryPolicy

调度 COMPUTE 节点时，系统 SHALL 优先使用 `ComputeUnitDef.retry_policy.max_attempts`；为 0 时 fallback 至 `Engine` 全局 `DefaultRetryPolicy`。`attempt` 递增 SHALL 持久化至 store。

#### Scenario: 单元级 max_attempts 生效

- **WHEN** unit `retry_policy.max_attempts` 为 5，全局默认为 3
- **AND** Execute 连续失败
- **THEN** 第 5 次失败后才返回错误，attempt 持久化为 5

#### Scenario: 单元级为 0 使用全局默认

- **WHEN** unit `retry_policy.max_attempts` 为 0
- **THEN** 使用 `DefaultRetryPolicy.max_attempts`

### Requirement: 多 spawn journal 记录

当 mutation 包含多个 `SpawnSpec`，`CommitHop` SHALL 在 journal 中记录全部 `spawned_refs`。`spawned_ref`（单数）SHALL 等于 `spawned_refs[0]` 以保持兼容。

#### Scenario: 三个子实例全部记入 journal

- **WHEN** spawn mutation 创建 3 个子实例
- **THEN** journal 条目 `spawned_refs` 长度为 3

## MODIFIED Requirements

### Requirement: ExecutionRecord 幂等键

系统 SHALL 为每个 hop 创建 `ExecutionRecord`，幂等键 `idempotency_key = hash(entity_id, node_id, input_sequence)`。同一幂等键至多一条 `COMMITTED` 记录。`attempt` 递增 SHALL 持久化至 store，优先使用单元级 `RetryPolicy`。

#### Scenario: 重复调度已提交 hop

- **WHEN** Scheduler 调度 hop 时发现同 `idempotency_key` 已有 `COMMITTED` ExecutionRecord
- **THEN** 跳过 Execute，使用 `AdvanceInstanceNode` 或等价操作确保 `current_node_id` 与已提交路由一致

#### Scenario: 崩溃后安全重试

- **WHEN** ExecutionRecord 为 `RUNNING` 且无对应 committed journal
- **THEN** Scheduler 允许重新调用 `Execute`，`attempt` 递增并持久化，直至 `CommitHop` 成功或 attempt 耗尽
