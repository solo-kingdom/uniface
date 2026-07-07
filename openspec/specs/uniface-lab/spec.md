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

`daghttp` SHALL 作为自包含应用整体落在 `lab/app/daghttp/` 目录下：handler/service、`StringApp` 装配函数、`lab.hello` / `lab.echo` 计算单元、echo fixture、`Config` schema（`Store` / `FixturesDir`）、生命周期入口（`Serve` / `LoadConfig`）均位于该路径下，不得散布到 `lab/internal/wiring/` 或其他跨域共享目录。`lab-dag-http` SHALL 通过 `lab/app/daghttp.Serve(ctx, addr, cfg)` 启动服务，main 包不再持有 StringApp 与装配细节。

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

#### Scenario: daghttp 实现全部落在 `lab/app/daghttp/`

- **WHEN** 查看 daghttp 应用的所有源代码（除 `lab/cmd/lab-dag-http/main.go` 外）
- **THEN** 所有 `.go` 文件与 fixture 均位于 `lab/app/daghttp/` 之下
- **AND** `lab/internal/wiring/` 中不再保留 daghttp 专属函数（`NewDAGHTTP` / `registerLabUnits` / `helloFunc` / `echoFunc`）
- **AND** `LabConfig.DAG` 字段类型为 `daghttp.Config`，DAGConfig 不再定义于 wiring 包

#### Scenario: main 仅持有 flag 与生命周期入口

- **WHEN** 阅读 `lab/cmd/lab-dag-http/main.go`
- **THEN** 文件 SHALL 仅包含 flag 解析、信号 `ctx`、`daghttp.LoadConfig()` 调用、`daghttp.Serve(ctx, addr, cfg)` 调用
- **AND** SHALL NOT 直接持有 `*app.StringApp`、`registerLabUnits`、`helloFunc`、`echoFunc`、`daghttp.NewService` 中的任意一个

### Requirement: DAG HTTP 按域生命周期

系统 SHALL 将 `daghttp` 纳入 lab 域注册表（二进制 `lab-dag-http`，默认端口 `8086`，无 compose 中间件依赖），并提供按域目标 `lab-build-dag-http`、`lab-up-dag-http`、`lab-down-dag-http`，行为与既有域目标一致。daghttp 域的可执行构建路径 SHALL 为 `lab/cmd/lab-dag-http`，其源码归属 SHALL 为 `lab/app/daghttp/`。

#### Scenario: 按域启动 DAG HTTP

- **WHEN** 执行 `make lab-up-dag-http`
- **THEN** 仅构建并启动 `lab-dag-http serve` 进程，不启动其他域进程，也不启动任何 compose 中间件

#### Scenario: 按域关停

- **WHEN** 已执行 `make lab-up-dag-http`，随后执行 `make lab-down-dag-http`
- **THEN** 仅停止 `lab-dag-http` 进程，不影响其他运行中的 lab 域进程

#### Scenario: 多域组合包含 DAG HTTP

- **WHEN** 执行 `make lab-up LAB_MODULES=dag,daghttp`
- **THEN** 同时启动 `lab-dag` 与 `lab-dag-http` 两个进程，且不启动额外 compose 服务

### Requirement: lab/app/ 顶级目录承载自包含应用

系统 SHALL 在 `lab/` 下提供 `lab/app/` 顶级目录，用于承载「自包含 lab 应用」—— 即「handler / 装配 / unit / fixture / 配置 schema / 生命周期入口」均落在该应用目录之内的端到端可部署单元。`lab/app/` SHALL NOT 包含跨应用共享的基础设施（后者仍居于 `lab/internal/`）。本变更仅要求 `lab/app/daghttp/` 一个应用落地；其它 lab CLI（lab-kv、lab-config、lab-lb、lab-queue、lab-dag、lab-ui）在本变更中 SHALL 维持现有位置不动。

#### Scenario: lab/app/ 仅承载 self-contained 应用

- **WHEN** 列出 `lab/app/` 下任一应用目录
- **THEN** 该目录下的源码 SHALL 仅引用 `pkg/` 公共包与同包内部符号
- **AND** SHALL NOT 引用 `lab/internal/wiring` 中专门为该应用新增的专属函数

#### Scenario: lab/internal/wiring 不再持有 daghttp 专属装配

- **WHEN** 阅读 `lab/internal/wiring/daghttp.go`
- **THEN** 该文件 SHALL 不存在（daghttp 专属代码已迁出）
- **AND** `lab/internal/wiring/config.go` SHALL 仅保留跨域共享的 `LabConfig` / `KVConfig` / `LBConfig` / `QueueConfig` / `ServicesConfig`，`DAGConfig` 由 `lab/app/daghttp` 自有

