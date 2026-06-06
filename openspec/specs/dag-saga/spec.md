# dag-saga Specification

## Purpose
TBD - created by archiving change add-dag-engine. Update Purpose after archive.
## Requirements
### Requirement: 正向 hop 压栈

当 `COMPUTE` 节点成功 `CommitHop` 且配置了 `compensator_unit_id`，系统 SHALL 向 `SagaState.stack` push `CompensationFrame`（含 `node_id`、`unit_id`、`compensator_unit_id`、`forward_sequence`）。`SagaState` SHALL 与 `EntityInstance` 持久化存储。

#### Scenario: 成功 charge 后压栈

- **WHEN** charge 节点 commit 成功且 compensator 为 `order.refund`
- **THEN** stack 新增一帧，`forward_sequence` 等于该 hop 输出 sequence

#### Scenario: TERMINAL 不压栈

- **WHEN** 节点 kind 为 `TERMINAL`
- **THEN** 系统不向 saga stack push 帧

### Requirement: 失败触发补偿

当 `EntityMutation.fail` 且 `trigger_compensation=true`，系统 SHALL 将实例 `status` 置为 `COMPENSATING`，按 LIFO 顺序 pop stack 并调用 `Compensator.Compensate`。

#### Scenario: 逆序补偿

- **WHEN** 正向经过 validate → charge 后 charge 失败触发补偿
- **THEN** 先执行 charge 的 compensator，不执行 validate 的 compensator（若 validate 无 compensator）

#### Scenario: 补偿成功到达 COMPENSATED

- **WHEN** 所有补偿帧均 `COMPENSATION_COMMITTED`
- **THEN** 实例 `status` 变为 `COMPENSATED`

### Requirement: 补偿幂等键

每个补偿执行 SHALL 使用幂等键 `hash(entity_id, "comp", forward_sequence, compensator_unit_id)`。同一键至多一条 `COMPENSATION_COMMITTED` journal。

#### Scenario: 补偿重试不重复提交

- **WHEN** compensator 第一次执行后崩溃，重试同一补偿帧
- **THEN** 仅一条 `COMPENSATION_COMMITTED` journal

#### Scenario: 补偿失败可重试

- **WHEN** compensator 返回 retryable 错误且 attempt 未耗尽
- **THEN** 系统按 `RetryPolicy` 重试，不 pop 下一帧直至当前帧 committed 或标记失败

### Requirement: 补偿后终态

补偿完成后，系统 SHALL 根据图路由将实例导向 `TERMINAL_OUTCOME_FAILURE` 或保持 `COMPENSATED` 终态。

#### Scenario: 补偿后进入失败终止节点

- **WHEN** 补偿全部完成且图配置补偿后 transition 指向 `term_failure`
- **THEN** 实例到达 `TERMINAL` 且 outcome 为 `FAILURE`

