package invocation_test

import (
	"testing"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag/entity"
	"github.com/solo-kingdom/uniface/pkg/dag/invocation"
	"github.com/solo-kingdom/uniface/pkg/dag/testpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// TestMarshalAny_EncodeDecodeRoundTrip 验证 protobuf message 与 Any 双向转换。
func TestMarshalAny_EncodeDecodeRoundTrip(t *testing.T) {
	order := &testpb.Order{OrderId: "o-1", Amount: 99.5, Status: "ok", Approved: true}
	any, err := invocation.MarshalAny(order)
	if err != nil {
		t.Fatalf("MarshalAny: %v", err)
	}
	if any.TypeUrl != "type.googleapis.com/dag.testpb.Order" {
		t.Fatalf("TypeUrl = %q", any.TypeUrl)
	}
	var dst testpb.Order
	if err := invocation.UnmarshalAny(any, &dst); err != nil {
		t.Fatalf("UnmarshalAny: %v", err)
	}
	if !proto.Equal(&dst, order) {
		t.Fatalf("round trip mismatch: got %+v", &dst)
	}
}

// TestMarshalAny_NilMessage 验证 nil message 返回错误。
func TestMarshalAny_NilMessage(t *testing.T) {
	if _, err := invocation.MarshalAny(nil); err == nil {
		t.Fatal("MarshalAny(nil) = nil error, want error")
	}
}

// TestUnmarshalSnapshot_OK 验证 snapshot payload 解码为目标 message。
func TestUnmarshalSnapshot_OK(t *testing.T) {
	order := &testpb.Order{OrderId: "s-1", Amount: 1, Status: "validated"}
	any, _ := invocation.MarshalAny(order)
	snap := entity.NewSnapshot(
		&dagv1.EntityRef{EntityId: "x"},
		&dagv1.EntityTypeKey{EntityType: "t", PayloadSchemaVersion: "v1"},
		3, any,
	)
	var dst testpb.Order
	if err := invocation.UnmarshalSnapshot(snap, &dst); err != nil {
		t.Fatalf("UnmarshalSnapshot: %v", err)
	}
	if dst.OrderId != "s-1" {
		t.Fatalf("OrderId = %q", dst.OrderId)
	}
}

// TestUnmarshalSnapshot_NilSnapshot 验证 nil snapshot 返回错误。
func TestUnmarshalSnapshot_NilSnapshot(t *testing.T) {
	var dst testpb.Order
	if err := invocation.UnmarshalSnapshot(nil, &dst); err == nil {
		t.Fatal("UnmarshalSnapshot(nil) = nil error, want error")
	}
}

// TestUnmarshalSnapshot_NilPayload 验证 nil payload 返回错误。
func TestUnmarshalSnapshot_NilPayload(t *testing.T) {
	snap := entity.NewSnapshot(
		&dagv1.EntityRef{EntityId: "x"},
		&dagv1.EntityTypeKey{EntityType: "t", PayloadSchemaVersion: "v1"},
		0, nil,
	)
	var dst testpb.Order
	if err := invocation.UnmarshalSnapshot(snap, &dst); err == nil {
		t.Fatal("UnmarshalSnapshot(nil payload) = nil error, want error")
	}
}

// TestUnmarshalSnapshot_TypeMismatch 验证类型不匹配返回错误。
func TestUnmarshalSnapshot_TypeMismatch(t *testing.T) {
	// payload 是 StringValue，目标却是 Order。
	any, _ := invocation.MarshalAny(wrapperspb.String("hello"))
	snap := entity.NewSnapshot(
		&dagv1.EntityRef{EntityId: "x"},
		&dagv1.EntityTypeKey{EntityType: "t", PayloadSchemaVersion: "v1"},
		0, any,
	)
	var dst testpb.Order
	err := invocation.UnmarshalSnapshot(snap, &dst)
	if err == nil {
		t.Fatal("expected type mismatch error, got nil")
	}
}

// TestMarshalString_RoundTrip 验证 StringValue 编解码 helper。
func TestMarshalString_RoundTrip(t *testing.T) {
	any, err := invocation.MarshalString("lab echo")
	if err != nil {
		t.Fatalf("MarshalString: %v", err)
	}
	snap := entity.NewSnapshot(
		&dagv1.EntityRef{EntityId: "x"},
		&dagv1.EntityTypeKey{EntityType: "t", PayloadSchemaVersion: "v1"},
		1, any,
	)
	got, err := invocation.UnmarshalString(snap)
	if err != nil {
		t.Fatalf("UnmarshalString: %v", err)
	}
	if got != "lab echo" {
		t.Fatalf("got %q", got)
	}
}

// TestUnmarshalAny_NilAny 验证 UnmarshalAny(nil, ...) 返回错误。
func TestUnmarshalAny_NilAny(t *testing.T) {
	var dst testpb.Order
	if err := invocation.UnmarshalAny(nil, &dst); err == nil {
		t.Fatal("UnmarshalAny(nil) = nil error, want error")
	}
}

// TestUnmarshalAny_NilDst 验证 UnmarshalAny(..., nil) 返回错误。
func TestUnmarshalAny_NilDst(t *testing.T) {
	any, _ := invocation.MarshalAny(&testpb.Order{OrderId: "x"})
	if err := invocation.UnmarshalAny(any, nil); err == nil {
		t.Fatal("UnmarshalAny(..., nil) = nil error, want error")
	}
}
