## ADDED Requirements

### Requirement: 请求式 DAG 应用封装

系统 SHALL 在现有 Invoker、标准内存 Runtime、Loader 与 Codec 之上提供轻量应用封装，用于常见“输入 payload、执行 graph、返回终态结果”的请求式 DAG 调用场景。该封装 SHALL 组合现有底层组件，不得替换或改变 `dag.Engine`、`invocation.Invoker`、`invocation/memory.Runtime` 的既有语义。

#### Scenario: 构造独立请求式 Runtime

- **WHEN** 调用方创建轻量应用封装实例
- **THEN** 系统创建或持有一个独立的 DAG Runtime、Invoker 与注册上下文
- **AND** 不使用包级全局 Runtime 或全局注册表

#### Scenario: 底层 Invoker 仍可直接使用

- **WHEN** 调用方继续直接使用 `invocation.Invoker.Invoke`
- **THEN** 既有 `InvokeRequest`、`InvokeResult`、终态返回和错误透传语义保持不变

#### Scenario: 不内置业务语义

- **WHEN** 调用方创建轻量应用封装但未显式注册实体类型、计算单元或图
- **THEN** 系统 SHALL NOT 自动注册 `lab.Generic`、`lab.hello`、`lab.echo`、固定 graph ID 或固定 fixture 路径

### Requirement: 类型化请求调用辅助

系统 SHALL 为请求式 DAG 调用提供类型化 payload 便捷辅助。辅助 SHALL 复用现有 protobuf `Any` codec 能力，并至少覆盖 protobuf message 与 string payload 的编码、解码和终态 payload 读取。

#### Scenario: string payload 请求调用

- **WHEN** 调用方传入 graph ID、entity ID 与 string payload 发起请求式调用
- **THEN** 系统将 string 编码为 `google.protobuf.StringValue` payload 并调用底层 Invoker
- **AND** 当实例排空至 `COMPLETED` 且终态 snapshot 可解码为 string 时返回该 string

#### Scenario: protobuf message 请求调用

- **WHEN** 调用方传入 protobuf message 作为初始 payload 并提供目标输出 message
- **THEN** 系统将输入编码为 `anypb.Any`
- **AND** 调用完成后将终态 snapshot payload 解码到目标输出 message

#### Scenario: 失败终态保留结果

- **WHEN** DAG 排空至 `FAILED`、`COMPENSATED` 或 `CANCELLED`
- **THEN** 类型化调用 SHALL 返回包含实例状态与可读取 snapshot 的结果信息
- **AND** 不得把失败终态伪装成 `COMPLETED`

#### Scenario: WAITING 状态不被隐藏

- **WHEN** DAG 排空后进入 `WAITING`
- **THEN** 类型化调用 SHALL 向调用方暴露 `WAITING` 状态
- **AND** 不得继续同步等待外部 signal

### Requirement: 简化注册与图加载辅助

系统 SHALL 提供上层注册与图加载辅助，降低请求式调用场景中注册实体类型、Go ComputeUnit、ComputeUnitDef 与 graph 文件的样板代码。公共 Loader 的“只解析文档、不绑定文件定位”契约 SHALL 保持不变。

#### Scenario: 注册 string 实体类型

- **WHEN** 调用方通过轻量封装注册 string 实体类型并提供 entity type 与 schema version
- **THEN** 系统注册对应 `EntityTypeRegistration`
- **AND** payload type URL 为 `type.googleapis.com/google.protobuf.StringValue`

#### Scenario: 注册函数式 string compute unit

- **WHEN** 调用方注册一个以 string 为输入并产出 string 的函数式 compute unit
- **THEN** 系统注册匹配的 `ComputeUnitDef`
- **AND** 系统将该函数适配为 `dag.ComputeUnit` 实现

#### Scenario: 按文件加载图

- **WHEN** 调用方通过轻量封装加载 YAML 或 JSON graph 文件
- **THEN** 系统使用公共 Loader 解析文档
- **AND** 校验并注册返回的 `GraphSpec` 与内联 `ComputeUnitDef`

#### Scenario: 按 graph ID 从目录加载图

- **WHEN** 调用方配置 graph 目录并按 graph ID 加载图
- **THEN** 系统按上层约定定位图文件并完成加载注册
- **AND** 公共 Loader 包本身 SHALL NOT 新增固定目录或文件名约定
