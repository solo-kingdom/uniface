## ADDED Requirements

### Requirement: OpRecorder 类型化结果记录

`lab/internal/web/api` 的 `OpRecorder` SHALL 暴露 `RecordResult(op, detail string, res ResultSentinel)` 方法 —— 接受一个类型化结果（`*app.StringCallResult` 隐式实现 `ResultSentinel`）并自动派生 `Operation.OK` 字段：

- `res == nil` → `OK = false`，`Error = "nil result"`
- `res.IsCompleted() == true` → `OK = true`，`Error = ""`
- `res.IsCompleted() == false` → `OK = false`，`Error` 优先取 `res.Err().Error()`，否则取 `"status=<res.Status()>"`
- `Detail` 透传调用方提供的字符串

`ResultSentinel` 接口 SHALL 至少包含 `IsCompleted() bool` 与 `Status() string` 两个方法；`Err() error` 为可选方法（缺省时 recorder 回退到 `status=<Status>` 形式）。

调用方 SHALL 不再需要 `isCompleted := res.IsCompleted(); rec.Record(op, detail, isCompleted, nil)` 的手工派生代码；改写为 `rec.RecordResult(op, detail, res)`。

#### Scenario: COMPLETED 自动派生 ok=true

- **WHEN** 调用 `rec.RecordResult("echo", "e1", res)` 其中 `res.IsCompleted() == true`
- **THEN** 内部 `Operation` 的 `OK = true`，`Error` 为空

#### Scenario: FAILED 自动派生 ok=false 与错误信息

- **WHEN** 调用 `rec.RecordResult("echo", "e1", res)` 其中 `res.IsCompleted() == false`、`res.Err() != nil` 且错误信息为 `"unit failed"`
- **THEN** 内部 `Operation` 的 `OK = false`，`Error = "unit failed"`

#### Scenario: nil 入参

- **WHEN** 调用 `rec.RecordResult("echo", "e1", nil)`
- **THEN** 内部 `Operation` 的 `OK = false`，`Error` 含 `"nil result"`
- **AND** 不 panic

#### Scenario: 接口向后兼容

- **WHEN** 现有调用方继续使用 `rec.Record(op, detail, ok, err)`
- **THEN** 行为与本次变更前一致
