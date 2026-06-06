## MODIFIED Requirements

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
