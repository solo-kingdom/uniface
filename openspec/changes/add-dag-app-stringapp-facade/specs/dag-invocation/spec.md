## ADDED Requirements

### Requirement: StringApp 类型化运行时门面

`pkg/dag/invocation/app` SHALL 提供 `StringApp` 类型 —— 一个预注册 `google.protobuf.StringValue` 实体类型并封装 `EntityTypeKey` 的 `Runtime` 视图，**专门**服务于以 string 为唯一 payload 类型的"输入 payload、执行 graph、返回终态值"同步调用场景。

`StringApp` SHALL 暴露下列方法（不暴露 `TypeKey`，调用方无需关心）：

- `RegisterUnit(unitID string, fn StringFunc) error` —— 注册函数式 string compute unit；`unitID` 唯一；注册失败时 StringApp SHALL 自动 `Close` 底层 Runtime
- `InvokeString(ctx context.Context, graphID, entityID, payload string) (*StringCallResult, error)` —— 类型化调用，隐藏 `TypeKey` 参数
- `LoadGraphID(graphID string) (*dagv1.GraphSpec, error)` —— 透传底层 `Runtime.LoadGraphID`
- `Close() error` —— 透传底层 `Runtime.Close`
- `LoadedGraphs() map[string]string` —— 透传底层 `Runtime.LoadedGraphs`

`NewStringApp(opts ...Option) (*StringApp, error)` SHALL 预注册 StringValue 实体类型（`entityType = "app.String"`，`schemaVersion = "v1"`，可被 Option 覆盖）；预注册失败时 SHALL 关闭底层 Runtime 并返回 error。

`StringApp` SHALL NOT 替换 `app.Runtime` 的既有 API —— 两者并存；调用方可继续直接使用 `app.Runtime` 获得完整能力。`StringApp` SHALL NOT 自动注册任何业务计算单元、graph 或 fixture。

#### Scenario: 构造与注册单元

- **WHEN** 调用 `NewStringApp()` 获得 `StringApp` 后，依次调用 `RegisterUnit("lab.hello", helloFunc)` 与 `RegisterUnit("lab.echo", echoFunc)`
- **THEN** 两次 `RegisterUnit` 均返回 nil
- **AND** StringApp 内部 Runtime 已注册对应 `EntityTypeRegistration`（`type.googleapis.com/google.protobuf.StringValue`）与两条 `ComputeUnitDef`

#### Scenario: 注册失败自动关闭

- **WHEN** `RegisterUnit` 因 `unitID` 重复或底层 Engine 错误返回 error
- **THEN** StringApp SHALL 已在返回 error 前 `Close` 底层 Runtime
- **AND** 后续 `InvokeString` / `Close` 调用 SHALL 返回"`runtime closed`"类错误而非 panic

#### Scenario: InvokeString 隐藏 TypeKey

- **WHEN** 调用 `InvokeString(ctx, "echo", "e1", "hello")` 加载了 graph_id="echo" 的图
- **THEN** 调用方不需要提供 `TypeKey`
- **AND** 内部自动以 StringApp 绑定的 `EntityTypeKey` 构造 `StringCall` 并委托 `Runtime.InvokeString`

#### Scenario: 与 app.Runtime 并存

- **WHEN** 业务方使用既有 `app.Runtime` 直接调用 `InvokeString` 传 `StringCall{TypeKey: ...}` 的代码
- **THEN** 行为与本次变更前保持一致
- **AND** StringApp 不影响 `app.Runtime` 的内部状态

#### Scenario: 不内置业务单元

- **WHEN** 调用 `NewStringApp()` 不调用任何 `RegisterUnit`
- **THEN** 返回的 StringApp 内部 Runtime 不包含 `lab.hello` / `lab.echo` / 任何 `lab.*` 命名空间下的单元
- **AND** 加载任何引用这些单元的 graph 会得到"`unit not registered`"错误

### Requirement: 实体 ID 生成器

`pkg/dag/invocation/app` SHALL 提供 `EntityIDGen` 类型 —— 线程安全的 entity ID 序列生成器。

`Runtime.NewEntityIDGen(prefix string) *EntityIDGen` SHALL 返回以 `prefix` 为命名前缀的生成器；`prefix` 为空时使用默认值 `"dag"`。`EntityIDGen.Next() string` SHALL 返回 `<prefix>-<n>` 格式字符串，其中 `n` 为从 1 开始的全局单调递增计数器（基于 `sync/atomic.Uint64`）。

`EntityIDGen` SHALL 保证：
- 同一实例上多次并发 `Next()` 调用返回的 ID 唯一
- 多次调用返回的 ID 序列号严格单调递增
- 调用方不持有 `EntityIDGen` 的情况下，调用 `Runtime.NewEntityIDGen` 多次 SHALL 返回相互独立的生成器（不共享计数器）

#### Scenario: 计数器单调递增

- **WHEN** 创建 `gen := rt.NewEntityIDGen("http")` 并串行调用 `gen.Next()` 三次
- **THEN** 三次返回依次为 `"http-1"`、`"http-2"`、`"http-3"`

#### Scenario: 并发安全

- **WHEN** 1000 个 goroutine 并发调用同一个 `gen.Next()`
- **THEN** 1000 次返回全部唯一
- **AND** 不出现 race detector 报告

#### Scenario: 默认 prefix

- **WHEN** 调用 `rt.NewEntityIDGen("")`
- **THEN** 返回的 `EntityIDGen` 第一次 `Next()` 返回 `"dag-1"`

#### Scenario: 多生成器独立计数

- **WHEN** `genA := rt.NewEntityIDGen("a")`、`genB := rt.NewEntityIDGen("b")`，分别 `Next()` 一次
- **THEN** `genA` 返回 `"a-1"`，`genB` 返回 `"b-1"`
- **AND** 两者计数器互不影响
