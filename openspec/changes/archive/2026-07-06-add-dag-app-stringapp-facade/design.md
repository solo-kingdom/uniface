## Context

`lab/internal/daghttp` 是「HTTP→DAG→响应」最小范式验证地，自身 ~505 行，架在 ~5600 行地基上。地基中 `pkg/dag/invocation/app`（440 行）已提供"轻量请求式 DAG 封装"语义，但缺少"对最常见 string-payload 场景的最短路径"；`pkg/rpc/server`（528 行 + http 子包 184 行）提供传输无关抽象，但缺少"把 DAG 终态翻译为 HTTP 响应"的小桥；`lab/internal/web/api` 的 `OpRecorder`（~70 行）只接受 `(op, detail, ok, err)`，把"如何由 DAG 结果派生 ok"的语义推回每个调用方。本次新增 3 块横向复用能力，让 daghttp 与未来同类 handler 共享同一套样板消除路径。

约束：
- 不修改 `pkg/dag/memory` 引擎、不修改 `pkg/rpc/server` 核心 `Server` 接口
- 不引入新外部依赖
- 不破坏 `app.Runtime` / `Invoker` / `Loader` / `Codec` 既有 API
- `StringApp` 不得隐式注册任何 `lab.*` 业务单元

## Goals / Non-Goals

**Goals:**

- 消灭 `lab/internal/daghttp.Runtime` 包装层（82 行 pass-through）
- 让 handler 写 `idGen.Next()` 而不是 `atomic.Uint64 + Sprintf`
- 让 handler 写 `dagbridge.ResponseForTerminalResult(res)` 而不是 `if IsCompleted { 200 } else { 500 }`
- 让 handler 写 `rec.RecordResult(op, detail, res)` 而不是 `rec.Record(op, detail, res.IsCompleted(), nil)`
- `StringApp` 失败时自动 close 底层 Runtime，调用方不再写三遍 `_ = rt.Close(); return nil, err`
- 解耦 `wiring/daghttp.go` 对 daghttp 内部相对路径的硬编码

**Non-Goals:**

- 不修改 `lab/internal/dag` 那套 698 行大 Runtime
- 不引入 `MessageApp`（message-payload 场景沿用 `app.Runtime.InvokeMessage`）
- 不为 `RecordResult` 设计完整 ResultSentinel 体系（仅 3 方法最小集）
- 不改 `pkg/rpc/server` 已有的 `Server` / `Transport` / `Middleware` 接口
- 不引入新传输（gRPC / WebSocket）
- 不动 fixture 解析器（loader）

## Decisions

### D1: `StringApp` 嵌 `*Runtime` 而非重新实现

**决策**: `StringApp` 用组合（`type StringApp struct { *Runtime; typeKey *EntityTypeKey }`）而非另起一套。

```go
type StringApp struct {
    *Runtime
    typeKey *dagv1.EntityTypeKey
}
```

**理由**:
- 复用 `Runtime` 全部已有方法（`LoadGraphID` / `LoadedGraphs` / `Close` / `RegisterGraph` 等）零成本
- 调用方可经 `StringApp.Runtime` 字段访问底层 Runtime（仅在 `LoadGraphFromDir` 等 StringApp 未覆盖的方法上需要）
- 避免重新实现 `Runtime` 的 17 个公共方法

**替代**:
- 独立类型嵌入 `*invocationmemory.Runtime` —— 放弃，会丢失 `app.Runtime` 的 `LoadedGraphs` / `recordLoaded` / `graphDir` 等行为
- 全部独立实现 —— 放弃，~150 行重复代码

### D2: `StringApp` 注册失败自动 close 底层 Runtime

**决策**: `StringApp.RegisterUnit` 在调用 `Runtime.RegisterStringUnit` 返回 error 时，先 `s.Runtime.Close()` 再返回 error。

```go
func (s *StringApp) RegisterUnit(unitID string, fn StringFunc) error {
    if err := s.Runtime.RegisterStringUnit(unitID, s.typeKey, fn); err != nil {
        _ = s.Runtime.Close()
        return err
    }
    return nil
}
```

**理由**:
- 调用方构造 StringApp 时通常串行注册 2~5 个单元；任一失败都意味着整体构造失败
- 强制调用方写 5 遍 `_ = sa.Close(); return nil, err` 与既有 `daghttp.Runtime` 同样的痛点
- `Close()` 失败被忽略（与 `defer rt.Close()` 风格一致）

**替代**:
- 暴露 builder + Build —— 增加 API 表面，权衡后认为不值得（lab 子项目普遍用法是 2~3 单元）
- 暴露 `Must*` 变体（`MustRegisterUnit`）—— 不推荐，掩盖错误；不必要

