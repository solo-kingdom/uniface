# uniface-lab Specification

## Purpose
TBD - created by archiving change add-uniface-lab. Update Purpose after archive.
## Requirements
### Requirement: 独立 lab 子模块

系统 SHALL 在仓库根目录提供 `lab/` 独立 Go 子模块，通过 `replace` 引用 `github.com/solo-kingdom/uniface` 及各实现子模块。`make test` 与 `scripts/tag.sh` SHALL NOT 包含 lab 模块。

#### Scenario: lab 模块独立构建
- **WHEN** 在 `lab/` 目录执行 `go build ./...`
- **THEN** 编译成功，且不修改根模块 `go.mod` 的依赖

#### Scenario: tag 排除 lab
- **WHEN** 执行 `scripts/tag.sh vX.Y.Z --dry-run`
- **THEN** 输出 tag 列表不包含 `lab/vX.Y.Z`

### Requirement: KV 验证 CLI

系统 SHALL 提供 `lab-kv` 命令行工具，支持 KV 存储的 CRUD、List、Exists 操作，并支持在 redis、boltdb、aerospike 实现间通过配置切换。

#### Scenario: KV 写入与读取
- **WHEN** 执行 `lab-kv set --key foo --value bar` 后执行 `lab-kv get --key foo`
- **THEN** 输出值为 `bar`

#### Scenario: KV 实现切换
- **WHEN** 配置 `kv.impl` 从 `redis` 改为 `boltdb` 并重启 `lab-kv serve`
- **THEN** 工具使用 BoltDB 实现，业务命令接口不变

#### Scenario: KV conformance 运行
- **WHEN** 执行 `lab-kv run-conformance`
- **THEN** 对当前实现运行一致性用例集并输出通过/失败结果

#### Scenario: KV serve 模式
- **WHEN** 执行 `lab-kv serve`
- **THEN** 在默认端口 8081 暴露 HTTP API，返回当前实现状态与最近操作记录

### Requirement: Config 验证 CLI

系统 SHALL 提供 `lab-config` 命令行工具，支持配置的 Put、Get、Delete、Watch、WatchPrefix 操作，默认使用 consul 实现。

#### Scenario: Config 写入与读取
- **WHEN** 执行 `lab-config put --key app/name --value myapp` 后执行 `lab-config get --key app/name`
- **THEN** 输出值为 `myapp`

#### Scenario: Config watch 事件
- **WHEN** 执行 `lab-config watch --prefix app/` 后另一终端修改 `app/name`
- **THEN** watch 终端输出变更事件

#### Scenario: Config serve 模式
- **WHEN** 执行 `lab-config serve`
- **THEN** 在默认端口 8082 暴露 HTTP API，包含配置树与 watch 事件流

### Requirement: Load Balancer 验证 CLI

系统 SHALL 提供 `lab-lb` 命令行工具，支持实例 Add/Remove/Update、Select 选择、算法切换（roundrobin、random、weighted、consistenthash）及选择分布模拟。

#### Scenario: 实例注册与选择
- **WHEN** 注册两个实例后执行 `lab-lb select --key user-1`
- **THEN** 返回一个已注册实例的 ID

#### Scenario: 算法切换
- **WHEN** 配置 `lb.algo` 从 `roundrobin` 改为 `consistenthash` 并重启
- **THEN** 相同 key 的选择结果具有确定性

#### Scenario: 选择分布模拟
- **WHEN** 执行 `lab-lb simulate --n 1000`
- **THEN** 输出各实例被选中的次数分布

#### Scenario: LB serve 模式
- **WHEN** 执行 `lab-lb serve`
- **THEN** 在默认端口 8083 暴露 HTTP API，包含实例列表与分布数据

### Requirement: Queue 验证 CLI

系统 SHALL 提供 `lab-queue` 命令行工具，支持消息 Publish、Subscribe、BatchPublish，并支持在 kafka、nats、rabbitmq、natsjetstream 实现间切换。

#### Scenario: 消息发布与订阅
- **WHEN** 终端 A 执行 `lab-queue subscribe --topic demo`，终端 B 执行 `lab-queue publish --topic demo --body '{"msg":"hi"}'`
- **THEN** 终端 A 收到消息

#### Scenario: Queue 实现切换
- **WHEN** 配置 `queue.impl` 从 `nats` 改为 `kafka` 并重启
- **THEN** 工具使用 Kafka 实现，命令接口不变

