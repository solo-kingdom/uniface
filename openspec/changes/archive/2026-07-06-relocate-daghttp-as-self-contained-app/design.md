## Context

`lab-dag-http` 是「HTTP→DAG→响应」编排范式的最小验证 CLI，当前代码布局如下：

```
lab/
├── cmd/lab-dag-http/main.go         # flag + 信号 + 调 wiring.NewDAGHTTP
├── internal/daghttp/
│   ├── handler.go                   # Service + Echo + Status
│   ├── handler_test.go              # Echo/Status 单元测试
│   └── fixtures/graphs/echo.yaml    # echo DAG 图
└── internal/wiring/
    ├── daghttp.go                   # NewDAGHTTP / registerLabUnits / helloFunc / echoFunc
    └── config.go                    # LabConfig { DAG DAGConfig {...} }
```

问题：
- `lab/internal/daghttp/` 同时存在 handler、fixture、与 `lab/internal/wiring/daghttp.go` 中的 StringApp 装配 / lab 业务单元，呈「应用碎片跨包」状态
- `lab/internal/` 语义上是跨域基础设施位置，daghttp 这种独立应用被混在其中易让读者误解为「内部共享库」
- `DAGConfig` schema 跨域住在 `wiring` 包，但它的所有字段（`Store`、`FixturesDir`）只服务于 daghttp
- 外部读者要拼出 daghttp 完整边界需要在 `cmd/...`、`internal/daghttp/...`、`internal/wiring/daghttp.go` 三处来回跳转

约束：
- 不修改 `pkg/` 下任何公共 API
- 不改变 lab CLI 的对外行为（端口 8086、`POST /echo`、`GET /api/status` 不变）
- 不为本次重构引入新外部依赖
- `lab/Makefile` / 根 `Makefile` / `docker-compose.yml` 中按域目标与现有依赖不动

## Goals / Non-Goals

**Goals:**

- 把 daghttp 重组为单一「自包含应用」，根目录定位为 `lab/app/daghttp/`
- 让「除了 `pkg/` 基础包外的 daghttp 代码」全部落在该应用下，外部读者只看一棵目录就能识别全部边界
- 在 `lab/app/daghttp` 内补齐缺失的「应用门面」：`Serve(ctx, addr, cfg) error` 一行启动 CLI 业务
- 把 `DAGConfig` schema 迁移至 `lab/app/daghttp/config.go`，应用自治其配置结构
- 把 `wiring/daghttp.go` 中 daghttp 专属内容（`NewDAGHTTP`、`registerLabUnits`、`helloFunc`、`echoFunc`）内聚到 daghttp 包
- 精简 `lab/cmd/lab-dag-http/main.go`：仅保留 flag + 信号 + 调 `daghttp.Serve`
- 保持现有 `lab-dag-http serve` 的行为、端口、路由、日志格式完全一致

**Non-Goals:**

- 不修改 `lab/cmd/lab-dag-http/main.go` 的 CLI 名称 / 子命令 (`serve`) / 帮助文本风格
- 不调整 `lab-kv` / `lab-config` / `lab-lb` / `lab-queue` / `lab-dag` / `lab-ui` 的位置与装配
- 不重构 `LabConfig` / `KVConfig` / `LBConfig` / `QueueConfig` / `ServicesConfig` 跨域结构
- 不为本次重构引入新传输 / 协议 / 持久化后端
- 不拆分 `daghttp.Service` 现有接口边界
- 不改 fixture YAML 格式
- 不为新增 `app/` 顶级目录写 README（仅就本变更落地，其余 lab 应用暂不跟随迁移）

## Decisions

### D1: 新建 `lab/app/` 顶级目录承载「应用自治」

**决策**: 在 `lab/` 下新建 `app/` 子目录承载「自包含 lab 应用」。本次仅迁 `daghttp` 一个，未来如有需要可继续迁其它 lab CLI。`internal/` 仍保留跨域共享基础设施（`wiring`、`web/api`、`fixtures`、`conformance`、`dag/` 等）。

**理由**:
- 「应用」与「基础设施」在语义上正交：前者是端到端可部署单元，后者是跨应用复用部件
- 顶层 `app/` 比 `applications/` 短；与 Go 生态惯用 `cmd/` `internal/` `pkg/` 命名对齐（避免引入新模式）
- 暂不迁移其他 lab CLI 降低本次变更范围；如确认收益再统一迁，避免提前预定

**替代**:
- 直接放 `lab/daghttp/` 而不新增 `app/` 中间层 —— 放弃，单应用扁平布局可读但跨应用上下文缺乏统一前缀
- 把所有 lab CLI 一次性迁到 `lab/app/<name>/` —— 放弃，超出本变更范围；与「封装程度验收聚焦 daghttp」目标冲突
- 保留 `lab/internal/daghttp/` 但内聚 `wiring/daghttp.go` —— 放弃，路径仍误读（仍躺在 `internal/`）

### D2: `lab/app/daghttp` 暴露 `Serve` + `LoadConfig` 双入口

**决策**: 导出 `Serve(ctx context.Context, addr string, cfg *Config) error` 与 `LoadConfig() (*Config, error)`，使 `lab/cmd/lab-dag-http/main.go` 缩到 ~30 行。

```go
// lab/app/daghttp/serve.go
func LoadConfig() (*Config, error) { /* 解析 LAB_CONFIG / configs/default.yaml 等 */ }
func Serve(ctx context.Context, addr string, cfg *Config) error {
    rt, svc, err := buildRuntime(cfg)
    if err != nil { return err }
    defer rt.Close()
    srv := rpchttp.NewHTTPServer(addr)
    if err := svc.Register(srv); err != nil { return err }
    fmt.Printf("lab-dag-http listening on %s (POST /echo)\n", addr)
    return srv.Start(ctx)
}
```

