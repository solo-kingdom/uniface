## Context

uniface 根模块保持零依赖，各实现（redis、boltdb、aerospike、consul、kafka、nats 等）为独立子模块。现有验证分散在 `pkg/*/_test.go` 中：轻量、断言驱动、部分依赖 `skipIfNoXxx` 跳过。缺少一个**独立可运行**的工程来：

1. 在真实中间件环境下手动/交互式验证五类能力
2. 演示**实现切换**（uniface 核心卖点）
3. 提供可观测 Dashboard，而非仅 `t.Fatal` 输出

探索阶段已确认：验证工程命名为 `lab/`，形态为 5 个独立 CLI + 1 个 `lab-ui`，五能力域全开，不含业务语义（DAG 用通用 fixture 图）。

## Goals / Non-Goals

**Goals:**

- 新增 `lab/` 独立 Go 子模块，通过 `replace` 引用主库及实现子模块
- 提供 `lab-kv`、`lab-config`、`lab-lb`、`lab-queue`、`lab-dag` 五个 CLI，各域可独立运行
- 各 CLI `serve` 模式暴露 HTTP API（:8081–:8085），供 `lab-ui` 聚合
- `lab-ui`（:3000）提供简单 Web Dashboard，展示五域状态与操作结果
- `internal/wiring/` 支持配置/环境变量切换实现
- `docker-compose.yml` + `make lab-up` 一键启动验证环境
- 根 `Makefile`/`tag.sh` 排除 lab，不污染主库发布

**Non-Goals:**

- 业务应用（订单、电商等）
- 修改 `pkg/` 公开接口
- 重量级前端（React/Vue）或 node 构建链
- DAG 分布式 Worker / 多服务拆分
- lab 模块版本 tag 或作为库发布

## Decisions

### D1: 目录与模块路径

**决策**: `lab/` 为独立子模块 `github.com/solo-kingdom/uniface/lab`。

```
lab/
├── go.mod
├── cmd/
│   ├── lab-kv/ lab-config/ lab-lb/ lab-queue/ lab-dag/ lab-ui/
├── internal/
│   ├── wiring/ kv/ config/ lb/ queue/ dag/
│   ├── conformance/ fixtures/ web/
├── configs/default.yaml
└── docker-compose.yml
```

**理由**: 与 redis、aerospike 等子模块模式一致；重依赖隔离在 lab，根模块零依赖不变。

**替代方案**: 外部仓库 `uniface-lab`。放弃——monorepo 内 replace 开发体验更好。

### D2: CLI 独立 + serve 模式

**决策**: 每个能力域一个二进制；`serve` 子命令启动 HTTP API + 后台状态收集。

| 工具 | 默认端口 | 职责 |
|------|----------|------|
| lab-kv | 8081 | KV CRUD、conformance、实现切换 |
| lab-config | 8082 | Config CRUD、watch 事件流 |
| lab-lb | 8083 | 实例管理、选择模拟、算法切换 |
| lab-queue | 8084 | 发布/订阅、bench、实现切换 |
| lab-dag | 8085 | 图加载、实例管理、signal、journal |
| lab-ui | 3000 | Dashboard 聚合 |

**理由**: 用户要求多个独立 CLI；serve 模式解耦 UI 与域逻辑，可单独调试某一域。

**替代方案**: 单一 `lab` 二进制 + 子命令。放弃——独立二进制边界更清晰，符合用户选择。

### D3: Web UI 技术栈

**决策**: `net/http` + `go:embed` 静态 HTML + htmx 局部刷新；各域 API 返回 JSON，UI 通过 fetch/htmx 轮询。

**理由**: 零前端构建链；「简单看状态」足够；Go 单二进制部署。

**替代方案**: React SPA。放弃——过重，与 Non-goals 冲突。

### D4: wiring 工厂层

**决策**: `configs/default.yaml` 定义默认实现；环境变量 `LAB_<DOMAIN>_IMPL` 覆盖；切换实现需重启对应 CLI。

```yaml
kv:      { impl: redis, addr: localhost:6379 }
config:  { impl: consul, addr: localhost:8500 }
lb:      { algo: roundrobin }
queue:   { impl: nats, addr: localhost:4222 }
dag:     { store: memory }
```

**理由**: 直观演示热切换；与 uniface Options 模式一致。

### D5: DAG 验证方式

**决策**: `fixtures/graphs/` 存放通用 YAML 图（echo、saga_compensate、fork_join）；内置通用 ComputeUnit（echo、fail_once、rollback）；订单图不作为 fixture。

**理由**: DAG 是通用框架，验证项目不应绑定业务场景；现有 `integration_test.go` 逻辑可提炼为通用 unit，保留在 pkg 内做回归。

### D6: 构建与发布隔离

**决策**:

- `make test` 不跑 lab；新增 `lab-build`、`lab-up`、`lab-down`
- `scripts/tag.sh` 的 `find go.mod` 排除 `./lab/`
- 可选 `go.work` 加入 lab（本地开发，不强制提交）

**理由**: lab 是验证工具，不是发布产物。

### D7: 分期交付顺序

**决策**: P0 骨架 → P1 KV+Config → P2 LB+Queue → P3 DAG → P4 打磨。五域全开但按阶段实现，每阶段可独立验收。

**理由**: 降低首轮复杂度；KV 切换故事最先可演示。

## Risks / Trade-offs

| 风险 | 缓解 |
|------|------|
| docker-compose 服务多，本地资源占用高 | compose profiles 按需启服务；README 说明最小子集 |
| 五 CLI + UI 六进程，启动复杂 | `make lab-up` 统一编排；lab-ui 显示连接失败提示 |
| lab 与 pkg 接口演进不同步 | lab 仅 consume 公开接口；CI 加 `lab-build` 编译检查 |
| htmx 功能有限 | 本期仅需状态展示；复杂交互走 CLI |
| DAG 仅 memory store | 配置注明限制；二期可接 KV 持久化 |

## Migration Plan

1. 创建 `lab/go.mod` 与目录骨架，不影响现有 pkg
2. 更新 Makefile、tag.sh（排除 lab）
3. 分阶段实现五域 CLI + lab-ui
4. 补充 `lab/README.md` 使用说明
5. 无需迁移或回滚——纯新增，删除 `lab/` 即可回退

## Open Questions

- CI 是否在默认 pipeline 跑 `lab-build`？（建议：是，不跑 `lab-up` 集成）
- `go.work` 是否提交到仓库？（建议：可选，文档说明本地用法）
