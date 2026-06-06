## 1. 实体与接口辅助

- [x] 1.1 `WaitingInstance` 增加 `AcceptedSignals` 字段（`interface.go`）
- [x] 1.2 实现 `ValidateOutputType`、`ValidateSchemaCompatible`（`entity/entity.go`）
- [x] 1.3 为 entity 校验函数添加单元测试

## 2. Registry 图版本解析

- [x] 2.1 `Registry` 增加 `latestGraphs` 追踪与 `GetLatestGraphVersion`
- [x] 2.2 实现 `ResolveGraphForInstance`，修正 `graph.ResolveGraphVersion` ON_NODE 分支
- [x] 2.3 添加 PIN_ON_NODE 解析单元测试

## 3. LineStore 加固

- [x] 3.1 `CommitHop` 在 `COMPENSATION_COMMITTED` 时原子 pop saga 栈；幂等命中 reconcile pop
- [x] 3.2 新增 `UpdateExecutionAttempt`、`AdvanceInstanceNode`
- [x] 3.3 `CommitHop` 幂等与 saga reconcile 单元测试

## 4. Engine / Scheduler 核心修复

- [x] 4.1 `Engine` 增加 per-entity 锁；`RunOnce`/`DeliverSignal`/超时处理加锁
- [x] 4.2 全路径改用 `ResolveGraphForInstance` 替代直接 `GetGraph`
- [x] 4.3 重写 `processCompensation`：先 Compensate 后 CommitHop；使用 `forward_snapshot`
- [x] 4.4 压栈时写入 `forward_snapshot`；spawn 使用 `JOURNAL_KIND_SPAWNED`
- [x] 4.5 `commitCompute` 前校验 output/schema；`handleExecuteError` 持久化 attempt
- [x] 4.6 修复 `advanceAfterCommit` 调用 `AdvanceInstanceNode`
- [x] 4.7 修复 `processTimeouts` 使用正确 node_id/sequence；实现 `accepted_signals`
- [x] 4.8 `enterWait` 持久化 `AcceptedSignals`

## 5. 集成测试与验证

- [x] 5.1 补偿崩溃重试测试（Compensate 成功、CommitHop 前中断场景）
- [x] 5.2 `accepted_signals` 与 PIN_ON_NODE 集成测试
- [x] 5.3 超时路由与并发 RunOnce 防双 Execute 测试
- [x] 5.4 运行 `make test` 全模块通过
