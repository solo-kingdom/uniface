package entity

import (
	"testing"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag/testpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestMergeSignalPayloadSameType(t *testing.T) {
	typeKey := &dagv1.EntityTypeKey{EntityType: "order.Order", PayloadSchemaVersion: "v1"}
	base, _ := anypb.New(&testpb.Order{Status: "PENDING"})
	incoming, _ := anypb.New(&testpb.Order{Approved: true})
	snap := NewSnapshot(&dagv1.EntityRef{EntityId: "o-1"}, typeKey, 0, base)

	merged, err := MergeSignalPayload(snap, "approval", incoming)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := anypb.UnmarshalNew(merged.Payload, proto.UnmarshalOptions{DiscardUnknown: true})
	if err != nil {
		t.Fatal(err)
	}
	order, ok := msg.(*testpb.Order)
	if !ok {
		t.Fatalf("expected Order, got %T", msg)
	}
	if order.Status != "PENDING" || !order.Approved {
		t.Fatalf("unexpected merge result: %+v", order)
	}
}

func TestMergeSignalPayloadDifferentType(t *testing.T) {
	typeKey := &dagv1.EntityTypeKey{EntityType: "order.Order", PayloadSchemaVersion: "v1"}
	base, _ := anypb.New(&testpb.Order{Status: "PENDING"})
	incoming, _ := anypb.New(&testpb.Order{Approved: true})
	snap := NewSnapshot(&dagv1.EntityRef{EntityId: "o-1"}, typeKey, 0, base)
	snap.Payload.TypeUrl = "type.googleapis.com/other.Type"

	merged, err := MergeSignalPayload(snap, "approval", incoming)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := anypb.UnmarshalNew(merged.Payload, proto.UnmarshalOptions{DiscardUnknown: true})
	if err != nil {
		t.Fatal(err)
	}
	wrapper, ok := msg.(*dagv1.SignalPayload)
	if !ok {
		t.Fatalf("expected SignalPayload wrapper, got %T", msg)
	}
	if wrapper.SignalName != "approval" {
		t.Fatalf("unexpected signal name %q", wrapper.SignalName)
	}
}

func TestShouldMergeSignalPayloadDefault(t *testing.T) {
	if !ShouldMergeSignalPayload(nil) {
		t.Fatal("default should merge")
	}
	if !ShouldMergeSignalPayload(&dagv1.WaitNodeConfig{}) {
		t.Fatal("unset optional should merge")
	}
	v := false
	if ShouldMergeSignalPayload(&dagv1.WaitNodeConfig{MergeSignalPayload: &v}) {
		t.Fatal("explicit false should not merge")
	}
}
