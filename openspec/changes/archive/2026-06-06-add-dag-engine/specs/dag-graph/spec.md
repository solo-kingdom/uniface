# DAG Graph

数据驱动图规格、节点种类与路由解析。

- **Protobuf**: `api/dag/v1/graph.proto`
- **接口映射**: `pkg/dag/graph/`

---

## ADDED Requirements

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

`GraphResolver` SHALL 根据当前 `EntitySnapshot` 与 `NodeDef.transitions` 按 `priority` 降序评估 `Condition`，返回唯一 `target_node_id`。`Condition` 一期 SHALL 支持 `FieldPredicate` 与 `always`。

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

`NodeKind_JOIN` 的节点 SHALL 配置 `JoinSpec`，包含 `barriers` 与 `JoinPolicy`。全部屏障满足前 SHALL NOT 推进实例。

#### Scenario: JOIN_ALL_SUCCESS 等待子实例

- **WHEN** join 节点配置两个 barrier，分别指向 `payment-1` 与 `payment-2`，policy 为 `JOIN_ALL_SUCCESS`
- **AND** 仅 `payment-1` 为 `COMPLETED`
- **THEN** 父实例停留在 join 节点，不写入 `JOIN_COMMITTED`

#### Scenario: 全部完成后推进

- **WHEN** 所有 barrier 对应子实例均为 `COMPLETED`
- **THEN** 系统写入 `JOIN_COMMITTED` journal，`sequence` 加 1，并按 transitions 继续路由

### Requirement: 图版本绑定策略

`EntityInstance` SHALL 记录 `GraphPinPolicy`：`GRAPH_PIN_ON_START`、`GRAPH_PIN_ON_NODE` 或 `GRAPH_PIN_EXPLICIT`（预留）。`GRAPH_PIN_ON_START` 时实例 SHALL 在整个生命周期使用启动时锁定的 `GraphVersion`。

#### Scenario: PIN_ON_START 不随热更新变更

- **WHEN** 实例以 `GRAPH_PIN_ON_START` 创建，绑定 `order-fulfillment/v1`
- **AND** registry 发布 `v2`
- **THEN** 该实例解析图时仍使用 `v1`
