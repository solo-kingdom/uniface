## Why

uniface 作为基础设施抽象层，目前验证手段分散在各 `pkg` 包内的单元/冒烟测试中，缺少一个独立、可运行、可交互的验证工程来证明 KV、Config、Load Balancer、Queue、DAG 五类能力在真实环境下可用、可切换、可观测。需要一个与主库隔离的 `lab/` 工程，作为 living validation 平台，而非业务应用或订单 demo。

## What Changes

- 新增独立 Go 子模块 `lab/`（`github.com/solo-kingdom/uniface/lab`），通过 `replace` 引用主库及各实现子模块，不参与版本 tag 发布
- 新增 5 个独立 CLI 工具：`lab-kv`、`lab-config`、`lab-lb`、`lab-queue`、`lab-dag`，各覆盖一个 uniface 能力域
- 新增 `lab-ui` Web Dashboard，聚合展示五域运行状态与操作结果
- 新增 `internal/wiring/` 工厂层，支持通过配置/环境变量切换中间件实现
- 新增 `docker-compose.yml` 与 `Makefile` 目标（`lab-build`、`lab-up`、`lab-down`），一键启动验证环境
- 新增 DAG 通用 fixture 图（echo、saga、fork_join），不含订单等业务语义
- 更新根 `Makefile` 与 `scripts/tag.sh`，排除 `lab/` 模块

## Capabilities

### New Capabilities

- `uniface-lab`: uniface 能力验证台——独立 lab 模块、五域 CLI、`lab-ui` Dashboard、实现切换与一致性验证

### Modified Capabilities

（无——本变更不修改 `pkg/` 内既有接口契约，仅新增消费侧验证工程）

## Non-goals

- 不构建任何业务应用（订单、电商等）；DAG 仅用通用 fixture 图
- 不修改 `pkg/` 公开接口或既有实现的行为
- 不为 lab 创建版本 tag 或作为库发布
- 不引入重量级前端构建链（React/Vue 等）
- 不在本期实现 DAG 分布式 Worker 或多服务拆分

## Impact

- **新增目录**: `lab/`（cmd、internal、configs、fixtures）
- **构建**: 根 `Makefile` 增加 lab 目标；`make test` 默认不包含 lab
- **发布**: `scripts/tag.sh` 排除 `lab/` 子模块
- **依赖**: lab 模块可引入 chi/htmx 等轻量依赖，不污染根模块零依赖原则
- **CI**: 可选增加 `make lab-build` 编译检查；集成测试需 Docker 环境