### Requirement: OpRecorder 类型化结果记录

`lab/internal/web/api` 的 `OpRecorder` SHALL 暴露 `RecordResult(op, detail string, res ResultSentinel)` 方法 —— 接受一个类型化结果（`*app.StringCallResult` 隐式实现 `ResultSentinel`）并自动派生 `Operation.OK` 字段：

- `res == nil` → `OK = false`，`Error = "nil result"`
- `res.IsCompleted() == true` → `OK = true`，`Error = ""`
- `res.IsCompleted() == false` → `OK = false`，`Error` 优先取 `res.Err().Error()`，否则取 `"status=<res.Status()>"`
- `Detail` 透传调用方提供的字符串

`ResultSentinel` 接口 SHALL 至少包含 `IsCompleted() bool` 与 `Status() string` 两个方法；`Err() error` 为可选方法（缺省时 recorder 回退到 `status=<Status>` 形式）。

调用方 SHALL 不再需要 `isCompleted := res.IsCompleted(); rec.Record(op, detail, isCompleted, nil)` 的手工派生代码；改写为 `rec.RecordResult(op, detail, res)`。

#### Scenario: COMPLETED 自动派生 ok=true

- **WHEN** 调用 `rec.RecordResult("echo", "e1", res)` 其中 `res.IsCompleted() == true`
- **THEN** 内部 `Operation` 的 `OK = true`，`Error` 为空

#### Scenario: FAILED 自动派生 ok=false 与错误信息

- **WHEN** 调用 `rec.RecordResult("echo", "e1", res)` 其中 `res.IsCompleted() == false`、`res.Err() != nil` 且错误信息为 `"unit failed"`
- **THEN** 内部 `Operation` 的 `OK = false`，`Error = "unit failed"`

#### Scenario: nil 入参

- **WHEN** 调用 `rec.RecordResult("echo", "e1", nil)`
- **THEN** 内部 `Operation` 的 `OK = false`，`Error` 含 `"nil result"`
- **AND** 不 panic

#### Scenario: 接口向后兼容

- **WHEN** 现有调用方继续使用 `rec.Record(op, detail, ok, err)`
- **THEN** 行为与本次变更前一致

### Requirement: DAG Signal HTTP 验证 CLI

系统 SHALL 提供 `lab-dag-signal` 命令行工具（源码位于 `lab/app/dagsignal/`，CLI 入口 `lab/cmd/lab-dag-signal/main.go`），基于 `pkg/dag/invocation/app.StringApp` 装配实体类型与图加载，经 `sa.Runtime.Memory().Engine()` 访问底层 `StartInstance` / `DeliverSignal` / `DrainInstance` / `GetInstance`，演示「HTTP 请求 → 实例停在 WAITING → 另一端点 signal 推进到终态」的异步编排范式。工具 SHALL NOT 调用 `StringApp.InvokeString`（同步入口）；工具 SHALL NOT 修改 `pkg/rpc/server/dagbridge`（保持其同步 WAITING→500 语义）。

入口图 SHALL 为 `approval`：`entry` 指向 `wait` 节点，`signal: approval`；`approval` signal → success 终态，其它 → failure 终态（兜底 `condition: always`）。图 SHALL NOT 包含任何 COMPUTE 节点（演示焦点为 WAIT + signal 路由）。

HTTP 端点 SHALL 在默认端口 `8087` 暴露：
- `POST /start` —— 请求体作为 payload（可空），生成唯一 entityID，`StartInstance` + `DrainInstance` 推进；SHALL 返回 `202 Accepted` + JSON `{"entity_id":"...","status":"WAITING"}`。启动/drain 失败 SHALL 返回 `500`
- `POST /signal/{entityID}` —— `DeliverSignal`（signal 名默认 `approval`，可经 `?signal=` query 覆盖）+ `DrainInstance`；终态 → `200`，仍 WAITING → `202`，失败终态 → `500`，signal 名不匹配 → `400`，实例不存在 → `404`
- `GET /instances/{entityID}` —— 透传 `GetInstance`；存在 → `200` + 状态 JSON，不存在 → `404`
- `GET /api/status` —— 返回 `api.Status{Domain:"dagsignal", ...}`，与 daghttp `/api/status` 同构

#### Scenario: start 返回 WAITING 与 entityID

- **WHEN** 执行 `curl -X POST http://localhost:8087/start -d 'hello'`
- **THEN** 返回 `202 Accepted`
- **AND** 响应体为 JSON，包含非空 `entity_id` 与 `"status":"WAITING"`

#### Scenario: signal 推进到 COMPLETED