#### Scenario: Queue serve 模式
- **WHEN** 执行 `lab-queue serve`
- **THEN** 在默认端口 8084 暴露 HTTP API，包含当前实现、topic 与最近消息

### Requirement: DAG 验证 CLI

系统 SHALL 提供 `lab-dag` 命令行工具，支持加载通用 fixture 图、启动实例、查询状态、注入信号、查看 journal 与 saga 状态。工具 SHALL NOT 绑定订单等业务语义。

#### Scenario: 加载通用图并启动实例
- **WHEN** 执行 `lab-dag graph load --file fixtures/graphs/echo.yaml` 后执行 `lab-dag start --graph echo --entity-id inst-001`
- **THEN** 创建实例并返回 RUNNING 或后续状态

#### Scenario: 信号注入
- **WHEN** 实例处于 WAITING 状态，执行 `lab-dag signal --entity-id inst-001 --signal approve`
- **THEN** 实例继续执行

#### Scenario: Journal 查询
- **WHEN** 执行 `lab-dag journal --entity-id inst-001`
- **THEN** 输出该实例的 journal 条目列表

#### Scenario: DAG serve 模式
- **WHEN** 执行 `lab-dag serve`
- **THEN** 在默认端口 8085 暴露 HTTP API，包含实例列表、当前节点与 journal

### Requirement: Web Dashboard

系统 SHALL 提供 `lab-ui` 进程，在默认端口 3000 提供 Web Dashboard，聚合展示五域 CLI 的连接状态、当前实现/算法、最近操作与错误信息。

#### Scenario: Dashboard 首页
- **WHEN** 五域 CLI 均以 serve 模式运行，访问 `http://localhost:3000`
- **THEN** 页面展示 KV、Config、LB、Queue、DAG 五个域的健康状态卡片

#### Scenario: 域面板详情
- **WHEN** 在 Dashboard 点击 KV 面板
- **THEN** 展示当前 KV 实现、连接状态、最近操作记录

#### Scenario: 域离线提示
- **WHEN** 某域 CLI 未启动
- **THEN** Dashboard 对应卡片显示离线状态，不导致整页崩溃

### Requirement: 一键启动环境

系统 SHALL 提供 `docker-compose.yml` 与 Makefile 目标，支持一键构建并启动验证环境，并支持按能力域（`kv`、`config`、`lb`、`queue`、`dag`、`ui`）独立构建、启动与关停。

#### Scenario: 构建 lab 二进制
- **WHEN** 执行 `make lab-build`
- **THEN** 编译 lab-kv、lab-config、lab-lb、lab-queue、lab-dag、lab-ui 六个二进制

#### Scenario: 启动验证环境
- **WHEN** 执行 `make lab-up`
- **THEN** 启动 docker-compose 中间件（按需）、五域 serve 进程与 lab-ui

#### Scenario: 关停验证环境
- **WHEN** 执行 `make lab-down`
- **THEN** 停止 lab 相关进程与 compose 服务

#### Scenario: 按域构建
- **WHEN** 执行 `make lab-build-dag` 或 `make lab-build LAB_MODULES=dag`
- **THEN** 仅编译 `lab-dag` 二进制，不编译其他 lab 工具

#### Scenario: 按域启动
- **WHEN** 执行 `make lab-up-dag` 或 `make lab-up LAB_MODULES=dag`
- **THEN** 仅启动 `lab-dag serve` 进程，不启动其他域 serve 进程；且不启动 DAG 不需要的 compose 中间件

#### Scenario: 按域关停
- **WHEN** 已执行 `make lab-up-dag`，随后执行 `make lab-down-dag` 或 `make lab-down LAB_MODULES=dag`
- **THEN** 仅停止 `lab-dag` 进程，不影响其他仍在运行的 lab 域进程

#### Scenario: 多域选择
- **WHEN** 执行 `make lab-up LAB_MODULES=kv,dag`
- **THEN** 启动 `lab-kv` 与 `lab-dag` 及其各自需要的 compose profile（`kv` 域启动 redis 相关服务，`dag` 域不启动额外 compose 服务）

### Requirement: 配置与 wiring

系统 SHALL 通过 `configs/default.yaml` 定义各域默认实现与连接参数，并支持环境变量 `LAB_<DOMAIN>_IMPL` 覆盖。实现切换 SHALL 在重启对应 CLI 后生效。

