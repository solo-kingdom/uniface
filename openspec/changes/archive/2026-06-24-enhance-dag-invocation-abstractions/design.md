## Context

现有 `pkg/dag/invocation` 已经把请求式执行路径收敛为 `Invoker.Invoke`，`invocation/memory.Runtime` 也封装了内存 Registry、LineStore、Engine 与 Invoker 的创建。但调用方仍然需要显式处理：

- entity type 与 payload type URL 注册
- `ComputeUnitDef` 与 Go 实现的配对注册
- YAML/JSON 图文件定位、加载、校验与注册
- `anypb.Any` payload 编解码
- `InvokeRequest` 中 `EntityRef`、`EntityTypeKey`、`GraphVersion`、pin policy 的组装
- 终态 payload 与实例状态到业务协议的映射

这些步骤对复杂 DAG 是必要的控制面，但对“HTTP 请求同步触发一个 DAG 并返回终态 payload”这类场景显得过重。`lab-dag-http` 当前正好暴露了这个摩擦点。

## Goals / Non-Goals

**Goals:**

- 在现有 `dag-invocation` 能力上新增一层轻量请求式调用封装，服务常见同步调用场景。
- 让业务方可以用少量配置注册实体类型、函数式 compute unit、图目录，并以类型化方式调用 graph。
- 保持底层 `dag.Engine`、`invocation.Invoker`、`memory.Runtime`、`loader`、`codec` API 稳定可用。
- 让 `lab-dag-http` 使用该封装作为验证样例，降低简单示例中的样板代码。

**Non-Goals:**

- 不改变 DAG 调度、幂等、Signal、Saga、HttpUnit 等底层能力。
- 不为所有业务 payload 设计通用序列化框架；首期复用 protobuf `Any`，并提供 string/protobuf message 便捷路径。
- 不在公共封装中内置 lab graph、lab entity type 或 fixture 路径。
- 不把 WAITING/Signal 的长生命周期交互隐藏成同步请求成功。

## Decisions

### 1. 在 invocation 外围新增应用级 facade

新增上层 facade（命名可在实现时确定，如 `invocation/app` 或 `invocation/simple`），组合 `invocation/memory.Runtime`、loader 与 codec。它负责常见装配和请求式调用，但不直接实现调度逻辑。

可选方案是继续向 `invocation.Invoker` 增加便捷方法。该方案会让核心 Invoker 同时承担底层请求抽象和应用装配职责，边界变模糊，因此不优先采用。

### 2. 保持显式配置，不使用全局注册表

facade 通过实例持有配置与注册结果，调用方显式创建 Runtime、注册类型、注册 unit、加载图。这样符合仓库“不要使用全局变量维护状态”的约束，也便于多个 lab 进程或测试并行运行。

可选方案是提供包级默认 Runtime。它会让示例更短，但会引入生命周期、并发测试和污染风险。

### 3. 提供类型化便捷路径，但底层仍以 Any 为核心

首期提供 string 与 protobuf message 的便捷调用和注册辅助，例如函数式 string unit 可被适配为 `dag.ComputeUnit`，请求调用可直接返回 string 或解码到目标 protobuf message。底层仍复用 `invocation.Marshal*`、`Unmarshal*` 与 `anypb.Any`，避免另起一套 codec。

可选方案是引入泛型 payload codec。泛型能减少类型断言，但容易把 serialization、schema 与 protobuf type URL 的策略复杂化，首期不作为核心路径。

### 4. 图文件定位属于上层约定

公共 `loader` 继续只负责解析文档，不绑定目录或文件名；facade 可提供 `LoadGraphFile`、`LoadGraphDir`、`LoadGraphID` 等上层约定，降低 lab 和业务入口重复代码。

### 5. 终态语义由封装暴露，不替调用方决定协议

facade 的调用结果应保留 `EntityInstance`、snapshot/payload 与 `TerminalError` 等信息。HTTP 200/500、业务错误码、重试策略仍由调用方映射。

## Risks / Trade-offs

- [Risk] 轻量层过度抽象后遮蔽 DAG 核心概念 → Mitigation: 命名和文档明确其仅覆盖请求式同步调用，复杂编排继续使用底层 API。
- [Risk] string 便捷路径被误认为公共推荐 payload 形态 → Mitigation: 规格中声明 string 主要用于 lab、示例和简单场景，protobuf message 仍是一等路径。
- [Risk] `lab-dag-http` 迁移时不小心破坏与 `lab-dag` 隔离 → Mitigation: 保留独立 Runtime、fixtures 与 HTTP API 的规格和测试。
- [Risk] facade 与现有 `invocation/memory.Runtime` 功能重复 → Mitigation: facade 只组合并简化装配，不复制底层 registry/engine/invoker 实现。

## Migration Plan

1. 新增轻量 facade 与单元测试，覆盖 entity type 注册、函数式 unit 注册、图加载、string/protobuf 调用、失败终态返回。
2. 保持现有 `invocation`、`invocation/memory` 与 loader 测试通过，确保 API 兼容。
3. 迁移 `lab-dag-http` 使用 facade，保留对 `rpc.Server`、独立 fixture、独立 Runtime 和状态页的验证。
4. 更新文档和 README 中的 DAG HTTP 示例，展示轻量调用方式。
5. 回滚时可让 `lab-dag-http` 退回现有手动 Runtime 装配，底层 API 不受影响。

## Open Questions

- facade 子包命名采用 `invocation/app`、`invocation/simple` 还是 `invocation/request` 更贴近仓库语义？
- 函数式 unit 首期是否只支持 `func(context.Context, string) (string, error)`，还是同时支持无 context 的简化签名？
- `LoadGraphID` 是否默认使用 `<graphID>.yaml`，并在缺失时尝试 `<graphID>.json`？