- **WHEN** 对 `POST /start` 返回的 `entity_id` 执行 `curl -X POST http://localhost:8087/signal/{entity_id}`
- **THEN** 返回 `200 OK`
- **AND** 响应体 JSON `"status"` 为 `"COMPLETED"`

#### Scenario: signal 名不匹配返回 400

- **WHEN** 执行 `curl -X POST "http://localhost:8087/signal/{entity_id}?signal=unknown"`
- **THEN** 返回 `400 Bad Request`
- **AND** 响应体包含 signal 不匹配信息

#### Scenario: signal 不存在的实例返回 404

- **WHEN** 执行 `curl -X POST http://localhost:8087/signal/nonexistent`
- **THEN** 返回 `404 Not Found`

#### Scenario: 查询实例状态

- **WHEN** 对已 start 的 `entity_id` 执行 `curl http://localhost:8087/instances/{entity_id}`
- **THEN** 返回 `200 OK`
- **AND** 响应体 JSON 包含 `"status"` 字段（WAITING 或 COMPLETED）

#### Scenario: 查询不存在的实例返回 404

- **WHEN** 执行 `curl http://localhost:8087/instances/nonexistent`
- **THEN** 返回 `404 Not Found`

#### Scenario: status 返回域信息

- **WHEN** 执行 `curl http://localhost:8087/api/status`
- **THEN** 返回 `200 OK` + `application/json`
- **AND** 响应体包含 `"domain":"dagsignal"`

#### Scenario: dagsignal 实现全部落在 `lab/app/dagsignal/`

- **WHEN** 查看 dagsignal 应用的所有源代码（除 `lab/cmd/lab-dag-signal/main.go` 外）
- **THEN** 所有 `.go` 文件与 fixture 均位于 `lab/app/dagsignal/` 之下
- **AND** SHALL NOT 引用 `lab/internal/wiring` 中专为 dagsignal 新增的专属函数
- **AND** SHALL NOT 调用 `StringApp.InvokeString` 或 `dagbridge.ResponseForTerminalResult`

#### Scenario: main 仅持有 flag 与生命周期入口

- **WHEN** 阅读 `lab/cmd/lab-dag-signal/main.go`
- **THEN** 文件 SHALL 仅包含 flag 解析、信号 `ctx`、`dagsignal.LoadConfig()` 调用、`dagsignal.Serve(ctx, addr, cfg)` 调用
- **AND** SHALL NOT 直接持有 `dag.Engine`、`*app.StringApp`、`dagsignal.NewService` 中的任意一个

#### Scenario: 配置自治与 env 覆写

- **WHEN** 设置 `LAB_CONFIG` 指向含 `dagsignal: {store: memory, fixtures_dir: /tmp/x}` 的 yaml，并设置 `LAB_DAGSIGNAL_FIXTURES_DIR=/tmp/y`
- **THEN** `LoadConfig()` 返回的 `FixturesDir` SHALL 为 `/tmp/y`（env 覆写 yaml）
- **AND** 未设置 env 且 yaml 无 `fixtures_dir` 时 SHALL 回退到 `DefaultFixturesDir`（`app/dagsignal/fixtures/graphs`）

### Requirement: DAG Signal 按域生命周期

系统 SHALL 将 `dagsignal` 纳入 lab 域注册表（二进制 `lab-dag-signal`，默认端口 `8087`，无 compose 中间件依赖），并提供按域目标 `lab-build-dag-signal`、`lab-up-dag-signal`、`lab-down-dag-signal`，行为与既有 `daghttp` 域目标一致。dagsignal 域的可执行构建路径 SHALL 为 `lab/cmd/lab-dag-signal`，其源码归属 SHALL 为 `lab/app/dagsignal/`。

#### Scenario: 按域启动 DAG Signal

- **WHEN** 执行 `make lab-up-dag-signal`
- **THEN** 仅构建并启动 `lab-dag-signal serve` 进程，不启动其他域进程，也不启动任何 compose 中间件

#### Scenario: 按域关停

- **WHEN** 已执行 `make lab-up-dag-signal`，随后执行 `make lab-down-dag-signal`
- **THEN** 仅停止 `lab-dag-signal` 进程，不影响其他运行中的 lab 域进程

#### Scenario: 多域组合包含 DAG Signal

- **WHEN** 执行 `make lab-up LAB_MODULES=daghttp,dagsignal`
- **THEN** 同时启动 `lab-dag-http` 与 `lab-dag-signal` 两个进程，且不启动额外 compose 服务

#### Scenario: 全量启动包含 DAG Signal

- **WHEN** 执行 `make lab-up`（即 `LAB_MODULES=all`）
- **THEN** 启动的进程集合 SHALL 包含 `lab-dag-signal`
- **AND** `lab/Makefile` 的 `MODULES` 列表 SHALL 包含 `dagsignal`

