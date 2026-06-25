## 1. 公共轻量封装

- [x] 1.1 确定并创建请求式 DAG 应用封装子包结构，避免与现有 `invocation.Invoker` 职责混淆
- [x] 1.2 实现独立 Runtime/facade 构造，组合 `invocation/memory.Runtime`、Invoker、Loader 与 Codec，且不使用全局状态
- [x] 1.3 实现 string 实体类型注册辅助，自动使用 `google.protobuf.StringValue` payload type URL
- [x] 1.4 实现函数式 string compute unit 适配器与注册辅助，自动注册匹配的 `ComputeUnitDef` 和 Go 实现
- [x] 1.5 实现 protobuf message 输入输出的请求式调用辅助，复用现有 `anypb.Any` codec
- [x] 1.6 实现 string 输入输出的请求式调用辅助，保留实例状态和 snapshot 结果信息

## 2. 图加载与调用语义

- [x] 2.1 实现按文件加载 YAML/JSON graph 的上层辅助，并注册 `GraphSpec` 与内联 `ComputeUnitDef`
- [x] 2.2 实现按 graph ID 从目录加载 graph 的上层约定，保持公共 loader 不绑定文件定位
- [x] 2.3 确保 `FAILED`、`COMPENSATED`、`CANCELLED` 终态不会被包装成成功结果
- [x] 2.4 确保 `WAITING` 状态向调用方显式暴露，且同步调用不继续等待外部 signal

## 3. 测试覆盖

- [x] 3.1 添加轻量封装构造与无全局状态的单元测试
- [x] 3.2 添加 string entity type、函数式 unit 注册和 echo graph 调用测试
- [x] 3.3 添加 protobuf message payload 调用和解码测试
- [x] 3.4 添加失败终态、取消终态和 WAITING 状态返回测试
- [x] 3.5 添加图文件加载和 graph ID 目录加载测试
- [x] 3.6 回归运行现有 `pkg/dag/invocation`、`pkg/dag/invocation/memory` 与 loader 测试

## 4. Lab 迁移与文档

- [x] 4.1 迁移 `lab/internal/daghttp` 使用新的请求式 DAG 轻量封装装配 echo 图和 string unit
- [x] 4.2 保留 `lab-dag-http` 与 `lab-dag` 的 Runtime、fixtures 和 HTTP API 隔离
- [x] 4.3 更新 `lab-dag-http` 测试，确认 handler 不再直接构造 `invocation.InvokeRequest` 或手写 `anypb.Any` 编解码
- [x] 4.4 更新 `lab/README.md` 中 DAG HTTP 示例，展示轻量封装后的接入方式
- [x] 4.5 运行根模块和 lab 模块相关测试，确认公共 API 兼容且 lab 行为不变
