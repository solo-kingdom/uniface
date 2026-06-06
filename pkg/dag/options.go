package dag

import "time"

// Options 引擎操作级配置。
type Options struct {
	DefaultRetryPolicy RetryOptions
	SchedulerInterval  time.Duration
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
