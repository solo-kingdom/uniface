package entity

import (
	"fmt"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"google.golang.org/protobuf/types/known/anypb"
)

// ValidateTypeKey 校验 EntityTypeKey 非空。
func ValidateTypeKey(key *dagv1.EntityTypeKey) error {
	if key == nil || key.EntityType == "" || key.PayloadSchemaVersion == "" {
		return dag.ErrInvalidEntityType
	}
	return nil
}

// ValidateSnapshot 校验快照类型键。
func ValidateSnapshot(snapshot *dagv1.EntitySnapshot) error {
	if snapshot == nil {
		return dag.ErrInvalidEntityType
	}
	return ValidateTypeKey(snapshot.TypeKey)
}

// ValidateSpawnSpec 校验 SpawnSpec 含显式 graph。
func ValidateSpawnSpec(spec *dagv1.SpawnSpec) error {
	if spec == nil || spec.Ref == nil || spec.Ref.EntityId == "" {
		return dag.ErrInvalidSpawn
	}
	if err := ValidateTypeKey(spec.TypeKey); err != nil {
		return err
	}
	if spec.Graph == nil || spec.Graph.GraphId == "" || spec.Graph.Version == "" {
		return dag.ErrInvalidSpawn
	}
	return nil
}

// ValidatePayloadTypeURL 校验 payload type_url 与注册项一致。
func ValidatePayloadTypeURL(reg *dagv1.EntityTypeRegistration, payload *anypb.Any) error {
	if reg == nil {
		return dag.ErrInvalidEntityType
	}
	if payload == nil {
		return nil
	}
	if payload.TypeUrl != reg.PayloadTypeUrl {
		return fmt.Errorf("%w: expected %q got %q", dag.ErrInvalidEntityType, reg.PayloadTypeUrl, payload.TypeUrl)
	}
	return nil
}

// CloneSnapshot 复制快照。
func CloneSnapshot(s *dagv1.EntitySnapshot) *dagv1.EntitySnapshot {
	if s == nil {
		return nil
	}
	out := *s
	if s.Ref != nil {
		ref := *s.Ref
		out.Ref = &ref
	}
	if s.TypeKey != nil {
		key := *s.TypeKey
		out.TypeKey = &key
	}
	if s.Payload != nil {
		out.Payload = &anypb.Any{TypeUrl: s.Payload.TypeUrl, Value: append([]byte(nil), s.Payload.Value...)}
	}
	return &out
}

// NewSnapshot 创建新快照。
func NewSnapshot(ref *dagv1.EntityRef, key *dagv1.EntityTypeKey, seq int64, payload *anypb.Any) *dagv1.EntitySnapshot {
	return &dagv1.EntitySnapshot{
		Ref:      ref,
		TypeKey:  key,
		Sequence: seq,
		Payload:  payload,
	}
}

// TypeKeyEqual 比较两个 EntityTypeKey。
func TypeKeyEqual(a, b *dagv1.EntityTypeKey) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.EntityType == b.EntityType && a.PayloadSchemaVersion == b.PayloadSchemaVersion
}
