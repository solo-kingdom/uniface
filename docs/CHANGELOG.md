# 文档变更日志

本文件记录 docs/ 目录下所有文档的变更历史。

---

## 2026-03-08 (第二次更新) - 重大重构

### 重构目的
按照代码路径重新组织文档结构，使文档路径与代码路径、prompts 路径保持一致。

### 目录结构变更

**新增目录**：
- `docs/rpc/governance/load-balancer/` - RPC 负载均衡器文档
- `docs/storage/kv/` - KV 存储文档
- `docs/storage/config/` - 配置存储文档

**文件移动**：
- `docs/load-balancer-implementation-plan.md` 
  → `docs/rpc/governance/load-balancer/implementation-plan.md`

**新增文件**：
- `docs/storage/kv/README.md` - KV 存储模块说明
- `docs/storage/config/README.md` - 配置存储模块说明

**更新文件**：
- `docs/README.md` - 更新文档索引，反映新结构

### 路径对应关系

```
代码路径                    文档路径                          Prompts 路径
pkg/rpc/governance/    →  docs/rpc/governance/         →  prompts/features/rpc/governance/
  loadbalancer/             load-balancer/                  load-balancer/
  
pkg/storage/kv/        →  docs/storage/kv/             →  prompts/features/storage/kv/

pkg/storage/config/    →  docs/storage/config/         →  prompts/features/storage/config/
```

### 变更说明
1. 文档路径现在与代码实现路径保持一致
2. 每个模块都有独立的 README.md 说明文档
3. 实施计划文档放在对应模块目录下
4. 便于查找和维护

---

## 2026-03-08 (第一次更新)

### 新增
- **README.md** - 创建文档索引中心
  - 添加文档导航表格
  - 分类整理文档（核心规范、实施计划）
  - 提供快速开始指南
  - 添加文档编写规范说明

- **CHANGELOG.md** - 变更日志

### 目的
- 整理 docs/ 目录结构
- 提供清晰的文档导航
- 便于开发者和 AI 助手快速找到所需文档

### 变更说明
根据项目 README.md 中的 AI 代码生成规范，整理了 docs/ 目录：
1. 创建了文档索引页面，方便文档查找和导航
2. 保持了原有文档不变
3. 添加了文档更新记录和变更日志

---

## 文档变更规范

每次修改文档时，请在此文件中记录：
1. 变更日期
2. 新增/修改/删除的文档
3. 变更目的和说明
4. 路径变更（如有）
