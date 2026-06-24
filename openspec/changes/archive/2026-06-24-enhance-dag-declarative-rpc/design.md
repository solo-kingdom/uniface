## Context

`pkg/dag` 内核已闭环四种 NodeKind、Saga 补偿、信号路由、动态 JOIN（见 `enhance-dag-complex-workflows` 归档）。`ComputeUnit` 当前只能是进程内 Go 接口实现：

```go
type ComputeUnit interface {
    Execute(ctx context.Context, snapshot *dagv1.EntitySnapshot) (*dagv1.EntityMutation, error)
}
```

业务方要为每个 COMPUTE 节点写 Go struct、在启动时 `RegisterComputeUnitImpl`、重新编译部署。这阻碍 DAG 作为"独立引擎产品"对外交付——目标场景（在线业务 pipeline、实时数据 pipeline）需要节点能**配置驱动**地调用远程 HTTP 服务。

本变更在保持内核不变的前提下，引入**声明式 ComputeUnit 实现**：`ComputeUnitDef` 携带配置，引擎构造对应适配器。第一期落地 `HttpUnit`，覆盖内部 REST 服务调用。Balancer 集成让"调服务 B"不必硬编码地址。

## Goals / Non-Goals

**Goals:**

- `ComputeUnitDef` 支持声明式实现，与进程内 Go 注册互斥但共存
- 内置 `HttpUnit`：service/url、method、path、headers、body、response 映射、timeout、retry_on
- payload 表达 Level 0（整包）/ Level 1（字段路径）
- response → mutation 默认（2xx → update）+ 显式映射
- HTTP 错误分类：5xx 重试、4xx → fail
- 通过 `pkg/rpc/governance/loadbalancer` 解析服务实例
- lab YAML schema 升级，让 condition / priority / HttpUnit 在配置层可表达

**Non-Goals:**

- `GrpcUnit` / `RpcUnit` 统一抽象（零用例不预先抽象）
- CEL 表达式、payload Level 2 模板引擎
- 持久化 LineStore、跨进程 spawn、callback、分布式 worker
- 调度后台化、实例列表查询（后续小迭代）

## Decisions

### D1: 声明式实现挂在 ComputeUnitDef，而非独立 Registry

**决策**: `ComputeUnitDef` 新增 `oneof implementation`，与 `RegisterComputeUnitImpl` 注册的 Go 实现二选一。Registry 解析 unit 时优先返回声明式实现。

```protobuf
message ComputeUnitDef {
  // ... 现有字段 ...
  oneof implementation {
    HttpUnit http = 10;
  }
}
```

**理由**: 声明式 unit 的配置本质上是 unit 定义的一部分（输入/输出类型、副作用同样适用）。挂在 ComputeUnitDef 上让 GraphSpec 自包含——业务方注册图时一并声明所有 unit 配置，无需额外注册步骤。

**替代**: 独立 `UnitImplementationRegistry`——放弃，多一个注册步骤、多一处一致性校验，无收益。

### D2: HttpUnit 同时支持 service 与 url，service 走 Balancer

**决策**:

```protobuf
message HttpUnit {
  string service = 1;   // 走 Balancer.SelectClient 解析，优先
  string url = 2;       // 直连兜底（service 为空时使用）
  string method = 3;    // 默认 POST
  string path = 4;
  map<string, string> headers = 5;
  BodyTemplate request_body = 6;
  ResponseMapping response = 7;
  google.protobuf.Duration timeout = 8;  // 默认 30s
  RetryClassification retry_on = 9;
}
```

`service` 非空时，引擎通过注入的 `Balancer[http.Client]`（或等价接口）选实例；`service` 与 `url` 同时为空则校验失败。`path` 与 Balancer 选出的 base URL 拼接。

**理由**: 内部微服务场景下，service 名比 URL 更稳定（实例上下线由 Balancer 感知）；外部 / 调试场景保留 url 直连。

**替代**: 仅支持 url，业务方自己做服务发现——放弃，违背 uniface 抽象层价值。

### D3: BodyTemplate Level 0/1，不开模板引擎

**决策**:

```protobuf
message BodyTemplate {
  string field_path = 1;  // 空 = 整个 snapshot.payload（Level 0）
                          // 非空 = snapshot.payload.<path>（Level 1，复用 resolveFieldPath）
}
```

- Level 0（默认）：`field_path` 为空 → 整个 `snapshot.payload`（`Any`）按 `protojson` 序列化为 JSON body
- Level 1：`field_path = "order"` → 取 `snapshot.payload.order` 子字段，序列化

**理由**: 现有 `pkg/dag/graph/predicate.go` 的 `resolveFieldPath` 已支持 protobuf 字段路径与一层 repeated 索引，零新代码。Level 2 模板（`"order-{{.id}}"`）需引入表达式引擎，复杂度跳升，留作后续。

