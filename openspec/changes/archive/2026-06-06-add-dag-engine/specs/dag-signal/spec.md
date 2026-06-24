# DAG Signal

等待外部信号：WaitSignal、DeliverSignal 与 WAITING 状态机。

- **Protobuf**: `api/dag/v1/entity.proto`（WaitSignal）, `api/dag/v1/runtime.proto`（SignalDelivery）
- **接口映射**: `pkg/dag/runtime/`

---

## ADDED Requirements

### Requirement: 进入 WAITING 状态

当 `COMPUTE` 返回 `mutation.wait`，或调度 `NodeKind_WAIT` 节点时，系统 SHALL 将实例 `status` 置为 `WAITING`，记录 `current_node_id`，并 SHALL NOT 增加 `sequence`（直至信号解除）。

#### Scenario: wait mutation 进入等待

- **WHEN** `Execute` 返回 `EntityMutation{wait: {signal_name: "manual_approval"}}`
- **THEN** 实例 `status` 为 `WAITING`，Scheduler 不再调度该实例正向 hop

#### Scenario: WAIT 节点配置驱动

- **WHEN** 路由进入 `NodeKind_WAIT` 且 `WaitNodeConfig.signal_name` 为 `manual_approval`
- **THEN** 实例进入 `WAITING`，等待同名信号

### Requirement: DeliverSignal 去重

`Engine.DeliverSignal` SHALL 使用 `delivery_id` 去重，幂等键 `hash(entity_id, signal_name, delivery_id)`。重复投递 SHALL 返回成功且不重复推进。

#### Scenario: 首次信号推进

- **WHEN** 实例处于 `WAITING`，调用 `DeliverSignal{signal_name: "manual_approval", delivery_id: "D1"}`
- **THEN** 写入 `SIGNAL_RECEIVED` journal，实例 `status` 恢复 `RUNNING`，`GraphResolver` 从 wait 节点继续

#### Scenario: 重复 delivery_id 忽略

- **WHEN** 相同 `delivery_id` 再次 `DeliverSignal`
- **THEN** 返回 nil，实例 sequence 不增加，无新 journal

### Requirement: 信号名称校验

`DeliverSignal.signal_name` SHALL 匹配等待中的 `signal_name` 或 `accepted_signals` 列表之一，否则返回 `ErrSignalMismatch`。

#### Scenario: 错误信号被拒绝

- **WHEN** 实例等待 `manual_approval`，收到 `signal_name: "wrong_signal"`
- **THEN** 返回 `ErrSignalMismatch`，实例保持 `WAITING`

### Requirement: 等待超时

当 `WaitSignal.deadline` 或 `WaitNodeConfig.default_deadline` 到期，系统 SHALL 按 `on_timeout_target_node_id` 路由（通常为 `TERMINAL_OUTCOME_FAILURE`）。

#### Scenario: 超时进入失败终止

- **WHEN** 等待超过 deadline 且无信号到达
- **THEN** 实例路由到超时配置的 target 节点

#### Scenario: 超时前收到信号正常继续

- **WHEN** deadline 前收到合法信号
- **THEN** 实例不触发超时路由，按 signal 后 transitions 继续
