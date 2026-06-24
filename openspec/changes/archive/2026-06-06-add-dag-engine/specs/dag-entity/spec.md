# DAG Entity

实体实例线模型：身份、快照、变更与扇出。

- **Protobuf**: `api/dag/v1/entity.proto`, `api/dag/v1/common.proto`
- **接口映射**: `pkg/dag/entity/`

---

## ADDED Requirements

### Requirement: 实体类型二元组强制校验

系统 SHALL 要求所有 `EntitySnapshot` 和 `SpawnSpec` 携带非空 `EntityTypeKey`，包含 `entity_type` 与 `payload_schema_version`。系统 SHALL 通过 `Registry.ResolveType` 校验 `payload` 的 `type_url` 与注册项一致。

#### Scenario: 合法快照写入

- **WHEN** `EntitySnapshot` 的 `type_key` 为 `{entity_type: "order.Order", payload_schema_version: "v1"}` 且 payload type_url 与 registry 匹配
- **THEN** 系统接受该快照，返回 nil

#### Scenario: 缺少 schema version 被拒绝

- **WHEN** `EntitySnapshot` 的 `payload_schema_version` 为空
- **THEN** 系统返回 `ErrInvalidEntityType` 错误

### Requirement: 实体实例线身份

系统 SHALL 以 `EntityRef.entity_id` 作为实例线的全局唯一标识。`EntityInstance` SHALL 维护 `sequence`（单调递增）、`status`、`current_node_id`、`graph_version` 与 `graph_pin_policy`。

#### Scenario: 启动实例

- **WHEN** 调用 `Engine.StartInstance` 并提供 `EntityRef`、`initial_payload` 与 `GraphVersion`
- **THEN** 系统创建 `EntityInstance`，`sequence` 为 0，`status` 为 `RUNNING`，并写入初始 `EntitySnapshot`

#### Scenario: 同 entity_id 重复启动被拒绝

- **WHEN** `entity_id` 已存在活跃实例，再次 `StartInstance` 使用相同 `entity_id`
- **THEN** 系统返回 `ErrInstanceAlreadyExists` 错误

### Requirement: EntityMutation 变更意图

计算单元 SHALL 通过 `EntityMutation` 返回变更意图，支持 `update`、`spawn`、`wait`、`complete`、`fail` 之一。系统 SHALL 在 `CommitHop` 之前不将 mutation 效果写入可见实例状态。

#### Scenario: update 推进序列

- **WHEN** `ComputeUnit.Execute` 返回 `EntityMutation{update: snapshot'}` 且 `CommitHop` 成功
- **THEN** `EntityInstance.sequence` 增加 1，最新快照为 `snapshot'`

#### Scenario: spawn 创建子实例

- **WHEN** mutation 包含 `spawn` 且每个 `SpawnSpec` 均含显式 `GraphVersion`
- **THEN** 系统为每个 `SpawnSpec` 创建独立子实例，写入 `SPAWNED` journal，子实例 `parent` 指向源 `EntityRef`

#### Scenario: spawn 缺少 graph 被拒绝

- **WHEN** `SpawnSpec` 未指定 `GraphVersion` 或 `graph_id`/`version` 为空
- **THEN** 系统返回 `ErrInvalidSpawn` 错误，不创建子实例

### Requirement: 实例内 schema 不静默升级

系统 SHALL NOT 在同一条实例线内自动变更 `payload_schema_version`。schema 升级 MUST 通过显式 MIGRATE 节点或新建实例完成。

#### Scenario: 同实例 type_key 不一致被拒绝

- **WHEN** committed snapshot 的 `type_key` 与实例初始注册类型不兼容（registry 无 compatible_inputs）
- **THEN** 系统返回 `ErrIncompatibleSchema` 错误
