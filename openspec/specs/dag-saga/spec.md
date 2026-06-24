# dag-saga Specification

## Purpose
TBD - created by archiving change add-dag-engine. Update Purpose after archive.
## Requirements
### Requirement: 正向 hop 压栈

当 `COMPUTE` 节点成功 `CommitHop` 且配置了 `compensator_unit_id`，系统 SHALL 向 `SagaState.stack` push `CompensationFrame`（含 `node_id`、`unit_id`、`compensator_unit_id`、`forward_sequence`、`forward_snapshot`）。`SagaState` SHALL 与 `EntityInstance` 持久化存储。

#### Scenario: 成功 charge 后压栈

- **WHEN** charge 节点 commit 成功且 compensator 为 `order.refund`
- **THEN** stack 新增一帧，`forward_sequence` 等于该 hop 输出 sequence，且 `forward_snapshot` 等于该 hop 输出 payload

#### Scenario: TERMINAL 不压栈

- **WHEN** 节点 kind 为 `TERMINAL`
- **THEN** 系统不向 saga stack push 帧

### Requirement: 失败触发补偿

当 `EntityMutation.fail` 且 `trigger_compensation=true`，系统 SHALL 将实例 `status` 置为 `COMPENSATING`，按 LIFO 顺序读取 stack 顶帧、调用 `Compensator.Compensate`，并在 `CommitHop(COMPENSATION_COMMITTED)` 成功后才 pop 该帧。

#### Scenario: 逆序补偿

- **WHEN** 正向经过 validate → charge 后 charge 失败触发补偿
- **THEN** 先执行 charge 的 compensator，不执行 validate 的 compensator（若 validate 无 compensator）

#### Scenario: 补偿成功到达 COMPENSATED

- **WHEN** 所有补偿帧均 `COMPENSATION_COMMITTED`
- **THEN** 实例 `status` 变为 `COMPENSATED`

#### Scenario: Compensate 后崩溃可重试

- **WHEN** `Compensate` 已成功但 `CommitHop` 尚未完成
- **THEN** 栈顶帧仍存在，重试时再次调用 `Compensate`（业务须幂等），直至 journal committed

### Requirement: 补偿幂等键

每个补偿执行 SHALL 使用幂等键 `hash(entity_id, "comp", forward_sequence, compensator_unit_id)`。同一键至多一条 `COMPENSATION_COMMITTED` journal。`CommitHop` 幂等命中时 SHALL reconcile pop 对应栈帧。补偿失败时 SHALL 按单元级 `RetryPolicy` 重试，不 pop 下一帧。

#### Scenario: 补偿重试不重复提交

- **WHEN** compensator 第一次执行后崩溃，重试同一补偿帧
- **THEN** 仅一条 `COMPENSATION_COMMITTED` journal

#### Scenario: 补偿失败可重试

- **WHEN** compensator 返回错误且 attempt 未耗尽（按单元级 `RetryPolicy`）
- **THEN** 系统递增 attempt 并持久化，不 pop 下一帧

#### Scenario: attempt 耗尽标记失败

- **WHEN** compensator 连续失败且 attempt 达到 `max_attempts`
- **THEN** 实例 `status` 变为 `FAILED`，stack 保持不变供人工介入

### Requirement: 补偿后终态

补偿完成后，系统 SHALL 根据图路由将实例导向 `TERMINAL_OUTCOME_FAILURE` 或保持 `COMPENSATED` 终态。

#### Scenario: 补偿后进入失败终止节点

- **WHEN** 补偿全部完成且图配置补偿后 transition 指向 `term_failure`
- **THEN** 实例到达 `TERMINAL` 且 outcome 为 `FAILURE`

### Requirement: 补偿连续处理

`processCompensation` 在 `CommitHop(COMPENSATION_COMMITTED)` 成功后，若 saga stack 仍非空，系统 SHALL 在同一次 `processInstance` 调用中继续处理下一帧（单帧上限 100）。栈空后 SHALL 将实例 `status` 置为 `COMPENSATED`。

#### Scenario: 两帧补偿同 tick 完成

- **WHEN** stack 含 validate 与 charge 两帧 compensator
- **THEN** 单次 `processInstance` 依次执行两帧补偿，最终 stack 为空

#### Scenario: 超过 100 帧返回错误

- **WHEN** stack 深度超过 100
- **THEN** 返回错误，不无限循环

### Requirement: 补偿后路由 TERMINAL

补偿全部完成且 `current_node_id` 对应节点存在 transitions 时，系统 SHALL 调用 `GraphResolver` 尝试路由。若下一节点为 `TERMINAL`，系统 SHALL 提交 terminal hop 并设置对应 `InstanceStatus`。

#### Scenario: 补偿后进入失败终止节点

- **WHEN** 补偿全部完成且当前节点 transition 指向 `term_failure`（TERMINAL，outcome FAILURE）
- **THEN** 实例到达 `TERMINAL` 且 `status` 为 `FAILED`

