# rpc-dag-bridge Specification

## Purpose
TBD - created by archiving change add-dag-app-stringapp-facade. Update Purpose after archive.
## Requirements
### Requirement: 终态到 HTTP 响应的统一映射

`dagbridge` SHALL 提供 `ResponseForTerminalResult(r *app.StringCallResult) *rpcserver.Response`，按 `StringCallResult` 的实例状态返回统一的 `rpcserver.Response`：

- `IsCompleted() == true` → `StatusCode = 200`，`Body = r.Value`
- `IsWaiting() == true` → `StatusCode = 500`，`Body` 含 `"instance still WAITING"` 说明（同步调用不应当进入 WAITING）
- 其余终态（`FAILED` / `COMPENSATED` / `CANCELLED`） → `StatusCode = 500`，`Body` 形如 `"terminal <Status>: <Value>"`
- 入参 `r == nil` → `StatusCode = 500`，`Body` 含 `"nil dag result"` 说明

`dagbridge` SHALL NOT 改变 `StringCallResult` 内容，不得 panic，不得发起新的 DAG 调用。

#### Scenario: COMPLETED 映射 200

- **WHEN** 调用 `ResponseForTerminalResult` 传入 `IsCompleted() == true` 的 `StringCallResult`，其 `Value` 为 `"echo:hello"`
- **THEN** 返回 `Response{StatusCode: 200, Body: []byte("echo:hello")}`

#### Scenario: FAILED 映射 500 并附原因

- **WHEN** 调用 `ResponseForTerminalResult` 传入 `Status() == FAILED`、`Value` 为 `"boom"` 的 `StringCallResult`
- **THEN** 返回 `Response{StatusCode: 500}`，且 `Body` 包含 `"FAILED"` 与 `"boom"`

#### Scenario: WAITING 在同步调用上下文映射 500

- **WHEN** 调用 `ResponseForTerminalResult` 传入 `IsWaiting() == true` 的 `StringCallResult`
- **THEN** 返回 `Response{StatusCode: 500}`，且 `Body` 含 `"WAITING"`

#### Scenario: nil 入参

- **WHEN** 调用 `ResponseForTerminalResult(nil)`
- **THEN** 返回 `Response{StatusCode: 500}`，且 `Body` 含 `"nil dag result"`
- **AND** 不 panic

### Requirement: 与现有 rpc 抽象正交

`dagbridge` SHALL 仅依赖 `pkg/dag/invocation/app` 与 `pkg/rpc/server`；SHALL NOT 引入 `chi`、`gorilla/mux` 等 HTTP 路由库；SHALL NOT 直接调用 `net/http`。`dagbridge` SHALL 与 `pkg/rpc/server/http` 传输解耦 —— 不依赖具体传输实现。

#### Scenario: 依赖边界

- **WHEN** 在 `pkg/rpc/server/dagbridge` 目录执行 `go list -deps`
- **THEN** 依赖链 SHALL NOT 包含 `github.com/go-chi/chi/v5` 或其他 HTTP 路由库
- **AND** SHALL NOT 包含 `github.com/solo-kingdom/uniface/pkg/rpc/server/http` 子包

### Requirement: 单元可独立测试

`ResponseForTerminalResult` SHALL 为纯函数（无包级状态、无 I/O），其单测 SHALL 覆盖上述五种状态（COMPLETED / WAITING / FAILED / COMPENSATED / CANCELLED）外加 nil 入参，共 6 个 case。

#### Scenario: 覆盖五种终态

- **WHEN** 执行 `go test ./pkg/rpc/server/dagbridge/...`
- **THEN** 至少包含 6 个 `ResponseForTerminalResult` 子测试，对应五种 InstanceStatus + nil

