# 项目结构

本文档描述 Uniface 项目的实际目录结构和组织方式。

```
uniface/
├── pkg/                           # 公共包（可复用库）
│   ├── rpc/governance/
│   │   ├── config/               # 配置存储接口
│   │   │   ├── interface.go      # 接口定义
│   │   │   ├── options.go        # Options 模式
│   │   │   ├── errors.go         # 错误定义
│   │   │   └── consul/           # Consul 实现（Go 子模块）
│   │   └── loadbalancer/         # 负载均衡器
│   │       ├── interface.go      # 接口定义（泛型 Balancer[T]）
│   │       ├── options.go        # Options 模式
│   │       ├── errors.go         # 错误定义
│   │       ├── base/             # 基础实现（客户端缓存、实例管理）
│   │       ├── implementations/  # 算法实现
│   │       │   ├── consistenthash/
│   │       │   ├── random/
│   │       │   ├── roundrobin/
│   │       │   └── weighted/
│   │       └── shard/            # 分片管理器
│   │           ├── interface.go
│   │           ├── manager.go
│   │           └── errors.go
│   └── storage/
│       ├── kv/                   # KV 存储接口
│       │   ├── interface.go      # 接口定义
│       │   ├── options.go        # Options 模式
│       │   ├── errors.go         # 错误定义
│       │   ├── redis/            # Redis 实现（Go 子模块）
│       │   ├── aerospike/        # Aerospike 实现（Go 子模块）
│       │   └── boltdb/           # BoltDB 实现（Go 子模块）
│       └── config/               # 配置存储接口
│           ├── interface.go
│           ├── options.go
│           └── errors.go
│
├── docs/                          # 文档（与代码路径保持对应）
│   ├── AI_CODING_RULES.md        # AI 编码规则
│   ├── PROJECT_STRUCTURE.md      # 本文件
│   ├── README.md                 # 文档索引中心
│   ├── CHANGELOG.md              # 文档变更日志
│   ├── rpc/governance/
│   │   ├── load-balancer/        # 对应 pkg/rpc/governance/loadbalancer/
│   │   │   ├── implementation-plan.md
│   │   │   └── shard/           # 对应 pkg/.../loadbalancer/shard/
│   │   │       └── aerospike/
│   │   └── config/              # 对应 pkg/rpc/governance/config/
│   └── storage/
│       ├── kv/                   # 对应 pkg/storage/kv/
│       └── config/               # 对应 pkg/storage/config/
│
├── openspec/                      # OpenSpec spec-driven 工作流
│   ├── config.yaml               # 工作流配置
│   ├── specs/                    # 能力规格（capability specs）
│   │   ├── kv-storage/spec.md
│   │   ├── config-storage/spec.md
│   │   ├── load-balancer/spec.md
│   │   └── shard-manager/spec.md
│   └── changes/                  # 变更管理
│       └── archive/              # 已归档的变更
│
├── specs/                         # 历史规格归档（只读参考）
│   ├── architecture/
│   └── features/
│       ├── rpc/governance/
│       └── storage/kv/
│
├── prompts/                       # AI 提示词模板（只读，历史参考）
│   ├── architecture/
│   ├── features/
│   └── tasks/
│
├── go.mod                         # Go 模块定义 (Go 1.24)
├── Makefile                       # 构建自动化
├── AI.MD                          # AI 开发规则
├── CLAUDE.md                      # Claude/AI 助手指令
├── README.md                      # 项目概述
└── LICENSE                        # MIT 许可证
```

## 目录说明

### `pkg/`

公共包，包含可被外部项目导入的稳定 API。

**Go 子模块**（独立 go.mod）：
- `pkg/storage/kv/redis/` - Redis KV 实现
- `pkg/storage/kv/aerospike/` - Aerospike KV 实现
- `pkg/storage/kv/boltdb/` - BoltDB KV 实现
- `pkg/rpc/governance/config/consul/` - Consul 配置实现

### `docs/`

项目文档，路径与 `pkg/` 代码路径保持对应关系。

| 文档路径 | 对应代码 |
|---------|---------|
| `docs/rpc/governance/load-balancer/` | `pkg/rpc/governance/loadbalancer/` |
| `docs/rpc/governance/config/` | `pkg/rpc/governance/config/` |
| `docs/storage/kv/` | `pkg/storage/kv/` |
| `docs/storage/config/` | `pkg/storage/config/` |

### `openspec/`

OpenSpec spec-driven 开发工作流目录。

- `specs/` - 能力规格（capability specs），定义各功能的接口契约和行为规格
- `changes/` - 活跃的变更管理，每个变更包含 proposal、design、specs、tasks

### `specs/`

历史规格归档。包含原始的需求规格文件（编号格式如 `00-iface.md`），用于历史参考。新功能开发使用 `openspec/specs/`。

### `prompts/`

AI 提示词模板，只读参考。包含原始的需求描述和实现指导。

## 核心原则

### 1. 接口优先

所有功能先定义接口 (`interface.go`)，实现在独立子目录中。

### 2. 泛型抽象

使用 Go 泛型实现类型安全：`Storage[T any]`、`Balancer[T any]`。

### 3. Options 模式

可配置函数接受 `opts ...Option`，灵活组合配置。

### 4. 三层文档结构

```
openspec/specs/    → 能力规格（定义"做什么"）
docs/              → 设计文档（记录"怎么做"）
pkg/               → 代码实现（"做出来"）
```

### 5. 文档路径一致性

文档路径与代码路径保持对应，便于查找。注意命名差异：代码用 `loadbalancer`（Go 包名无连字符），文档用 `load-balancer`。

## 添加新功能

使用 OpenSpec spec-driven 工作流：

1. 运行 `/opsx:propose <描述>` 创建变更提案
2. 系统自动生成 proposal、design、specs、tasks
3. 运行 `/opsx:apply` 按任务实施
4. 完成后运行 `/opsx:archive` 归档

## Go 子模块管理

项目使用 Go 多模块仓库（multi-module repository），部分实现作为独立 Go 子模块管理：

```bash
make mod    # 整理所有模块依赖
make build  # 构建所有模块
make test   # 测试所有模块
```
