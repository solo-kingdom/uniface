## ADDED Requirements

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

## MODIFIED Requirements

### Requirement: DeliverSignal 去重

`Engine.DeliverSignal` SHALL 使用 `delivery_id` 去重，幂等键 `hash(entity_id, signal_name, delivery_id)`。重复投递 SHALL 返回成功且不重复推进。首次成功投递 SHALL 合并 payload（若配置允许）、写入 `SIGNAL_RECEIVED` journal，并调用 `GraphResolver` 从 wait 节点继续。

#### Scenario: 首次信号推进

- **WHEN** 实例处于 `WAITING`，调用 `DeliverSignal{signal_name: "manual_approval", delivery_id: "D1"}`
- **THEN** 写入 `SIGNAL_RECEIVED` journal，实例 `status` 恢复 `RUNNING`，`GraphResolver` 从 wait 节点继续

#### Scenario: 重复 delivery_id 忽略

- **WHEN** 相同 `delivery_id` 再次 `DeliverSignal`
- **THEN** 返回 nil，实例 sequence 不增加，无新 journal
