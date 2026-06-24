package entity

import (
	"testing"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
)

func TestValidateOutputType(t *testing.T) {
	typeKey := &dagv1.EntityTypeKey{EntityType: "order.Order", PayloadSchemaVersion: "v1"}
	otherKey := &dagv1.EntityTypeKey{EntityType: "order.Order", PayloadSchemaVersion: "v2"}

	unit := &dagv1.ComputeUnitDef{
		UnitId:         "order.validate",
		InputTypeKey:   typeKey,
		OutputTypeKeys: []*dagv1.EntityTypeKey{typeKey},
	}
	snap := NewSnapshot(&dagv1.EntityRef{EntityId: "o-1"}, typeKey, 1, nil)

	if err := ValidateOutputType(unit, snap); err != nil {
		t.Fatalf("expected match: %v", err)
	}
	badSnap := NewSnapshot(&dagv1.EntityRef{EntityId: "o-1"}, otherKey, 1, nil)
	if err := ValidateOutputType(unit, badSnap); err != dag.ErrTypeMismatch {
		t.Fatalf("expected ErrTypeMismatch, got %v", err)
	}

	unit.OutputTypeKeys = nil
	if err := ValidateOutputType(unit, badSnap); err != nil {
		t.Fatalf("empty output_type_keys should allow any: %v", err)
	}
}

func TestValidateSchemaCompatible(t *testing.T) {
	v1 := &dagv1.EntityTypeKey{EntityType: "order.Order", PayloadSchemaVersion: "v1"}
	v2 := &dagv1.EntityTypeKey{EntityType: "order.Order", PayloadSchemaVersion: "v2"}
	other := &dagv1.EntityTypeKey{EntityType: "payment.Payment", PayloadSchemaVersion: "v1"}

	reg := &dagv1.EntityTypeRegistration{
		TypeKey:          v1,
		CompatibleInputs: []*dagv1.EntityTypeKey{v2},
	}
	snap := NewSnapshot(&dagv1.EntityRef{EntityId: "o-1"}, v1, 1, nil)
	if err := ValidateSchemaCompatible(v1, reg, snap); err != nil {
		t.Fatalf("same type should pass: %v", err)
	}

	snapV2 := NewSnapshot(&dagv1.EntityRef{EntityId: "o-1"}, v2, 1, nil)
	if err := ValidateSchemaCompatible(v1, reg, snapV2); err != nil {
		t.Fatalf("compatible input should pass: %v", err)
	}

	snapOther := NewSnapshot(&dagv1.EntityRef{EntityId: "o-1"}, other, 1, nil)
	if err := ValidateSchemaCompatible(v1, reg, snapOther); err != dag.ErrIncompatibleSchema {
		t.Fatalf("expected ErrIncompatibleSchema, got %v", err)
	}
}