### D3: `EntityIDGen` 用 `sync/atomic.Uint64` 而非 `chan struct{}` 或全局 ID

**决策**: `EntityIDGen` 内部持 `atomic.Uint64`，每次 `Next()` `Add(1)` 取新值，格式 `<prefix>-<n>`。

```go
type EntityIDGen struct {
    counter atomic.Uint64
    prefix  string
}
func (g *EntityIDGen) Next() string {
    n := g.counter.Add(1)
    return fmt.Sprintf("%s-%d", g.prefix, n)
}
```

**理由**:
- `atomic.Uint64` 比 mutex 快一个量级，N 并发 goroutine 争用也仅是原子加
- 格式简单，可读，便于日志追踪
- 每次 `NewEntityIDGen` 创建独立计数器，lab 子项目不共享 ID 空间（避免"lab-1"与"lab-1"碰撞）

**替代**:
- `google/uuid` —— 放弃，UUID 不可读、与现有 `lab-1` / `http-1` 命名风格不一致
- 全局包级 `var idCounter atomic.Uint64` —— 放弃，跨域串号；daghttp 想要 `"http-1"`，未来 kv 想要 `"kv-1"`
- `crypto/rand` —— 放弃，太长

### D4: `dagbridge` 作为子包而非方法挂在 `rpcserver.Response` 上

**决策**: 新建 `pkg/rpc/server/dagbridge` 子包，提供 `ResponseForTerminalResult` 纯函数。

```go
// pkg/rpc/server/dagbridge/dagbridge.go
package dagbridge
import (
    "github.com/solo-kingdom/uniface/pkg/dag/invocation/app"
    rpcserver "github.com/solo-kingdom/uniface/pkg/rpc/server"
)
func ResponseForTerminalResult(r *app.StringCallResult) *rpcserver.Response { ... }
```

**理由**:
- `pkg/rpc/server` 根包零外部依赖；导入 `pkg/dag/invocation/app` 会拉进 `pkg/dag/memory` 等重依赖，破坏根包纯洁性
- 子包可被 lab 模块按需导入；`pkg/rpc/server` 根包仍保持"任何 transport 都能用"
- `dagbridge` 仅 ~30 行，不值得独立顶级包

**替代**:
- 把函数挂到 `pkg/dag/invocation/app` —— 放弃，`app` 不应反向依赖 `pkg/rpc/server`（分层破坏）
- 把函数挂到 `app.StringCallResult` 方法 —— 放弃，循环依赖；且违反"app 不应知道 HTTP"

### D5: `ResultSentinel` 最小接口（3 方法）

**决策**: `ResultSentinel` 接口仅 3 方法：`IsCompleted() bool` / `Status() string` / `Err() error`（`Err` 可缺省）。

```go
type ResultSentinel interface {
    IsCompleted() bool
    Status() string
}
type ResultSentinelWithErr interface {
    ResultSentinel
    Err() error
}
```

**理由**:
- `*app.StringCallResult` 已有 `IsCompleted()` 与 `Status()`（代理到 `*CallResult`），只需补 `Err() error`（代理到 `TerminalErr()`）
- `Err` 拆成可选方法，简化其它 `ResultSentinel` 实现方（如未来 `*app.CallResult` 仅需前两个）
- recorder 内部做 type assertion：`if e, ok := res.(ResultSentinelWithErr); ok { ... }`

**替代**:
- 单接口 3 方法强制实现 —— 拒绝，对无法暴露 `Err()` 的实现方不友好
- 引入 `errors.Join` 风格的 sentinel error —— 过度设计

### D6: `OpRecorder.RecordResult` 用 type assertion 走最优路径

**决策**: `RecordResult` 接受 `ResultSentinel` 接口，运行时 type assert 到 `ResultSentinelWithErr`（若实现）取 `Err()`；否则 `Error` 字段填 `"status=<Status>"`。

```go
func (r *OpRecorder) RecordResult(op, detail string, res ResultSentinel) {
    if res == nil {
        r.Record(op, detail, false, errors.New("nil result"))
        return
    }
    ok := res.IsCompleted()
    var err error
    if !ok {
        if e, hasErr := res.(ResultSentinelWithErr); hasErr && e.Err() != nil {
            err = e.Err()
        } else {
            err = fmt.Errorf("status=%s", res.Status())
        }
    }
    r.Record(op, detail, ok, err)
}
```

**理由**:
- 类型断言成本极低（interface 内部 itab 比较），热路径上无负担
- recorder 仍复用既有 `Record` 内部锁与环形缓冲逻辑

**替代**:
- 复制 `Record` 内部逻辑到 `RecordResult` —— 放弃，重复代码 + 锁状态分散

