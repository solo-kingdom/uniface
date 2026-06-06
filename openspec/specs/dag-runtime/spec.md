# dag-runtime Specification

## Purpose
TBD - created by archiving change add-dag-engine. Update Purpose after archive.
## Requirements
### Requirement: Engine 实例生命周期

`Engine` SHALL 提供 `StartInstance`、`GetInstance`、`CancelInstance`。`CancelInstance` SHALL 将实例置为 `CANCELLED` 并拒绝后续 hop。

#### Scenario: 查询实例

- **WHEN** 调用 `GetInstance` 传入有效 `EntityRef`
- **THEN** 返回当前 `EntityInstance` 与可查询 journal

#### Scenario: 取消后拒绝执行

- **WHEN** 实例 `status` 为 `CANCELLED`
- **THEN** Scheduler 不再调度该实例的新 hop

### Requirement: ExecutionRecord 幂等键

系统 SHALL 为每个 hop 创建 `ExecutionRecord`，幂等键 `idempotency_key = hash(entity_id, node_id, input_sequence)`。同一幂等键至多一条 `COMMITTED` 记录。

#### Scenario: 重复调度已提交 hop

- **WHEN** Scheduler 调度 hop 时发现同 `idempotency_key` 已有 `COMMITTED` ExecutionRecord
- **THEN** 跳过 Execute，直接使用已提交结果推进路由

#### Scenario: 崩溃后安全重试

- **WHEN** ExecutionRecord 为 `RUNNING` 且无对应 `NODE_COMMITTED` journal
- **THEN** Scheduler 允许重新调用 `Execute`，直至 `CommitHop` 成功或 attempt 耗尽

### Requirement: CommitHop 原子提交

`CommitHop` SHALL 在同一事务（或等价原子操作）内完成：写入 `LineJournalEntry{KIND_NODE_COMMITTED}`、更新 `EntityInstance`（sequence、snapshot、current_node）、必要时 push `SagaState` stack。

#### Scenario: 提交前实例不可见推进

- **WHEN** `Execute` 已返回 mutation 但 `CommitHop` 尚未完成
- **THEN** `EntityInstance.sequence` 保持不变

#### Scenario: 提交后 journal 与实例一致

- **WHEN** `CommitHop` 成功
- **THEN** journal 中 `output_snapshot.sequence` 等于实例当前 `sequence`

### Requirement: ComputeUnit 类型契约

调度 `COMPUTE` 节点前，系统 SHALL 校验 `EntitySnapshot.type_key` 等于 `ComputeUnitDef.input_type_key`。mutation 产出的 `type_key` SHALL 属于 `output_type_keys` 或为合法 `spawn`。

#### Scenario: 输入类型不匹配被拒绝

- **WHEN** snapshot `type_key` 与 unit `input_type_key` 不一致
- **THEN** 返回 `ErrTypeMismatch`，不调用 `Execute`

### Requirement: SideEffectClass 一期约束

系统 SHALL 支持 `SIDE_EFFECT_NONE` 与 `SIDE_EFFECT_IDEMPOTENT`。`SIDE_EFFECT_EXTERNAL` SHALL 返回 `ErrUnsupportedSideEffect`。

#### Scenario: IDEMPOTENT 要求业务幂等

- **WHEN** unit 标记为 `SIDE_EFFECT_IDEMPOTENT` 且 Execute 因崩溃被调用两次
- **THEN** 框架保证至多一次 `COMMITTED`；业务外部副作用须通过幂等键去重

### Requirement: 黄金路径集成测试

内存 MVP SHALL 提供集成测试，覆盖：Start → COMPUTE → WAIT → COMPUTE（含崩溃重试）→ Spawn → JOIN → TERMINAL SUCCESS。

#### Scenario: charge 崩溃重试无重复提交

- **WHEN** 集成测试在 charge 节点第一次 Execute 后模拟崩溃
- **AND** 重启 Scheduler 后重试
- **THEN** 仅存在一条 charge 的 `NODE_COMMITTED` journal，sequence 连续无跳号

