## Why

uniface 已沉淀 KV/Config/LB/Queue/DAG 五类能力接口，但「对外暴露服务」至今耦合在各 lab CLI 的手写 `net/http` + chi 样板里，缺少统一抽象，难以让同一套业务处理器在不同传输（HTTP、gRPC 等）间复用与热切换。同时，DAG 引擎仅作为「引擎验证台」被命令行驱动，缺少「用 DAG 编排一次请求处理」的端到端示例。本变更新增统一 RPC Server 抽象（`pkg/rpc/server`），并以独立 `lab-dag-http` 模块演示「HTTP 请求经 DAG 处理后返回」的请求编排范式。

## What Changes

- 新增 `pkg/rpc/server`：面向接口的统一服务抽象（`Server`、传输无关的 `Handler` 注册语义、Options、errors），定义启动、注册、优雅关闭生命周期，预留多传输扩展点；遵循根模块零依赖原则。
- 新增 `pkg/rpc/server/http`：基于标准库 `net/http` 的首个传输实现（标准库不引入外部依赖，保持根模块零依赖）。
- 新增独立 lab 模块 `lab-dag-http`：对外仅暴露 `POST /echo` 端点；每次请求包装为一个 `EntityInstance`，经线性 DAG 图（compute echo → terminal）排空到终态后，将终态 payload 作为 HTTP 响应返回。
- `lab-dag-http` 通过统一 `rpc.Server` 抽象启动，验证「同一 handler 可在不同传输间切换」的封装目标。
- `lab/Makefile` 与根 `Makefile` 增加按域目标 `lab-build-dag-http` / `lab-up-dag-http` / `lab-down-dag-http`（复用 lab-modular-targets 的域注册表机制）。
- 更新 `lab/README.md`、`CLAUDE.md`、`docs/`（镜像 `pkg/` 路径）。

## Capabilities

### New Capabilities

- `rpc-server`: 统一对外服务抽象契约——服务生命周期（启动/注册/优雅关闭）、传输无关的 Handler 注册语义、HTTP 传输实现契约与多传输扩展点。

### Modified Capabilities

- `uniface-lab`: 新增「DAG HTTP 服务验证 CLI」需求——独立 lab 模块以统一 rpc.Server 启动，echo 端点经 DAG 处理请求；并扩展「一键启动环境」的按域目标至新模块。

## Impact

- **代码**: 新增 `pkg/rpc/server/`（interface/options/errors）与 `pkg/rpc/server/http/`；新增 `lab/cmd/lab-dag-http/` 与 `lab/internal/daghttp/`（请求→实例适配 + 路由注册）。
- **构建脚本**: `lab/Makefile`、根 `Makefile`（`lab/docker-compose.yml` 无新增中间件）。
- **依赖**: 根模块保持零外部依赖（HTTP 实现仅用标准库 `net/http`）；lab 子模块沿用现有依赖。
- **兼容性**: 纯新增，不修改现有 lab CLI 接口与 `pkg/dag` 引擎。

## Non-goals

- gRPC 传输实现（仅在接口预留扩展点，待首个真实 gRPC 场景评估）。
- 修改现有 `lab-dag` 的引擎验证台职责或其 HTTP API。
- 异步/信号驱动的请求处理模型（本期仅同步排空到终态）。
- 持久化 LineStore、分布式 worker、并发限流/认证中间件。
- 修改 `prompts/` 目录。
