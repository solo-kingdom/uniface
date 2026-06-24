package memory

import (
	"context"
	"testing"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag/entity"
	"github.com/solo-kingdom/uniface/pkg/dag/runtime"
	"github.com/solo-kingdom/uniface/pkg/dag/testpb"
)

func TestCommitHopMultiSpawnJournal(t *testing.T) {
	store := NewLineStore()
	ctx := context.Background()
	ref := &dagv1.EntityRef{EntityId: "parent-1"}
	req := &dagv1.StartInstanceRequest{
		Ref:            ref,
		TypeKey:        &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema},
		GraphVersion:   &dagv1.GraphVersion{GraphId: graphID, Version: graphVersion},
		GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_START,
	}
	if _, err := store.CreateInstance(ctx, req, "spawn"); err != nil {
		t.Fatal(err)
	}
	spawned := []*dagv1.EntityRef{
		{EntityId: "pay-1"},
		{EntityId: "pay-2"},
		{EntityId: "pay-3"},
	}
	idem := runtime.HopIdempotencyKey(ref.EntityId, "spawn", 0)
	if err := store.CommitHop(ctx, &dagv1.HopCommit{
		Ref:            ref,
		NodeId:         "spawn",
		InputSequence:  0,
		IdempotencyKey: idem,
		NextNodeId:     "join",
		NextStatus:     dagv1.InstanceStatus_INSTANCE_STATUS_RUNNING,
		Spawned:        spawned,
		JournalKind:    dagv1.JournalKind_JOURNAL_KIND_SPAWNED,
	}); err != nil {
		t.Fatal(err)
	}
	journal, err := store.ListJournal(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range journal {
		if e.Kind != dagv1.JournalKind_JOURNAL_KIND_SPAWNED {
			continue
		}
		if len(e.SpawnedRefs) != 3 {
			t.Fatalf("expected 3 spawned_refs, got %d", len(e.SpawnedRefs))
		}
		if e.SpawnedRef.EntityId != "pay-1" {
			t.Fatalf("spawned_ref compat expected pay-1, got %q", e.SpawnedRef.EntityId)
		}
		return
	}
	t.Fatal("spawn journal not found")
}

func TestListChildrenByCorrelationPrefix(t *testing.T) {
	store := NewLineStore()
	ctx := context.Background()
	parent := &dagv1.EntityRef{EntityId: "parent-1"}
	childType := &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema}
	gv := &dagv1.GraphVersion{GraphId: graphID, Version: graphVersion}
	for _, id := range []string{"pay-1", "pay-2", "other-1"} {
		corr := id
		if id == "other-1" {
			corr = "other-1"
		}
		req := &dagv1.StartInstanceRequest{
			Ref:            &dagv1.EntityRef{EntityId: id},
			TypeKey:        childType,
			GraphVersion:   gv,
			GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_START,
			Parent:         parent,
			CorrelationId:  corr,
		}
		if _, err := store.CreateInstance(ctx, req, "term_success"); err != nil {
			t.Fatal(err)
		}
	}
	children, err := store.ListChildrenByCorrelationPrefix(parent, "pay-")
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 2 {
		t.Fatalf("expected 2 children with pay- prefix, got %d", len(children))
	}
}

func TestListSpawnedFromJournal(t *testing.T) {
	store := NewLineStore()
	ctx := context.Background()
	ref := &dagv1.EntityRef{EntityId: "parent-1"}
	req := &dagv1.StartInstanceRequest{
		Ref:            ref,
		TypeKey:        &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema},
		GraphVersion:   &dagv1.GraphVersion{GraphId: graphID, Version: graphVersion},
		GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_START,
	}
	if _, err := store.CreateInstance(ctx, req, "spawn"); err != nil {
		t.Fatal(err)
	}
	spawned := []*dagv1.EntityRef{{EntityId: "c-1"}, {EntityId: "c-2"}}
	if err := store.CommitHop(ctx, &dagv1.HopCommit{
		Ref:            ref,
		NodeId:         "spawn",
		InputSequence:  0,
		IdempotencyKey: "spawn-1",
		NextNodeId:     "join",
		NextStatus:     dagv1.InstanceStatus_INSTANCE_STATUS_RUNNING,
		Spawned:        spawned,
		JournalKind:    dagv1.JournalKind_JOURNAL_KIND_SPAWNED,
	}); err != nil {
		t.Fatal(err)
	}
	refs, err := store.ListSpawnedFromJournal(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 {
		t.Fatalf("expected 2 spawned refs, got %d", len(refs))
	}
}

func TestSignalMergeIncrementsSequence(t *testing.T) {
	store := NewLineStore()
	ctx := context.Background()
	ref := &dagv1.EntityRef{EntityId: "order-1"}
	req := &dagv1.StartInstanceRequest{
		Ref:            ref,
		TypeKey:        &dagv1.EntityTypeKey{EntityType: orderType, PayloadSchemaVersion: orderSchema},
		InitialPayload: orderAny(&testpb.Order{OrderId: "o-1"}),
		GraphVersion:   &dagv1.GraphVersion{GraphId: graphID, Version: graphVersion},
	}
	if _, err := store.CreateInstance(ctx, req, "wait"); err != nil {
		t.Fatal(err)
	}
	merged := orderAny(&testpb.Order{OrderId: "o-1", Approved: true})
	if err := store.CommitHop(ctx, &dagv1.HopCommit{
		Ref:            ref,
		NodeId:         "wait",
		InputSequence:  0,
		IdempotencyKey: "sig-1",
		OutputSnapshot: entity.NewSnapshot(ref, req.TypeKey, 1, merged),
		NextNodeId:     "next",
		NextStatus:     dagv1.InstanceStatus_INSTANCE_STATUS_RUNNING,
		JournalKind:    dagv1.JournalKind_JOURNAL_KIND_SIGNAL_RECEIVED,
		SignalName:     "approval",
		DeliveryId:     "D1",
	}); err != nil {
		t.Fatal(err)
	}
	inst, err := store.GetInstance(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if inst.Sequence != 1 {
		t.Fatalf("expected sequence 1 after signal merge, got %d", inst.Sequence)
	}
}
