# dag-units Specification

## Purpose
TBD - created by archiving change enhance-dag-declarative-rpc. Update Purpose after archive.
## Requirements
### Requirement: ComputeUnitDef 声明式实现

`ComputeUnitDef` SHALL 支持可选的 `implementation` oneof 字段，承载声明式 unit 配置（首期 `HttpUnit`）。当 `implementation` 非空时，Registry 解析 unit 实现 SHALL 优先返回基于配置构造的适配器，而非进程内 Go 注册实现。`implementation` 与 `RegisterComputeUnitImpl` 注册 SHALL 互斥：同一 `unit_id` 同时存在两者时，`RegisterComputeUnit` 或 `RegisterComputeUnitImpl` SHALL 返回错误。

#### Scenario: 声明式 HttpUnit 被解析为 HTTP 适配器

- **WHEN** `ComputeUnitDef{unit_id: "order.charge", implementation: {http: {...}}}` 已注册
- **AND** 调度对应 COMPUTE 节点请求 `GetComputeUnitImpl("order.charge")`
- **THEN** Registry 返回基于 HttpUnit 配置构造的 `ComputeUnit` 实现，其 `Execute` 发起 HTTP 调用

#### Scenario: 声明式与 Go 注册互斥

- **WHEN** `ComputeUnitDef` 含 `implementation` 且 `unit_id="x"`
- **AND** 随后调用 `RegisterComputeUnitImpl("x", goImpl)`
- **THEN** 返回错误，拒绝注册

#### Scenario: 无 implementation 时回退 Go 注册

- **WHEN** `ComputeUnitDef{unit_id: "lab.echo"}` 无 `implementation` 字段
- **AND** `RegisterComputeUnitImpl("lab.echo", &echoUnit{})` 已执行
- **THEN** `GetComputeUnitImpl("lab.echo")` 返回 `echoUnit` 实例

### Requirement: HttpUnit 服务与请求配置

`HttpUnit` SHALL 通过 `service`（走注入的 `HttpClientResolver` 解析）或 `url`（直连）定位目标服务，二者至少一个非空，否则 `ValidateGraphSpec` 失败。`method` 默认 `POST`。`path` SHALL 与解析出的 base URL 拼接为最终请求 URL。`headers` 为静态键值对，v1 不支持动态取值。

#### Scenario: service 走 Balancer 解析

- **WHEN** `HttpUnit{service: "order-service", path: "/charge"}`
- **AND** 注入的 `HttpClientResolver` 返回 base URL `http://10.0.1.5:8080`
- **THEN** 实际请求 URL 为 `http://10.0.1.5:8080/charge`

#### Scenario: service 与 url 同时为空被拒绝

- **WHEN** `HttpUnit{service: "", url: ""}` 注册时
- **THEN** `RegisterComputeUnit` 返回 `ErrInvalidGraph`，拒绝该 unit

#### Scenario: url 直连兜底

- **WHEN** `HttpUnit{url: "http://legacy-svc/internal/api", path: "/charge"}`
- **AND** `service` 为空
- **THEN** 实际请求 URL 为 `http://legacy-svc/internal/api/charge`，不经过 `HttpClientResolver`

### Requirement: Body 构造层级

`BodyTemplate.field_path` SHALL 控制 request body 构造：

- `field_path` 为空（Level 0，默认）：整个 `snapshot.payload`（`Any`）按 protojson 序列化为 JSON body
- `field_path` 非空（Level 1）：取 `snapshot.payload.<field_path>` 子字段，按 protojson 序列化

`field_path` SHALL 复用 `pkg/dag/graph` 的 `resolveFieldPath` 规则（protobuf 字段名 + 一层 repeated 索引）。取值失败时（字段不存在）SHALL 返回错误，记入 journal `failure_reason`。

#### Scenario: Level 0 整包传递

- **WHEN** `HttpUnit{request_body: {}}` 且 `snapshot.payload` 为 `Order{id: "o1", amount: 100}`
- **THEN** HTTP request body 为 `{"id":"o1","amount":100}`

#### Scenario: Level 1 字段路径

- **WHEN** `HttpUnit{request_body: {field_path: "order"}}` 且 `snapshot.payload` 为 `Wrapper{order: Order{...}}`
- **THEN** HTTP request body 为 `order` 子字段的 JSON 表示

#### Scenario: 字段路径不存在失败

- **WHEN** `HttpUnit{request_body: {field_path: "missing"}}` 且 `snapshot.payload` 无 `missing` 字段
- **THEN** HttpUnit.Execute 返回错误，引擎按 retry/fail 策略处理（不重试路径错误）

### Requirement: Response 映射为 Mutation

`ResponseMapping.mode` SHALL 决定 HTTP response 如何转换为 `EntityMutation`：

- `MODE_AUTO`（默认）：HTTP 2xx → response body 反序列化为 `payload_type_url` 对应 proto message（空则复用输入 snapshot 的 type_url），包装为 `Any`，产出 `EntityMutation{update: {payload: <Any>}}`；若 `payload_field` 非空，从反序列化结果取子字段
- `MODE_MUTATION`：response body 视为 `EntityMutation` 的 JSON 表示，直接 apply

`on_success` 允许覆盖默认 update 语义：`COMPLETE` → `mutation.complete`（终止成功）；`FAIL` → `mutation.fail`（终止失败，可能触发补偿）。反序列化失败时 SHALL 产出 `mutation.fail`，`reason` 记录解析错误。

