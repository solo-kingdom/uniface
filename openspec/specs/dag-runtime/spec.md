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

系统 SHALL 为每个 hop 创建 `ExecutionRecord`，幂等键 `idempotency_key = hash(entity_id, node_id, input_sequence)`。同一幂等键至多一条 `COMMITTED` 记录。`attempt` 递增 SHALL 持久化至 store。

#### Scenario: 重复调度已提交 hop

- **WHEN** Scheduler 调度 hop 时发现同 `idempotency_key` 已有 `COMMITTED` ExecutionRecord
- **THEN** 跳过 Execute，使用 `AdvanceInstanceNode` 或等价操作确保 `current_node_id` 与已提交路由一致

#### Scenario: 崩溃后安全重试

- **WHEN** ExecutionRecord 为 `RUNNING` 且无对应 committed journal
- **THEN** Scheduler 允许重新调用 `Execute`，`attempt` 递增并持久化，直至 `CommitHop` 成功或 attempt 耗尽

### Requirement: CommitHop 原子提交

`CommitHop` SHALL 在同一事务（或等价原子操作）内完成：写入 journal、更新 `EntityInstance`（sequence、snapshot、current_node）、push `SagaState` stack，以及在 `COMPENSATION_COMMITTED` 时 pop saga 栈顶帧。

#### Scenario: 提交前实例不可见推进

- **WHEN** `Execute` 已返回 mutation 但 `CommitHop` 尚未完成
- **THEN** `EntityInstance.sequence` 保持不变

#### Scenario: 提交后 journal 与实例一致

- **WHEN** `CommitHop` 成功
- **THEN** journal 中 `output_snapshot.sequence` 等于实例当前 `sequence`

### Requirement: ComputeUnit 类型契约

调度 `COMPUTE` 节点前，系统 SHALL 校验 `EntitySnapshot.type_key` 等于 `ComputeUnitDef.input_type_key`。`update` mutation 产出的 `type_key` SHALL 属于 `output_type_keys`（非空时）且通过 `compatible_inputs` 校验，或为合法 `spawn`。

#### Scenario: 输入类型不匹配被拒绝

- **WHEN** snapshot `type_key` 与 unit `input_type_key` 不一致
- **THEN** 返回 `ErrTypeMismatch`，不调用 `Execute`

#### Scenario: 输出类型不匹配被拒绝

- **WHEN** `update` snapshot 的 `type_key` 不在 `output_type_keys` 中
- **THEN** 返回 `ErrTypeMismatch`，不写入 journal

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

### Requirement: 同实例 hop 串行

`Engine` SHALL 对同一 `entity_id` 的 `RunOnce` hop 处理与 `DeliverSignal` 互斥串行，防止并发双次 `Execute`。

#### Scenario: 并发 RunOnce 不双次 Execute

- **WHEN** 两个 goroutine 同时对同一 `entity_id` 调用 `RunOnce`
- **THEN** 同一 hop 的 `Execute` 至多产生一条 `COMMITTED` journal

