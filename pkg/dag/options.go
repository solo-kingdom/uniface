package dag

import (
	"context"
	"net/http"
	"time"
)

// Options 引擎操作级配置。
type Options struct {
	DefaultRetryPolicy RetryOptions
	SchedulerInterval  time.Duration
	// HttpClientResolver 用于声明式 HttpUnit 解析服务实例。nil 时 HttpUnit 仅支持 url 直连。
	HttpClientResolver HttpClientResolver
	// DrainMaxHops >0 时覆盖 DrainInstance 的 hop 上限推导值。
	DrainMaxHops int
	// DrainFactor 推导 hop 上限时节点数系数，默认 4。
	DrainFactor int
	// DrainAbsoluteMax DrainInstance hop 绝对硬顶，默认 1000。
	DrainAbsoluteMax int
	// DrainMinHops 推导 hop 下限，默认 1。
	DrainMinHops int
}

// RetryOptions 重试配置。
type RetryOptions struct {
	MaxAttempts int
}

// Option 修改 Options 的函数。
type Option func(*Options)

// Apply 应用选项。
func (o *Options) Apply(opts ...Option) *Options {
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// DefaultOptions 返回默认配置。
func DefaultOptions() *Options {
	return &Options{
		DefaultRetryPolicy: RetryOptions{MaxAttempts: 3},
		SchedulerInterval:  100 * time.Millisecond,
	}
}

// MergeOptions 合并多个选项。
func MergeOptions(opts ...Option) *Options {
	return DefaultOptions().Apply(opts...)
}

// WithMaxAttempts 设置默认最大重试次数。
func WithMaxAttempts(n int) Option {
	return func(o *Options) {
		o.DefaultRetryPolicy.MaxAttempts = n
	}
}

// WithSchedulerInterval 设置调度轮询间隔。
func WithSchedulerInterval(d time.Duration) Option {
	return func(o *Options) {
		o.SchedulerInterval = d
	}
}

// WithHttpClientResolver 注入声明式 HttpUnit 使用的服务实例解析器。
// 未注入时 HttpUnit 仅支持 url 直连；service 非空会在 Execute 时返回错误。
func WithHttpClientResolver(r HttpClientResolver) Option {
	return func(o *Options) {
		o.HttpClientResolver = r
	}
}

// WithDrainMaxHops 覆盖 DrainInstance 的 hop 上限（测试或特殊场景用）。
func WithDrainMaxHops(n int) Option {
	return func(o *Options) {
		o.DrainMaxHops = n
	}
}

// WithDrainFactor 设置 DrainInstance hop 上限推导系数（节点数 × 系数）。
func WithDrainFactor(n int) Option {
	return func(o *Options) {
		o.DrainFactor = n
	}
}

// WithDrainAbsoluteMax 设置 DrainInstance hop 绝对硬顶。
func WithDrainAbsoluteMax(n int) Option {
	return func(o *Options) {
		o.DrainAbsoluteMax = n
	}
}

// HttpClientResolver 将服务名解析为 HTTP 客户端与 base URL。
//
// 定义在 dag 根包（而非 units 子包）以避免 options 与 units 之间的循环引用。
// 实现方通常包装 uniface.Balancer[http.Client]，详见 pkg/dag/units/balanceradapter。
type HttpClientResolver interface {
	ResolveClient(ctx context.Context, service string) (*http.Client, string, error)
}
