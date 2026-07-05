## Why

`lab/internal/daghttp` 作为「HTTP→DAG→响应」最小范式验证地，自身仅 ~500 行；其中 ~150 行是地基本可吃掉的样板（类型注册与 close 兜底、entityID 自维护计数器、终态→HTTP 状态码翻译、OpRecorder 的 ok 推导、wiring 里的相对路径硬编码）。`pkg/dag/invocation/app`、`pkg/rpc/server` 已具备底层能力，但缺少把这些"每域都会重复一遍"的样板封装到地基的便捷面。本次新增若干**横向复用**的地基组件，让 daghttp 缩到 ~316 行，同时给未来其它"同步 DAG 暴露成 RPC"的域（kv/config/queue 等）提供同款省样板能力。

## What Changes

- **`pkg/dag/invocation/app` 新增 `StringApp`**：预注册 `google.protobuf.StringValue` 实体类型并封装 `TypeKey`，对外暴露 `RegisterUnit` / `InvokeString` / `LoadGraphID` / `Close`；注册失败时自动关闭底层 Runtime，调用方不再写三遍 `_ = rt.Close(); return nil, err`
- **`pkg/dag/invocation/app` 新增 `EntityIDGen`**：原子计数器驱动的 entity ID 生成器，按 prefix 格式化；`Runtime.NewEntityIDGen(prefix)` 工厂方法
- **`pkg/rpc/server` 新增 `dagbridge` 子包**：提供 `ResponseForTerminalResult(*app.StringCallResult) *Response` —— 把 DAG 终态映射到 `200/500` 响应；覆盖 COMPLETED / WAITING / FAILED / COMPENSATED / CANCELLED 五种状态
- **`lab/internal/web/api` 扩展 `OpRecorder`**：新增 `RecordResult(op, detail, ResultSentinel)` —— 调用方传入 `*app.StringCallResult` 由 recorder 派生 `ok` 与 `err`，handler 不再手工 `IsCompleted()`→`bool` 推导
- **`lab/internal/wiring` 解耦**：`DAGConfig.FixturesDir` 改为必填，移除"默认指向 daghttp 内部目录"的硬编码；fixtures 默认值下沉到 daghttp 自身
- **`lab/internal/daghttp` 简化**：
  - 删除 `runtime.go`（82 行包装层），handler 直接持有 `*app.StringApp`
  - `handler.go` 用 `idGen.Next()` 替代手工计数器；终态映射改用 `dagbridge.ResponseForTerminalResult`；记录用 `rec.RecordResult`
  - wiring 路径默认值放到 `daghttp` 包内

## Capabilities

### New Capabilities

- `rpc-dag-bridge`: `pkg/rpc/server/dagbridge` 包 —— 把 `app.StringCallResult` 终态映射到 `rpcserver.Response`，覆盖五种状态码语义

### Modified Capabilities

- `dag-invocation`: 新增 `StringApp` 与 `EntityIDGen` 两项 Requirement；`StringApp` 作为"类型化 Runtime 门面"组合实体类型注册 + 单元注册 + InvokeString；`EntityIDGen` 提供线程安全 entity ID 生成
- `uniface-lab`: 新增 Requirement —— `OpRecorder.RecordResult` 接受类型化结果并派生 ok 字段

## Impact

- **新增包**：`pkg/rpc/server/dagbridge`（~30 行）
- **新增文件**：`pkg/dag/invocation/app/string_app.go`（~80 行）、`pkg/dag/invocation/app/entity_id.go`（~30 行）
- **修改文件**：`pkg/dag/invocation/app/invoke.go`（追加 `StringCallResult.Err()` 等 ~5 行）、`lab/internal/web/api/status.go`（追加 `RecordResult` ~15 行）、`lab/internal/wiring/daghttp.go`（-5 行）、`lab/internal/daghttp/runtime.go`（删除，-82 行）、`lab/internal/daghttp/handler.go`（-40 行）、`lab/internal/daghttp/handler_test.go`（-50 行）
- **依赖与破坏性**：无 proto 变更；无外部依赖变化；`app.Runtime` 现有 API 保持不变（纯追加）

## Non-goals

- 不修改 `pkg/dag/memory` 引擎、`pkg/rpc/server` 核心 `Server` 接口
- 不引入 `MessageApp`（仅做 StringApp；message-payload 场景沿用 `app.Runtime.InvokeMessage`）
- 不为 `RecordResult` 设计完整 ResultSentinel 体系（仅暴露 `IsCompleted` / `Status` / `Err` 三方法最小集）
- 不重构 `lab/internal/dag` 那套 698 行大 Runtime（独立路径，本次不动）
- 不动 `pkg/dag/invocation/app` 的 `StringCall` / `MessageCall` / `InvokeString` 既有签名
- 不引入新的传输（gRPC / WebSocket）；dagbridge 仍以 `*rpcserver.Response` 输出
