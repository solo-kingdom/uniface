## ADDED Requirements

### Requirement: 请求式 DAG Invoker

系统 SHALL 提供可复用的 DAG Invoker 抽象，用于对单个实例执行 `StartInstance`、`DrainInstance` 与终态 snapshot 读取。Invoker SHALL 组合现有 `dag.Engine` 与 `dag.LineStore`，不得改变 `Engine` 的既有生命周期接口语义。

#### Scenario: 成功调用并返回终态结果

- **WHEN** 调用方传入有效 `EntityRef`、`EntityTypeKey`、初始 payload 与 `GraphVersion`
- **AND** 目标图可在 `DrainInstance` 中排空至 `COMPLETED`
- **THEN** Invoker 返回包含 `EntityInstance` 与终态 `EntitySnapshot` 的结果
- **AND** 返回实例状态为 `COMPLETED`

#### Scenario: 失败终态作为结果返回

- **WHEN** 目标图排空至 `FAILED`、`COMPENSATED` 或 `CANCELLED`
- **THEN** Invoker 返回当前 `EntityInstance` 与可读取到的 `EntitySnapshot`
- **AND** error 为 nil，终态到业务错误或协议错误的映射由调用方决定

#### Scenario: WAITING 实例提前返回

- **WHEN** 目标实例在排空过程中进入 `WAITING`
- **THEN** Invoker 返回状态为 `WAITING` 的 `EntityInstance`
- **AND** 不继续空转调度该实例

#### Scenario: Drain 错误透传

- **WHEN** `DrainInstance` 因上下文取消、hop 上限耗尽或底层存储错误返回 error
- **THEN** Invoker SHALL 透传该 error
- **AND** 在可读取当前实例或 snapshot 时返回部分结果

### Requirement: 标准内存 Runtime 装配

系统 SHALL 提供标准内存 Runtime 装配辅助，用于创建并持有 `memory.Registry`、`memory.LineStore`、`memory.Engine` 与 Invoker。装配辅助 SHALL 支持注册实体类型、图、计算单元定义、Go 计算单元实现、补偿器与 `HttpClientResolver`。

#### Scenario: 构造可运行内存 Runtime

- **WHEN** 调用方通过装配辅助注册实体类型、计算单元定义、计算单元实现与图
- **THEN** 返回的 Runtime 能够启动并排空匹配图实例
- **AND** 调用方无需直接创建 `memory.Registry`、`memory.LineStore` 与 `memory.Engine`

#### Scenario: 不内置 lab 语义

- **WHEN** 调用方创建标准内存 Runtime 且未注册任何实体类型或计算单元
- **THEN** Runtime SHALL NOT 自动注册 `lab.Generic`、`lab.echo`、`lab.hello` 或任何 lab fixture

#### Scenario: 注入 HttpClientResolver

- **WHEN** 调用方在构造 Runtime 时注入 `dag.HttpClientResolver`
- **THEN** Runtime 内部 Engine SHALL 将 resolver 传递给 Registry
- **AND** 声明式 HttpUnit 在执行时可使用该 resolver 解析服务实例

### Requirement: Payload Codec 辅助

系统 SHALL 提供 payload codec 辅助，用于在 protobuf message 与 `anypb.Any`/`EntitySnapshot.Payload` 之间转换。Codec SHALL 作为 Invoker 外围辅助存在，Invoker 核心 API SHALL 仍支持直接传入和返回 `*anypb.Any`。

#### Scenario: protobuf message 编码为 Any

- **WHEN** 调用方传入有效 protobuf message
- **THEN** Codec 返回包含对应 type URL 与序列化字节的 `anypb.Any`

#### Scenario: snapshot payload 解码为目标 message

- **WHEN** 调用方传入包含 payload 的 `EntitySnapshot` 与目标 protobuf message
- **THEN** Codec SHALL 将 snapshot payload 解码到目标 message

#### Scenario: payload 缺失返回错误

- **WHEN** 调用方尝试解码 nil snapshot 或 nil payload
- **THEN** Codec SHALL 返回 error，而不是 panic

### Requirement: 声明式 Graph Loader

系统 SHALL 提供声明式 Graph Loader，将外部 YAML 或 JSON 文档解析为 `GraphSpec` 与可选内联 `ComputeUnitDef`。Loader SHALL 覆盖已有 DAG proto 能表达的图结构，不引入新的业务语义。

#### Scenario: 加载 compute 到 terminal 图

- **WHEN** 文档声明 graph id、version、entry、compute 节点、terminal 节点与 transition
- **THEN** Loader 返回等价的 `GraphSpec`
- **AND** 返回的 `GraphSpec` 可通过 `graph.ValidateGraphSpec`

#### Scenario: 加载内联 HttpUnit 定义

- **WHEN** compute 节点声明内联 `unit.http` 配置
- **THEN** Loader 返回对应 `ComputeUnitDef`
- **AND** 该定义的 `implementation` 为 HttpUnit 配置

#### Scenario: 不绑定 fixture 文件定位

- **WHEN** 调用方希望按 graph id 从目录加载文件
- **THEN** 公共 Loader SHALL NOT 强制约定 fixture 目录或文件名
- **AND** 文件定位由调用方或上层应用负责
