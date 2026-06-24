## MODIFIED Requirements

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
