## MODIFIED Requirements

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
