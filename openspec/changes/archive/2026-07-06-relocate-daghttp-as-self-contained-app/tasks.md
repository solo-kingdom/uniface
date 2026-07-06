## 1. 创建 `lab/app/daghttp/` 应用目录骨架

- [x] 1.1 在 `lab/app/daghttp/` 下创建 `fixtures/graphs/echo.yaml`（从 `lab/internal/daghttp/fixtures/graphs/echo.yaml` `git mv` 到此）
- [x] 1.2 暂保留 `lab/internal/daghttp/handler.go` 与 `handler_test.go` 原位，验证 `cd lab && go build ./...` 仍通过（双份存在期）

## 2. 同包内聚 handler 与 fixtures

- [x] 2.1 `git mv lab/internal/daghttp/handler.go lab/app/daghttp/handler.go` 并修改包声明为 `package daghttp`
- [x] 2.2 `git mv lab/internal/daghttp/handler_test.go lab/app/daghttp/handler_test.go` 并修改包声明为 `package daghttp`
- [x] 2.3 更新 `handler.go` 与 `handler_test.go` 中的导入路径（移除对 `lab/internal/daghttp` 自身的引用，改为同包）
- [x] 2.4 验证 `cd lab && go build ./app/daghttp/...` 与 `go test ./app/daghttp/...` 通过

## 3. 配置 schema 内聚到 daghttp 包

- [x] 3.1 新建 `lab/app/daghttp/config.go`，包含 `Config { Store string; FixturesDir string }` 与 yaml 标签 `store` / `fixtures_dir`
- [x] 3.2 在 `lab/internal/wiring/config.go` 中删除 `DAGConfig` 类型定义，将 `LabConfig.DAG` 字段改为 `daghttp.Config` 类型，import `lab/app/daghttp`
- [x] 3.3 验证 `cd lab && go build ./...` 通过

## 4. 装配与 unit 函数内聚

- [x] 4.1 新建 `lab/app/daghttp/units.go`：定义包私有 `helloFunc` / `echoFunc` / `registerUnits(sa *app.StringApp) error`，逻辑与 `wiring/daghttp.go` 原 `registerLabUnits` 一致
- [x] 4.2 新建 `lab/app/daghttp/serve.go`：定义 `LoadConfig() (*Config, error)`（解析 `LAB_CONFIG` 或 `configs/default.yaml` 中 `dag` 段并应用 `LAB_DAG_*` 环境变量覆写）与 `Serve(ctx context.Context, addr string, cfg *Config) error`（构建 StringApp + 注册 unit + 加载 echo fixture + 注册路由到 `*rpcserver.Server` + Start）
- [x] 4.3 `Serve` 内部使用 `defer rt.Close()` 处理启动失败兜底；保留原 `lab-dag-http listening on %s` 启动日志格式

## 5. 删除旧装配路径

- [x] 5.1 删除 `lab/internal/wiring/daghttp.go`（其全部 daghttp 专属代码已迁出）
- [x] 5.2 删除 `lab/internal/daghttp/` 整个目录（迁移完成后已为空壳）
- [x] 5.3 全文搜 `lab/internal/daghttp` 与 `wiring.NewDAGHTTP` / `wiring.DAGConfig` 残留引用，确认为 0

## 6. 重写 CLI main

- [x] 6.1 重写 `lab/cmd/lab-dag-http/main.go`：仅保留 `flag.NewFlagSet("serve", ...)` 解析 `-addr`、信号 `ctx`、`daghttp.LoadConfig()` 调用、`fmt.Println("lab-dag-http listening on ...")` 之前的 `daghttp.Serve(ctx, *addr, cfg)` 调用
- [x] 6.2 保留原 usage 帮助文本与子命令分发（`serve` / `-h` / `--help` / 默认报错）

## 7. spec 同步

- [x] 7.1 更新 `openspec/specs/uniface-lab/spec.md` 中「DAG HTTP 服务验证 CLI」与「DAG HTTP 按域生命周期」Requirements 内所有 `lab/internal/daghttp` 路径引用为 `lab/app/daghttp`
- [x] 7.2 新增「lab/app/ 顶级目录承载自包含应用」Requirement 至上游 spec（应用 `ADDED Requirements`）；同步从本变更的 `specs/uniface-lab/spec.md` 移除对应章节（archive 时由 openspec 自动 merge）

## 8. 验证与归档

- [x] 8.1 `cd lab && go build ./...` 通过
- [x] 8.2 `cd lab && go vet ./...` 通过
- [x] 8.3 `cd lab && go test ./app/daghttp/...` 通过（含原有 handler_test 用例，不应有测试覆盖退化）
- [x] 8.4 `go run ./cmd/lab-dag-http serve -h` 帮助文本与变更前一致
- [x] 8.5 `go run ./cmd/lab-dag-http serve` 后 `curl -X POST http://localhost:8086/echo -d 'hello'` 返回 200 与 `echo:hello, hello`
- [x] 8.6 `go run ./cmd/lab-dag-http serve` 后 `curl http://localhost:8086/api/status` 返回包含 `"domain":"daghttp"` 的 JSON
- [x] 8.7 `make lab-build-dag-http` 在 lab 子模块根目录下通过
- [x] 8.8 `openspec validate relocate-daghttp-as-self-contained-app --strict` 通过；归档后 `openspec archive relocate-daghttp-as-self-contained-app --yes`
