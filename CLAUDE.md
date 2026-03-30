# CLAUDE.md - Uniface 项目

## 项目简介

Uniface (Unified Interface) 是一个 Go 基础设施接口层，提供 KV 存储、配置管理、RPC 治理等统一抽象，支持中间件热切换。

- **语言**: Go 1.24，大量使用泛型
- **模块**: `github.com/solo-kingdom/uniface`

## 项目结构

```
pkg/storage/kv/              # KV 存储接口 (Redis, Aerospike)
pkg/storage/config/           # 配置存储接口 (Consul)
pkg/rpc/governance/loadbalancer/  # 负载均衡 (roundrobin/random/weighted/consistenthash)
pkg/rpc/governance/loadbalancer/shard/  # 分片管理
pkg/rpc/governance/config/    # RPC 配置 (Consul)
specs/                        # 需求规格
docs/                         # 设计文档
prompts/                      # AI 提示词（只读）
```

## 开发规约

### 编码规范
- 遵循 `docs/AI_CODING_RULES.md` 和 `AI.MD` 的详细说明
- 遵循 Effective Go，使用 `gofmt` 格式化，最大行宽 120 字符
- 命名: PascalCase 导出、camelCase 私有、Err 前缀错误、Is/Has/Can 前缀布尔值
- Import 顺序: 标准库 → 第三方 → 内部

### 架构模式
- **接口优先**: 所有功能先定义接口 (`interface.go`)，实现在独立子目录
- **泛型**: 类型安全抽象 (`Storage[T any]`, `Balancer[T any]`)
- **Options 模式**: 可配置函数接受 `opts ...Option`
- **线程安全**: 所有实现使用 `sync.RWMutex`
- **资源管理**: 显式 `Close()` + 自动 `io.Closer` 检测
- **错误处理**: 自定义错误类型 + sentinel errors + `fmt.Errorf` 包装

### 文档
- 使用中文
- 三层结构: `specs/`(需求) → `docs/`(设计) → `pkg/`(实现)，路径保持一致
- Plan 以文档形式保存在 `docs/` 下

### 测试
- 覆盖率目标 >80%，所有导出函数必须有测试
- 随机算法支持 seed 确保确定性测试
- 包含 benchmark 测试

## 禁止事项

- 不要修改 `prompts/` 目录下的文件
- 不要使用全局变量维护状态
- 不要 panic，始终返回 error

## 构建

```bash
make mod    # 整理依赖
make build  # 构建
make test   # 测试
make clean  # 清理
```

注意: `pkg/storage/kv/redis` 是独立 Go 子模块。
