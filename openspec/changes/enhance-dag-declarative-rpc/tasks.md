## 1. Proto 契约扩展

- [x] 1.1 `api/dag/v1/unit.proto` 新增 `HttpUnit` message（service/url/method/path/headers/request_body/response/timeout/retry_on）
- [x] 1.2 `api/dag/v1/unit.proto` 新增 `BodyTemplate`（field_path）、`ResponseMapping`（Mode 枚举、payload_type_url、payload_field、on_success）、`RetryClassification`（retry_status_codes、fail_status_codes）
- [x] 1.3 `ComputeUnitDef` 新增 `oneof implementation { HttpUnit http = 10; }`
- [x] 1.4 执行 `make proto` 并确认 `api/dag/v1/*.pb.go` 生成无误

## 2. HttpClientResolver 接口与适配器

- [x] 2.1 `pkg/dag/units/resolver.go`：定义 `HttpClientResolver` 接口（`ResolveClient(ctx, service) (*http.Client, string, error)`）
- [x] 2.2 `pkg/dag/units/balanceradapter/`：实现 uniface.Balancer 到 `HttpClientResolver` 的适配器（可选依赖 `pkg/rpc/governance/loadbalancer`）
- [x] 2.3 单元测试：适配器将 `Balancer[http.Client]` 包装为 resolver，选实例返回 base URL

## 3. HttpUnit 核心实现

- [x] 3.1 `pkg/dag/units/http_unit.go`：`HttpUnit` struct 持有 `HttpUnit` proto 配置与 `HttpClientResolver`；实现 `dag.ComputeUnit` 接口
- [x] 3.2 request body 构造：Level 0（整包 `snapshot.payload` → protojson）与 Level 1（`field_path` 子字段，复用 `pkg/dag/graph.resolveFieldPath`）
- [x] 3.3 HTTP 调用执行：拼接 URL（service 走 resolver / url 直连 + path）、应用 headers、超时控制
- [x] 3.4 状态码分类：`RetryClassification` 命中 → 返回 retryable 错误；`fail_status_codes` 或未分类 4xx → 构造 `mutation.fail`；网络/超时 → retryable
- [x] 3.5 response → mutation 映射：`MODE_AUTO`（反序列化为 `payload_type_url` proto，包 `Any`，产出 update；`payload_field` 投影；`on_success` 覆盖）与 `MODE_MUTATION`（直接反序列化为 `EntityMutation`）
- [x] 3.6 反序列化失败处理：`MODE_AUTO` 失败 → `mutation.fail{reason: "response decode failed"}`
- [x] 3.7 单元测试：body 构造（Level 0/1/字段缺失）、状态码分类、response 映射（AUTO/MUTATION/反序列化失败）、resolver 缺失错误

## 4. Registry 与 Engine 接线

- [x] 4.1 `pkg/dag/options.go`：新增 `WithHttpClientResolver` Option 与 `Options.HttpClientResolver` 字段
- [x] 4.2 `pkg/dag/memory/registry.go`：`RegisterComputeUnit` 校验 `implementation` 非空时禁止同 `unit_id` 的 `RegisterComputeUnitImpl`，反之亦然
- [x] 4.3 `pkg/dag/memory/registry.go`：`GetComputeUnitImpl` 实现声明式优先解析——`implementation` 非空时按类型构造适配器（首期 HttpUnit，注入 engine 持有的 resolver）
- [x] 4.4 `pkg/dag/memory/engine.go`：`NewEngine` 接受 `HttpClientResolver` Option 并透传给 Registry/HttpUnit 构造
- [x] 4.5 单元测试：声明式优先于 Go 注册、无 implementation 回退 Go 注册、互斥校验拒绝重复注册

## 5. 图校验扩展

- [x] 5.1 `pkg/dag/graph/graph.go`：`ValidateGraphSpec` 校验 `HttpUnit.service` 与 `url` 至少一个非空；`SIDE_EFFECT_EXTERNAL` 与 `implementation` 组合拒绝
- [x] 5.2 单元测试：service 与 url 同时为空被拒；EXTERNAL + HttpUnit 组合被拒

## 6. Lab YAML schema 升级

- [x] 6.1 `lab/internal/dag/runtime.go`：`graphNodeYAML` 扩展 `unit` 字段为 `interface{}`——字符串（旧式 unit_id）或对象（含 `http` 子结构）
- [x] 6.2 `lab/internal/dag/runtime.go`：`graphTransitionYAML` 扩展 `condition`（always/field/signal）与 `priority` 字段，构造对应 `dagv1.Condition`
- [x] 6.3 `lab/internal/dag/runtime.go`：节点级 `retry_policy`、`wait` 节点 `deadline_seconds` 与 `on_timeout`、`join` 节点 `fail_parent_on_child_failure`
- [x] 6.4 新增 `lab/internal/fixtures/graphs/http_call.yaml`：演示 HttpUnit 调用 mock 服务的黄金路径
- [x] 6.5 升级 `lab/internal/fixtures/graphs/approval_branch.yaml`：补充 field/signal condition 表达审批通过/拒绝分支（验证 schema 升级让现有引擎能力可表达）
- [x] 6.6 lab 单元测试：YAML 解析覆盖 unit 对象、condition 各类、priority、retry_policy

## 7. 集成测试

- [x] 7.1 `pkg/dag/memory/http_unit_integration_test.go`：HttpUnit 2xx → update 黄金路径（mock HTTP server）
- [x] 7.2 集成测试：HttpUnit 4xx → `mutation.fail`，图配置 fail transition 进补偿分支
- [x] 7.3 集成测试：HttpUnit 5xx → retry → 第 N 次成功；retry 耗尽 → fail
- [x] 7.4 集成测试：`HttpClientResolver` mock 解析 service → base URL，HttpUnit 拼接 path 发请求
- [x] 7.5 集成测试：BodyTemplate Level 0 整包 / Level 1 字段路径传递
- [x] 7.6 集成测试：`MODE_MUTATION` response（spawn/update/complete）直接 apply
- [x] 7.7 集成测试：未注入 resolver 且 service 非空 → Execute 错误，journal 记 failure_reason
- [x] 7.8 `make test` 全绿

## 8. 端到端 Lab 验证

- [x] 8.1 lab wiring 注入 Balancer resolver（若 `pkg/rpc/governance/loadbalancer` 可用）或 nil resolver（仅支持 url 直连）
- [x] 8.2 `lab/internal/dag` 启动 mock HTTP server 作为 http_call fixture 的目标服务
- [x] 8.3 `make lab-up-dag` 端到端验证：`lab-dag graph load --graph http-call` + `start` + `journal` 查看 HttpUnit hop
- [x] 8.4 lab README 增补 HttpUnit 章节（配置示例、Balancer 集成说明）
