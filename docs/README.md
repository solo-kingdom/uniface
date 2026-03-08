# Uniface 文档中心

本目录包含 Uniface 项目的所有文档，旨在帮助开发者和 AI 助手更好地理解和使用本项目。

**文档路径与代码路径保持一致，便于查找和维护。**

---

## 📚 文档导航

### 核心规范

| 文档 | 说明 | 更新日期 |
|------|------|----------|
| [AI_CODING_RULES.md](./AI_CODING_RULES.md) | AI 代码生成规则与规范 | 2026-03-08 |
| [PROJECT_STRUCTURE.md](./PROJECT_STRUCTURE.md) | 项目目录结构和组织方式 | 2026-03-08 |
| [CHANGELOG.md](./CHANGELOG.md) | 文档变更日志 | 2026-03-08 |

### 功能模块文档

#### RPC 服务治理

| 模块 | 文档 | 对应代码 | 状态 |
|------|------|----------|------|
| 负载均衡器 | [rpc/governance/load-balancer/](./rpc/governance/load-balancer/) | `pkg/rpc/governance/loadbalancer/` | ✅ 已完成 |

#### 存储系统

| 模块 | 文档 | 对应代码 | 状态 |
|------|------|----------|------|
| KV 存储 | [storage/kv/](./storage/kv/) | `pkg/storage/kv/` | ✅ 已完成 |
| 配置存储 | [storage/config/](./storage/config/) | `pkg/storage/config/` | ✅ 已完成 |

---

## 🗂 文档组织原则

### 路径一致性

文档路径与代码路径保持对应关系：

```
docs/
├── rpc/                           # RPC 相关文档
│   └── governance/
│       └── load-balancer/         # 对应 pkg/rpc/governance/loadbalancer/
│           └── implementation-plan.md
│
└── storage/                       # 存储相关文档
    ├── kv/                        # 对应 pkg/storage/kv/
    │   └── README.md
    └── config/                    # 对应 pkg/storage/config/
        └── README.md
```

### 与 Prompts 的关系

```
prompts/features/                  # 需求定义
    ├── rpc/governance/load-balancer/
    └── storage/
        ├── kv/
        └── config/

docs/                              # 实施文档
    ├── rpc/governance/load-balancer/
    └── storage/
        ├── kv/
        └── config/

pkg/                               # 代码实现
    ├── rpc/governance/loadbalancer/
    └── storage/
        ├── kv/
        └── config/
```

**关系说明**：
- `prompts/` - 定义"做什么"（需求）
- `docs/` - 记录"怎么做"（设计决策、实施计划）
- `pkg/` - 实现"做出来"（代码）

---

## 🚀 快速开始

### 对于开发者

1. 先阅读 [PROJECT_STRUCTURE.md](./PROJECT_STRUCTURE.md) 了解项目结构
2. 根据需要查看对应模块的文档（路径与代码路径一致）
3. 阅读相关功能的实施计划文档了解设计决策
4. 查看 `prompts/` 目录了解功能需求来源

### 对于 AI 助手

1. **必须**先阅读 [AI_CODING_RULES.md](./AI_CODING_RULES.md)
2. 检查 `prompts/` 目录中的相关提示词（需求定义）
3. 查看对应模块的文档（设计决策和实施计划）
4. 生成代码时在文件顶部添加 prompt 引用

### 查找文档

根据代码路径查找对应文档：
- 代码：`pkg/rpc/governance/loadbalancer/`
- 文档：`docs/rpc/governance/load-balancer/`
- 需求：`prompts/features/rpc/governance/load-balancer/`

---

## 📝 文档编写规范

根据 `README.md` 中的规范：

1. ✅ **只写必要文档** - 避免过度生成
2. ✅ **使用中文** - 所有文档使用中文编写
3. ✅ **保存 Plan** - 实施计划以文档形式保存在 docs/ 下
4. ✅ **变更说明** - 每次修改要写简洁的变更说明
5. ✅ **路径一致** - 文档路径与代码路径、prompts 路径保持一致

### 新增模块文档步骤

1. 在 `docs/` 下创建与代码路径对应的目录
2. 创建 `README.md` 说明模块功能和使用方法
3. 创建 `implementation-plan.md` 记录设计决策（如需要）
4. 更新 `docs/README.md` 的索引
5. 在 `docs/CHANGELOG.md` 中记录变更

---

## 🔄 文档更新记录

### 2026-03-08
- **重大重构**: 按照代码路径重新组织文档结构
- 创建 `docs/rpc/governance/load-balancer/` 目录
- 创建 `docs/storage/kv/` 目录和 README
- 创建 `docs/storage/config/` 目录和 README
- 移动负载均衡器文档到对应路径
- 更新文档索引以反映新结构

---

## 📖 相关资源

- [项目 README](../README.md) - 项目概述
- [Prompts 目录](../prompts/) - AI 提示词模板
- [代码实现](../pkg/) - 实际代码实现

---

## 💡 提示

- 文档与代码同步更新
- 实施计划文档记录完整的设计决策过程
- 每个重要功能都应该有对应的实施计划文档
