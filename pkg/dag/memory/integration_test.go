package memory

import (
	"context"
	"sync"
	"testing"
	"time"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"github.com/solo-kingdom/uniface/pkg/dag/graph"
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

func TestCompensationRetryBeforeCommit(t *testing.T) {
	reg, store, _, _, refund := setupWithSpawn(t, &spawnUnit{failWithCompensation: true})
	sched := NewScheduler(reg, store, graph.NewResolver(), dag.DefaultOptions())
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
	if err := store.UpdateInstanceStatus(ref, dagv1.InstanceStatus_INSTANCE_STATUS_COMPENSATING); err != nil {
		t.Fatal(err)
	}
	frame := &dagv1.CompensationFrame{
		NodeId:            "charge",
		UnitId:            "order.charge",
		CompensatorUnitId: "order.refund",
		ForwardSequence:   1,
		ForwardSnapshot:   orderAny(&testpb.Order{OrderId: "o-1", Status: "charged"}),
	}
	if err := store.PushSagaFrame(ref, frame); err != nil {
		t.Fatal(err)
	}
	comp, err := reg.GetCompensator("order.refund")
	if err != nil {
		t.Fatal(err)
	}
	compCtx := &dagv1.CompensationContext{
		EntityId:        ref.EntityId,
		NodeId:          frame.NodeId,
		ForwardSequence: frame.ForwardSequence,
		Snapshot:        frame.ForwardSnapshot,
	}
	if err := comp.Compensate(ctx, compCtx); err != nil {
		t.Fatal(err)
	}
	saga, err := store.GetSagaState(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(saga.Stack) != 1 {
		t.Fatalf("stack frame should remain before CommitHop, got %d", len(saga.Stack))
	}
	inst, err := store.GetInstance(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if err := sched.processCompensation(ctx, inst); err != nil {
		t.Fatal(err)
	}
	if refund.count.Load() < 2 {
		t.Fatalf("expected Compensate called twice on retry, got %d", refund.count.Load())
	}
	saga, err = store.GetSagaState(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(saga.Stack) != 0 {
		t.Fatalf("expected empty stack after commit, got %d", len(saga.Stack))
	}
}

func TestAcceptedSignalsAlias(t *testing.T) {
	_, store, eng, _, _ := setupGoldenPath(t)
	ctx := context.Background()
	ref := &dagv1.EntityRef{EntityId: "order-1"}
	startOrder(t, eng, ref)
	runUntilWaiting(t, eng, ref)

	waiting, err := store.GetWaiting(ref)
	if err != nil {
		t.Fatal(err)
	}
	waiting.SignalName = "approval"
	waiting.AcceptedSignals = []string{"manual_approval"}
	if err := store.SetWaiting(ref, "wait_approval", waiting); err != nil {
		t.Fatal(err)
	}

	if err := eng.DeliverSignal(ctx, &dagv1.SignalDelivery{
		EntityId: ref.EntityId, SignalName: "manual_approval", DeliveryId: "D1",
	}); err != nil {
		t.Fatalf("accepted_signals alias should be accepted: %v", err)
	}
	inst, err := eng.GetInstance(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if inst.Status != dagv1.InstanceStatus_INSTANCE_STATUS_RUNNING {
		t.Fatalf("expected RUNNING after accepted signal, got %v", inst.Status)
	}
}

func TestPinOnNodeUsesLatestGraph(t *testing.T) {
	reg, _, eng, _, _ := setupGoldenPath(t)
	ctx := context.Background()
	ref := &dagv1.EntityRef{EntityId: "order-pin"}
	_, err := eng.StartInstance(ctx, &dagv1.StartInstanceRequest{
		Ref:            ref,
		TypeKey:        &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema},
		InitialPayload: orderAny(&testpb.Order{OrderId: "o-pin"}),
		GraphVersion:   &dagv1.GraphVersion{GraphId: graphID, Version: graphVersion},
		GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_NODE,
	})
	if err != nil {
		t.Fatal(err)
	}

	v2 := goldenGraphSpec()
	v2.Version = &dagv1.GraphVersion{GraphId: graphID, Version: "v2"}
	v2.Nodes["validate"].Transitions[0].TargetNodeId = "term_failure"
	if err := reg.RegisterGraph(v2); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 10; i++ {
		if err := eng.RunOnce(ctx); err != nil {
			t.Fatal(err)
		}
	}
	inst, err := eng.GetInstance(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if inst.CurrentNodeId != "term_failure" {
		t.Fatalf("PIN_ON_NODE should route via v2 to term_failure, got %q", inst.CurrentNodeId)
	}
}

func TestTimeoutHopUsesWaitNode(t *testing.T) {
	_, store, eng, _, _ := setupGoldenPath(t)
	ctx := context.Background()
	ref := &dagv1.EntityRef{EntityId: "order-timeout"}
	req := &dagv1.StartInstanceRequest{
		Ref:            ref,
		TypeKey:        &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema},
		GraphVersion:   &dagv1.GraphVersion{GraphId: graphID, Version: graphVersion},
		GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_START,
	}
	if _, err := store.CreateInstance(ctx, req, "wait_approval"); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Second)
	if err := store.SetWaiting(ref, "wait_approval", &dag.WaitingInstance{
		Ref:                   ref,
		Deadline:              past,
		OnTimeoutTargetNodeID: "term_failure",
		SignalName:            "manual_approval",
	}); err != nil {
		t.Fatal(err)
	}
	if err := eng.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	journal, err := store.ListJournal(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range journal {
		if e.Kind == dagv1.JournalKind_JOURNAL_KIND_SIGNAL_RECEIVED && e.NodeId == "wait_approval" {
			return
		}
	}
	t.Fatal("expected timeout journal with wait node id")
}

func TestConcurrentRunOnceNoDoubleExecute(t *testing.T) {
	_, _, eng, charge, _ := setupGoldenPath(t)
	ctx := context.Background()
	ref := &dagv1.EntityRef{EntityId: "order-concurrent"}
	startOrder(t, eng, ref)

	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			_ = eng.RunOnce(ctx)
		}()
	}
	wg.Wait()

	journal, err := eng.store.ListJournal(ctx, ref)
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
		t.Fatalf("expected exactly 1 committed validate hop, got %d (executes=%d)", committed, charge.execCount.Load())
	}
}

func TestSpawnJournalKind(t *testing.T) {
	_, store, eng, _, _ := setupGoldenPath(t)
	ctx := context.Background()
	ref := &dagv1.EntityRef{EntityId: "order-spawn-journal"}
	startOrder(t, eng, ref)
	runUntilWaiting(t, eng, ref)
	if err := eng.DeliverSignal(ctx, &dagv1.SignalDelivery{
		EntityId: ref.EntityId, SignalName: "manual_approval", DeliveryId: "D1",
	}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		if err := eng.RunOnce(ctx); err != nil && err != context.Canceled {
			t.Fatal(err)
		}
		inst, _ := eng.GetInstance(ctx, ref)
		if inst.CurrentNodeId == "join" || inst.Status == dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED {
			break
		}
	}
	journal, err := store.ListJournal(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range journal {
		if e.NodeId == "spawn_payments" && e.Kind == dagv1.JournalKind_JOURNAL_KIND_SPAWNED {
			return
		}
	}
	t.Fatal("expected JOURNAL_KIND_SPAWNED for spawn hop")
}

func TestExecutionAttemptPersisted(t *testing.T) {
	_, store, eng, _, _ := setupGoldenPath(t)
	ctx := context.Background()
	ref := &dagv1.EntityRef{EntityId: "order-retry"}
	startOrder(t, eng, ref)
	runUntilWaiting(t, eng, ref)
	if err := eng.DeliverSignal(ctx, &dagv1.SignalDelivery{
		EntityId: ref.EntityId, SignalName: "manual_approval", DeliveryId: "D1",
	}); err != nil {
		t.Fatal(err)
	}
	if err := eng.RunOnce(ctx); err != nil && err != context.Canceled {
		t.Fatal(err)
	}
	idem := runtime.HopIdempotencyKey(ref.EntityId, "charge", 1)
	rec, err := store.GetExecution(ctx, idem)
	if err != nil {
		t.Fatal(err)
	}
	if rec == nil || rec.Attempt < 1 {
		t.Fatalf("expected persisted attempt >= 1, got %+v", rec)
	}
}