#### Scenario: 默认配置加载
- **WHEN** `lab-kv serve` 启动且未设置环境变量
- **THEN** 使用 `configs/default.yaml` 中 `kv` 段的配置

#### Scenario: 环境变量覆盖
- **WHEN** 设置 `LAB_KV_IMPL=boltdb` 后启动 `lab-kv serve`
- **THEN** 使用 BoltDB 实现，忽略 yaml 中的 impl 值

### Requirement: DAG HTTP 服务验证 CLI

系统 SHALL 提供独立 lab 模块 `lab-dag-http`（`lab/cmd/lab-dag-http`），对外仅暴露 `POST /echo` 端点，并通过统一 `pkg/rpc/server` 抽象启动（SHALL NOT 直接手写 `net/http` 样板）。每次 `/echo` 请求 SHALL 包装为一个独立 `EntityInstance`，经 `lab-dag-http` 自有 echo 图排空到终态后，将终态 payload 作为响应体返回；`COMPLETED` 映射 HTTP 200，`FAILED`/`COMPENSATED` 映射 HTTP 500 并附失败原因。

`lab-dag-http` SHALL 与 `lab/internal/dag` 验证台完全隔离：不得复用 `lab/internal/dag.Runtime`、`lab/internal/dag` fixtures 或其 HTTP API。`lab-dag-http` SHALL 优先复用根模块公共 `pkg/dag/invocation` 请求式轻量封装装配其验证所需的 graph、entity type 与 compute units；当需要验证底层能力时 MAY 继续直接使用公共 Runtime/Invoker/Loader/Codec 抽象。

#### Scenario: echo 请求经 DAG 返回

- **WHEN** 执行 `curl -X POST http://localhost:8086/echo -d 'hello'`
- **THEN** 响应状态码为 200
- **AND** 响应体为 `lab-dag-http` 自有 echo 图终态 payload

#### Scenario: 终态失败映射 5xx

- **WHEN** 一个请求经 DAG 排空后实例终态为 `FAILED` 或 `COMPENSATED`
- **THEN** 响应状态码为 500，响应体包含失败原因

#### Scenario: 通过统一 Server 抽象启动

- **WHEN** 执行 `lab-dag-http serve`
- **THEN** 服务经 `pkg/rpc/server` 的 `Server` 抽象启动并监听 8086，而非进程内直接调用 `http.ListenAndServe`

#### Scenario: 与 lab-dag 运行时隔离

- **WHEN** 同时启动 `lab-dag serve` 与 `lab-dag-http serve`
- **THEN** 两个进程使用各自独立的 DAG runtime、fixtures 与 HTTP API
- **AND** `lab-dag-http` 不依赖 `lab/internal/dag.Runtime`

#### Scenario: 使用轻量请求式封装装配

- **WHEN** `lab-dag-http` 注册 echo 图、`lab.hello` 与 `lab.echo` 计算单元
- **THEN** 装配代码 SHALL 通过公共请求式 DAG 轻量封装完成常见注册、图加载和 string payload 调用
- **AND** 业务 handler 不需要直接构造 `invocation.InvokeRequest` 或手写 `anypb.Any` payload 编解码

### Requirement: DAG HTTP 按域生命周期

系统 SHALL 将 `daghttp` 纳入 lab 域注册表（二进制 `lab-dag-http`，默认端口 `8086`，无 compose 中间件依赖），并提供按域目标 `lab-build-dag-http`、`lab-up-dag-http`、`lab-down-dag-http`，行为与既有域目标一致。

#### Scenario: 按域启动 DAG HTTP

- **WHEN** 执行 `make lab-up-dag-http`
- **THEN** 仅构建并启动 `lab-dag-http serve` 进程，不启动其他域进程，也不启动任何 compose 中间件

#### Scenario: 按域关停

- **WHEN** 已执行 `make lab-up-dag-http`，随后执行 `make lab-down-dag-http`
- **THEN** 仅停止 `lab-dag-http` 进程，不影响其他运行中的 lab 域进程

#### Scenario: 多域组合包含 DAG HTTP

- **WHEN** 执行 `make lab-up LAB_MODULES=dag,daghttp`
- **THEN** 同时启动 `lab-dag` 与 `lab-dag-http` 两个进程，且不启动额外 compose 服务

