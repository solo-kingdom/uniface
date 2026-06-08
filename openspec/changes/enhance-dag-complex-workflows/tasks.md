## 1. Proto 契约扩展

- [x] 1.1 `graph.proto` 新增 `SignalPredicate`、`DynamicJoinBarrier`；`Condition`/`JoinSpec`/`WaitNodeConfig` 扩展字段
- [x] 1.2 `entity.proto` 或 `common.proto` 新增 `SignalPayload` wrapper；`runtime.proto` 的 `HopCommit` 加 `failure_reason`，`LineJournalEntry` 加 `repeated spawned_refs`
- [x] 1.3 执行 `make proto` 并确认 `api/dag/v1/*.pb.go` 生成无误

## 2. 图校验与路由

- [x] 2.1 `graph.ValidateGraphSpec` 实现 COMPUTE/WAIT 必填、兜底边、环检测
- [x] 2.2 `graph/predicate.go` 实现 `EvalSignalPredicate`；`ResolveTransitions` 支持 signal 上下文参数
- [x] 2.3 `graph` 单元测试：signal_predicate 路由、校验失败场景、环检测

## 3. LineStore 扩展

- [x] 3.1 实现 `ListChildrenByCorrelationPrefix`、`ListSpawnedFromJournal`
- [x] 3.2 `CommitHop` 支持 `failure_reason`、多 `spawned_refs`；信号 merge 后 snapshot sequence 递增
- [x] 3.3 `LineStore` 单元测试：多 spawn journal、prefix 查询

## 4. 信号投递与 payload 合并

- [x] 4.1 实现 `entity.MergeSignalPayload`（同 type_url merge + SignalPayload wrapper fallback）
- [x] 4.2 `Engine.DeliverSignal` 接入 payload merge 与 `signal_predicate` 路由
- [x] 4.3 信号相关测试：payload 合并、审批分支、merge_signal_payload=false

## 5. 动态 JOIN

- [x] 5.1 `Scheduler.checkJoinBarriers` 扩展 `dynamic_barriers` 逻辑（prefix 匹配 + expected_count + policy）
- [x] 5.2 集成测试：spawn N 子实例 + dynamic join 全部完成 / 部分完成 / JOIN_ANY_SUCCESS

## 6. 运行时可靠性

- [x] 6.1 `processCompute`/`advanceAfterCommit`/`processJoin` 捕获 `ErrNoTransition` → FAILED + journal
- [x] 6.2 单元级 `RetryPolicy` 接线（COMPUTE）；`handleExecuteError` 读取 unit def
- [x] 6.3 `processCompensation` 补偿 retry + 同 tick 连续 pop（上限 100）+ 补偿后 TERMINAL 路由
- [x] 6.4 Saga 与 runtime 测试：补偿连续处理、retry 耗尽、ErrNoTransition 失败

## 7. 集成与 Lab

- [x] 7.1 扩展 `integration_test.go`：信号分支黄金路径 + 动态拆单 join 场景
- [x] 7.2 `lab/internal/dag` 增加 fixture 演示审批分支与动态 join（可选 YAML）
- [x] 7.3 `make test` 全绿