#### Scenario: 默认 update 整包替换

- **WHEN** HttpUnit 收到 HTTP 200，body 为 `{"transaction_id":"t1","status":"success"}`
- **AND** `payload_type_url` 为空，输入 snapshot type_url 为 `type.googleapis.com/order.Order`
- **THEN** response 反序列化为 `Order` proto message，包装为 `Any`，产出 `mutation.update`

#### Scenario: payload_field 子结构投影

- **WHEN** response 为 `{"result":{"order":{...}},"metadata":{...}}`
- **AND** `payload_field: "result.order"`，`payload_type_url` 对应 `Order`
- **THEN** 取 `result.order` 作为新 payload

#### Scenario: 反序列化失败转 fail

- **WHEN** response body 不是合法 JSON 或不匹配 `payload_type_url` schema
- **THEN** 产出 `mutation.fail{reason: "response decode failed: <err>"}`，引擎按 fail 处理

#### Scenario: MODE_MUTATION 直接 apply

- **WHEN** `mode: MODE_MUTATION`，response body 为 `{"spawn":{"specs":[...]}}`
- **THEN** 反序列化为 `EntityMutation`，直接作为 Execute 返回值

### Requirement: HTTP 错误分类与重试

`RetryClassification` SHALL 按状态码分类 HTTP 错误：

- 命中 `retry_status_codes`（默认 `[502, 503, 504]`）或网络/超时错误：按 `ComputeUnitDef.retry_policy` 重试，attempt 持久化
- 命中 `fail_status_codes`（默认 `[400, 401, 403, 404, 409, 422]`）或未分类 4xx：构造 `EntityMutation{fail: {reason: "http <status>", trigger_compensation: false}}`
- 未分类 5xx：保守归 fail（避免无限重试未知错误）

`retry_policy` 耗尽后 SHALL 沿用现有 `handleExecuteError` 路径（最终返回错误，引擎按 retry/fail 决策）。

#### Scenario: 503 触发重试

- **WHEN** HttpUnit 收到 HTTP 503，`retry_status_codes` 含 503
- **AND** 当前 attempt < `retry_policy.max_attempts`
- **THEN** attempt 递增并持久化，下次调度重新调用 Execute

#### Scenario: 404 触发 fail

- **WHEN** HttpUnit 收到 HTTP 404，`fail_status_codes` 含 404
- **THEN** 产出 `mutation.fail{reason: "http 404"}`，不重试

#### Scenario: 重试耗尽最终失败

- **WHEN** 连续 503 达到 `max_attempts`
- **THEN** `handleExecuteError` 返回错误，引擎按现有逻辑标记实例失败

#### Scenario: 网络错误归类为可重试

- **WHEN** HTTP 调用因连接拒绝或超时返回 `net.Error`
- **THEN** 视为可重试错误，按 `retry_policy` 处理

### Requirement: 服务实例解析（Balancer 集成）

`HttpUnit` SHALL 通过 `pkg/dag` 定义的 `HttpClientResolver` 接口解析服务实例：

```go
type HttpClientResolver interface {
    ResolveClient(ctx context.Context, service string) (*http.Client, string, error)
}
```

引擎构造时通过 Option 注入 resolver；未注入时（nil），`HttpUnit.service` 非空 SHALL 在 Execute 时返回错误（仅 `url` 直连可用）。Resolver 返回错误（如 Balancer 无可用实例）SHALL 视为可重试错误。

#### Scenario: 注入 resolver 解析成功

- **WHEN** 引擎通过 `WithHttpClientResolver` 注入适配 uniface.Balancer 的 resolver
- **AND** `HttpUnit{service: "order-service"}`
- **THEN** Execute 调用 `resolver.ResolveClient(ctx, "order-service")` 获取 `*http.Client` 与 base URL

#### Scenario: 未注入 resolver 且 service 非空

- **WHEN** 引擎未注入 `HttpClientResolver`
- **AND** `HttpUnit{service: "order-service"}`
- **THEN** Execute 返回错误（不发起请求），记入 journal `failure_reason`

#### Scenario: resolver 返回无可用实例

- **WHEN** `resolver.ResolveClient` 返回 "no instance available" 错误
- **THEN** 视为可重试错误，按 `retry_policy` 处理

### Requirement: 声明式 unit SideEffectClass 默认

`HttpUnit` 对应的 `ComputeUnitDef.side_effect_class` SHALL 为 `SIDE_EFFECT_IDEMPOTENT`（推荐默认）或 `SIDE_EFFECT_NONE`（纯查询 GET 场景）。`SIDE_EFFECT_EXTERNAL` SHALL 沿用现有约束返回 `ErrUnsupportedSideEffect`。`SIDE_EFFECT_IDEMPOTENT` 模式下，业务方 SHALL 自行保证 HTTP 服务幂等（v1 不自动注入 idempotency key 到请求）。

#### Scenario: IDEMPOTENT 为推荐默认

- **WHEN** `ComputeUnitDef` 含 `HttpUnit` 实现且 `side_effect_class` 未显式声明
- **THEN** Registry 接受注册，文档推荐业务方按 IDEMPOTENT 语义设计 B 端幂等性

#### Scenario: EXTERNAL 仍被拒绝

- **WHEN** `ComputeUnitDef{side_effect_class: SIDE_EFFECT_EXTERNAL, implementation: {http: {...}}}`
- **THEN** `RegisterComputeUnit` 返回 `ErrUnsupportedSideEffect`

