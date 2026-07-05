// StringApp —— 面向"输入 string payload、执行 graph、返回终态值"同步调用场景的
// Runtime 门面，预注册 StringValue 实体类型并封装 EntityTypeKey，调用方不再持有
// typeKey 字段，也无需写"注册失败时关 runtime"的样板。
//
// StringApp 组合（嵌入 *Runtime）而非替代：调用方可经 sa.Runtime 访问 *Runtime
// 全部公共方法（LoadGraphFromDir / RegisterGraph / Memory 等）。
package app

import (
	"context"
	"errors"
	"fmt"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
)

// StringEntityType 是 StringApp 默认注册的实体类型名。
const StringEntityType = "app.String"

// StringSchemaVersion 是 StringApp 默认注册的 schema version。
const StringSchemaVersion = "v1"

// StringApp 预注册 StringValue 实体类型并封装 EntityTypeKey 的 Runtime 视图。
type StringApp struct {
	*Runtime
	typeKey *dagv1.EntityTypeKey
}

// NewStringApp 构造 StringApp 并预注册 StringValue 实体类型。
//
// 默认 entityType=StringEntityType、schemaVersion=StringSchemaVersion，可被
// WithStringEntityType / WithStringSchemaVersion 覆盖。
// 预注册失败时自动关闭底层 Runtime 并返回 error。
func NewStringApp(opts ...Option) (*StringApp, error) {
	rt := NewWithMemory()
	for _, opt := range opts {
		opt(rt)
	}
	entityType := rt.stringEntityType
	if entityType == "" {
		entityType = StringEntityType
	}
	schemaVersion := rt.stringSchemaVersion
	if schemaVersion == "" {
		schemaVersion = StringSchemaVersion
	}
	typeKey, err := rt.RegisterStringEntityType(entityType, schemaVersion)
	if err != nil {
		_ = rt.Close()
		return nil, fmt.Errorf("app: StringApp register entity type: %w", err)
	}
	return &StringApp{Runtime: rt, typeKey: typeKey}, nil
}

// TypeKey 返回内部绑定的 EntityTypeKey（供需要显式传 typeKey 的调用方使用）。
func (s *StringApp) TypeKey() *dagv1.EntityTypeKey { return s.typeKey }

// RegisterUnit 注册函数式 string compute unit。
//
// 失败时（unitID 重复 / 底层错误）自动 Close 底层 Runtime 并返回 error。
func (s *StringApp) RegisterUnit(unitID string, fn StringFunc) error {
	if err := s.Runtime.RegisterStringUnit(unitID, s.typeKey, fn); err != nil {
		_ = s.Runtime.Close()
		return err
	}
	return nil
}

// InvokeString 是 StringApp 的类型化调用入口 —— 隐藏 TypeKey，调用方只需提供
// graphID / entityID / payload 三个语义字段。
func (s *StringApp) InvokeString(ctx context.Context, graphID, entityID, payload string) (*StringCallResult, error) {
	if s == nil {
		return nil, errors.New("app: nil StringApp")
	}
	return s.Runtime.InvokeString(ctx, &StringCall{
		GraphID:  graphID,
		EntityID: entityID,
		Payload:  payload,
		TypeKey:  s.typeKey,
	})
}

// LoadGraphID 透传底层 Runtime.LoadGraphID。
func (s *StringApp) LoadGraphID(graphID string) (*dagv1.GraphSpec, error) {
	return s.Runtime.LoadGraphID(graphID)
}

// LoadedGraphs 透传底层 Runtime.LoadedGraphs。
func (s *StringApp) LoadedGraphs() map[string]string {
	return s.Runtime.LoadedGraphs()
}

// Close 透传底层 Runtime.Close。
func (s *StringApp) Close() error {
	return s.Runtime.Close()
}

// WithStringEntityType 覆盖 StringApp 默认的实体类型名（不影响 *Runtime 既有行为）。
func WithStringEntityType(entityType string) Option {
	return func(r *Runtime) {
		r.stringEntityType = entityType
	}
}

// WithStringSchemaVersion 覆盖 StringApp 默认的 schema version（不影响 *Runtime 既有行为）。
func WithStringSchemaVersion(schemaVersion string) Option {
	return func(r *Runtime) {
		r.stringSchemaVersion = schemaVersion
	}
}
