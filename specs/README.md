# 历史规格归档

> **注意**: 此目录为历史规格归档，仅作参考。新功能开发请使用 OpenSpec 工作流（`openspec/specs/`）。

此目录包含项目早期使用 prompts 驱动开发模式时编写的需求规格文件。这些文件记录了原始的功能需求和设计决策，具有历史参考价值。

## 结构

- `architecture/` - 系统架构和高层决策
- `features/` - 各功能模块的需求规格（编号格式如 `00-iface.md`、`01-redis.md`）

## 当前工作流

项目已转向 OpenSpec spec-driven 工作流：

- **能力规格**: `openspec/specs/` - 各功能的接口契约和行为规格
- **变更管理**: `openspec/changes/` - 通过 `/opsx:propose` 创建，`/opsx:apply` 实施
- **设计文档**: `docs/` - 与代码路径对应的设计和实施记录

## 文件说明

每个规格文件包含：
- 功能的原始需求描述
- 上下文和依赖关系
- 实现指导（已由 OpenSpec specs 接管）
