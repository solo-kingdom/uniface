## Context

`pkg/dag` 当前已经具备底层执行能力：`Engine` 管理实例生命周期，`Registry` 管理类型、图和计算单元，`LineStore` 管理实例线存储，`DrainInstance` 支持同步排空到终态或 `WAITING`。这些能力适合实现引擎，但业务入口常见需求更接近“把一次请求包装成 DAG 实例，运行到可返回状态，再读取结果”。

`lab-dag-http` 正好暴露了这层缺口。为了一个 `POST /echo`，它需要自行创建 memory runtime、注册实体类型和 unit、加载图、启动实例、排空实例、读取 snapshot、解码 payload、映射终态。类似代码如果出现在多个业务入口，会让 DAG 作为基础设施抽象的复用门槛过高。

## Goals / Non-Goals

**Goals:**

- 在 `pkg/dag` 下提供请求式 DAG 调用抽象，封装 `StartInstance -> DrainInstance -> GetSnapshot` 的常见路径。
- 提供标准内存运行时装配辅助，减少调用方手动组合 `memory.Registry`、`memory.LineStore`、`memory.Engine` 的样板。
- 将 payload 编解码抽象成可复用边界，支持 protobuf `Any` 与业务 payload 的双向转换。
- 将声明式图加载能力从 lab 示例中沉淀到公共层，但不绑定 lab fixture 布局。
- 修正 lab 规格：`lab-dag-http` 与 `lab-dag` 完全隔离，只共享根模块公共抽象。

**Non-Goals:**

- 不改变现有 `dag.Engine`、`dag.Registry`、`dag.LineStore` 接口语义。
- 不把 `lab/internal/dag` 变成公共依赖。
- 不把 lab 的 `OpRecorder`、dashboard、CLI 命令或 fixture 目录约定迁入根模块。
- 不在首期引入持久化 runtime、分布式 scheduler 或异步 worker 托管能力。

## Decisions

### Decision 1: 新增请求式 Invoker，而不是抬高 Engine 接口

新增 `dagruntime.Invoker` 或等价子包，负责一次性调用：

```go
type InvokeRequest struct {
    Ref            *dagv1.EntityRef
    TypeKey        *dagv1.EntityTypeKey
    InitialPayload *anypb.Any
    GraphVersion   *dagv1.GraphVersion
    GraphPinPolicy dagv1.GraphPinPolicy
    Options        []dag.Option
}

type InvokeResult struct {
    Instance *dagv1.EntityInstance
    Snapshot *dagv1.EntitySnapshot
}
```

Invoker 组合已有 `Engine` 与 `LineStore`，不替代底层接口。这样保留 Engine 面向调度和生命周期的清晰边界，同时给业务入口一个更合适的调用面。

备选方案是直接给 `Engine` 增加 `Invoke`。该方案会让底层引擎承担 snapshot 读取和调用结果语义，扩大接口职责；首期不采用。

### Decision 2: Runtime Builder 只处理通用装配，不携带业务语义

提供 memory runtime 装配辅助，例如 `memoryruntime.New(...)`，内部创建 `Registry`、`LineStore`、`Engine`，并支持注册：

- `EntityTypeRegistration`
- `GraphSpec`
- `ComputeUnitDef`
- Go `ComputeUnit`/`Compensator`
- `HttpClientResolver`

Builder 不内置 `lab.Generic`、`StringValue`、`lab.echo` 等示例语义。这些仍由 lab 或业务方显式声明。

备选方案是抽取 `lab/internal/dag.Runtime`。该方案会把 lab 的 fixture、状态记录、mock HTTP 和 CLI 需求混入公共层，不采用。

### Decision 3: Payload Codec 独立于 Invoker

Invoker 的核心输入输出使用 `*anypb.Any` 和 snapshot，避免绑定具体业务类型。另提供可选 codec 辅助：

- 将 protobuf message 编码为 `Any`
- 将 snapshot payload 解码为指定 protobuf message
- 提供 StringValue 等常见 wrapper 的轻量 helper

这样 HTTP、CLI、测试和业务服务可以共享编解码逻辑，但仍能在需要时直接处理 `Any`。

### Decision 4: 声明式图加载进入公共辅助层

将 YAML/JSON 到 `GraphSpec` 与内联 `ComputeUnitDef` 的解析能力沉淀到公共包，至少覆盖 lab 现有声明式图语法中已经属于 DAG 能力的部分：compute、terminal、wait、join、transition condition、retry_policy、inline HttpUnit。

Loader 不处理“从哪个 fixture 目录按 graphID 找文件”的 lab 约定；文件定位由调用方负责。

### Decision 5: lab-dag-http 保持独立验证入口

`lab-dag-http` 不复用 `lab/internal/dag.Runtime`。后续实现时，它应使用新的公共 DAG runtime/invoker/loader/codec 抽象进行自身装配，保持与 `lab-dag` 的 runtime、fixtures 和 API 独立。

## Risks / Trade-offs

- [Risk] 新增上层抽象可能过早固化 API。→ 通过子包承载首期 API，并保持底层 Engine 不变；优先暴露小接口和结构体。
- [Risk] Graph loader 范围过大，容易变成第二套 DSL。→ 只沉淀已有 DAG proto 的声明式映射，不增加 proto 没有表达的新语义。
- [Risk] Codec 泛型设计可能复杂。→ 首期以 protobuf `Any` 和 `proto.Message` helper 为主，避免强行覆盖非 protobuf payload。
- [Risk] lab 重构可能误伤验证台行为。→ 通过现有 lab 测试和 `lab-dag-http` golden path 测试锁定行为。

## Migration Plan

1. 先新增公共 runtime/invoker/codec/loader 能力与测试，不改动 lab 行为。
2. 用新公共抽象重写 `lab/internal/daghttp` 的本地胶水，保持 HTTP API 和响应行为不变。
3. 视重复情况再评估 `lab/internal/dag` 是否部分采用公共 loader/runtime builder，但不作为本变更必须完成项。
4. 若新公共 API 不满足 `lab-dag-http`，优先调整新 API，而不是在 lab 里继续堆叠适配层。

## Open Questions

- 公共包命名采用 `pkg/dag/runtime`、`pkg/dag/invocation`，还是 `pkg/dag/memory/runtime` 分层，需要实现前最终确认。
- Graph loader 首期是否支持 JSON 输入，还是只支持 YAML 并保留结构体层以便未来扩展。
