package app

import (
	"context"
	"errors"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"github.com/solo-kingdom/uniface/pkg/dag/entity"
	"github.com/solo-kingdom/uniface/pkg/dag/invocation"
)

// StringFunc 是以 string 为输入并产出 string 的函数式计算单元签名。
type StringFunc func(ctx context.Context, input string) (string, error)

// RegisterStringEntityType 注册 string 实体类型，payload type URL 为 StringValue。
func (r *Runtime) RegisterStringEntityType(entityType, schemaVersion string) (*dagv1.EntityTypeKey, error) {
	if err := r.rt.RegisterEntityTypeSimple(entityType, schemaVersion, StringPayloadTypeURL); err != nil {
		return nil, err
	}
	return &dagv1.EntityTypeKey{EntityType: entityType, PayloadSchemaVersion: schemaVersion}, nil
}

// RegisterStringUnit 注册函数式 string compute unit，自动创建匹配的 ComputeUnitDef。
func (r *Runtime) RegisterStringUnit(unitID string, typeKey *dagv1.EntityTypeKey, fn StringFunc) error {
	if typeKey == nil {
		return errors.New("app: typeKey must not be nil")
	}
	adapter := &stringUnitAdapter{fn: fn, typeKey: typeKey}
	def := &dagv1.ComputeUnitDef{
		UnitId:          unitID,
		InputTypeKey:    typeKey,
		OutputTypeKeys:  []*dagv1.EntityTypeKey{typeKey},
		SideEffectClass: dagv1.SideEffectClass_SIDE_EFFECT_NONE,
	}
	return r.rt.RegisterComputeUnitFull(def, adapter)
}

type stringUnitAdapter struct {
	fn      StringFunc
	typeKey *dagv1.EntityTypeKey
}

func (a *stringUnitAdapter) Execute(ctx context.Context, snapshot *dagv1.EntitySnapshot) (*dagv1.EntityMutation, error) {
	input, err := readStringPayload(snapshot)
	if err != nil {
		return nil, err
	}
	out, err := a.fn(ctx, input)
	if err != nil {
		return nil, err
	}
	payload, err := invocation.MarshalString(out)
	if err != nil {
		return nil, err
	}
	return &dagv1.EntityMutation{
		Intent: &dagv1.EntityMutation_Update{
			Update: entity.NewSnapshot(snapshot.Ref, snapshot.TypeKey, snapshot.Sequence+1, payload),
		},
	}, nil
}

var _ dag.ComputeUnit = (*stringUnitAdapter)(nil)

func readStringPayload(snapshot *dagv1.EntitySnapshot) (string, error) {
	if snapshot == nil {
		return "", nil
	}
	s, err := invocation.UnmarshalString(snapshot)
	if err != nil {
		if snapshot.Payload == nil {
			return "", nil
		}
		return "", err
	}
	return s, nil
}
