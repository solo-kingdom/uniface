# dag-graph Specification

## Purpose
TBD - created by archiving change add-dag-engine. Update Purpose after archive.
## Requirements
### Requirement: GraphSpec 节点种类

`GraphSpec` SHALL 包含 `entry_node_id` 与 `nodes` 映射。每个 `NodeDef` SHALL 声明 `NodeKind`：`COMPUTE`、`WAIT`、`JOIN` 或 `TERMINAL`。

#### Scenario: TERMINAL 节点无出边

- **WHEN** `NodeDef.kind` 为 `TERMINAL`
- **THEN** 该节点的 `transitions` 必须为空，且 MUST 设置 `terminal_outcome`

#### Scenario: 禁止空 target_node_id

- **WHEN** 任意 `Transition.target_node_id` 为空字符串
- **THEN** 图校验失败，返回 `ErrInvalidGraph`

### Requirement: 显式终止节点

实例流转结束 MUST 通过到达 `TERMINAL` 节点完成。系统 SHALL NOT 使用空 target 表示结束。

#### Scenario: 到达 SUCCESS 终止

- **WHEN** 实例路由到 `TERMINAL` 节点且 `terminal_outcome` 为 `SUCCESS`
- **THEN** 实例 `status` 变为 `COMPLETED`

#### Scenario: 到达 FAILURE 终止

- **WHEN** 实例路由到 `TERMINAL` 节点且 `terminal_outcome` 为 `FAILURE`
- **THEN** 实例 `status` 变为 `FAILED`

### Requirement: GraphResolver 条件路由

`GraphResolver` SHALL 根据当前 `EntitySnapshot` 与 `NodeDef.transitions` 按 `priority` 降序评估 `Condition`，返回唯一 `target_node_id`。`Condition` SHALL 支持 `FieldPredicate`、`signal_predicate` 与 `always`。

#### Scenario: FieldPredicate 命中高优先级边

- **WHEN** 节点有两条 transition，`priority` 10 的条件 `amount > 10000` 为真，`priority` 0 为 `always`
- **THEN** Resolver 返回 `priority` 10 的 `target_node_id`

#### Scenario: FieldPredicate 从 Any 解码求值

- **WHEN** `FieldPredicate.field_path` 为 `amount`，snapshot payload 可解码为含 `amount` 字段的 protobuf
- **THEN** 系统按 `Op` 与 `value` 正确求值

#### Scenario: 无命中 transition 报错

- **WHEN** 所有 `Condition` 均为 false 且无默认边
- **THEN** Resolver 返回 `ErrNoTransition`

### Requirement: JOIN 节点屏障

`NodeKind_JOIN` 的节点 SHALL 配置 `JoinSpec`，包含静态 `barriers` 和/或 `dynamic_barriers` 与 `JoinPolicy`。全部屏障满足前 SHALL NOT 推进实例。

#### Scenario: JOIN_ALL_SUCCESS 等待子实例

- **WHEN** join 节点配置两个 barrier，分别指向 `payment-1` 与 `payment-2`，policy 为 `JOIN_ALL_SUCCESS`
- **AND** 仅 `payment-1` 为 `COMPLETED`
- **THEN** 父实例停留在 join 节点，不写入 `JOIN_COMMITTED`

#### Scenario: 全部完成后推进

- **WHEN** 所有 barrier 对应子实例均为 `COMPLETED`
- **THEN** 系统写入 `JOIN_COMMITTED` journal，`sequence` 加 1，并按 transitions 继续路由

### Requirement: 图版本绑定策略

`EntityInstance` SHALL 记录 `GraphPinPolicy`：`GRAPH_PIN_ON_START`、`GRAPH_PIN_ON_NODE` 或 `GRAPH_PIN_EXPLICIT`（预留）。`GRAPH_PIN_ON_START` 时实例 SHALL 在整个生命周期使用启动时锁定的 `GraphVersion`。`GRAPH_PIN_ON_NODE` 时每个 hop SHALL 通过 `Registry.ResolveGraphForInstance` 解析该 `graph_id` 的最新注册版本。

#### Scenario: PIN_ON_START 不随热更新变更

