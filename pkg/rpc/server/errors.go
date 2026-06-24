package server

import (
	"errors"
	"fmt"
)

// 以下为常见简单场景的 sentinel errors，均支持 errors.Is。
var (
	// ErrRouteExists 在重复注册同一路由时返回。
	ErrRouteExists = errors.New("route already registered")

	// ErrServerClosed 在服务已关闭后再次操作时返回。
	ErrServerClosed = errors.New("server closed")

	// ErrNoTransport 在未配置 Transport 就 Start 时返回。
	ErrNoTransport = errors.New("no transport configured")

	// ErrInvalidHandler 在注册 nil handler 时返回。
	ErrInvalidHandler = errors.New("invalid handler")
)

// ServerError 表示带上下文的服务错误，包裹底层错误并提供操作与路由信息。
// 支持 errors.Is（通过 Unwrap）与 errors.As。
type ServerError struct {
	// Op 是失败的操作名（如 "Handle"、"Start"）。
	Op string
	// Key 是相关路由标识（如 "POST /echo"），可空。
	Key string
	// Err 是底层错误。
	Err error
}

// Error 返回格式化的错误信息。
func (e *ServerError) Error() string {
	if e.Key == "" {
		return fmt.Sprintf("server %s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("server %s %q: %v", e.Op, e.Key, e.Err)
}

// Unwrap 返回底层错误，供 errors.Is / errors.As 使用。
func (e *ServerError) Unwrap() error {
	return e.Err
}

// NewServerError 用操作名、路由键与底层错误构造 ServerError。err 为 nil 时返回 nil。
func NewServerError(op, key string, err error) error {
	if err == nil {
		return nil
	}
	return &ServerError{Op: op, Key: key, Err: err}
}