**替代**: 直接 CEL——放弃，YAML 里写 CEL 比写 Go code 还难读，违背"配置驱动"初衷。

### D4: Response 默认整包替换 payload，可选字段投影

**决策**:

```protobuf
message ResponseMapping {
  enum Mode {
    MODE_UNSPECIFIED = 0;
    MODE_AUTO = 1;        // 默认：2xx → update，response 整体作为新 payload
    MODE_MUTATION = 2;    // response 是 EntityMutation JSON（Level 3 escape hatch）
  }
  Mode mode = 1;
  string payload_type_url = 2;  // 空 = 复用输入 snapshot 的 type_url
  string payload_field = 3;     // 从 response 取子字段作为 payload
  TerminalOutcome on_success = 4; // 默认 UPDATE；可显式 COMPLETE/FAIL
}
```

- `MODE_AUTO`（默认）：
  - HTTP 2xx → response body 反序列化（JSON → 按 `payload_type_url` 或输入 type_url 对应的 proto message，包成 `Any`）→ mutation.update
  - `payload_field` 非空时，从反序列化结果取子字段
  - `on_success` 允许直接 COMPLETE（终止成功）或 FAIL（终止失败）
- `MODE_MUTATION`：response 体即 `EntityMutation` 的 JSON 表示，引擎直接 apply（业务方在 B 内构造完整 mutation，最高灵活性）

**理由**: 默认行为覆盖"调一下服务、把结果存起来"主路径；`MODE_MUTATION` 作为 B 端 DAG-aware 的逃生口。两级覆盖 95% 场景。

**替代**: 强制 B 返回 EntityMutation——放弃，对已有 RPC 服务侵入太大。

### D5: 错误分类——状态码白名单驱动重试

**决策**:

```protobuf
message RetryClassification {
  repeated int32 retry_status_codes = 1;  // 默认 [502, 503, 504]
  repeated int32 fail_status_codes = 2;   // 默认 [400, 401, 403, 404, 409, 422]
  // 其他 4xx/5xx 默认归 fail
}
```

- 命中 `retry_status_codes` 或网络/超时错误：按 `ComputeUnitDef.retry_policy` 重试，attempt 持久化
- 命中 `fail_status_codes` 或未分类 4xx：构造 `EntityMutation{fail: {reason: http_status, trigger_compensation: false}}`，引擎按图路由（可能进补偿）
- 未分类 5xx：保守按 fail（避免无限重试未知错误）

**理由**: 重试白名单显式优于隐式默认；4xx → fail 让业务方可通过 HTTP 状态码触发补偿分支。

### D6: Balancer 依赖通过接口注入，保根模块零硬依赖

**决策**: `pkg/dag` 定义 `HttpClientResolver` 接口：

```go
// pkg/dag/units/resolver.go
type HttpClientResolver interface {
    ResolveClient(ctx context.Context, service string) (*http.Client, string, error)
    // 返回 client 与 base URL
}
```

`pkg/dag/units/`（仍在根模块）提供 `HttpUnit` 实现，**构造时**接收 `HttpClientResolver`。Balancer 适配器由调用方（lab、业务进程）注入：

```go
// 调用方代码（不在 pkg/dag 内）
engine := memory.NewEngine(reg, store, dag.WithHttpClientResolver(
    balanceradapter.New(balancer),
))
```

**理由**: `pkg/dag` 当前零依赖（除 protobuf）。Balancer 在同根模块下，但通过接口注入保持可测试性、允许 mock、并让 DAG 引擎在无 Balancer 场景下仍可用（注入 nil resolver → HttpUnit 仅支持 url 直连）。

**替代**: HttpUnit 直接 import loadbalancer 包——放弃，破坏可测试性且耦合两域内部细节。

### D7: 声明式 unit 与 Go 注册的优先级

**决策**: `Registry.GetComputeUnitImpl(unitID)` 解析顺序：

1. 若 `ComputeUnitDef.implementation` 非空：构造声明式适配器（HttpUnit 等），返回
2. 否则查 `unitImpls` map，返回 Go 注册实现
3. 都没有：返回错误

**理由**: 声明式优先让 GraphSpec 自包含——同一 unit_id 在不同图里可以有不同的 HttpUnit 配置（虽然实践中不推荐）；Go 注册作为 escape hatch（复杂业务逻辑、有状态 unit）。

**校验**: `RegisterComputeUnit` 时若 `implementation` 非空，**禁止**同时 `RegisterComputeUnitImpl` 同 unit_id（返回错误），避免运行时歧义。

### D8: HttpUnit 的 SideEffectClass 默认

**决策**: HttpUnit 的 `ComputeUnitDef.side_effect_class`：

