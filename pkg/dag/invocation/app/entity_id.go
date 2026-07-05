package app

import (
	"fmt"
	"sync/atomic"
)

// defaultEntityIDPrefix 是 NewEntityIDGen("") 时使用的默认 prefix。
const defaultEntityIDPrefix = "dag"

// EntityIDGen 基于 atomic.Uint64 的线程安全 entity ID 生成器。
//
// 每次 NewEntityIDGen 返回独立计数器（不共享），保证不同 prefix 命名空间互不串号；
// Next() 返回 "<prefix>-<n>" 格式字符串，n 从 1 开始全局单调递增（实例内）。
type EntityIDGen struct {
	counter atomic.Uint64
	prefix  string
}

// NewEntityIDGen 创建以 prefix 为命名前缀的生成器；prefix 为空时使用 "dag"。
func (r *Runtime) NewEntityIDGen(prefix string) *EntityIDGen {
	if prefix == "" {
		prefix = defaultEntityIDPrefix
	}
	return &EntityIDGen{prefix: prefix}
}

// Next 返回 "<prefix>-<n>" 格式字符串，n 单调递增（实例内全局）。
func (g *EntityIDGen) Next() string {
	if g == nil {
		return ""
	}
	n := g.counter.Add(1)
	return fmt.Sprintf("%s-%d", g.prefix, n)
}
