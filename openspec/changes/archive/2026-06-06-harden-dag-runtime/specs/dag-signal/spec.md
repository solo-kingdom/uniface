## MODIFIED Requirements

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
