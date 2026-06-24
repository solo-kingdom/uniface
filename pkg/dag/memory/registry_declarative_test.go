package memory

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"github.com/solo-kingdom/uniface/pkg/dag/testpb"
	"google.golang.org/protobuf/types/known/anypb"
)

func httpUnitDef(unitID, url string) *dagv1.ComputeUnitDef {
	return &dagv1.ComputeUnitDef{
		UnitId:          unitID,
		InputTypeKey:    &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema},
		OutputTypeKeys:  []*dagv1.EntityTypeKey{{EntityType: orderType, PayloadSchemaVersion: orderSchema}},
		SideEffectClass: dagv1.SideEffectClass_SIDE_EFFECT_IDEMPOTENT,
		Implementation: &dagv1.ComputeUnitDef_Http{Http: &dagv1.HttpUnit{Url: url}},
	}
}

func TestRegistry_DeclarativeUnitPriorityOverGoImpl(t *testing.T) {
	reg := NewRegistry()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"orderId":"d1","amount":1,"status":"ok","approved":true}`))
	}))
	defer srv.Close()

	def := httpUnitDef("order.declarative", srv.URL)
	if err := reg.RegisterComputeUnit(def); err != nil {
		t.Fatal(err)
	}
	// Go 注册应被互斥校验拒绝。
	err := reg.RegisterComputeUnitImpl("order.declarative", &validateUnit{})
	if err == nil {
		t.Fatal("expected error registering Go impl alongside declarative")
	}
	impl, err := reg.GetComputeUnitImpl("order.declarative")
	if err != nil {
		t.Fatalf("GetComputeUnitImpl: %v", err)
	}
	snap := &dagv1.EntitySnapshot{
		Ref:      &dagv1.EntityRef{EntityId: "e1"},
		TypeKey:  &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema},
		Sequence: 1,
	}
	a, _ := anypb.New(&testpb.Order{OrderId: "in"})
	snap.Payload = a
	mut, err := impl.Execute(context.Background(), snap)
	if err != nil {
		t.Fatalf("Execute declarative: %v", err)
	}
	if _, ok := mut.GetIntent().(*dagv1.EntityMutation_Update); !ok {
		t.Fatalf("expected HttpUnit update, got %T", mut.GetIntent())
	}
}

func TestRegistry_FallbackToGoImplWhenNoImplementation(t *testing.T) {
	reg := NewRegistry()
	def := &dagv1.ComputeUnitDef{
		UnitId:          "order.go",
		InputTypeKey:    &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema},
		SideEffectClass: dagv1.SideEffectClass_SIDE_EFFECT_NONE,
	}
	if err := reg.RegisterComputeUnit(def); err != nil {
		t.Fatal(err)
	}
	goImpl := &validateUnit{}
	if err := reg.RegisterComputeUnitImpl("order.go", goImpl); err != nil {
		t.Fatal(err)
	}
	impl, err := reg.GetComputeUnitImpl("order.go")
	if err != nil {
		t.Fatalf("GetComputeUnitImpl: %v", err)
	}
	if got, ok := impl.(*validateUnit); !ok || got != goImpl {
		t.Fatalf("expected Go impl fallback to same *validateUnit, got %T", impl)
	}
}

func TestRegistry_DeclarativeRejectsRegistrationAfterGoImpl(t *testing.T) {
	reg := NewRegistry()
	if err := reg.RegisterComputeUnitImpl("order.x", &validateUnit{}); err != nil {
		t.Fatal(err)
	}
	def := httpUnitDef("order.x", "http://example")
	err := reg.RegisterComputeUnit(def)
	if err == nil {
		t.Fatal("expected error registering declarative def alongside Go impl")
	}
	if !errors.Is(err, dag.ErrInvalidGraph) {
		t.Fatalf("expected ErrInvalidGraph, got %v", err)
	}
}

func TestRegistry_DeclarativeUnitCachesInstance(t *testing.T) {
	reg := NewRegistry()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"orderId":"d1","amount":1,"status":"ok","approved":true}`))
	}))
	defer srv.Close()
	def := httpUnitDef("order.cached", srv.URL)
	if err := reg.RegisterComputeUnit(def); err != nil {
		t.Fatal(err)
	}
	a, _ := reg.GetComputeUnitImpl("order.cached")
	b, _ := reg.GetComputeUnitImpl("order.cached")
	if a != b {
		t.Fatal("expected cached declarative unit instance")
	}
}
