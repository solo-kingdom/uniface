## ADDED Requirements

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

## MODIFIED Requirements

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
