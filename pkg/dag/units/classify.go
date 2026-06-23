package units

import (
	"fmt"
	"net/http"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
)

// classifyAndMap 按状态码分类并完成 response → mutation 映射。
//
// 决策树（spec D5 / ResponseMapping）：
//   - 2xx → 走 ResponseMapping（MODE_AUTO 默认 / MODE_MUTATION）
//   - 命中 retry_status_codes（默认 502/503/504）→ 可重试错误（返回 error）
//   - 命中 fail_status_codes（默认 400/401/403/404/409/422）→ mutation.fail（trigger_compensation=false）
//   - 未分类 4xx/5xx → 保守 fail
func (u *HttpUnit) classifyAndMap(resp *http.Response, body []byte, snapshot *dagv1.EntitySnapshot) (*dagv1.EntityMutation, error) {
	status := resp.StatusCode
	if status >= 200 && status < 300 {
		return u.mapResponse(body, snapshot)
	}
	retryCodes, failCodes := u.effectiveStatusCodes()
	if containsInt32(retryCodes, int32(status)) {
		return nil, NewRetryableError(u.config.GetService(), status, nil)
	}
	if containsInt32(failCodes, int32(status)) || status >= 400 {
		return failMutation(fmt.Sprintf("http %d", status), false), nil
	}
	// 3xx 等其他情况：保守 fail。
	return failMutation(fmt.Sprintf("http %d", status), false), nil
}

// effectiveStatusCodes 返回生效的重试/失败状态码列表（缺省用默认值）。
func (u *HttpUnit) effectiveStatusCodes() (retry, fail []int32) {
	retry = defaultRetryStatusCodes
	fail = defaultFailStatusCodes
	if rc := u.config.GetRetryOn(); rc != nil {
		if len(rc.RetryStatusCodes) > 0 {
			retry = rc.RetryStatusCodes
		}
		if len(rc.FailStatusCodes) > 0 {
			fail = rc.FailStatusCodes
		}
	}
	return retry, fail
}

func containsInt32(list []int32, v int32) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
