# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目简介

Uniface (Unified Interface) 是一个 Go 基础设施抽象层，提供 KV 存储、配置管理、RPC 治理（负载均衡）等统一接口，通过面向接口编程实现中间件热切换。

- **语言**: Go 1.24，大量使用泛型
- **模块**: `github.com/solo-kingdom/uniface`
- **根模块零依赖**；具体实现（redis、aerospike、boltdb、consul）为独立 Go 子模块，各有自己的 `go.mod`

## 构建命令

```bash
make mod      # 整理所有模块依赖（根模块 + 子模块）
make build    # 构建所有模块
make test     # 测试所有模块
make clean    # 清理 bin/、*.test、*.out
```

运行单个测试：
```bash
go test -v -run TestFunctionName ./pkg/storage/kv/...
```

**注意**: `pkg/storage/kv/redis`、`pkg/storage/kv/aerospike`、`pkg/storage/kv/boltdb`、`pkg/rpc/governance/config/consul` 是独立 Go 子模块。修改这些目录后需在各子模块内单独执行 `go mod tidy`。

## 架构

### 接口优先

每个功能领域在包根目录的 `interface.go` 中定义公开接口，实现在子目录中。标准文件布局：

```
pkg/<domain>/<feature>/
  interface.go    # 公开接口定义
  options.go      # 函数式 Options 模式
  errors.go       # sentinel errors + 自定义错误类型
  <impl>/         # 具体实现（子模块）
```

### 核心接口

- **`kv.Storage`** (`pkg/storage/kv/`) — 非泛型。方法：`Get/Set/Delete`、`BatchGet/BatchSet/BatchDelete`、`Exists/List`、`Close`。值使用 `interface{}` 配合指针解码。
- **`config.Storage`** (`pkg/storage/config/`、`pkg/rpc/governance/config/`) — 非泛型。在 CRUD 基础上增加 watch/subscribe 语义（`Watch`、`WatchPrefix`、`Handler` 回调）。
- **`Balancer[T any]`** (`pkg/rpc/governance/loadbalancer/`) — 泛型，参数化为客户端类型。方法：`Select`、`SelectClient`、`Add/Remove/Update`、`GetAll`、`Close`。定义 `Instance` 结构体（ID、Address、Port、Weight、Metadata）。
- **`shard.Manager`** (`pkg/rpc/governance/loadbalancer/shard/`) — 非泛型。通过组合 `Balancer[interface{}]` 实现基于 key 的路由。

### 结构模式

**函数式 Options** — 全局统一使用：
```go
type Options struct { ... }
type Option func(*Options)
func DefaultOptions() *Options { ... }
func MergeOptions(opts ...Option) *Options { return DefaultOptions().Apply(opts...) }
func WithXxx(...) Option { ... }
```
实现子模块使用 `Config` 结构体代替 `Options`（连接级配置 vs 操作级配置）。

**Base 嵌入（模板方法）** — `base.BaseBalancer[T]` 提供实例管理、双重检查锁定的客户端缓存、线程安全、`Close()` 及自动 `io.Closer` 检测。具体实现（roundrobin、random、weighted、consistenthash）嵌入 Base 后只需实现 `Select`/`SelectClient`。

**组合** — `ShardManager` 包装 `Balancer[interface{}]` 而非嵌入，提供简化的 key-based API。

**错误处理** — 两层结构：sentinel errors（`errors.New`）处理简单场景，自定义错误结构体（含 `Op`/`Key`/`Err` 字段及 `Unwrap()`）提供上下文信息。均支持 `errors.Is()`/`errors.As()`。

**线程安全** — 所有实现使用 `sync.RWMutex`；无锁计数器使用 `atomic.AddUint64`。

**资源管理** — 所有接口包含 `Close()`。`BaseBalancer` 通过 `any(client).(io.Closer)` 自动检测泛型客户端类型是否实现 `io.Closer`。

### 文档与代码路径映射

三层结构，路径保持对应：
- `openspec/specs/` — 能力规格（接口契约、行为规格），变更的基线
- `docs/` — 设计文档（镜像 `pkg/` 路径；注意：代码用 `loadbalancer`，文档用 `load-balancer`）
- `pkg/` — 代码实现

## OpenSpec 工作流

使用 spec-driven 工作流进行功能开发：
- `/opsx:propose <描述>` — 创建变更提案（自动生成 proposal、design、specs、tasks）
- `/opsx:apply` — 按任务实施变更
- `/opsx:archive` — 归档已完成的变更
- `/opsx:explore` — 探索模式，思考伙伴

## 编码规范

- 遵循 Effective Go，`gofmt` 格式化，最大行宽 120 字符
- 命名：PascalCase 导出、camelCase 私有、`Err` 前缀错误、`Is/Has/Can` 前缀布尔值
- Import 顺序：标准库 → 第三方 → 内部
- 文档和注释使用中文

## 禁止事项

- 不要修改 `prompts/` 目录下的文件
- 不要使用全局变量维护状态
- 不要 panic，始终返回 `error`
