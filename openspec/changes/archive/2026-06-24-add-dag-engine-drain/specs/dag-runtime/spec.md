## ADDED Requirements

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
