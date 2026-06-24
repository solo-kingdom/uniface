// Package units 提供声明式 ComputeUnit 内置实现（首期 HttpUnit）。
//
// 本包保持根模块零依赖原则：通过 HttpClientResolver 接口注入服务实例解析能力，
// Balancer 适配器由调用方（lab、业务进程）在 pkg/dag/units/balanceradapter 中按需注入。
//
// 注：HttpClientResolver 接口定义在 pkg/dag 根包（dag.HttpClientResolver），
// 以便 dag.Options 引用而不产生循环依赖。本包以类型别名重新导出，保持 resolver 的逻辑归属。
package units

import (
	"github.com/solo-kingdom/uniface/pkg/dag"
)

// HttpClientResolver 是 dag.HttpClientResolver 的类型别名。
//
// 实现方通常包装 uniface.Balancer[http.Client]：
//   - ResolveClient 返回的 *http.Client 用于发起请求
//   - 返回的 baseURL 形如 "http://10.0.1.5:8080"，HttpUnit 将在其后拼接 path
//
// 当解析失败（如 Balancer 无可用实例）SHALL 返回错误，HttpUnit 将其视为可重试错误。
type HttpClientResolver = dag.HttpClientResolver
