## 1. pkg/dag 接口与错误

- [x] 1.1 在 `pkg/dag/interface.go` 的 `Engine` 接口新增 `DrainInstance(ctx, ref, opts...) (*EntityInstance, error)`
- [x] 1.2 在 `pkg/dag/errors.go` 新增 `ErrDrainExceeded` 哨兵错误
- [x] 1.3 在 `pkg/dag/options.go` 新增 `WithDrainMaxHops(n int)` 与 Options 字段（可选系数/硬顶配置）

## 2. memory 引擎实现

- [x] 2.1 在 `pkg/dag/memory/engine.go` 实现 `DrainInstance`：终态/WAITING 终止、`RunOnce` 循环、ctx 取消
- [x] 2.2 实现 hop 上限推导：从 Registry 读图节点数 × 系数，受绝对硬顶约束
- [x] 2.3 新增 `engine_drain_test.go`：覆盖 echo 排空 COMPLETED、WAITING 提前返回、hop 上限 `ErrDrainExceeded`、ctx 取消

## 3. lab 调用方迁移

- [x] 3.1 在 `lab/internal/dag/runtime.go` 新增 `Drain(ctx, entityID)` 委托 `engine.DrainInstance`
- [x] 3.2 重构 `lab/internal/daghttp/handler.go`：删除 `drain`/`maxDrainIters`/`isTerminal`，改用 `Runtime.Drain`
- [x] 3.3 更新 `lab/internal/daghttp/handler_test.go`（如有）确保 echo 集成仍通过

## 4. 文档与验证

- [x] 4.1 在 `pkg/dag/README.md` 补充 `DrainInstance` 与 `RunOnce` 语义对比说明
- [x] 4.2 运行 `go test ./pkg/dag/... ./lab/internal/daghttp/...` 全绿
