## 1. 应用骨架与配置

- [x] 1.1 新建 `lab/app/dagsignal/config.go`：定义 `Config { Store string yaml:"store"; FixturesDir string yaml:"fixtures_dir" }`，常量 `DefaultFixturesDir = "app/dagsignal/fixtures/graphs"` 与 `defaultGraphID = "approval"`
- [x] 1.2 新建 `lab/app/dagsignal/serve.go`：实现 `LoadConfig() (*Config, error)`（解析 `LAB_CONFIG`/`configs/default.yaml` 的 `dagsignal` 段，应用 `LAB_DAGSIGNAL_STORE`/`LAB_DAGSIGNAL_FIXTURES_DIR` 覆写，`FixturesDir` 回退 `DefaultFixturesDir`）；结构与 daghttp `LoadConfig` 对齐
- [x] 1.3 在 `serve.go` 实现 `buildRuntime(cfg) (*app.StringApp, error)`：`app.NewStringApp(WithGraphDir(cfg.FixturesDir), WithLoaderDefaults("lab.Generic","v1"))` → `LoadGraphID("approval")`；当前仅支持 `store=memory`，其它返回 `unsupported dagsignal store` 错误；dagsignal 不注册 COMPUTE unit（演示焦点为 WAIT+signal）
- [x] 1.4 在 `serve.go` 实现 `Serve(ctx, addr, cfg) error`：`buildRuntime` + `defer rt.Close()` + `NewService(rt, defaultGraphID)` + `rpchttp.NewHTTPServer(addr)` + `svc.Register(srv)` + 打印 `lab-dag-signal listening on %s (POST /start)` + `srv.Start(ctx)`

## 2. fixture

- [x] 2.1 新建 `lab/app/dagsignal/fixtures/graphs/approval.yaml`：`graph_id: approval`，`entry: wait`，`wait` 节点 `kind: wait` + `signal: approval` + `deadline_seconds: 3600` + `on_timeout: failure`，transitions 含 `condition: signal: name: approval → success` 与 `condition: always → failure` 兜底；`success`/`failure` 为 terminal 节点

## 3. handler 与异步映射

- [x] 3.1 新建 `lab/app/dagsignal/handler.go`：定义 `Service struct { engine dag.Engine; typeKey *dagv1.EntityTypeKey; graphID string; rec *api.OpRecorder; idGen *app.EntityIDGen }`，`NewService(rt *app.StringApp, graphID string)` 经 `rt.Runtime.Memory().Engine()` 与 `rt.TypeKey()` 装配
- [x] 3.2 在 `handler.go` 定义包私有异步映射纯函数 `responseForInstance(inst *dagv1.EntityInstance) *rpcserver.Response`：WAITING→202、COMPLETED→200、FAILED/COMPENSATED/CANCELLED→500、RUNNING→202；body 为 JSON `{"entity_id","status","error"?}`；`inst==nil`→500；SHALL NOT 调用 `dagbridge.ResponseForTerminalResult`
- [x] 3.3 在 `handler.go` 实现 `Start(ctx, *rpcserver.Request)`：body 作 payload → `idGen.Next()` → `engine.StartInstance`（参考 `lab/internal/dag/runtime.go` 的 `Start` 与 `pkg/dag/memory/integration_test.go:80-124`）→ `engine.DrainInstance` → `GetInstance` → 映射响应；失败 → `rec.Record` + 500
- [x] 3.4 在 `handler.go` 实现 `Signal(ctx, *rpcserver.Request)`：从 path 取 `{entityID}`、从 query `?signal=` 取 signal 名（默认 `approval`）→ `engine.DeliverSignal(SignalDelivery{EntityId, SignalName, DeliveryId})` → `DrainInstance` → `GetInstance` → 映射；`ErrSignalMismatch`→400；实例不存在→404
- [x] 3.5 在 `handler.go` 实现 `Instances(ctx, *rpcserver.Request)`：从 path 取 `{entityID}` → `GetInstance` → 存在映射响应、不存在→404
- [x] 3.6 在 `handler.go` 实现 `Status` 与 `StatusInfo()`：`api.Status{Domain:"dagsignal", Impl:"memory", Healthy:true, RecentOps:rec.Snapshot(), Extra:map[string]any{"graph":graphID, "loaded_graphs":rt.LoadedGraphs()}, CollectedAt:time.Now()}`
- [x] 3.7 在 `handler.go` 实现 `Register(srv rpcserver.Server) error`：注册 `POST /start`、`POST /signal/{entityID}`、`GET /instances/{entityID}`、`GET /api/status`（路径参数用 rpc.Server 既有匹配语义；若不支持路径参数，回退为 `/signal?entity_id=` query 形式并记录决策）

