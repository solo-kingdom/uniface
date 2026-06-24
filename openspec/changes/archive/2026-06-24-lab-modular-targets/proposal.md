## Why

当前 `make lab-build` / `lab-up` / `lab-down` 一次性构建并启动全部六个 lab 进程及全部 compose 中间件，开发者在只验证单一能力域（如 DAG）时仍需承担全量启动成本，影响迭代效率。需要将 lab 构建与生命周期拆分为可按域独立操作的目标，同时保留一键全量启动的便捷性。

## What Changes

- 在 `lab/Makefile` 引入域（module）维度：`kv`、`config`、`lb`、`queue`、`dag`、`ui`，支持通过变量 `LAB_MODULES` 选择子集，默认仍为全部
- 新增按域目标：`build-<module>`、`up-<module>`、`down-<module>`；`up`/`down` 仅操作所选域对应的二进制与 compose profile
- 根 `Makefile` 增加对应转发目标（如 `lab-up-dag`）及参数化用法（`make lab-up LAB_MODULES=dag`）
- 为各域定义 compose profile 与进程依赖映射（如 `dag`/`lb` 无外部依赖，`kv` 对应 `kv` profile）
- 更新 `lab/README.md` 与根 `Makefile help`，说明按域测试的用法
- 保留现有 `lab-build` / `lab-up` / `lab-down` 行为不变（等价于 `LAB_MODULES=all`）

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `uniface-lab`：扩展「一键启动环境」需求，增加按域独立构建、启动、关停的 Makefile 目标与场景

## Non-goals

- 不修改各 `lab-*` CLI 的命令接口或 HTTP API
- 不拆分 `lab-ui` 为多实例部署架构；按域启动时 `ui` 仍可展示离线域卡片
- 不在本期引入新的 compose 服务或中间件
- 不改变 `make test` 对 lab 的排除策略

## Impact

- **构建脚本**: `lab/Makefile`、根 `Makefile`
- **文档**: `lab/README.md`、`CLAUDE.md`（lab 命令说明）
- **运行时**: 无代码变更；仅 Makefile 与文档
- **兼容性**: 现有 `lab-build`/`lab-up`/`lab-down` 保持向后兼容
