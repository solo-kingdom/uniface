## Why

项目已引入 OpenSpec 工作流（openspec/），但现有文档体系仍基于旧的三层结构（specs/ → docs/ → pkg/）。文档内容存在多处过时和不一致：

- `docs/PROJECT_STRUCTURE.md` 描述了不存在的 `cmd/`、`internal/`、`test/` 目录
- `specs/` 目录包含旧格式的功能规格，与 `openspec/specs/` 功能重叠
- `docs/` 内部结构混乱：`docs/features/` 与 `docs/rpc/`、`docs/storage/` 层级不一致
- 文档引用 `prompts/` 作为核心工作流入口，但实际已转向 OpenSpec 驱动开发
- `AI.MD` 和 `CLAUDE.md` 未反映 OpenSpec 工作流

需要重建文档体系，使其与 OpenSpec spec-driven 工作流对齐，为后续的 spec 编程提供清晰的文档基础。

## What Changes

- 重写 `docs/PROJECT_STRUCTURE.md`，准确反映当前项目目录结构（含 openspec/、子模块等）
- 重写 `docs/README.md`，更新文档导航和工作流说明（纳入 OpenSpec）
- 整理 `docs/` 内部结构，统一按 `docs/{domain}/{module}/` 组织
- 迁移 `specs/` 中的有效规格内容到 `openspec/specs/`，建立 OpenSpec 兼容的 spec 结构
- 更新 `AI.MD`，反映 OpenSpec 工作流
- 更新 `CLAUDE.md`，补充 OpenSpec 相关说明
- 更新 `docs/CHANGELOG.md`，记录本次文档重建

## Capabilities

### New Capabilities

- `openspec-specs-foundation`: 为 openspec/specs/ 建立完整的能力规格（capability specs）结构，将现有已实现功能（KV 存储、配置存储、负载均衡、分片管理）的 spec 纳入 OpenSpec 管理

### Modified Capabilities

（无现有 OpenSpec 能力规格需要修改）

## Impact

- **文档文件**：`docs/`、`specs/`、`AI.MD`、`CLAUDE.md` 多个文件将被重写或大幅修改
- **开发者工作流**：从旧的 prompts/ 驱动转向 OpenSpec spec-driven 工作流
- **AI 助手**：更新指令文件，使 AI 助手正确使用 OpenSpec 流程
- **不涉及代码变更**：仅文档重建，不影响 `pkg/` 下的任何代码
