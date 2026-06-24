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

系统 SHALL 为每个 hop 创建 `ExecutionRecord`，幂等键 `idempotency_key = hash(entity_id, node_id, input_sequence)`。同一幂等键至多一条 `COMMITTED` 记录。`attempt` 递增 SHALL 持久化至 store，优先使用单元级 `RetryPolicy`。

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

`ComputeUnitDef` SHALL 支持可选的 `implementation` oneof 字段承载声明式 unit 配置（首期 `HttpUnit`，详见 `dag-units` capability）。Registry 解析 unit 实现 SHALL 遵循以下顺序：

1. 若 `implementation` 非空：基于配置构造声明式适配器（如 HttpUnit 适配器），返回
2. 否则查进程内 Go 注册（`RegisterComputeUnitImpl`），返回
3. 两者皆无：返回错误

#### Scenario: 输入类型不匹配被拒绝

- **WHEN** snapshot `type_key` 与 unit `input_type_key` 不一致
- **THEN** 返回 `ErrTypeMismatch`，不调用 `Execute`

#### Scenario: 输出类型不匹配被拒绝

- **WHEN** `update` snapshot 的 `type_key` 不在 `output_type_keys` 中
- **THEN** 返回 `ErrTypeMismatch`，不写入 journal

#### Scenario: 声明式实现优先于 Go 注册

- **WHEN** `ComputeUnitDef{unit_id: "x", implementation: {http: {...}}}` 已注册
- **AND** 调度对应 COMPUTE 节点请求 `GetComputeUnitImpl("x")`
- **THEN** 返回基于 HttpUnit 配置构造的适配器，不查询 `unitImpls` map

#### Scenario: 无 implementation 回退 Go 注册

- **WHEN** `ComputeUnitDef{unit_id: "lab.echo"}` 无 `implementation`
- **AND** `RegisterComputeUnitImpl("lab.echo", &echoUnit{})` 已执行
- **THEN** `GetComputeUnitImpl("lab.echo")` 返回 `echoUnit` 实例

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

### Requirement: ErrNoTransition 实例失败

当 Scheduler 在 COMPUTE、JOIN 或 advance 路径收到 `ErrNoTransition`，系统 SHALL 将实例 `status` 置为 `FAILED`，`current_node_id` 保持当前节点，并 SHALL 写入带 `failure_reason` 的 journal 条目。系统 SHALL NOT 无限重试该 hop。

#### Scenario: 条件路由全部未命中

- **WHEN** COMPUTE 节点所有 transition 条件均为 false
- **THEN** 实例 `status` 变为 `FAILED`，无新的 `NODE_COMMITTED` 成功推进

#### Scenario: 失败后不再调度

- **WHEN** 实例因 `ErrNoTransition` 变为 `FAILED`
- **THEN** 后续 `RunOnce` 不再对该实例调用 `Execute`

### Requirement: 单元级 RetryPolicy

调度 COMPUTE 节点时，系统 SHALL 优先使用 `ComputeUnitDef.retry_policy.max_attempts`；为 0 时 fallback 至 `Engine` 全局 `DefaultRetryPolicy`。`attempt` 递增 SHALL 持久化至 store。

#### Scenario: 单元级 max_attempts 生效

- **WHEN** unit `retry_policy.max_attempts` 为 5，全局默认为 3
- **AND** Execute 连续失败
- **THEN** 第 5 次失败后才返回错误，attempt 持久化为 5

#### Scenario: 单元级为 0 使用全局默认

- **WHEN** unit `retry_policy.max_attempts` 为 0
- **THEN** 使用 `DefaultRetryPolicy.max_attempts`

### Requirement: 多 spawn journal 记录

当 mutation 包含多个 `SpawnSpec`，`CommitHop` SHALL 在 journal 中记录全部 `spawned_refs`。`spawned_ref`（单数）SHALL 等于 `spawned_refs[0]` 以保持兼容。

#### Scenario: 三个子实例全部记入 journal

- **WHEN** spawn mutation 创建 3 个子实例
- **THEN** journal 条目 `spawned_refs` 长度为 3

### Requirement: Engine 实例排空 DrainInstance

`Engine` SHALL 提供 `DrainInstance(ctx, ref, opts...)`，对指定实例循环调用 `RunOnce` 直至满足终止条件或 `ctx` 取消或 hop 上限耗尽。

终止条件：

- 实例 `status` 为终态（`COMPLETED`、`FAILED`、`COMPENSATED`、`CANCELLED`）→ 返回当前 `EntityInstance`，`error` 为 nil。
- 实例 `status` 为 `WAITING` → 返回当前 `EntityInstance`，`error` 为 nil（阻塞于外部信号，不再空转 `RunOnce`）。

hop 上限 SHALL 由实例所属 `GraphSpec` 节点数乘以可配置系数推导，并受绝对硬顶约束；达上限仍未满足终止条件时 SHALL 返回 `ErrDrainExceeded`。

`ctx` 取消时 SHALL 返回 `ctx.Err()` 与当前可查到的实例快照（若存在）。

#### Scenario: echo 图同步排空至 COMPLETED

- **WHEN** 调用方对 echo 图实例执行 `StartInstance` 后立即 `DrainInstance`
- **THEN** 返回的实例 `status` 为 `COMPLETED`
- **AND** 内部 `RunOnce` 调用次数大于 1 且不超过推导 hop 上限

#### Scenario: WAITING 实例提前返回

- **WHEN** 实例在排空过程中进入 `WAITING` 且未收到信号
- **THEN** `DrainInstance` 返回该实例且 `error` 为 nil
- **AND** 不再继续调用 `RunOnce`

#### Scenario: hop 上限耗尽

- **WHEN** 实例在 hop 上限内既未终态也未进入 `WAITING`（例如路由环或调度 bug）
- **THEN** 返回 `ErrDrainExceeded`
- **AND** `errors.Is(err, ErrDrainExceeded)` 为真

#### Scenario: 上下文取消

- **WHEN** `DrainInstance` 执行期间 `ctx` 被取消
- **THEN** 返回 `context.Canceled` 或 `context.DeadlineExceeded`

