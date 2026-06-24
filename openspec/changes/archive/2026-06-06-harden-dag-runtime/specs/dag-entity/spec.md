## MODIFIED Requirements

### Requirement: EntityMutation 变更意图

计算单元 SHALL 通过 `EntityMutation` 返回变更意图，支持 `update`、`spawn`、`wait`、`complete`、`fail` 之一。系统 SHALL 在 `CommitHop` 之前不将 mutation 效果写入可见实例状态。

#### Scenario: update 推进序列

- **WHEN** `ComputeUnit.Execute` 返回 `EntityMutation{update: snapshot'}` 且 `CommitHop` 成功
- **THEN** `EntityInstance.sequence` 增加 1，最新快照为 `snapshot'`

#### Scenario: spawn 创建子实例

- **WHEN** mutation 包含 `spawn` 且每个 `SpawnSpec` 均含显式 `GraphVersion`
- **THEN** 系统为每个 `SpawnSpec` 创建独立子实例，写入 `JOURNAL_KIND_SPAWNED` journal，子实例 `parent` 指向源 `EntityRef`

#### Scenario: spawn 缺少 graph 被拒绝

- **WHEN** `SpawnSpec` 未指定 `GraphVersion` 或 `graph_id`/`version` 为空
- **THEN** 系统返回 `ErrInvalidSpawn` 错误，不创建子实例

### Requirement: 实例内 schema 不静默升级

系统 SHALL NOT 在同一条实例线内自动变更 `payload_schema_version`。committed snapshot 的 `type_key` 与实例初始类型不兼容时 SHALL 返回 `ErrIncompatibleSchema`。兼容类型 MUST 在 `EntityTypeRegistration.compatible_inputs` 中显式声明。

#### Scenario: 同实例 type_key 不一致被拒绝

- **WHEN** committed snapshot 的 `type_key` 与实例初始注册类型不兼容（registry 无 compatible_inputs 匹配）
- **THEN** 系统返回 `ErrIncompatibleSchema` 错误
