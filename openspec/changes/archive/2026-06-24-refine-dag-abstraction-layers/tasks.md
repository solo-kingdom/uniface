## 1. 公共调用抽象

- [x] 1.1 确定公共包命名与导出 API 边界（Invoker、InvokeRequest、InvokeResult）。✔ `pkg/dag/invocation`（含 memory/loader 子包）
- [x] 1.2 实现请求式 Invoker，组合现有 `dag.Engine` 与 `dag.LineStore` 完成 Start、Drain、Snapshot 读取。
- [x] 1.3 为 Invoker 添加成功终态、失败终态、WAITING 返回、Drain 错误透传的单元测试。

## 2. 内存 Runtime 装配

- [x] 2.1 实现标准 memory runtime 装配辅助，封装 Registry、LineStore、Engine 与 Invoker 创建。✔ `pkg/dag/invocation/memory`
- [x] 2.2 支持注册 EntityType、GraphSpec、ComputeUnitDef、Go ComputeUnit、Compensator 与 HttpClientResolver。
- [x] 2.3 添加测试验证 runtime 不内置 lab 语义，且注入 HttpClientResolver 后声明式 HttpUnit 可解析服务。

## 3. Payload Codec

- [x] 3.1 实现 protobuf message 到 `anypb.Any` 的编码 helper。
- [x] 3.2 实现从 `EntitySnapshot.Payload` 解码到目标 protobuf message 的 helper。
- [x] 3.3 添加 nil snapshot、nil payload、类型不匹配等错误路径测试。

## 4. 声明式 Graph Loader

- [x] 4.1 将通用 YAML 图解析能力迁入公共 DAG 辅助包，覆盖 compute、terminal、wait、join 与 transition condition。✔ `pkg/dag/invocation/loader`
- [x] 4.2 支持内联 HttpUnit 与 retry_policy 解析并返回对应 ComputeUnitDef。
- [x] 4.3 添加 loader 测试，覆盖基础图、HttpUnit 图、非法图与不绑定 fixture 文件定位。

## 5. lab-dag-http 迁移

- [x] 5.1 使用公共 memory runtime、Invoker、Codec、Loader 重写 `lab/internal/daghttp` 的运行时胶水。
- [x] 5.2 保持 `lab-dag-http` 自有 fixtures、units 与 runtime，不复用 `lab/internal/dag.Runtime`。
- [x] 5.3 更新或补充 `lab-dag-http` 测试，验证 golden path、空 body、失败终态、状态接口和路由注册行为不变。

## 6. 文档与验证

- [x] 6.1 更新 lab README 或相关文档，说明 `lab-dag-http` 与 `lab-dag` 隔离但共享公共 `pkg/dag` 抽象。
- [x] 6.2 运行根模块 DAG 相关测试与 lab DAG HTTP 测试。
- [x] 6.3 运行 `openspec status --change refine-dag-abstraction-layers` 并确认变更 apply-ready。
