// Package balanceradapter 将 uniface.Balancer[http.Client] 适配为 dag 声明式 unit 所需的 HttpClientResolver。
//
// 本包是 dag 域与 loadbalancer 域之间的胶水层，由调用方（lab、业务进程）按需引入。
// pkg/dag/units 核心仅依赖 HttpClientResolver 接口，不直接 import loadbalancer，
// 保持 dag 在无 Balancer 场景下仍可用（注入 nil resolver → HttpUnit 仅支持 url 直连）。
package balanceradapter

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/solo-kingdom/uniface/pkg/dag/units"
	"github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer"
)

// ClientFactory 按 Instance 创建 *http.Client。默认使用全局共享的 defaultHTTPClient。
type ClientFactory func(*loadbalancer.Instance) (*http.Client, error)

// Adapter 将 Balancer[*http.Client] 包装为 HttpClientResolver。
//
// ResolveClient 流程：
//  1. balancer.Select 选一个实例（单次调用，避免重复选择不同实例）
//  2. 由 Instance.Address:Port 拼出 baseURL
//  3. 按 Instance.ID 复用缓存的 *http.Client（缺失则调用 ClientFactory 创建）
type Adapter struct {
	balancer loadbalancer.Balancer[*http.Client]
	factory  ClientFactory

	mu      sync.RWMutex
	clients map[string]*http.Client
}

// Option 修改 Adapter 配置。
type Option func(*Adapter)

// WithClientFactory 设置自定义客户端工厂（默认使用共享 *http.Client）。
func WithClientFactory(f ClientFactory) Option {
	return func(a *Adapter) {
		a.factory = f
	}
}

// New 构造适配器。balancer 用于按 service 选实例。
func New(balancer loadbalancer.Balancer[*http.Client], opts ...Option) *Adapter {
	a := &Adapter{
		balancer: balancer,
		factory:  defaultFactory,
		clients:  make(map[string]*http.Client),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// ResolveClient 实现 units.HttpClientResolver。
// service 参数透传给 balancer（当前 loadbalancer 单 balancer 单 service 语义；
// 多服务场景由调用方按 service 维护多个 balancer，或后续扩展 balancer 按 service 路由）。
func (a *Adapter) ResolveClient(ctx context.Context, service string) (*http.Client, string, error) {
	inst, err := a.balancer.Select(ctx)
	if err != nil {
		return nil, "", err
	}
	if inst == nil {
		return nil, "", fmt.Errorf("balancer returned nil instance for service %q", service)
	}
	baseURL := fmt.Sprintf("http://%s:%d", inst.Address, inst.Port)
	client, err := a.clientFor(inst)
	if err != nil {
		return nil, "", err
	}
	return client, baseURL, nil
}

func (a *Adapter) clientFor(inst *loadbalancer.Instance) (*http.Client, error) {
	a.mu.RLock()
	if c, ok := a.clients[inst.ID]; ok {
		a.mu.RUnlock()
		return c, nil
	}
	a.mu.RUnlock()

	a.mu.Lock()
	defer a.mu.Unlock()
	if c, ok := a.clients[inst.ID]; ok {
		return c, nil
	}
	c, err := a.factory(inst)
	if err != nil {
		return nil, err
	}
	a.clients[inst.ID] = c
	return c, nil
}

var defaultHTTPClient = &http.Client{}

func defaultFactory(_ *loadbalancer.Instance) (*http.Client, error) {
	return defaultHTTPClient, nil
}

// 编译期断言：Adapter 实现 units.HttpClientResolver。
var _ units.HttpClientResolver = (*Adapter)(nil)
