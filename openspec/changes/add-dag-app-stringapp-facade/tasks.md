## 1. StringApp 与 EntityIDGen 地基

- [x] 1.1 新建 `pkg/dag/invocation/app/string_app.go`：定义 `StringApp` 类型（嵌 `*Runtime` + 持 `*EntityTypeKey`）、`NewStringApp(opts ...Option) (*StringApp, error)` 构造函数；预注册 StringValue 实体类型（默认 `entityType="app.String"`、`schemaVersion="v1"`，可被 Option 覆盖）；预注册失败时自动 `Close` 底层 Runtime
- [x] 1.2 在 `pkg/dag/invocation/app/string_app.go` 实现 `(*StringApp).RegisterUnit(unitID string, fn StringFunc) error` —— 委托 `Runtime.RegisterStringUnit`；失败时先 `Close` 再返回 error
- [x] 1.3 在 `pkg/dag/invocation/app/string_app.go` 实现 `(*StringApp).InvokeString(ctx, graphID, entityID, payload string) (*StringCallResult, error)` —— 内部构造 `StringCall{TypeKey: s.typeKey, ...}` 后委托 `Runtime.InvokeString`
- [x] 1.4 在 `pkg/dag/invocation/app/string_app.go` 透传 `LoadGraphID` / `LoadedGraphs` / `Close` 方法
- [x] 1.5 新建 `pkg/dag/invocation/app/entity_id.go`：定义 `EntityIDGen` 类型（嵌 `atomic.Uint64` + `prefix string`）、`Next() string` 方法（格式 `<prefix>-<n>`）、`(*Runtime).NewEntityIDGen(prefix string) *EntityIDGen` 工厂方法
- [x] 1.6 在 `pkg/dag/invocation/app/invoke.go` 给 `StringCallResult` 加 `Err() error` 方法 —— 委托 `CallResult.TerminalErr()`
- [x] 1.7 新建 `pkg/dag/invocation/app/string_app_test.go`：覆盖 4 个 scenario（构造 + 注册 / 注册失败自动 close / InvokeString 隐藏 TypeKey / 不内置 lab 单元）
- [x] 1.8 新建 `pkg/dag/invocation/app/entity_id_test.go`：覆盖 4 个 scenario（计数器单调 / 1000 并发 / 默认 prefix / 多 gen 独立）
- [x] 1.9 在 `pkg/dag/invocation/app/runtime.go` 暴露 `StringEntityType` 与 `StringSchemaVersion` 常量供 `StringApp` 与调用方共享

## 2. dagbridge 桥接

- [x] 2.1 新建 `pkg/rpc/server/dagbridge/dagbridge.go`：定义 `ResponseForTerminalResult(r *app.StringCallResult) *rpcserver.Response` 纯函数；按 `IsCompleted` / `IsWaiting` / 其他终态 / nil 分支返回 200 或 500 响应
- [x] 2.2 在 `pkg/rpc/server/dagbridge/dagbridge.go` 加 `doc.go` 解释包职责与依赖边界（仅依赖 `app` + `rpcserver`，不引 `chi` / `gorilla`，不引 `pkg/rpc/server/http`）
- [x] 2.3 新建 `pkg/rpc/server/dagbridge/dagbridge_test.go`：6 个子测试覆盖 COMPLETED / WAITING / FAILED / COMPENSATED / CANCELLED / nil

## 3. OpRecorder 类型化结果

- [x] 3.1 在 `lab/internal/web/api/status.go` 定义 `ResultSentinel` 接口（`IsCompleted() bool` + `Status() string`）与 `ResultSentinelWithErr` 接口（`ResultSentinel` + `Err() error`）
- [x] 3.2 在 `lab/internal/web/api/status.go` 实现 `(*OpRecorder).RecordResult(op, detail string, res ResultSentinel)` —— 通过 type assert 走 `ResultSentinelWithErr` 或回退到 `status=<Status>`；nil 入参写 `errors.New("nil result")`
- [x] 3.3 在 `lab/internal/web/api/status_test.go`（新建或扩展既有测试）覆盖 4 个 scenario（COMPLETED ok=true / FAILED 派生 err / nil / 与原 `Record` 行为一致）

## 4. wiring 解耦

- [x] 4.1 修改 `lab/internal/wiring/daghttp.go`：删除 `if cfg.FixturesDir == "" { fixtures = "internal/daghttp/fixtures/graphs" }` 逻辑；改为 `cfg.FixturesDir == ""` 时返回 error 并提示 `lab/internal/daghttp.DefaultFixturesDir`
- [x] 4.2 在 `lab/internal/wiring/config.go` 更新 `DAGConfig` 注释，标注 `FixturesDir` 必填
- [x] 4.3 同步检查 `lab/configs/default.yaml` 中 `dag.fixtures_dir` 字段已存在；若缺失则补上

## 5. daghttp 切换

- [x] 5.1 在 `lab/internal/daghttp/handler.go` 新增导出常量 `DefaultFixturesDir = "internal/daghttp/fixtures/graphs"`
- [x] 5.2 删除 `lab/internal/daghttp/runtime.go` 整文件
- [x] 5.3 修改 `lab/internal/daghttp/handler.go`：
  - `Service.rt` 字段类型从 `*Runtime` 改为 `*app.StringApp`
  - `Service.idCounter` 字段删除；新增 `Service.idGen *app.EntityIDGen`
  - `NewService(rt *app.StringApp, graphID string)` 内部 `idGen = rt.NewEntityIDGen("http")`
  - `Echo` 中 `s.nextEntityID()` 改为 `s.idGen.Next()`
  - `Echo` 中终态映射改为 `return dagbridge.ResponseForTerminalResult(res), nil`
  - `Echo` 中 `s.rec.Record` 改为 `s.rec.RecordResult`
- [x] 5.4 修改 `lab/internal/wiring/daghttp.go`：调用 `daghttp.NewRuntime` 改为直接构造 `app.NewStringApp` + `app.NewStringApp.RegisterUnit` + 加载 fixture；返回 `*app.StringApp` 与 `*daghttp.Service`
- [x] 5.5 同步更新 `lab/cmd/lab-dag-http/main.go` 中 `rt` / `svc` 类型签名（如有依赖）
- [x] 5.6 修改 `lab/internal/daghttp/handler_test.go`：
  - `setupService` / `setupFailingService` 改为直接构造 `app.StringApp` + `RegisterUnit` + `LoadFixture` 后 `NewService`
  - `TestHandler_UsesAppFacade` 追加 `if strings.Contains(body, "atomic.Uint64") { t.Fatal("handler.go 不应自维护 entityID 计数器") }` 白名单

## 6. 验证

- [x] 6.1 根模块 `make test`（`go test ./...`）全绿
- [x] 6.2 lab 模块 `cd lab && go test ./...` 全绿
- [x] 6.3 `go vet ./...` 在根模块与 lab 模块均无 warning
- [x] 6.4 `go build ./...` 在根模块与 lab 模块均成功
- [x] 6.5 手工 `cd lab && ./bin/lab-dag-http serve` + `curl -X POST http://localhost:8086/echo -d 'hello'` 返回 200 + `echo:hello, hello`
- [x] 6.6 手工 `cd lab && ./bin/lab-dag-http serve` + `curl http://localhost:8086/api/status` 返回 JSON 含 `daghttp` 与 `loaded_graphs`
- [x] 6.7 跑 `openspec validate add-dag-app-stringapp-facade` 无错误
