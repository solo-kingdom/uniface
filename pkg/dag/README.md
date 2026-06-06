# pkg/dag — 实体实例 DAG 执行引擎

以**实体实例为线、计算单元为节点**的工作流编排框架。业务实现 `ComputeUnit` / `Compensator`，框架负责调度、持久化、Journal、Saga 与类型注册。

## 架构

```
api/dag/v1/*.proto     # Protobuf 契约
pkg/dag/
  interface.go         # Engine, ComputeUnit, LineStore, Registry ...
  entity/              # 实体校验与快照辅助
  graph/               # GraphSpec 校验与 GraphResolver
  runtime/             # 幂等键生成
  memory/              # MVP 内存实现（Engine + LineStore + Registry）
```

## 核心概念

| 概念 | 说明 |
|------|------|
| EntityInstance | 实例线，含 status、sequence、current_node |
| GraphSpec | 数据驱动图：COMPUTE / WAIT / JOIN / TERMINAL |
| ExecutionRecord | hop 幂等键 `hash(entity_id, node_id, input_sequence)` |
| CommitHop | 原子提交 journal + instance + saga stack |
| SagaState | 补偿栈，失败时 LIFO 执行 Compensator |

## 快速开始

```go
reg := memory.NewRegistry()
store := memory.NewLineStore()
eng := memory.NewEngine(reg, store)

// 注册类型、图、计算单元 ...
inst, err := eng.StartInstance(ctx, &dagv1.StartInstanceRequest{...})

for inst.Status == RUNNING {
    _ = eng.RunOnce(ctx)
}

// 等待节点收到信号
_ = eng.DeliverSignal(ctx, &dagv1.SignalDelivery{...})
```

## 黄金路径

集成测试 `pkg/dag/memory/integration_test.go` 覆盖：

**Start → Validate → Wait → Charge（崩溃重试）→ Spawn → Join → Terminal SUCCESS**

以及失败分支：**Charge 失败 → Refund 补偿 → COMPENSATED**

## 构建

```bash
make proto   # 生成 api/dag/v1/*.pb.go
make build
make test
```

## MVP 限制

- 单进程内存存储，无分布式 Worker
- FieldPredicate 仅支持标量与一层 repeated 下标
- SideEffect 仅 NONE + IDEMPOTENT
- EXTERNAL 副作用与 CEL 路由为二期
