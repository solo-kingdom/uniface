## Why

当前 `dag-invocation` 已封装 `StartInstance -> DrainInstance -> GetSnapshot`，但业务方仍需手写实体类型注册、ComputeUnit 定义、payload 编解码、图文件加载和 `InvokeRequest` 组装。像 `lab-dag-http` 这样简单的请求编排示例因此暴露过多底层装配细节，降低 DAG 作为基础设施抽象的易用性。

现在需要在现有 Invoker/Runtime/Loader/Codec 之上补一层轻量应用封装，让常见“请求进、DAG 排空、响应出”的场景能用更少样板接入，同时保留底层 API 给复杂编排使用。

## What Changes

- 在 `dag-invocation` 能力中新增轻量请求式 DAG 调用封装，面向常见同步调用场景提供更高层 API。
- 提供类型化 payload 便捷能力，优先覆盖 protobuf message 与 string 等 lab/示例常用输入输出。
- 提供图目录/图 ID 加载约定的上层辅助，不改变公共 loader “不绑定文件定位”的底层契约。
- 提供函数式或简化注册辅助，降低注册 EntityType、ComputeUnitDef 与 Go ComputeUnit 实现的重复代码。
- 梳理非目标场景：复杂生命周期、异步 WAITING/Signal、Saga 补偿和声明式 HTTP Unit 仍继续使用底层 DAG API 或既有能力。
- 将 `lab-dag-http` 作为应用封装的验证样例，展示简单 HTTP 请求编排不再需要直接接触大部分底层装配。

## Capabilities

### New Capabilities

- 无

### Modified Capabilities

- `dag-invocation`: 增加请求式 DAG 应用封装、类型化调用、简化注册与上层图加载约定的行为要求。
- `uniface-lab`: 调整 `lab-dag-http` 验证要求，要求其优先使用新的请求式 DAG 轻量封装演示简单 HTTP 编排。

## Non-goals

- 不替换或删除现有 `dag.Engine`、`invocation.Invoker`、`invocation/memory.Runtime`、`loader`、`codec` API。
- 不在轻量封装中内置 lab 业务语义、固定 graph ID、固定 entity type 或固定 fixture 路径。
- 不改变 DAG 调度、幂等、Saga、Signal、声明式 HTTP Unit 的底层行为。
- 不引入外部依赖或全局状态。

## Impact

- 影响 `pkg/dag/invocation` 及其子包，可能新增应用级子包或 facade 类型。
- 影响 `lab/internal/daghttp` 的装配方式和相关测试。
- 需要更新 `openspec/specs/dag-invocation` 与 `openspec/specs/uniface-lab` 的行为契约。
- 需要补充单元测试和 lab 级回归测试，确保底层 API 兼容且简单场景代码量明显降低。