**理由**:
- `Serve` 封装 StringApp 装配 + fixture 加载 + unit 注册 + StringApp 自动 close 兜底 + rpc.Server 注册 + 启停，避免 `cmd/main.go` 复刻这些步骤
- `LoadConfig` 仍保留在 daghttp 包自治其配置 schema（DAGConfig 全文迁入），但允许未来 `cmd/lab-dag-http/main.go` 直接用，无需跨包 import
- 端口拼接 / 启动日志保留与现状一致（`lab-dag-http listening on ...`）

**替代**:
- 暴露 `Builder` 让 cmd 自己 Build —— 放弃，增加 API 表面且调用方仍需操心 close 兜底
- 让 main 直接持有 StringApp —— 放弃，复刻 wiring 现状，无收益

### D3: `DAGConfig` 迁入 `lab/app/daghttp/config.go`

**决策**: 把 `wiring.DAGConfig { Store, FixturesDir }` 迁入 `lab/app/daghttp.Config`，跨域 `LabConfig.DAG` 字段类型改为 `daghttp.Config`。

**理由**:
- `DAGConfig` 仅服务于 daghttp；其全部字段（store、fixtures 目录）都是 daghttp 自治概念
- 跨域 `LabConfig.DAG` 仅作为「聚合入口」引用回具体 schema 是合理的依赖方向（应用 ↔ 共享配置）
- `lab/internal/web/api` 等不依赖 DAGConfig，迁出无破坏

**替代**:
- 保留 `wiring.DAGConfig` —— 放弃，应用层与基础设施层耦合
- 给 `LabConfig.DAG` 类型直接换成 map[string]string —— 放弃，失去 schema 校验

### D4: 单元 / 装配 / fixture / handler 同包

**决策**: 所有 daghttp 专属代码统一包名 `package daghttp`，落在 `lab/app/daghttp/` 下多文件（`config.go` / `serve.go` / `units.go` / `handler.go`）。

**理由**:
- Go 同包内不跨目录边界共享未导出符号是天然内聚单元
- `lab.hello` / `lab.echo` 单元、`registerLabUnits`、`helloFunc` / `echoFunc` 实现细节全部私有（包内函数），handler 不感知
- fixture 通过相对路径 `fixtures/graphs/echo.yaml` 同包访问，无需环境变量污染

**替代**:
- 拆 `lab/app/daghttp/internal` 子目录包 —— 放弃，单层扁平对小应用更易读
- 把 unit 放子包 `units/` —— 放弃，仅 2 个 unit，拆分收益不抵目录层级代价

## Risks / Trade-offs

- **路径迁移风险（既有测试与脚本）** → Mitigation: 仅 lab 模块内部路径变更；先全局搜 `lab/internal/daghttp` 与 `wiring.NewDAGHTTP`、`wiring.DAGConfig` 引用面，更新导入语句；同时更新 `openspec/specs/uniface-lab/spec.md` 路径引用
- **`wiring/daghttp.go` 删除后空壳被引用** → Mitigation: 迁移完两个文件后再次执行 `go build ./...` + `go vet ./...` 确认无未解析引用
- **新 `lab/app/` 顶级目录与既有目录语义冲突** → Mitigation: 在本次完成后于 `lab/README.md` 标注 `app/` 用途；本变更不强制其它 lab CLI 迁移
- **godoc / pkg.go.dev 路径变化** → Mitigation: 仅 lab 子模块内部路径，godoc 不展示 lab（lab 是独立 Go module）
- **`DAGConfig` 字段顺序错位导致 yaml 解析差异** → Mitigation: 字段名 (`Store` / `FixturesDir`) 与 yaml 标签 (`store` / `fixtures_dir`) 一比一对应迁移

## Migration Plan

1. **先行**：在 `lab/app/daghttp/` 新建空包，确保 `go build ./...` 仍通过（先并行不删原文件）
2. **同包搬迁**：以 `git mv` 把 `lab/internal/daghttp/handler.go` / `handler_test.go` / `fixtures/` 移入 `lab/app/daghttp/`，同时把包名声明改为一致
3. **装配内聚**：把 `wiring/daghttp.go` 切分为 `lab/app/daghttp/units.go`（含 `registerLabUnits` / `helloFunc` / `echoFunc`）与 `lab/app/daghttp/serve.go`（含 `Serve` / `LoadConfig` / 装配），原 `wiring/daghttp.go` 删除
4. **配置迁移**：把 `DAGConfig` 从 `wiring/config.go` 迁入 `lab/app/daghttp/config.go`；`LabConfig.DAG` 字段类型改为 `daghttp.Config`，移除原 `wiring.DAGConfig`
5. **重写 main.go**：`lab/cmd/lab-dag-http/main.go` 改为 `flag` + `signal.NotifyContext` + `daghttp.LoadConfig()` + `daghttp.Serve(ctx, *addr, cfg)`
6. **spec 同步**：更新 `openspec/specs/uniface-lab/spec.md` 中 DAG HTTP 章节的路径引用
7. **回归**：`cd lab && go build ./... && go test ./app/daghttp/... && go test ./cmd/lab-dag-http/...`（若未来补 cmd 测试）；执行 `make lab-build-dag-http` 与 `lab-dag-http serve -h` 验证 CLI 帮助文本不变

回滚策略：单原子 PR；任一步失败可通过 `git revert` 整体回滚。

## Open Questions

- 是否本变更后需为其它 lab CLI（lab-kv、lab-config、lab-lb、lab-queue、lab-dag、lab-ui）开启跟进迁移到 `lab/app/<name>/` 的 follow-up 变更？本次先不答，留作后续评估
- `lab/internal/dag` 那套 698 行验证台是否一并迁？**Non-goals** 已声明不动