- **WHEN** 实例以 `GRAPH_PIN_ON_START` 创建，绑定 `order-fulfillment/v1`
- **AND** registry 发布 `v2`
- **THEN** 该实例解析图时仍使用 `v1`

#### Scenario: PIN_ON_NODE 使用最新图

- **WHEN** 实例以 `GRAPH_PIN_ON_NODE` 创建，初始绑定 `order-fulfillment/v1`
- **AND** registry 随后注册 `order-fulfillment/v2`
- **THEN** 下一 hop 调度时使用 `v2` 的 `GraphSpec`

### Requirement: Condition 支持 signal_predicate

`Condition` SHALL 在 `oneof kind` 中支持 `signal_predicate`。`SignalPredicate` SHALL 包含可选 `signal_name` 与 `payload_predicate`（`FieldPredicate`）。`GraphResolver` 在 `DeliverSignal` 触发的路由中 SHALL 评估 `signal_predicate`；在 COMPUTE hop 正常路由中 SHALL 评估 `field_predicate` 与 `always`。

#### Scenario: 审批通过信号路由到成功分支

- **WHEN** wait 节点收到 `signal_name: "approval"` 且 payload 合并后 `approved=true`
- **AND** transition 配置 `signal_predicate{signal_name:"approval", payload_predicate:{field_path:"approved", op:EQ, value:"true"}}`
- **THEN** Resolver 返回该 transition 的 `target_node_id`

#### Scenario: signal_name 不匹配跳过边

- **WHEN** 收到 `signal_name: "rejection"` 但 transition 要求 `signal_name: "approval"`
- **THEN** 该 transition 不匹配，继续评估其他边

### Requirement: JoinSpec 动态屏障

`JoinSpec` SHALL 支持 `repeated DynamicJoinBarrier dynamic_barriers`。每个 `DynamicJoinBarrier` SHALL 包含 `correlation_prefix`、`expected_count`（0 表示以最近一次 `SPAWNED` journal 的子实例数为期望）与 `JoinPolicy`。全部 dynamic barrier 满足前 SHALL NOT 推进父实例。

#### Scenario: 动态 N 个子实例全部完成

- **WHEN** spawn hop 创建 3 个子实例，`correlation_id` 分别为 `pay-1`、`pay-2`、`pay-3`
- **AND** join 节点配置 `dynamic_barriers[{correlation_prefix:"pay-", expected_count:0, policy:JOIN_ALL_SUCCESS}]`
- **AND** 仅 2 个子实例为 `COMPLETED`
- **THEN** 父实例停留在 join 节点

#### Scenario: 全部完成后推进

- **WHEN** 3 个子实例均为 `COMPLETED`
- **THEN** 系统写入 `JOIN_COMMITTED` journal 并按 transitions 继续路由

#### Scenario: JOIN_ANY_SUCCESS 部分完成

- **WHEN** dynamic barrier policy 为 `JOIN_ANY_SUCCESS` 且至少 1 个子实例 `COMPLETED`
- **THEN** join 屏障满足，父实例推进

### Requirement: 图静态校验扩展

`ValidateGraphSpec` SHALL 额外校验：`COMPUTE` 节点 `unit_id` 非空；`WAIT` 节点 `wait_config` 中 `signal_name` 或 `accepted_signals` 至少一项非空；非 TERMINAL/WAIT/JOIN 节点在存在条件路由时 MUST 至少有一条 `always=true` 兜底 transition；图 MUST 通过环检测（WAIT 超时边不计入主 transition 环）。

#### Scenario: COMPUTE 缺少 unit_id 被拒绝

- **WHEN** `NodeDef.kind` 为 `COMPUTE` 且 `unit_id` 为空
- **THEN** `ValidateGraphSpec` 返回 `ErrInvalidGraph`

#### Scenario: 条件路由无兜底边被拒绝

- **WHEN** COMPUTE 节点所有 transition 均为 `FieldPredicate` 且无 `always=true`
- **THEN** `ValidateGraphSpec` 返回 `ErrInvalidGraph`

#### Scenario: 检测到环被拒绝

- **WHEN** 图存在仅由 COMPUTE transition 构成的环且无 TERMINAL 出口
- **THEN** `ValidateGraphSpec` 返回 `ErrInvalidGraph`

