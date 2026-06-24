## Why

当前 `pkg/dag` 已提供 Engine、Registry、LineStore、GraphResolver 与声明式 Unit 等底层能力，但业务入口仍需要反复手写 `StartInstance -> DrainInstance -> GetSnapshot -> payload 解码 -> 终态映射` 这套胶水。`lab-dag-http` 暴露了这个抽象缺口：一个很小的请求编排需求，却需要自行装配运行时、图加载、payload 编解码与调用结果处理。

## What Changes

- 新增可复用的 DAG 调用层抽象，面向“给定 graph + payload，同步排空到终态并返回结果”的常见业务模式。
- 沉淀标准内存运行时装配能力，降低业务方手动组合 Registry、LineStore、Engine、EntityType、ComputeUnit 与 GraphSpec 的样板代码。
- 提供通用 payload 编解码边界，避免 `Any`、`StringValue`、snapshot 解码逻辑在业务入口和 lab 示例中重复扩散。
- 梳理声明式图加载边界，明确哪些 YAML/JSON 到 `GraphSpec`/`ComputeUnitDef` 的能力应进入公共包，哪些仍属于 lab fixture。
- 更新 `uniface-lab` 规格中 `lab-dag-http` 的冲突描述：`lab-dag-http` 应保持独立验证进程与独立运行时装配，不再要求复用 `lab/internal/dag.Runtime`。

## Capabilities

### New Capabilities

- `dag-invocation`: 定义可复用 DAG Runtime/Invoker/Codec/Loader 抽象，用于请求式启动、排空并读取 DAG 实例结果。

### Modified Capabilities

- `uniface-lab`: 修正 `lab-dag-http` 规格，明确其与 `lab-dag` 完全隔离，但可复用公共 `pkg/dag` 抽象。

## Impact

- 影响 `pkg/dag` 公共 API：新增上层调用/运行时辅助包或子包，不破坏现有 Engine 接口。
- 影响 `pkg/dag/memory` 或新建 memory runtime helper：提供标准内存运行时装配。
- 影响 `lab/internal/daghttp`：后续实现可用新公共抽象缩减本地胶水代码。
- 影响 OpenSpec：新增 `dag-invocation` 能力规格，修改 `uniface-lab` 中 `lab-dag-http` 要求。

## Non-goals

- 不重写 DAG Engine、Scheduler、LineStore 的底层执行语义。
- 不把 lab 私有 fixture、dashboard、CLI 状态记录能力迁入根模块。
- 不要求 `lab-dag-http` 复用 `lab/internal/dag`，二者仍作为独立验证入口存在。
