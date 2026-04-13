## Context

Uniface 项目已从原始的 prompts 驱动开发模式演进到 OpenSpec spec-driven 工作流。目前项目文档存在以下问题：

1. **PROJECT_STRUCTURE.md 过时**：描述了 `cmd/`、`internal/`、`test/` 等不存在的目录，未提及 `openspec/`、`aerospike/`、`boltdb/` 等实际存在的子模块
2. **docs/ 结构混乱**：`docs/features/` 包含实施记录（plan、changes、summary），与 `docs/rpc/`、`docs/storage/` 的模块文档层级不一致
3. **specs/ 与 openspec/specs/ 重叠**：旧的 `specs/` 目录（含编号格式如 `00-iface.md`）与新的 `openspec/specs/` 功能重叠，需要统一
4. **AI 指令未更新**：`AI.MD` 和 `CLAUDE.md` 未反映 OpenSpec 工作流（/opsx:propose、/opsx:apply 等）
5. **prompts/ 角色转变**：prompts/ 从"需求入口"变为"只读参考"，docs/README.md 仍将其作为核心工作流入口

### 当前文档清单

| 路径 | 类型 | 问题 |
|------|------|------|
| `docs/PROJECT_STRUCTURE.md` | 项目结构 | 严重过时 |
| `docs/README.md` | 文档索引 | 引用旧工作流 |
| `docs/CHANGELOG.md` | 变更日志 | 需继续记录 |
| `docs/AI_CODING_RULES.md` | 编码规则 | 基本准确 |
| `docs/rpc/governance/load-balancer/` | 设计文档 | 有效，保留 |
| `docs/storage/kv/README.md` | 模块文档 | 有效，保留 |
| `docs/storage/config/README.md` | 模块文档 | 有效，保留 |
| `docs/features/rpc/governance/config/` | 实施记录 | 需整理 |
| `docs/features/rpc/governance/load-balancer/shard/` | 实施记录 | 需整理 |
| `docs/features/rpc/governance/load-balancer/shard/aerospike/` | 实施记录 | 需整理 |
| `specs/` | 旧规格 | 需迁移 |
| `AI.MD` | AI 指令 | 需更新 |
| `CLAUDE.md` | AI 指令 | 需更新 |

## Goals / Non-Goals

**Goals:**

- 重写 `docs/PROJECT_STRUCTURE.md`，准确反映当前项目结构
- 重写 `docs/README.md`，更新文档导航，纳入 OpenSpec 工作流
- 为 `openspec/specs/` 建立已实现功能的 capability spec 结构，使后续开发有清晰的起点
- 整理 `docs/features/` 下的实施记录，统一归入模块文档目录
- 更新 `AI.MD` 和 `CLAUDE.md` 以反映 OpenSpec 工作流
- 保留 `specs/` 旧目录但标注为归档（或移除冗余内容）

**Non-Goals:**

- 不修改 `pkg/` 下的任何代码
- 不修改 `prompts/` 目录（只读）
- 不重写 `docs/AI_CODING_RULES.md`（内容基本准确）
- 不删除有价值的历史设计文档

## Decisions

### D1: OpenSpec specs 结构设计

**决策**: 为已实现的功能建立 `openspec/specs/<capability>/spec.md` 结构，每个 spec 反映当前实现的接口契约。

**能力划分**:
```
openspec/specs/
├── kv-storage/spec.md          # KV 存储接口 (Storage[T])
├── config-storage/spec.md      # 配置存储接口
├── load-balancer/spec.md       # 负载均衡器接口 (Balancer[T])
└── shard-manager/spec.md       # 分片管理器
```

**理由**: 将已实现的接口契约纳入 OpenSpec 管理，后续新增功能或修改时可直接基于现有 spec 创建变更。每个 spec 描述接口的 SHALL 行为，作为未来变更的基线。

**替代方案**: 不建立现有功能 spec，等新功能时再创建。放弃，因为缺少基线会导致后续 spec 无法区分"已实现"和"新增"。

### D2: docs/features/ 整理策略

**决策**: 将 `docs/features/` 下的实施记录按模块归入对应目录：
- `docs/features/rpc/governance/config/*` → `docs/rpc/governance/config/`
- `docs/features/rpc/governance/load-balancer/shard/*` → `docs/rpc/governance/load-balancer/shard/`
- `docs/features/rpc/governance/load-balancer/shard/aerospike/*` → `docs/rpc/governance/load-balancer/shard/aerospike/`

归入后删除 `docs/features/` 目录。

**理由**: 消除 `docs/features/` 和 `docs/rpc/` 之间的层级冲突，使文档路径统一为 `docs/{domain}/{module}/`。

**替代方案**: 保留 `docs/features/` 作为历史记录。放弃，因为会造成文档查找混乱。

### D3: specs/ 旧目录处理

**决策**: 保留 `specs/` 目录，在 `specs/README.md` 中标注为"历史规格归档"，说明新功能开发使用 `openspec/specs/`。

**理由**: `specs/` 中的内容有历史参考价值（记录了原始需求），且 git 历史中已有记录。直接删除可能丢失有用的上下文。

**替代方案**: 将 specs/ 内容迁移到 openspec/specs/。部分放弃——仅将核心接口契约提取到 openspec/specs/，其余保留在 specs/ 作为历史参考。

### D4: AI.MD 和 CLAUDE.md 更新范围

**决策**:
- `AI.MD`: 更新第 3、4、5 条规则，将 "prompts/ 驱动" 改为 "OpenSpec spec-driven" 工作流
- `CLAUDE.md`: 在"开发规约"中增加 OpenSpec 工作流说明，更新项目结构部分

**理由**: 最小化变更，仅更新与工作流相关的部分。

## Risks / Trade-offs

- **[文档迁移可能丢失上下文]** → 迁移前确认每个文件内容，仅移动不删除，git 历史可追溯
- **[openspec/specs/ 建立可能与实际代码不同步]** → spec 基于当前已实现的接口定义编写，后续通过 /opsx:apply 保持同步
- **[docs/features/ 移动后引用链接失效]** → 更新 docs/README.md 中的所有链接
