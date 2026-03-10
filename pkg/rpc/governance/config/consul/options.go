// Package consul 提供基于 Consul 的配置存储实现。
// 实现了 config.Storage 接口，支持配置的读取、写入、监听等功能。
//
// 基于 specs/features/rpc/governance/config/01 consul.md 实现
package consul

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/wii/uniface/pkg/rpc/governance/config"
)

// Options 定义了 Consul 配置存储的特定选项。
type Options struct {
	// Consul 客户端配置
	Address    string // Consul 服务器地址，默认为 "127.0.0.1:8500"
	Scheme     string // 协议类型，"http" 或 "https"，默认为 "http"
	Token      string // ACL Token
	Namespace  string // Consul Enterprise 命名空间
	Datacenter string // 数据中心
	TokenFile  string // Token 文件路径

	// TLS 配置
	TLSConfig *tls.Config // TLS 配置

	// HTTP 配置
	HttpClient *http.Client   // 自定义 HTTP 客户端
	HttpAuth   *HttpBasicAuth // HTTP 基础认证
	WaitTime   time.Duration  // 等待时间（用于阻塞查询）

	// 存储配置
	KeyPrefix string // 配置键前缀，默认为 "config/"
}

// HttpBasicAuth 定义 HTTP 基础认证信息。
type HttpBasicAuth struct {
	Username string
	Password string
}

// Option 是修改 Options 的函数类型。
type Option func(*Options)

// Apply 应用给定的选项到当前 Options。
func (o *Options) Apply(opts ...Option) *Options {
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// DefaultOptions 返回 Consul 配置存储的默认选项。
func DefaultOptions() *Options {
	return &Options{
		Address:   "127.0.0.1:8500",
		Scheme:    "http",
		KeyPrefix: "config/",
		WaitTime:  30 * time.Second,
	}
}

// WithAddress 设置 Consul 服务器地址。
func WithAddress(addr string) Option {
	return func(o *Options) {
		o.Address = addr
	}
}

// WithScheme 设置协议类型（http 或 https）。
func WithScheme(scheme string) Option {
	return func(o *Options) {
		o.Scheme = scheme
	}
}

// WithToken 设置 ACL Token。
func WithToken(token string) Option {
	return func(o *Options) {
		o.Token = token
	}
}

// WithNamespace 设置 Consul Enterprise 命名空间。
func WithNamespace(ns string) Option {
	return func(o *Options) {
		o.Namespace = ns
	}
}

// WithDatacenter 设置数据中心。
func WithDatacenter(dc string) Option {
	return func(o *Options) {
		o.Datacenter = dc
	}
}

// WithTokenFile 设置 Token 文件路径。
func WithTokenFile(path string) Option {
	return func(o *Options) {
		o.TokenFile = path
	}
}

// WithTLSConfig 设置 TLS 配置。
func WithTLSConfig(tlsConfig *tls.Config) Option {
	return func(o *Options) {
		o.TLSConfig = tlsConfig
	}
}

// WithHttpClient 设置自定义 HTTP 客户端。
func WithHttpClient(client *http.Client) Option {
	return func(o *Options) {
		o.HttpClient = client
	}
}

// WithHttpAuth 设置 HTTP 基础认证。
func WithHttpAuth(username, password string) Option {
	return func(o *Options) {
		o.HttpAuth = &HttpBasicAuth{
			Username: username,
			Password: password,
		}
	}
}

// WithWaitTime 设置阻塞查询的等待时间。
func WithWaitTime(d time.Duration) Option {
	return func(o *Options) {
		o.WaitTime = d
	}
}

// WithKeyPrefix 设置配置键前缀。
func WithKeyPrefix(prefix string) Option {
	return func(o *Options) {
		o.KeyPrefix = prefix
	}
}

// ToConfigOptions 将 Consul Options 转换为通用的 config.Options。
func (o *Options) ToConfigOptions() []config.Option {
	var opts []config.Option

	if o.KeyPrefix != "" {
		opts = append(opts, config.WithNamespace(o.KeyPrefix))
	}

	return opts
}
