package dagbridge

import (
	"fmt"

	"github.com/solo-kingdom/uniface/pkg/dag/invocation/app"
	rpcserver "github.com/solo-kingdom/uniface/pkg/rpc/server"
)

// ResponseForTerminalResult 把 *app.StringCallResult 终态映射到统一 rpcserver.Response。
//
//   - IsCompleted() → 200 + Value
//   - IsWaiting() → 500（同步调用上下文不应进入 WAITING；映射为错误响应）
//   - 其它终态（FAILED / COMPENSATED / CANCELLED）→ 500 + "terminal <Status>: <Value>"
//   - r == nil → 500 + "nil dag result"，不 panic
//
// 函数为纯函数：无包级状态、无 I/O，不得发起新的 DAG 调用，不得修改 r。
func ResponseForTerminalResult(r *app.StringCallResult) *rpcserver.Response {
	if r == nil {
		return &rpcserver.Response{
			StatusCode: 500,
			Body:       []byte("nil dag result"),
		}
	}
	if r.IsCompleted() {
		return &rpcserver.Response{
			StatusCode: 200,
			Body:       []byte(r.Value),
		}
	}
	if r.IsWaiting() {
		return &rpcserver.Response{
			StatusCode: 500,
			Body:       []byte("terminal WAITING: instance still WAITING (sync call)"),
		}
	}
	return &rpcserver.Response{
		StatusCode: 500,
		Body:       []byte(fmt.Sprintf("terminal %s: %s", r.Status(), r.Value)),
	}
}
