## 1. OpenSpec Specs 基础建设

- [x] 1.1 创建 `openspec/specs/kv-storage/spec.md`，编写 KV Storage 接口的完整行为规格
- [x] 1.2 创建 `openspec/specs/config-storage/spec.md`，编写 Config Storage 接口的完整行为规格
- [x] 1.3 创建 `openspec/specs/load-balancer/spec.md`，编写 Load Balancer 接口的完整行为规格
- [x] 1.4 创建 `openspec/specs/shard-manager/spec.md`，编写 Shard Manager 的完整行为规格

## 2. docs/ 结构整理

- [x] 2.1 移动 `docs/features/rpc/governance/config/*` 到 `docs/rpc/governance/config/`
- [x] 2.2 移动 `docs/features/rpc/governance/load-balancer/shard/*` 到 `docs/rpc/governance/load-balancer/shard/`
- [x] 2.3 移动 `docs/features/rpc/governance/load-balancer/shard/aerospike/*` 到 `docs/rpc/governance/load-balancer/shard/aerospike/`
- [x] 2.4 删除空的 `docs/features/` 目录

## 3. 核心文档重写

- [x] 3.1 重写 `docs/PROJECT_STRUCTURE.md`，准确反映当前项目目录结构（含 openspec/、子模块、pkg/ 下所有模块）
- [x] 3.2 重写 `docs/README.md`，更新文档导航和三层结构说明（openspec/specs/ → docs/ → pkg/），纳入 OpenSpec 工作流入口
- [x] 3.3 更新 `docs/CHANGELOG.md`，记录本次文档重建变更

## 4. AI 指令文件更新

- [x] 4.1 更新 `AI.MD`，将 prompts 驱动说明改为 OpenSpec spec-driven 工作流
- [x] 4.2 更新 `CLAUDE.md`，在项目结构中增加 openspec/ 说明，在开发规约中增加 OpenSpec 工作流说明

## 5. 旧 specs/ 目录处理

- [x] 5.1 更新 `specs/README.md`，标注为"历史规格归档"，说明新功能开发使用 `openspec/specs/`
