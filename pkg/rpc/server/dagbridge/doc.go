// Package dagbridge 把 app.StringCallResult 终态映射到 rpcserver.Response，
// 为「DAG 同步调用 → HTTP 响应」提供统一、无传输依赖的翻译层。
//
// 依赖边界：
//   - 仅依赖 pkg/dag/invocation/app 与 pkg/rpc/server；
//   - 不引入 chi / gorilla/mux 等 HTTP 路由库；
//   - 不直接调用 net/http；
//   - 不依赖具体传输实现（如 pkg/rpc/server/http）。
//
// 用途：lab/internal/daghttp、未来的 kv/config/queue 等"同步 DAG 暴露成 RPC"
// 域共享同一套"终态 → 200/500 响应"映射，避免每个域重复实现。
package dagbridge
