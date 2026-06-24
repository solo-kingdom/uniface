## MODIFIED Requirements

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

每个补偿执行 SHALL 使用幂等键 `hash(entity_id, "comp", forward_sequence, compensator_unit_id)`。同一键至多一条 `COMPENSATION_COMMITTED` journal。`CommitHop` 幂等命中时 SHALL reconcile pop 对应栈帧。

#### Scenario: 补偿重试不重复提交

- **WHEN** compensator 第一次执行后崩溃，重试同一补偿帧
- **THEN** 仅一条 `COMPENSATION_COMMITTED` journal

#### Scenario: 补偿失败可重试

- **WHEN** compensator 返回 retryable 错误且 attempt 未耗尽
- **THEN** 系统按 `RetryPolicy` 重试，不 pop 下一帧直至当前帧 committed 或标记失败
