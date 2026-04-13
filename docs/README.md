# Uniface 文档中心

本目录包含 Uniface 项目的所有文档，旨在帮助开发者和 AI 助手更好地理解和使用本项目。

---

## 文档体系

项目采用三层文档结构：

```
openspec/specs/    → 能力规格（定义"做什么"）
docs/              → 设计文档（记录"怎么做"）
pkg/               → 代码实现（"做出来"）
```

**OpenSpec spec-driven 工作流**是项目的主要开发模式：
- `/opsx:propose` - 创建变更提案
- `/opsx:apply` - 按任务实施
- `/opsx:archive` - 归档完成的变更

---

## 核心规范

| 文档 | 说明 |
|------|------|
| [AI_CODING_RULES.md](./AI_CODING_RULES.md) | AI 代码生成规则与编码规范 |
| [PROJECT_STRUCTURE.md](./PROJECT_STRUCTURE.md) | 项目目录结构和组织方式 |
| [CHANGELOG.md](./CHANGELOG.md) | 文档变更日志 |

---

## 功能模块文档

### 存储系统

| 模块 | 文档 | 对应代码 | 能力规格 |
|------|------|----------|---------|
| KV 存储 | [storage/kv/](./storage/kv/) | `pkg/storage/kv/` | `openspec/specs/kv-storage/` |
| 配置存储 | [storage/config/](./storage/config/) | `pkg/storage/config/` | `openspec/specs/config-storage/` |

### RPC 服务治理

| 模块 | 文档 | 对应代码 | 能力规格 |
|------|------|----------|---------|
| 负载均衡器 | [rpc/governance/load-balancer/](./rpc/governance/load-balancer/) | `pkg/rpc/governance/loadbalancer/` | `openspec/specs/load-balancer/` |
| Consul 配置 | [rpc/governance/config/](./rpc/governance/config/) | `pkg/rpc/governance/config/consul/` | - |
| 分片管理器 | [rpc/governance/load-balancer/shard/](./rpc/governance/load-balancer/shard/) | `pkg/rpc/governance/loadbalancer/shard/` | `openspec/specs/shard-manager/` |
| Aerospike 集成 | [rpc/governance/load-balancer/shard/aerospike/](./rpc/governance/load-balancer/shard/aerospike/) | `pkg/storage/kv/aerospike/` | - |

---

## 查找文档

### 按代码路径查找

根据代码路径查找对应文档：

```
代码：pkg/rpc/governance/loadbalancer/
文档：docs/rpc/governance/load-balancer/
规格：openspec/specs/load-balancer/
```

注意命名差异：代码用 `loadbalancer`（Go 包名无连字符），文档用 `load-balancer`。

### 按模块查找

每个模块的文档类型：
- **能力规格** (`openspec/specs/<name>/spec.md`) - 接口的行为契约
- **设计文档** (`docs/<path>/`) - 设计决策和实施记录
- **代码** (`pkg/<path>/`) - 接口定义和实现

---

## 相关资源

- [项目 README](../README.md) - 项目概述
- [PROJECT_STRUCTURE.md](./PROJECT_STRUCTURE.md) - 完整目录结构
- [OpenSpec Specs](../openspec/specs/) - 能力规格
- [历史规格](../specs/) - 旧格式需求规格（归档）
- [Prompts](../prompts/) - AI 提示词模板（只读参考）
