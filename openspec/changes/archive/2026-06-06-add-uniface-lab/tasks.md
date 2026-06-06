## 1. 骨架与基础设施

- [x] 1.1 创建 `lab/go.mod`，配置 replace 指向根模块及 redis/boltdb/aerospike/consul/kafka/nats/rabbitmq/natsjetstream 子模块
- [x] 1.2 创建目录骨架：`cmd/`、`internal/wiring`、`internal/web`、`configs/`、`internal/fixtures/graphs/`
- [x] 1.3 新增 `configs/default.yaml`，定义五域默认实现与连接参数
- [x] 1.4 实现 `internal/wiring/` 工厂层（kv、config、lb、queue、dag），支持 yaml + 环境变量覆盖
- [x] 1.5 新增 `lab/docker-compose.yml`（redis、consul、nats、kafka、rabbitmq、aerospike，含 profiles）
- [x] 1.6 更新根 `Makefile`：新增 `lab-build`、`lab-up`、`lab-down`；`make test` 排除 lab
- [x] 1.7 更新 `scripts/tag.sh`：`find go.mod` 排除 `./lab/`

## 2. 共享 Web 层

- [x] 2.1 实现 `internal/web/server.go`：基于 stdlib/chi 的 HTTP 服务框架
- [x] 2.2 实现 `internal/web/api/status.go`：统一 `/api/status` 响应结构（域名称、实现、健康、最近操作）
- [x] 2.3 实现 `internal/web/ui/`：嵌入式 HTML 模板 + htmx，含 index 首页与五域面板骨架

## 3. lab-kv

- [x] 3.1 实现 `cmd/lab-kv/main.go`：子命令 `set`、`get`、`delete`、`list`、`exists`、`run-conformance`、`serve`
- [x] 3.2 实现 `internal/kv/handler.go`：对接 `kv.Storage` 接口
- [x] 3.3 实现 `internal/conformance/kv.go`：跨实现一致性用例集（Set/Get/Delete/Batch/List/Exists）
- [x] 3.4 实现 `internal/kv/api.go`：serve 模式 HTTP 端点（status、operations、conformance result）
- [x] 3.5 补充 lab-ui KV 面板：展示实现、连接状态、最近操作、conformance 结果

## 4. lab-config

- [x] 4.1 实现 `cmd/lab-config/main.go`：子命令 `put`、`get`、`delete`、`watch`、`serve`
- [x] 4.2 实现 `internal/config/handler.go`：对接 `config.Storage` 接口（consul 实现）
- [x] 4.3 实现 `internal/config/api.go`：serve 模式 HTTP 端点（配置树、watch 事件 SSE/轮询）
- [x] 4.4 补充 lab-ui Config 面板：展示配置项与 watch 事件流

## 5. lab-lb

- [x] 5.1 实现 `cmd/lab-lb/main.go`：子命令 `add`、`remove`、`select`、`simulate`、`switch`、`serve`
- [x] 5.2 实现 `internal/lb/handler.go`：对接 `Balancer[interface{}]` 与 mock 客户端
- [x] 5.3 实现 `internal/lb/api.go`：serve 模式 HTTP 端点（实例列表、分布数据、shard 路由结果）
- [x] 5.4 补充 lab-ui LB 面板：展示实例、算法、选择分布

## 6. lab-queue

- [x] 6.1 实现 `cmd/lab-queue/main.go`：子命令 `publish`、`subscribe`、`bench`、`serve`
- [x] 6.2 实现 `internal/queue/handler.go`：对接 `Publisher`/`Subscriber` 接口，支持 JSON Codec
- [x] 6.3 实现 `internal/conformance/queue.go`：基础发布/订阅一致性用例（可选）
- [x] 6.4 实现 `internal/queue/api.go`：serve 模式 HTTP 端点（topic、最近消息、吞吐指标）
- [x] 6.5 补充 lab-ui Queue 面板：展示实现、topic、最近消息

## 7. lab-dag

- [x] 7.1 创建 `internal/fixtures/graphs/` 通用图：echo.yaml、saga_compensate.yaml、fork_join.yaml
- [x] 7.2 实现通用 ComputeUnit：echo、fail_once、rollback（不含订单业务）
- [x] 7.3 实现 `cmd/lab-dag/main.go`：子命令 `graph load`、`start`、`status`、`signal`、`journal`、`run-once`、`serve`
- [x] 7.4 实现 `internal/dag/handler.go`：对接 memory Engine + LineStore + Registry
- [x] 7.5 实现 `internal/dag/api.go`：serve 模式 HTTP 端点（实例列表、节点状态、journal、saga stack）
- [x] 7.6 补充 lab-ui DAG 面板：实例状态、journal 时间线、saga 栈

## 8. lab-ui 与一键启动

- [x] 8.1 实现 `cmd/lab-ui/main.go`：聚合五域 `/api/status`，默认端口 3000
- [x] 8.2 完善 Dashboard 首页：五域健康卡片、离线提示、刷新
- [x] 8.3 实现 `lab/Makefile`：`build`、`up`、`down` 脚本，编排 compose + 六进程
- [x] 8.4 验证 `make lab-up` 端到端：访问 Dashboard，五域均在线

## 9. 文档与 CI

- [x] 9.1 编写 `lab/README.md`：快速开始、各 CLI 用法、实现切换说明、 compose profiles
- [x] 9.2 根 README 或 CLAUDE.md 补充 lab 章节引用
- [x] 9.3 可选：CI 增加 `make lab-build` 编译检查步骤（仓库暂无 CI workflow，已在根 Makefile 提供 `lab-build` 目标）
