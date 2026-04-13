## ADDED Requirements

### Requirement: KV 存储 capability spec
系统 SHALL 在 `openspec/specs/kv-storage/spec.md` 中维护 KV 存储接口的 capability 规格，描述 `Storage[T any]` 接口的完整行为契约，包括 Set、Get、Delete、BatchSet、BatchGet、BatchDelete、Exists、Close 操作。

#### Scenario: 查看 KV 存储 spec
- **WHEN** 开发者或 AI 助手查看 `openspec/specs/kv-storage/spec.md`
- **THEN** 文件包含 KV Storage 接口的完整行为定义，涵盖基本操作、批量操作、资源管理和错误处理

#### Scenario: 基于 KV spec 创建变更
- **WHEN** 使用 `/opsx:propose` 对 KV 存储提出变更
- **THEN** 可基于现有 kv-storage spec 创建 modified capability

### Requirement: 配置存储 capability spec
系统 SHALL 在 `openspec/specs/config-storage/spec.md` 中维护配置存储接口的 capability 规格，描述 `Storage` 接口的完整行为契约，包括 Read、ReadWithCache、Write、Delete、Watch、WatchPrefix、Unwatch、UnwatchPrefix、List、ClearCache、Close 操作。

#### Scenario: 查看配置存储 spec
- **WHEN** 开发者或 AI 助手查看 `openspec/specs/config-storage/spec.md`
- **THEN** 文件包含 Config Storage 接口的完整行为定义，涵盖直接读取、缓存读取、变更监听和资源管理

### Requirement: 负载均衡器 capability spec
系统 SHALL 在 `openspec/specs/load-balancer/spec.md` 中维护负载均衡器接口的 capability 规格，描述 `Balancer[T any]` 接口的完整行为契约，包括 Select、SelectClient、Add、Remove、Update、GetAll、Close 操作，以及四种算法实现（round-robin、random、weighted、consistent-hash）的行为特征。

#### Scenario: 查看负载均衡器 spec
- **WHEN** 开发者或 AI 助手查看 `openspec/specs/load-balancer/spec.md`
- **THEN** 文件包含 Balancer 接口的完整行为定义，涵盖实例选择、客户端缓存、实例管理和算法特定行为

#### Scenario: 基于 key 的路由行为
- **WHEN** spec 中描述 Select 操作
- **THEN** 明确说明 key 参数的存在与否如何影响路由策略（consistent hash vs default strategy）

### Requirement: 分片管理器 capability spec
系统 SHALL 在 `openspec/specs/shard-manager/spec.md` 中维护分片管理器的 capability 规格，描述基于 LoadBalancer + consistent hash 的简化分片管理行为，包括 Select、SelectClient、Close 操作。

#### Scenario: 查看分片管理器 spec
- **WHEN** 开发者或 AI 助手查看 `openspec/specs/shard-manager/spec.md`
- **THEN** 文件包含 ShardManager 的完整行为定义，说明其作为 LoadBalancer wrapper 的设计及 stable key-based routing 行为

### Requirement: 项目结构文档准确性
系统 SHALL 维护 `docs/PROJECT_STRUCTURE.md`，使其准确反映当前项目目录结构，包括 `pkg/`、`docs/`、`specs/`、`prompts/`、`openspec/` 及所有 Go 子模块的实际布局。

#### Scenario: 新成员查阅项目结构
- **WHEN** 开发者阅读 `docs/PROJECT_STRUCTURE.md`
- **THEN** 文档中描述的每个目录都在项目中实际存在，且每个实际存在的顶层目录都在文档中被描述

#### Scenario: 文档路径一致性
- **WHEN** 开发者查看 `docs/PROJECT_STRUCTURE.md` 中的目录树
- **THEN** 目录树与 `ls` 命令的输出一致

### Requirement: 文档导航更新
系统 SHALL 维护 `docs/README.md` 作为文档索引中心，其内容 SHALL 反映 OpenSpec spec-driven 工作流，正确链接所有现有文档。

#### Scenario: AI 助手查阅文档导航
- **WHEN** AI 助手阅读 `docs/README.md`
- **THEN** 文档明确说明三层结构：`openspec/specs/`（能力规格）→ `docs/`（设计文档）→ `pkg/`（实现），并提供 OpenSpec 工作流的入口说明

#### Scenario: 查找模块文档
- **WHEN** 开发者需要查找某个模块的文档
- **THEN** 通过 `docs/README.md` 的导航表格可以找到该模块对应的所有文档（设计文档、spec、实现）

### Requirement: AI 指令文件更新
系统 SHALL 更新 `AI.MD` 和 `CLAUDE.md`，使其反映 OpenSpec spec-driven 工作流作为主要开发模式。

#### Scenario: AI 助手读取 AI.MD
- **WHEN** AI 助手读取 `AI.MD`
- **THEN** 规则中说明使用 OpenSpec 工作流（/opsx:propose、/opsx:apply）进行功能开发和变更管理，`prompts/` 目录为只读参考

#### Scenario: AI 助手读取 CLAUDE.md
- **WHEN** AI 助手读取 `CLAUDE.md`
- **THEN** 项目结构部分包含 `openspec/` 目录说明，开发规约中包含 OpenSpec 工作流说明

### Requirement: docs 内部结构统一
系统 SHALL 统一 `docs/` 目录结构为 `docs/{domain}/{module}/` 格式，消除 `docs/features/` 与模块目录之间的层级冲突。

#### Scenario: 查找 RPC 配置文档
- **WHEN** 开发者需要查找 RPC 配置模块的文档
- **THEN** 所有相关文档（设计文档、实施记录）都在 `docs/rpc/governance/config/` 目录下

#### Scenario: docs/features 目录清理
- **WHEN** 文档整理完成后
- **THEN** `docs/features/` 目录不再存在，所有内容已按模块归入对应目录