- 默认推荐 `SIDE_EFFECT_IDEMPOTENT`（HTTP 调用通常有副作用，业务方应通过幂等键保证安全）
- 显式声明 `SIDE_EFFECT_NONE`：用于纯查询（GET）场景
- 不允许 `SIDE_EFFECT_EXTERNAL`（保留为未来 outbox 模式）

引擎在 IDEMPOTENT 模式下：崩溃重试时，同一 idempotency_key 第二次调用 HTTP，业务方应通过请求头或 body 字段携带幂等键（由 BodyTemplate 投影 `runtime.idempotency_key`，留作 Level 2 增强；v1 由业务方在 B 端自行保证）。

### D9: YAML schema 升级范围

**决策**: `lab/internal/dag/runtime.go` 的 `parseGraphYAML` 升级：

| 现有 | 新增 |
|------|------|
| `kind: compute/wait/join/terminal` | 不变 |
| `unit: lab.echo`（字符串） | `unit:` 可为对象，含 `http` 子结构 |
| `transitions: [{target: x}]` | `transitions: [{target, condition, priority}]` |
| `compensator: foo` | 不变 |
| —— | `retry_policy: {max_attempts, initial_backoff}` |
| —— | `wait` 节点：`deadline_seconds`, `on_timeout` |
| —— | `join` 节点：`fail_parent_on_child_failure` |

`condition` 子结构：`{always: true}` / `{field: {path, op, value}}` / `{signal: {name, payload_predicate: {path, op, value}}}`。

**理由**: 让现有引擎能力（FieldPredicate、signal_predicate、priority、RetryPolicy、动态 JOIN 配置）在 lab YAML 层可表达，否则 lab fixture 无法演示真实业务场景。

## Risks / Trade-offs

| 风险 | 缓解 |
|------|------|
| HttpUnit 同步阻塞占用 scheduler goroutine | v1 接受（单进程内存引擎本就串行 hop）；未来后台调度 + worker pool 时再加 concurrency 限制 |
| response JSON → protobuf 反序列化失败 | `MODE_AUTO` 下反序列化失败转为 `mutation.fail`，记入 journal 的 `failure_reason`，不致实例 silent stuck |
| service 解析失败（Balancer 无实例） | 返回 retryable 错误，按 RetryPolicy 重试；耗尽后 fail |
| 业务方误用 4xx 触发不必要补偿 | 文档强调 4xx → fail（不重试）；显式 `fail_status_codes` 可调；业务方应在图里设兜底 transition |
| HttpUnit 配置错误（path 不存在、type_url 错）静态检测难 | 引擎首次执行时快速失败；`ValidateGraphSpec` 可选 warn 级校验 service 已注册（避免启动顺序耦合） |
| proto 字段 additive 兼容性 | 全部 additive；旧客户端忽略 `implementation` oneof，仍走 Go 注册路径 |
| 根模块引入 net/http 依赖 | `net/http` 是标准库；自定义客户端注入仍可避免直连 |

## Migration Plan

1. 扩展 `api/dag/v1/unit.proto`：`ComputeUnitDef.implementation` oneof、`HttpUnit`、`BodyTemplate`、`ResponseMapping`、`RetryClassification` → `make proto`
2. 新增 `pkg/dag/units/`：`HttpUnit` 实现、`HttpClientResolver` 接口、body/response 处理
3. `pkg/dag/memory/registry.go`：声明式 unit 解析；`RegisterComputeUnit` 校验互斥
4. `pkg/dag/memory/engine.go`：`NewEngine` 接受 `HttpClientResolver` Option；HttpUnit 解析路径
5. lab：`runtime.go` YAML schema 升级；新增 `http_call.yaml` fixture；`serve` 启动时注入 Balancer resolver（若 `pkg/rpc/governance/loadbalancer` 可用）
6. 集成测试：HTTP unit 黄金路径（2xx → update）、4xx → fail → 补偿、5xx → retry → 成功、Balancer 解析、字段路径 body
7. 归档后 delta spec 合并至 `openspec/specs/dag-units/`、`openspec/specs/dag-runtime/`

**回滚**: 还原 proto 与 `pkg/dag` 改动；已持久化实例无影响（内存 MVP）。

## Open Questions

- `payload_field` 的 JSON path（`result.order.id`）是否一期支持多层？建议支持（与 `resolveFieldPath` 已有能力对齐），但 response 是 JSON 而非 protobuf 时需要 jsonpath 子集实现。**倾向**: 一期仅支持顶层字段 + protobuf field path（response 反序列化为 proto message 后取字段）；纯 JSON 嵌套路径留作 Level 1.5。
- HttpUnit 是否需要内置 tracing（OpenTelemetry header 注入）？建议一期不做，留给后续可观测性 change。
- Balancer 适配器放 `pkg/dag/units/balanceradapter/` 还是 `pkg/rpc/governance/loadbalancer/dagadapter/`？**倾向**前者（dag 是消费方），保持 dag 域自包含。
