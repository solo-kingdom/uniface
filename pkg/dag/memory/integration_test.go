package memory

import (
	"context"
	"testing"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"github.com/solo-kingdom/uniface/pkg/dag/runtime"
	"github.com/solo-kingdom/uniface/pkg/dag/testpb"
)

func TestCommitHopIdempotent(t *testing.T) {
	_, store, _, _, _ := setupGoldenPath(t)
	ctx := context.Background()
	req := &dagv1.StartInstanceRequest{
		Ref:            &dagv1.EntityRef{EntityId: "order-1"},
		TypeKey:        &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema},
		InitialPayload: orderAny(&testpb.Order{OrderId: "o-1"}),
		GraphVersion:   &dagv1.GraphVersion{GraphId: graphID, Version: graphVersion},
	}
	if _, err := store.CreateInstance(ctx, req, "validate"); err != nil {
		t.Fatal(err)
	}
	idem := runtime.HopIdempotencyKey("order-1", "validate", 0)
	commit := &dagv1.HopCommit{
		Ref:            req.Ref,
		NodeId:         "validate",
		InputSequence:  0,
		IdempotencyKey: idem,
		NextNodeId:     "wait_approval",
		NextStatus:     dagv1.InstanceStatus_INSTANCE_STATUS_RUNNING,
		JournalKind:    dagv1.JournalKind_JOURNAL_KIND_NODE_COMMITTED,
	}
	if err := store.CommitHop(ctx, commit); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitHop(ctx, commit); err != nil {
		t.Fatal(err)
	}
	journal, err := store.ListJournal(ctx, req.Ref)
	if err != nil {
		t.Fatal(err)
	}
	var committed int
	for _, e := range journal {
		if e.Kind == dagv1.JournalKind_JOURNAL_KIND_NODE_COMMITTED && e.NodeId == "validate" {
			committed++
		}
	}
	if committed != 1 {
		t.Fatalf("expected 1 committed journal, got %d", committed)
	}
}

func TestDeliverSignalDedup(t *testing.T) {
	_, _, eng, _, _ := setupGoldenPath(t)
	ctx := context.Background()
	ref := &dagv1.EntityRef{EntityId: "order-1"}
	startOrder(t, eng, ref)
	runUntilWaiting(t, eng, ref)

	delivery := &dagv1.SignalDelivery{EntityId: ref.EntityId, SignalName: "manual_approval", DeliveryId: "D1"}
	if err := eng.DeliverSignal(ctx, delivery); err != nil {
		t.Fatal(err)
	}
	seqAfterFirst, _ := eng.GetInstance(ctx, ref)
	if err := eng.DeliverSignal(ctx, delivery); err != nil {
		t.Fatal(err)
	}
	seqAfterSecond, _ := eng.GetInstance(ctx, ref)
	if seqAfterFirst.Sequence != seqAfterSecond.Sequence {
		t.Fatalf("duplicate delivery changed sequence")
	}
}

func TestGoldenPath(t *testing.T) {
	_, _, eng, charge, _ := setupGoldenPath(t)
	ctx := context.Background()
	ref := &dagv1.EntityRef{EntityId: "order-1"}
	startOrder(t, eng, ref)

	for i := 0; i < 50; i++ {
		if err := eng.RunOnce(ctx); err != nil && err != context.Canceled {
			t.Fatalf("run %d: %v", i, err)
		}
		inst, _ := eng.GetInstance(ctx, ref)
		if inst.Status == dagv1.InstanceStatus_INSTANCE_STATUS_WAITING {
			break
		}
	}
	if err := eng.DeliverSignal(ctx, &dagv1.SignalDelivery{
		EntityId: ref.EntityId, SignalName: "manual_approval", DeliveryId: "D1",
	}); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 50; i++ {
		if err := eng.RunOnce(ctx); err != nil && err != context.Canceled {
			t.Fatalf("post-signal run %d: %v", i, err)
		}
		inst, _ := eng.GetInstance(ctx, ref)
		if inst.CurrentNodeId == "spawn_payments" || inst.CurrentNodeId == "join" || inst.Status == dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED {
			break
		}
	}
	if charge.execCount.Load() < 2 {
		t.Fatalf("expected charge retry after crash, got %d executes", charge.execCount.Load())
	}

	for i := 0; i < 100; i++ {
		if err := eng.RunOnce(ctx); err != nil {
			t.Fatalf("final run %d: %v", i, err)
		}
		inst, _ := eng.GetInstance(ctx, ref)
		if inst.Status == dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED {
			return
		}
	}
	t.Fatal("instance did not complete")
}

func TestSagaCompensation(t *testing.T) {
	_, _, eng, charge, refund := setupWithSpawn(t, &spawnUnit{failWithCompensation: true})
	ctx := context.Background()
	ref := &dagv1.EntityRef{EntityId: "order-1"}
	startOrder(t, eng, ref)
	charge.crashOnce.Store(true)

	for i := 0; i < 30; i++ {
		_ = eng.RunOnce(ctx)
	}
	if err := eng.DeliverSignal(ctx, &dagv1.SignalDelivery{
		EntityId: ref.EntityId, SignalName: "manual_approval", DeliveryId: "D1",
	}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 80; i++ {
		if err := eng.RunOnce(ctx); err != nil && err != context.Canceled {
			t.Fatal(err)
		}
		inst, _ := eng.GetInstance(ctx, ref)
		if inst.Status == dagv1.InstanceStatus_INSTANCE_STATUS_COMPENSATED {
			if refund.count.Load() < 1 {
				t.Fatal("expected refund compensator invoked")
			}
			return
		}
	}
	t.Fatal("expected COMPENSATED status")
}

func startOrder(t *testing.T, eng *Engine, ref *dagv1.EntityRef) {
	t.Helper()
	_, err := eng.StartInstance(context.Background(), &dagv1.StartInstanceRequest{
		Ref:            ref,
		TypeKey:        &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema},
		InitialPayload: orderAny(&testpb.Order{OrderId: "o-1", Amount: 100}),
		GraphVersion:   &dagv1.GraphVersion{GraphId: graphID, Version: graphVersion},
		GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_START,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func runUntilWaiting(t *testing.T, eng *Engine, ref *dagv1.EntityRef) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < 20; i++ {
		if err := eng.RunOnce(ctx); err != nil {
			t.Fatal(err)
		}
		inst, err := eng.GetInstance(ctx, ref)
		if err != nil {
			t.Fatal(err)
		}
		if inst.Status == dagv1.InstanceStatus_INSTANCE_STATUS_WAITING {
			return
		}
	}
	t.Fatal("never reached WAITING")
}

func TestSignalMismatch(t *testing.T) {
	_, _, eng, _, _ := setupGoldenPath(t)
	ctx := context.Background()
	ref := &dagv1.EntityRef{EntityId: "order-1"}
	startOrder(t, eng, ref)
	runUntilWaiting(t, eng, ref)
	err := eng.DeliverSignal(ctx, &dagv1.SignalDelivery{
		EntityId: ref.EntityId, SignalName: "wrong_signal", DeliveryId: "D1",
	})
	if err != dag.ErrSignalMismatch {
		t.Fatalf("expected ErrSignalMismatch, got %v", err)
	}
}
