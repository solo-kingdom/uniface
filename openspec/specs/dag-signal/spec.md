# dag-signal Specification

## Purpose
TBD - created by archiving change add-dag-engine. Update Purpose after archive.
## Requirements
### Requirement: 进入 WAITING 状态

当 `COMPUTE` 返回 `mutation.wait`，或调度 `NodeKind_WAIT` 节点时，系统 SHALL 将实例 `status` 置为 `WAITING`，记录 `current_node_id`，并 SHALL NOT 增加 `sequence`（直至信号解除）。

#### Scenario: wait mutation 进入等待

- **WHEN** `Execute` 返回 `EntityMutation{wait: {signal_name: "manual_approval"}}`
- **THEN** 实例 `status` 为 `WAITING`，Scheduler 不再调度该实例正向 hop

#### Scenario: WAIT 节点配置驱动

- **WHEN** 路由进入 `NodeKind_WAIT` 且 `WaitNodeConfig.signal_name` 为 `manual_approval`
- **THEN** 实例进入 `WAITING`，等待同名信号

### Requirement: DeliverSignal 去重

`Engine.DeliverSignal` SHALL 使用 `delivery_id` 去重，幂等键 `hash(entity_id, signal_name, delivery_id)`。重复投递 SHALL 返回成功且不重复推进。首次成功投递 SHALL 合并 payload（若配置允许）、写入 `SIGNAL_RECEIVED` journal，并调用 `GraphResolver` 从 wait 节点继续。

#### Scenario: 首次信号推进

- **WHEN** 实例处于 `WAITING`，调用 `DeliverSignal{signal_name: "manual_approval", delivery_id: "D1"}`
- **THEN** 写入 `SIGNAL_RECEIVED` journal，实例 `status` 恢复 `RUNNING`，`GraphResolver` 从 wait 节点继续

#### Scenario: 重复 delivery_id 忽略

- **WHEN** 相同 `delivery_id` 再次 `DeliverSignal`
- **THEN** 返回 nil，实例 sequence 不增加，无新 journal

### Requirement: 信号名称校验

`DeliverSignal.signal_name` SHALL 匹配等待中的 `signal_name` 或 `accepted_signals` 列表之一，否则返回 `ErrSignalMismatch`。`WaitingInstance` SHALL 持久化 `accepted_signals`。

#### Scenario: 错误信号被拒绝

- **WHEN** 实例等待 `manual_approval`，收到 `signal_name: "wrong_signal"`
- **THEN** 返回 `ErrSignalMismatch`，实例保持 `WAITING`

#### Scenario: accepted_signals 别名可推进

- **WHEN** 实例等待配置 `signal_name: "approval"` 且 `accepted_signals: ["manual_approval"]`
- **AND** 收到 `signal_name: "manual_approval"`
- **THEN** 信号被接受，实例恢复 `RUNNING`

### Requirement: 等待超时

当 `WaitSignal.deadline` 或 `WaitNodeConfig.default_deadline` 到期，系统 SHALL 以当前 `current_node_id` 与 `sequence` 写入超时 hop，并按 `on_timeout_target_node_id` 路由。

#### Scenario: 超时进入失败终止

- **WHEN** 等待超过 deadline 且无信号到达
- **THEN** 实例路由到超时配置的 target 节点，hop 的 `node_id` 为等待节点而非 `entity_id`

#### Scenario: 超时前收到信号正常继续

- **WHEN** deadline 前收到合法信号
- **THEN** 实例不触发超时路由，按 signal 后 transitions 继续

### Requirement: 信号 payload 合并 snapshot

`DeliverSignal` 在去重成功后，若 `SignalDelivery.payload` 非空且等待节点 `merge_signal_payload` 为 true（默认 true），系统 SHALL 在 `CommitHop(SIGNAL_RECEIVED)` 前将 payload 合并入当前 `EntitySnapshot`。同 `type_url` 时 SHALL 使用 protobuf 字段级 merge；`type_url` 不同时 SHALL 写入 `SignalPayload` wrapper message。合并后 `sequence` SHALL 在 `CommitHop` 时加 1。

#### Scenario: 同类型 payload 字段级合并

- **WHEN** 当前 snapshot payload 为 `Order{status:PENDING}`，信号 payload 为 `Order{approved:true}`
- **THEN** 合并后 snapshot 为 `Order{status:PENDING, approved:true}`，sequence 加 1

#### Scenario: merge_signal_payload 关闭时不合并

- **WHEN** `WaitNodeConfig.merge_signal_payload` 为 false
- **THEN** 信号投递不修改 snapshot payload，sequence 不变

#### Scenario: 合并后 FieldPredicate 可求值

- **WHEN** 信号 payload 合并后含 `approved=true`
- **AND** transition 配置 `field_predicate{field_path:"approved", op:EQ, value:"true"}`
- **THEN** 路由命中该 transition