## 4. CLI main

- [x] 4.1 新建 `lab/cmd/lab-dag-signal/main.go`：`defaultAddr = ":8087"`，子命令分发（`serve`/`-h`/`--help`/`help`），`serve` 内 `flag.NewFlagSet("serve")` 解析 `-addr`、`signal.NotifyContext`、`dagsignal.LoadConfig()`、`dagsignal.Serve(ctx, *addr, cfg)`；usage 文本列出 `POST /start`、`POST /signal/{entityID}`、`GET /instances/{entityID}`、`GET /api/status`

## 5. Makefile 与配置

- [x] 5.1 在 `lab/Makefile` 的 `MODULES` 末尾追加 `dagsignal`，并新增三行 `MODULE_dagsignal_BIN := lab-dag-signal` / `MODULE_dagsignal_PROFILES :=` / `MODULE_dagsignal_PORT := 8087`，依赖既有 `module-targets` foreach 模板自动生成 `lab-build-dag-signal`/`lab-up-dag-signal`/`lab-down-dag-signal`
- [x] 5.2 在 `lab/configs/default.yaml` 追加 `dagsignal:` 段（`store: memory` + `fixtures_dir: app/dagsignal/fixtures/graphs`），与既有 `dag:` 段并列

## 6. 测试

- [x] 6.1 新建 `lab/app/dagsignal/handler_test.go`：`setupService(t, graphID)` helper（构造 StringApp + LoadGraphID("approval") + NewService）；`TestStart_ReturnsWaiting`（202 + 非空 entity_id + status:WAITING）
- [x] 6.2 在 `handler_test.go` 加 `TestSignal_PromotesToCompleted`（start→signal→200 + status:COMPLETED）、`TestSignal_NameMismatch_Returns400`（`?signal=unknown`→400）、`TestSignal_UnknownEntity_Returns404`、`TestInstances_ReturnsStatus`、`TestInstances_UnknownEntity_Returns404`
- [x] 6.3 在 `handler_test.go` 加 `TestStatus_ContainsDomain`（body 含 `"dagsignal"`）、`TestHandler_NotUsesInvokeString`（AST/字符串扫描确认 handler.go 不调 `InvokeString` 与 `dagbridge.ResponseForTerminalResult`）、`TestRegister`（重复注册 `/start` 报错）
- [x] 6.4 新建 `lab/app/dagsignal/serve_test.go`：`TestServe_RegisterRoutes`（buildRuntime + Register 不报错）、`TestServe_RejectsUnsupportedStore`、`TestServe_RequiresFixturesDir`、`TestLoadConfig_AppliesEnvOverrides`（LAB_DAGSIGNAL_* 覆写）、`TestLoadConfig_AppliesDefaults`、`TestLoadConfig_FileNotFound`

## 7. 文档

- [x] 7.1 更新 `lab/README.md`：新增「DAG Signal HTTP 服务（lab-dag-signal）」章节（镜像 daghttp 章节，描述 start→signal→instances 闭环 + curl 示例），在 CLI 工具表加 `lab-dag-signal | 8087 | serve`，在按域验证示例补 `make lab-up-dag-signal`
- [x] 7.2 在 `lab/README.md` 架构图 `lab/internal/` 行补 `daghttp/`（既有）的姊妹说明，或在 `app/` 目录说明处补充 dagsignal；确保读者能从 README 识别两个 DAG 应用并列

## 8. 验证与归档

- [x] 8.1 `cd lab && go build ./...` 通过
- [x] 8.2 `cd lab && go vet ./...` 通过
- [x] 8.3 `cd lab && go test ./app/dagsignal/...` 全部通过
- [x] 8.4 `cd lab && go test ./cmd/lab-dag-signal/...` 通过（若有）
- [x] 8.5 `go run ./cmd/lab-dag-signal serve -h` 帮助文本包含 8087 与 `/start`、`/signal`、`/instances` 端点
- [x] 8.6 `make lab-build-dag-signal` 在 lab 子模块根目录下通过
- [x] 8.7 `make lab-up-dag-signal` 后 `curl -X POST http://localhost:8087/start -d hi` → 202 + entity_id；`curl -X POST http://localhost:8087/signal/{id}` → 200 + COMPLETED；`curl http://localhost:8087/instances/{id}` → 200；`curl http://localhost:8087/api/status` 含 `"dagsignal"`
- [x] 8.8 `openspec validate add-dagsignal-lab-app --strict` 通过
- [x] 8.9 归档：`openspec archive add-dagsignal-lab-app --yes`（archive 时合并本变更 specs delta 至 `openspec/specs/uniface-lab/spec.md`，注意 archive 规则检查无敏感信息）
