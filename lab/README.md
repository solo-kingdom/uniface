# Uniface Lab

uniface 能力验证台——独立 lab 子模块，通过 CLI 与 Web Dashboard 验证 KV、Config、Load Balancer、Queue、DAG 五类能力，并以 `lab-dag-http` 演示「HTTP 请求经统一 RPC Server 抽象编排」。

## 快速开始

```bash
# 从仓库根目录 — 全量启动（与变更前行为一致）
make lab-build
make lab-up      # 启动 docker-compose + 六进程
make lab-down    # 停止
```

Dashboard: http://localhost:3000

### 按域验证

仅验证单一能力域时，无需启动全部中间件与进程：

```bash
# 仅 DAG（无 compose 依赖，最快）
make lab-up-dag
curl http://localhost:8085/api/status   # 或 CLI: lab/bin/lab-dag graph load --graph echo
make lab-down-dag

# 仅 DAG HTTP 服务（POST /echo 经 DAG 排空返回 echo:<body>）
make lab-up-dag-http
curl -X POST http://localhost:8086/echo -d 'hello'   # → echo:hello
make lab-down-dag-http

# 多域组合
make lab-up LAB_MODULES=kv,dag
make lab-up LAB_MODULES=dag,daghttp   # 同时启 lab-dag 与 lab-dag-http，不启 compose
make lab-down LAB_MODULES=dag       # 仅停 dag，不影响 kv
```

域与 compose profile 对应：`kv` → redis、`config` → consul、`queue` → nats；`lb` / `dag` / `ui` 无外部中间件。按域 `down` 只停对应进程，compose 容器需 `make lab-down` 全量清理。

## 配置

默认配置：`configs/default.yaml`

环境变量覆盖（重启对应 CLI 后生效）：

| 变量 | 说明 |
|------|------|
| `LAB_KV_IMPL` | `redis` / `boltdb` |
| `LAB_CONFIG_IMPL` | `consul` |
| `LAB_LB_IMPL` | `roundrobin` / `random` / `weighted` / `consistenthash` |
| `LAB_QUEUE_IMPL` | `nats` / `kafka` / `rabbitmq` / `natsjetstream` |
| `LAB_CONFIG` | 配置文件路径 |

## CLI 工具

| 工具 | 端口 | 子命令 |
|------|------|--------|
| `lab-kv` | 8081 | `set`, `get`, `delete`, `list`, `exists`, `run-conformance`, `serve` |
| `lab-config` | 8082 | `put`, `get`, `delete`, `watch`, `serve` |
| `lab-lb` | 8083 | `add`, `remove`, `select`, `simulate`, `switch`, `serve` |
| `lab-queue` | 8084 | `publish`, `subscribe`, `bench`, `serve` |
| `lab-dag` | 8085 | `graph load`, `start`, `status`, `signal`, `journal`, `run-once`, `serve` |
| `lab-dag-http` | 8086 | `serve`（`-addr` 覆盖地址） |
| `lab-ui` | 3000 | Dashboard 聚合 |

示例：

```bash
cd lab
LAB_KV_IMPL=boltdb ./bin/lab-kv set --key foo --value bar
LAB_KV_IMPL=boltdb ./bin/lab-kv get --key foo

./bin/lab-dag graph load --graph echo
./bin/lab-dag start --graph echo --entity-id inst-001
```

### 声明式 HttpUnit

DAG 支持**配置驱动**的 HTTP 计算单元——业务方无需为每个节点写 Go 代码，YAML 内联 `unit.http` 即可调用远程服务。fixture `http_call.yaml` 演示黄金路径：

```yaml
nodes:
  call:
    kind: compute
    unit:
      http:
        url: http://127.0.0.1:18099   # service 走 Balancer 解析；url 直连兜底
        path: /echo
        method: POST
        retry_on:
          retry_status_codes: [502, 503, 504]
          fail_status_codes: [400, 404]
        response:
          mode: auto                  # 默认：2xx → update；mode: mutation 直接 apply EntityMutation
          payload_field: Order        # 可选：从 response 投影子字段
    retry_policy:
      max_attempts: 3
    transitions:
      - target: done
```

内置 mock HTTP 服务（`serve` 自动启动，或 `start --mock-http 127.0.0.1:18099` 显式启动）回写处理后的 payload，演示 2xx → update 全流程：

```bash
./bin/lab-dag start --graph http_call --entity-id h1 --payload hello --mock-http 127.0.0.1:18099
# status=INSTANCE_STATUS_COMPLETED node=done
```

**Balancer 集成**：`http.service` 通过 `pkg/dag/units/balanceradapter` 包装 `Balancer[http.Client]` 解析实例（注入 `dag.WithHTTPResolver`）。lab wiring 默认注入 nil resolver（仅支持 `url` 直连）；业务进程可注入真实 Balancer 启用 `service` 路由。

### DAG HTTP 服务（lab-dag-http）

`lab-dag-http` 演示「HTTP 请求经统一 RPC Server 抽象编排」：通过 `pkg/rpc/server`
的 `NewHTTPServer` 启动（非直接手写 `net/http`），对外暴露 `POST /echo`。每次请求包装为
一个 `EntityInstance`，经 echo 图（`lab.echo` compute → terminal）排空到终态后，终态
payload 作为响应体返回：

```bash
make lab-up-dag-http
curl -X POST http://localhost:8086/echo -d 'hello'   # → echo:hello (200)
curl http://localhost:8086/api/status                 # 域状态
make lab-down-dag-http
```

终态映射：`COMPLETED` → 200；`FAILED`/`COMPENSATED` → 500 并附失败原因。它复用
`lab/internal/dag.Runtime`、`echo` fixture 与 `lab.echo` unit，不修改 `lab-dag` 引擎验证台。

## Docker Compose Profiles

```bash
docker compose --profile kv up -d redis
docker compose --profile config up -d consul
docker compose --profile queue up -d nats
docker compose --profile all up -d
```

可选 profiles：`kv`, `config`, `queue`, `queue-kafka`, `queue-rabbit`, `kv-aerospike`, `all`

> **注意**: Aerospike KV 适配器在当前 `pkg` 中未完整实现 `kv.Storage` 接口，lab wiring 暂不支持 `LAB_KV_IMPL=aerospike`，compose 仍提供 aerospike 服务供后续接入。

## 架构

```
lab/
├── cmd/           # 七个 CLI 入口（含 lab-dag-http）
├── internal/
│   ├── wiring/    # 工厂层（yaml + 环境变量）
│   ├── web/       # 共享 HTTP + htmx UI
│   ├── kv/ config/ lb/ queue/ dag/ daghttp/
│   ├── conformance/
│   └── fixtures/graphs/
├── scripts/       # 运维脚本（launch.sh：后台启动 + 记录 PID）
└── configs/default.yaml
```

## 构建

lab 为独立子模块，不参与根模块 `make test` 与 `scripts/tag.sh` 版本发布。

```bash
cd lab && make build
```
