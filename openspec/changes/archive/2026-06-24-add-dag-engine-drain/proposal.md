## Why

`Engine.RunOnce` 是调度器单步原语：每次调用至多推进一个实例的一个 hop，而非「跑完整张图」。当前 `lab/internal/daghttp` 与多处集成测试在业务层手写 `for { RunOnce; if terminal break }` 循环，并硬编码 `maxDrainIters=100` 作为防死循环兜底。这暴露了引擎内部调度语义，且上限与图规模无关，既难理解（节点数固定为何还要迭代？）也不安全（复杂图或重试/补偿可能超过 100 步）。

图执行引擎应提供「排空实例至终态」的高层 API，让调用方只关心 Start → Drain → 读结果，而非调度细节。

## What Changes

- 在 `pkg/dag.Engine` 新增 `DrainInstance(ctx, ref, opts...)`：对指定实例循环 `RunOnce` 直至终态、`ctx` 取消或达安全上限，返回终态 `EntityInstance` 与明确错误（如 `ErrDrainExceeded`）。
- `memory.Engine` 实现排空逻辑；上限基于图节点数与可配置系数推导，保留绝对硬顶作为防 bug 死循环兜底。
- `lab/internal/daghttp` 删除本地 `drain`/`maxDrainIters`，改为调用 `Engine.DrainInstance`。
- `lab/internal/dag.Runtime` 暴露 `Drain` 包装方法供 lab 使用。
- 集成测试中与「跑至终态」相关的 `RunOnce` 循环可逐步改用 `DrainInstance`（本期至少覆盖 echo 黄金路径）。

## Non-goals

- 不改变 `RunOnce` 语义或异步 worker 模型。
- 不在 `DrainInstance` 内自动投递外部信号（WAIT 节点仍须调用方 `DeliverSignal` 后再 Drain，或后续单独 change 支持 signal-aware drain）。
- 不引入持久化 store 或分布式调度。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `dag-runtime`：新增 `DrainInstance` 需求——引擎 SHALL 提供实例级排空至终态能力，含可推导的安全上限与超时/取消语义。

## Impact

- **API**：`pkg/dag/interface.go` 扩展 `Engine` 接口；`pkg/dag/errors.go` 可能新增 sentinel error。
- **实现**：`pkg/dag/memory/engine.go`；`pkg/dag/options.go` 可选 `WithDrainMaxHops`。
- **调用方**：`lab/internal/daghttp/handler.go`、`lab/internal/dag/runtime.go`；部分 `pkg/dag/memory/*_test.go`。
- **规格**：`openspec/specs/dag-runtime/spec.md` delta。
