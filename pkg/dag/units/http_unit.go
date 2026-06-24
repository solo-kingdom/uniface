package units

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
)

// 默认状态码分类（spec D5）。
var (
	defaultRetryStatusCodes = []int32{502, 503, 504}
	defaultFailStatusCodes  = []int32{400, 401, 403, 404, 409, 422}
)

const (
	defaultHTTPMethod  = "POST"
	defaultHTTPTimeout = 30 * time.Second
)

// RetryableError 表示一个可重试的 HTTP 错误（状态码命中 retry_status_codes 或网络/超时）。
// 引擎的 handleExecuteError 会对 Execute 返回的 error 透明重试。
type RetryableError struct {
	Service    string
	StatusCode int
	Err        error
}

func (e *RetryableError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("http unit retryable error (service=%q status=%d): %v", e.Service, e.StatusCode, e.Err)
	}
	return fmt.Sprintf("http unit retryable error (service=%q status=%d)", e.Service, e.StatusCode)
}

func (e *RetryableError) Unwrap() error { return e.Err }

// NewRetryableError 构造可重试错误。
func NewRetryableError(service string, status int, err error) error {
	return &RetryableError{Service: service, StatusCode: status, Err: err}
}

// IsRetryableError 判断错误是否为 RetryableError。
func IsRetryableError(err error) bool {
	var re *RetryableError
	return errors.As(err, &re)
}

// HttpUnit 声明式 HTTP 计算单元。持有 HttpUnit proto 配置与 HttpClientResolver。
//
// 实现 dag.ComputeUnit 接口：Execute 发起 HTTP 调用，按 ResponseMapping 将 response 转为 EntityMutation，
// 按 RetryClassification 将错误状态码/网络错误归为可重试或 fail。
type HttpUnit struct {
	config   *dagv1.HttpUnit
	resolver dag.HttpClientResolver
	client   *http.Client // url 直连场景使用（resolver 未注入时）
}

// NewHttpUnit 构造 HttpUnit。resolver 可为 nil（仅支持 url 直连）。
func NewHttpUnit(config *dagv1.HttpUnit, resolver dag.HttpClientResolver) *HttpUnit {
	return &HttpUnit{
		config:   config,
		resolver: resolver,
		client:   &http.Client{},
	}
}

// Config 返回 proto 配置（供测试与校验）。
func (u *HttpUnit) Config() *dagv1.HttpUnit { return u.config }

// Execute 实现 dag.ComputeUnit。
func (u *HttpUnit) Execute(ctx context.Context, snapshot *dagv1.EntitySnapshot) (*dagv1.EntityMutation, error) {
	if u.config == nil {
		return nil, errors.New("http unit: nil config")
	}
	ctx, cancel := context.WithTimeout(ctx, u.timeout())
	defer cancel()

	client, baseURL, err := u.resolveTarget(ctx)
	if err != nil {
		return nil, err
	}
	body, err := u.buildBody(snapshot)
	if err != nil {
		// 路径错误（字段不存在）属于配置错误，不应重试；返回 fail 让引擎按 fail 路由。
		return failMutation(fmt.Sprintf("request body build failed: %v", err), false), nil
	}
	req, err := u.buildRequest(ctx, baseURL, body)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		// 网络/超时错误归类为可重试。
		return nil, NewRetryableError(u.config.GetService(), 0, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return u.classifyAndMap(resp, respBody, snapshot)
}

// resolveTarget 解析目标 client 与 baseURL。
func (u *HttpUnit) resolveTarget(ctx context.Context) (*http.Client, string, error) {
	service := u.config.GetService()
	if service != "" {
		if u.resolver == nil {
			return nil, "", fmt.Errorf("http unit: service %q set but no HttpClientResolver injected", service)
		}
		client, baseURL, err := u.resolver.ResolveClient(ctx, service)
		if err != nil {
			// resolver 错误（如 Balancer 无可用实例）视为可重试。
			return nil, "", NewRetryableError(service, 0, err)
		}
		if client == nil {
			client = u.client
		}
		return client, baseURL, nil
	}
	// url 直连。
	return u.client, u.config.GetUrl(), nil
}

// buildRequest 构造 HTTP 请求。
func (u *HttpUnit) buildRequest(ctx context.Context, baseURL string, body []byte) (*http.Request, error) {
	method := u.config.GetMethod()
	if method == "" {
		method = defaultHTTPMethod
	}
	full := joinURL(baseURL, u.config.GetPath())
	var bodyReader io.Reader
	if body != nil {
		bodyReader = strings.NewReader(string(body))
	}
	req, err := http.NewRequestWithContext(ctx, method, full, bodyReader)
	if err != nil {
		return nil, err
	}
	for k, v := range u.config.GetHeaders() {
		req.Header.Set(k, v)
	}
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// joinURL 拼接 baseURL 与 path，处理多余的斜杠。
func joinURL(base, path string) string {
	if path == "" {
		return base
	}
	if base == "" {
		return path
	}
	if strings.HasSuffix(base, "/") && strings.HasPrefix(path, "/") {
		return base + path[1:]
	}
	if !strings.HasSuffix(base, "/") && !strings.HasPrefix(path, "/") {
		return base + "/" + path
	}
	return base + path
}

// timeout 返回配置的超时，缺省 30s。
func (u *HttpUnit) timeout() time.Duration {
	if u.config.GetTimeout() == nil {
		return defaultHTTPTimeout
	}
	d := u.config.GetTimeout().AsDuration()
	if d <= 0 {
		return defaultHTTPTimeout
	}
	return d
}

// failMutation 构造 fail mutation。
func failMutation(reason string, triggerCompensation bool) *dagv1.EntityMutation {
	return &dagv1.EntityMutation{Intent: &dagv1.EntityMutation_Fail{Fail: &dagv1.FailIntent{
		Reason:              reason,
		TriggerCompensation: triggerCompensation,
	}}}
}

var _ dag.ComputeUnit = (*HttpUnit)(nil)
