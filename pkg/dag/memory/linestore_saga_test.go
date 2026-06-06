package memory

import (
	"context"
	"testing"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag/runtime"
)

func TestCommitHopCompensationPopAndReconcile(t *testing.T) {
	store := NewLineStore()
	ctx := context.Background()
	ref := &dagv1.EntityRef{EntityId: "order-1"}
	req := &dagv1.StartInstanceRequest{
		Ref:            ref,
		TypeKey:        &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema},
		GraphVersion:   &dagv1.GraphVersion{GraphId: graphID, Version: graphVersion},
		GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_START,
	}
	if _, err := store.CreateInstance(ctx, req, "charge"); err != nil {
		t.Fatal(err)
	}
	frame := &dagv1.CompensationFrame{
		NodeId:            "charge",
		UnitId:            "order.charge",
		CompensatorUnitId: "order.refund",
		ForwardSequence:   1,
	}
	if err := store.PushSagaFrame(ref, frame); err != nil {
		t.Fatal(err)
	}
	idem := runtime.CompensationIdempotencyKey(ref.EntityId, frame.ForwardSequence, frame.CompensatorUnitId)
	commit := &dagv1.HopCommit{
		Ref:            ref,
		NodeId:         frame.NodeId,
		InputSequence:  frame.ForwardSequence,
		IdempotencyKey: idem,
		NextNodeId:     "charge",
		NextStatus:     dagv1.InstanceStatus_INSTANCE_STATUS_COMPENSATING,
		JournalKind:    dagv1.JournalKind_JOURNAL_KIND_COMPENSATION_COMMITTED,
	}
	if err := store.CommitHop(ctx, commit); err != nil {
		t.Fatal(err)
	}
	saga, err := store.GetSagaState(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(saga.Stack) != 0 {
		t.Fatalf("expected empty stack after commit, got %d frames", len(saga.Stack))
	}

	if err := store.PushSagaFrame(ref, frame); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitHop(ctx, commit); err != nil {
		t.Fatal(err)
	}
	saga, err = store.GetSagaState(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(saga.Stack) != 0 {
		t.Fatalf("idempotent reconcile should pop stack, got %d frames", len(saga.Stack))
	}
}
