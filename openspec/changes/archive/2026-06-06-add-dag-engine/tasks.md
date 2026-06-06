## 1. Protobuf 契约

- [x] 1.1 创建 `api/dag/v1/` 目录与 `common.proto`（EntityRef、EntityTypeKey、GraphVersion、InstanceStatus、RetryPolicy）
- [x] 1.2 创建 `entity.proto`（EntityInstance、EntitySnapshot、EntityMutation、SpawnSpec、WaitSignal）
- [x] 1.3 创建 `graph.proto`（GraphSpec、NodeDef、NodeKind、Transition、Condition、JoinSpec、WaitNodeConfig）
- [x] 1.4 创建 `unit.proto`（ComputeUnitDef、SideEffectClass）
- [x] 1.5 创建 `registry.proto`（EntityTypeRegistration）
- [x] 1.6 创建 `saga.proto`（SagaState、CompensationFrame、CompensationRecord、CompensationContext）
- [x] 1.7 创建 `runtime.proto`（StartInstanceRequest、LineJournalEntry、ExecutionRecord、SignalDelivery、HopCommit）
- [x] 1.8 配置 `buf` 或 `Makefile` 目标生成 Go proto 代码

## 2. 核心接口与错误

- [x] 2.1 创建 `pkg/dag/interface.go`（Engine、ComputeUnit、Compensator、GraphResolver、LineStore、Registry）
- [x] 2.2 创建 `pkg/dag/options.go`（函数式 Options 模式）
- [x] 2.3 创建 `pkg/dag/errors.go`（sentinel errors + 自定义错误类型，含 Op/EntityRef 上下文）

## 3. 实体与注册表

- [x] 3.1 实现 `pkg/dag/entity/` Go 类型与 proto 互转
- [x] 3.2 实现内存 `Registry`（GraphSpec、ComputeUnitDef、EntityTypeRegistration 注册与查询）
- [x] 3.3 实现 `EntityTypeKey` 强制校验与 `type_url` 一致性检查

## 4. 图解析器

- [x] 4.1 实现 `GraphSpec` 静态校验（entry 存在、TERMINAL 无出边、禁止空 target、可达 TERMINAL）
- [x] 4.2 实现 `FieldPredicate` 求值（proto.Unmarshal + field_path 标量/一层 repeated）
- [x] 4.3 实现 `GraphResolver.Resolve`（priority 评估、NodeKind 分支）
- [x] 4.4 实现 `GraphPinPolicy` 图版本选择逻辑

## 5. 运行时与 LineStore

- [x] 5.1 实现内存 `LineStore`（Instance、Snapshot、ExecutionRecord、Journal、SagaState 存储）
- [x] 5.2 实现 `idempotency_key` 生成与 `CreateExecution` CAS 逻辑
- [x] 5.3 实现 `CommitHop` 原子提交（journal + instance + saga push）
- [x] 5.4 实现 `Scheduler` hop 循环（COMPUTE 调度、状态推进）
- [x] 5.5 实现 `Engine`（StartInstance、GetInstance、CancelInstance）

## 6. 等待信号

- [x] 6.1 实现 `mutation.wait` 与 `NodeKind_WAIT` 进入 WAITING 逻辑
- [x] 6.2 实现 `DeliverSignal`（delivery_id 去重、SIGNAL_RECEIVED journal、恢复 RUNNING）
- [x] 6.3 实现等待超时调度（deadline → on_timeout_target_node_id）

## 7. Saga 补偿

- [x] 7.1 实现正向 hop `CompensationFrame` 压栈（CommitHop 同事务）
- [x] 7.2 实现 `Compensator` 调度与 `COMPENSATION_COMMITTED` journal
- [x] 7.3 实现失败触发 `COMPENSATING` → LIFO 补偿 → `COMPENSATED` 终态

## 8. Join 与 Spawn

- [x] 8.1 实现 `SpawnSpec` 子实例创建（显式 graph 校验、SPAWNED journal、parent 血缘）
- [x] 8.2 实现 `NodeKind_JOIN` 屏障检查与 `JOIN_COMMITTED` journal
- [x] 8.3 实现 JOIN 失败策略（子实例失败 → 父实例 Fail 可选路径）

## 9. 测试与文档

- [x] 9.1 单元测试：GraphResolver（FieldPredicate、priority、TERMINAL）
- [x] 9.2 单元测试：CommitHop EOS（重复调度、崩溃重试模拟）
- [x] 9.3 单元测试：DeliverSignal 去重、Saga 逆序补偿
- [x] 9.4 集成测试：黄金路径（Start → Validate → Wait → Charge 崩溃重试 → Spawn → Join → Terminal SUCCESS）
- [x] 9.5 集成测试：失败分支（charge 失败 → refund 补偿 → COMPENSATED）
- [x] 9.6 添加 `pkg/dag/README.md`（架构说明、接口用法、黄金路径示例）

## 10. 构建验证

- [x] 10.1 `make build` 通过
- [x] 10.2 `make test` 通过（含 `pkg/dag/...`）
- [x] 10.3 `gofmt` 格式化所有新增 Go 文件
