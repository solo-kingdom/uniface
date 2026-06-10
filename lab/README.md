# Uniface Lab

uniface 能力验证台——独立 lab 子模块，通过 CLI 与 Web Dashboard 验证 KV、Config、Load Balancer、Queue、DAG 五类能力。

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

# 多域组合
make lab-up LAB_MODULES=kv,dag
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
| `lab-ui` | 3000 | Dashboard 聚合 |

示例：

```bash
cd lab
LAB_KV_IMPL=boltdb ./bin/lab-kv set --key foo --value bar
LAB_KV_IMPL=boltdb ./bin/lab-kv get --key foo

./bin/lab-dag graph load --graph echo
./bin/lab-dag start --graph echo --entity-id inst-001
```

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
├── cmd/           # 六个 CLI 入口
├── internal/
│   ├── wiring/    # 工厂层（yaml + 环境变量）
│   ├── web/       # 共享 HTTP + htmx UI
│   ├── kv/ config/ lb/ queue/ dag/
│   ├── conformance/
│   └── fixtures/graphs/
└── configs/default.yaml
```

## 构建

lab 为独立子模块，不参与根模块 `make test` 与 `scripts/tag.sh` 版本发布。

```bash
cd lab && make build
```