### D7: wiring fixtures 默认值下沉到 daghttp 包

**决策**: `lab/internal/wiring/daghttp.go` 删掉 `if cfg.FixturesDir == "" { fixtures = "internal/daghttp/fixtures/graphs" }` 逻辑；`DAGConfig.FixturesDir` 改为必填（缺省时返回 error 并提示在 daghttp 包内常量查找）。daghttp 自身导出 `DefaultFixturesDir = "internal/daghttp/fixtures/graphs"`。

**理由**:
- 解决"wiring 公共层硬编码 daghttp 内部路径"的耦合
- 强制每个调用方（含测试）显式声明 fixtures 位置；漏配立刻报错

**替代**:
- 用环境变量 `LAB_DAGHTTP_FIXTURES_DIR` 兜底 —— 过度，环境变量本就是配置层面

### D8: 测试策略保留"handler.go 源码扫描"断言

**决策**: daghttp `handler_test.go` 中的 `TestHandler_UsesAppFacade` 继续存在并把 `anypb.` / `InvokeRequest` 列入黑名单字符串；新增一条白名单断言 `body MUST NOT contain "atomic.Uint64"`（验证 entityID 已下沉到地基）。

**理由**:
- 源码扫描断言是该项目的"架构防腐"机制，移除会放任未来再次穿透地基
- 新增的 `atomic.Uint64` 断言防止 entityID 反向回流 handler

## Risks / Trade-offs

| 风险 | 缓解 |
|------|------|
| `StringApp` 隐式关闭 Runtime 可能在测试中断言失败 | `RegisterUnit` 错误时 `Close` 是同步的；测试可用 `defer rt.Close()` 兜底 |
| `EntityIDGen` 全局 ID 计数器被多 gen 共享的误解 | 文档明确"每次 `NewEntityIDGen` 返回独立计数器"；spec 给出多 gen 独立性 scenario |
| `dagbridge` 子包位置引发循环依赖 | `dagbridge` 只依赖 `app` 与 `rpcserver`；反向无任何依赖 |
| `RecordResult` 接口引入让 `*app.StringCallResult` 多 1 个方法 | `Err()` 转发到 `TerminalErr()`，零成本 |
| `wiring` 必填 FixturesDir 破坏现有 yaml 用户 | `configs/default.yaml` 中 `dag.fixtures_dir` 已配置；缺省时仅命令直接调用方受影响 |
| 改 `daghttp.Runtime` 公开字段为 `*app.StringApp` 后，未来想加 daghttp 专属方法没地方挂 | `StringApp` 嵌 `*Runtime`，调用方仍能 `appStrApp.Runtime.RegisterGraph(...)`；专属方法可挂回 `Service` |

## Migration Plan

1. **PR1（地基能力）**：
   - 新增 `pkg/dag/invocation/app/string_app.go` + `entity_id.go`
   - 新增 `pkg/rpc/server/dagbridge/dagbridge.go`
   - 在 `lab/internal/web/api/status.go` 追加 `RecordResult` 与 `ResultSentinel` 接口
   - 在 `pkg/dag/invocation/app/invoke.go` 给 `StringCallResult` 加 `Err()` 方法
   - 各自配套单测
   - **回滚策略**：纯新增，对外既有 API 无破坏；revert commit 即可

2. **PR2（daghttp 切换）**：
   - `lab/internal/daghttp/runtime.go` 删除；`lab/internal/wiring/daghttp.go` 改用 `app.NewStringApp`
   - `lab/internal/daghttp/handler.go` 用 `idGen` / `dagbridge` / `RecordResult` 替换三段手工代码
   - `lab/internal/web/api/status.go` 同步导出 `DefaultFixturesDir` 或类似常量（实际下沉到 `lab/internal/daghttp/handler.go`）
   - 测试 `TestHandler_UsesAppFacade` 追加白名单
   - `make test` 全绿
   - **回滚策略**：revert commit；PR1 保留

## Open Questions

- **Q1**：`StringApp` 是否需要支持 `RegisterMessageUnit`（message-payload 单元）？当前决策是不做；未来若 kv/config 域需要，可补一个 `MessageApp`。
- **Q2**：`RecordResult` 是否要支持 `Op = ""` 跳过写入？当前决策是不支持；调用方按需不调用即可。
- **Q3**：`dagbridge` 是否要支持自定义状态码映射（200 / 4xx / 5xx）？当前决策是固定 200/500 两种；未来若有"业务已知 4xx"场景，可补 `ResponseForTerminalResultWith(r, opts)`。
- **Q4**：`EntityIDGen` 的 prefix 是否要支持格式模板（如 `http-{prefix}-{n}`）？当前决策是固定 `<prefix>-<n>`；Go template 是过度设计。
