## Context

lab 子模块当前通过 `lab/Makefile` 提供三个聚合目标：

- `build`：循环编译六个 `lab-*` 二进制
- `up`：`docker compose --profile all` + 启动全部 serve 进程
- `down`：停止全部进程 + `docker compose down`

`docker-compose.yml` 已支持按 profile 启动中间件（`kv`、`config`、`queue` 等），但 Makefile 的 `up`/`down` 未暴露域粒度控制。DAG、LB 等域无外部中间件依赖，全量启动浪费资源且增加端口冲突与调试噪音。

## Goals / Non-Goals

**Goals:**

- 支持按域（`kv`、`config`、`lb`、`queue`、`dag`、`ui`）独立 `build` / `up` / `down`
- 根目录可通过 `make lab-up-dag` 或 `make lab-up LAB_MODULES=dag` 调用
- 默认无 `LAB_MODULES` 时行为与现有一键目标完全一致
- `down` 按域停止时不得误杀其他域仍在运行的进程

**Non-Goals:**

- 不重构 CLI 代码或 wiring 层
- 不为每个 queue 实现（kafka/rabbit）单独建 Makefile 域；仍通过 `COMPOSE_PROFILES` 环境变量控制
- 不实现进程守护或 systemd 集成

## Decisions

### 1. 域注册表（Makefile 变量）

在 `lab/Makefile` 顶部定义域元数据，用单一数据源驱动 build/up/down：

```makefile
MODULES := kv config lb queue dag ui

# module -> binary, compose profiles (space-separated, 空表示无 compose)
MODULE_kv_BIN := lab-kv
MODULE_kv_PROFILES := kv

MODULE_dag_BIN := lab-dag
MODULE_dag_PROFILES :=

MODULE_ui_BIN := lab-ui
MODULE_ui_PROFILES :=
```

`LAB_MODULES` 默认为 `all`；解析为实际模块列表时，`all` 展开为 `$(MODULES)`。

**理由**：避免六个几乎相同的 target 块复制粘贴；新增域时只改注册表。

**备选**：每个域手写独立 target — 直观但维护成本高，否决。

### 2. 目标命名

| 层级 | 模式 | 示例 |
|------|------|------|
| lab/Makefile | `build-<module>`、`up-<module>`、`down-<module>` | `make up-dag` |
| lab/Makefile | 参数化 `build`/`up`/`down` | `make up LAB_MODULES=dag` |
| 根 Makefile | `lab-<action>-<module>` | `make lab-up-dag` |
| 根 Makefile | 参数化转发 | `make lab-up LAB_MODULES=dag,queue` |

**理由**：与 compose profile 命名一致；根目录短命令便于文档引用。

### 3. up 流程拆分

`up-<module>` 执行顺序：

1. `build-<module>`（或依赖 `build` 中对应子集）
2. 若 `MODULE_*_PROFILES` 非空：`docker compose --profile <profiles> up -d`
3. 后台启动该域 `serve`（或 `lab-ui`），写入 `bin/pids/<bin>.pid`

`dag` / `lb` 跳过 compose 步骤，仅编译并启动进程。

**理由**：DAG 验证无需 Redis/NATS；最小化启动即最快反馈。

### 4. down 流程拆分

`down-<module>` 仅：

1. 读取并 kill `bin/pids/<bin>.pid`（若存在）
2. 若该域 profiles 对应的 compose 服务无其他运行中 lab 进程依赖，则 `docker compose stop <services>`

简化实现：**按域 down 只停进程，不 stop compose 容器**（compose 由 `make down` 或 `make lab-down` 全量清理）。这样 `make lab-down-dag` 后 `make lab-up-kv` 不会因 compose 已被拆掉而失败。

**理由**：compose 容器复用成本低；避免跨域 down 时的 profile 依赖计算。全量 `down` 仍执行 `docker compose down`。

**备选**：按域精确 stop compose 服务 — 需追踪「哪些 profile 仍被其他域使用」，复杂度高，本期否决。

### 5. 多域选择

`LAB_MODULES=dag,ui` 或 `LAB_MODULES="dag ui"` 均支持（Make 中用 `subst` 统一为空格分隔列表）。

根 `Makefile` 将 `LAB_MODULES` 透传给 `$(MAKE) -C lab`。

### 6. 文档与 help

- `lab/README.md` 增加「按域验证」小节，以 DAG 为典型示例
- 根 `make help` 列出 `lab-up-<module>` 模式并注明 `LAB_MODULES`

## Risks / Trade-offs

- **[Risk] 部分域 down 后 compose 容器仍占用端口** → 文档说明用 `make lab-down` 做全量清理；开发者可 `docker compose down` 手动处理
- **[Risk] `lab-ui` 单独启动时其他域显示离线** → 符合现有 Dashboard 设计（离线卡片），非回归
- **[Risk] Makefile 复杂度上升** → 用注册表 + 通用宏/模板减少重复；tasks 中要求验证 `make -n` 干跑

## Migration Plan

1. 实现 lab/Makefile 域注册表与拆分目标，保持默认 `build`/`up`/`down` 不变
2. 根 Makefile 增加转发与 help
3. 更新 README / CLAUDE.md
4. 手动验证：`make lab-up-dag` → `curl :8085` → `make lab-down-dag`；`make lab-up` 仍启动全部

无数据迁移；可随时回滚 Makefile 变更。

## Open Questions

（无——方案足够明确，可直接实施）
