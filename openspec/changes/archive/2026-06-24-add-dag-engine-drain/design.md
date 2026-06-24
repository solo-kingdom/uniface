## Context

当前 `pkg/dag.Engine` 暴露 `RunOnce`：一次调度 tick，扫描 store 中所有可运行实例，对每个实例至多执行 **一个 hop**（一次 `Execute` + `CommitHop` 或等价推进）。这与「跑完整张图」是不同层次的原语。

因此 `lab/internal/daghttp` 在 HTTP handler 层实现了 `drain`：

```go
for i := 0; i < maxDrainIters; i++ {
    if terminal { return }
    RunOnce()
}
```

集成测试（如 `TestGoldenPath`）同样手写 `for i := 0; i < 50/100; i++ { RunOnce }` 循环。

**为何节点数固定仍需要多次迭代？**

- 固定的是 **图拓扑节点数 N**，不是 **RunOnce 调用次数**。每推进一个 hop 至少需要一次 `RunOnce`（有时因重试、补偿、saga pop 需要更多次）。
- echo 图（compute → terminal）最少 2 次 `RunOnce`；黄金路径含 WAIT、崩溃重试、spawn/join，测试里用 50–100 次才合理。
- `maxDrainIters=100` 是 **防死循环安全网**（图 bug、WAIT 无信号、路由环），不是「图有 100 个节点」。问题在于该逻辑不应散落在 HTTP 适配层，且 100 与具体图无关。

约束：根模块零外部依赖；遵循 `interface.go` / `options.go` / `errors.go` 布局；不 panic。

## Goals / Non-Goals

**Goals:**

- 在 `Engine` 提供 `DrainInstance`：对单一实例排空至 **终态** 或 **阻塞态**（WAITING），封装 `RunOnce` 循环。
- 上限由引擎根据实例所属图的节点规模推导，保留可配置系数与绝对硬顶。
- `daghttp` 与 lab Runtime 删除本地 `drain`/`maxDrainIters`，改调引擎 API。
- 规格化终态判定与错误语义（`ErrDrainExceeded`、`context.Canceled`）。

**Non-Goals:**

- 改变 `RunOnce` / Scheduler 内核语义。
- `DrainInstance` 内自动 `DeliverSignal` 或阻塞等待外部事件。
- 跨进程 worker 或持久化调度。

## Decisions

### 决策 1：`DrainInstance` 挂在 `Engine` 接口

```go
DrainInstance(ctx context.Context, ref *EntityRef, opts ...Option) (*EntityInstance, error)
```

- **理由**：排空是调度语义，与 `StartInstance`/`RunOnce` 同属引擎职责；HTTP/lab 层只应 Start → Drain → 读 payload。
- **备选**：仅在 `lab.Runtime` 封装。否决：测试与 future RPC 层仍会重复；引擎才是正交抽象层。

### 决策 2：终止条件 = 终态 ∪ WAITING（阻塞）

循环每轮：

1. `GetInstance`；若 `status` 为终态（`COMPLETED`/`FAILED`/`COMPENSATED`/`CANCELLED`）→ 成功返回。
2. 若 `status` 为 `WAITING` → 成功返回（实例需外部信号，继续 `RunOnce` 无进展，不应空转至硬顶）。
3. 否则 `RunOnce(ctx)`；若 `ctx` 取消 → 返回 `ctx.Err()`。
4. hop 计数 +1；若超过上限 → 返回 `ErrDrainExceeded`（包装当前实例快照）。

- **理由**：区分「跑完了」与「等信号」；避免 WAIT 场景无意义空转。
- **备选**：仅终态才返回。否决：调用方无法区分「还在跑」与「等信号」而不再次查询。

### 决策 3：hop 上限 = `max(nodes × factor, minHops)`  capped by `absoluteMax`

默认 `factor=4`（覆盖每节点重试 + 补偿），`absoluteMax=1000`（防 bug 死循环）。图节点数从 `Registry.GetGraph(instance.graph_version)` 读取；读失败回退 `absoluteMax` 的 conservative 子集（如 100）。

Option：`WithDrainMaxHops(n int)` 覆盖推导值（测试用）。

- **理由**：上限与图规模相关，比 HTTP 层写死 100 可解释；绝对硬顶保留安全网。
- **备选**：精确解析 journal 预测剩余 hop。否决：过度复杂，MVP 不需要。

### 决策 4：新增 `ErrDrainExceeded`

哨兵 error，`errors.Is` 可判；`DAGError{Op:"DrainInstance"}` 附带 `EntityRef`。

### 决策 5：迁移 daghttp

`handler.drain` 删除；`Echo` 调用 `s.rt.Drain(ctx, entityID)` → Runtime 委托 `engine.DrainInstance`。终态 payload 读取逻辑不变。

## Risks / Trade-offs

- **Drain 与全局 RunOnce 的交互**：`RunOnce` 仍处理 store 内 **所有** runnable 实例；`DrainInstance` 只关心目标实例，但每轮 `RunOnce` 可能顺带推进其他实例。→ lab 范围每请求独立 entityID、单线程排空，可接受；文档注明生产多实例并发时需 worker 模型（已有 Non-goal）。
- **WAIT 提前返回**：同步 echo 场景无影响；需信号驱动的调用方须 `DeliverSignal` 后再 `DrainInstance`。→ README / 方法注释说明。
- **接口扩展**：新增 `Engine` 方法，未来其他 Engine 实现须提供。→ 当前仅 `memory.Engine`，可同步实现。

## Migration Plan

1. 扩展 `pkg/dag` 接口与 `memory` 实现 + 单元/集成测试。
2. 更新 `lab.Runtime` 与 `daghttp`。
3. 归档 change 后 delta 合并进 `openspec/specs/dag-runtime/spec.md`。

回滚：保留 `RunOnce` 不变，删除 `DrainInstance` 与调用方改动即可。

## Open Questions

无阻塞项。是否在 `DrainOptions` 中支持「将 WAITING 视为错误而非正常返回」留待首个需要 strict-sync 的调用方再议（默认正常返回）。
