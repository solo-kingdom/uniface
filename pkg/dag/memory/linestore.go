package memory

import (
	"context"
	"strings"
	"sync"
	"time"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"github.com/solo-kingdom/uniface/pkg/dag/entity"
	"github.com/solo-kingdom/uniface/pkg/dag/runtime"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type instanceRecord struct {
	instance *dagv1.EntityInstance
	snapshot *dagv1.EntitySnapshot
	saga     *dagv1.SagaState
	journal  []*dagv1.LineJournalEntry
	waiting  *dag.WaitingInstance
}

// LineStore 内存 LineStore 实现。
type LineStore struct {
	mu         sync.RWMutex
	instances  map[string]*instanceRecord
	executions map[string]*dagv1.ExecutionRecord
	signals    map[string]struct{}
	closed     bool
}

func NewLineStore() *LineStore {
	return &LineStore{
		instances:  make(map[string]*instanceRecord),
		executions: make(map[string]*dagv1.ExecutionRecord),
		signals:    make(map[string]struct{}),
	}
}

func (s *LineStore) CreateInstance(ctx context.Context, req *dagv1.StartInstanceRequest, entryNodeID string) (*dagv1.EntityInstance, error) {
	if req == nil || req.Ref == nil || req.Ref.EntityId == "" {
		return nil, dag.ErrInstanceNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, dag.ErrStoreClosed
	}
	if _, exists := s.instances[req.Ref.EntityId]; exists {
		return nil, dag.ErrInstanceAlreadyExists
	}
	now := timestamppb.Now()
	policy := req.GraphPinPolicy
	if policy == dagv1.GraphPinPolicy_GRAPH_PIN_POLICY_UNSPECIFIED {
		policy = dagv1.GraphPinPolicy_GRAPH_PIN_ON_START
	}
	inst := &dagv1.EntityInstance{
		Ref:            req.Ref,
		TypeKey:        req.TypeKey,
		Status:         dagv1.InstanceStatus_INSTANCE_STATUS_RUNNING,
		Sequence:       0,
		CurrentNodeId:  entryNodeID,
		GraphVersion:   req.GraphVersion,
		GraphPinPolicy: policy,
		Parent:         req.Parent,
		CorrelationId:  req.CorrelationId,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	snap := entity.NewSnapshot(req.Ref, req.TypeKey, 0, req.InitialPayload)
	s.instances[req.Ref.EntityId] = &instanceRecord{
		instance: inst,
		snapshot: snap,
		saga:     &dagv1.SagaState{},
	}
	return cloneInstance(inst), nil
}

func (s *LineStore) GetInstance(ctx context.Context, ref *dagv1.EntityRef) (*dagv1.EntityInstance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, err := s.getRecord(ref)
	if err != nil {
		return nil, err
	}
	return cloneInstance(rec.instance), nil
}

// GetSnapshot 按实体引用读取当前快照（序列号 + 业务 payload）。
// 返回深拷贝，避免调用方修改 LineStore 内部状态；实例不存在时返回 ErrInstanceNotFound。
func (s *LineStore) GetSnapshot(ctx context.Context, ref *dagv1.EntityRef) (*dagv1.EntitySnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, err := s.getRecord(ref)
	if err != nil {
		return nil, err
	}
	return entity.CloneSnapshot(rec.snapshot), nil
}

func (s *LineStore) CreateExecution(ctx context.Context, record *dagv1.ExecutionRecord) (*dagv1.ExecutionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, dag.ErrStoreClosed
	}
	existing, ok := s.executions[record.IdempotencyKey]
	if ok {
		if existing.Status == dagv1.ExecutionStatus_EXECUTION_STATUS_COMMITTED {
			return cloneExecution(existing), nil
		}
		return cloneExecution(existing), nil
	}
	record.StartedAt = timestamppb.Now()
	if record.Status == dagv1.ExecutionStatus_EXECUTION_STATUS_UNSPECIFIED {
		record.Status = dagv1.ExecutionStatus_EXECUTION_STATUS_RUNNING
	}
	s.executions[record.IdempotencyKey] = cloneExecution(record)
	return cloneExecution(record), nil
}

func (s *LineStore) GetExecution(ctx context.Context, idempotencyKey string) (*dagv1.ExecutionRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.executions[idempotencyKey]
	if !ok {
		return nil, nil
	}
	return cloneExecution(rec), nil
}

func (s *LineStore) CommitHop(ctx context.Context, commit *dagv1.HopCommit) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return dag.ErrStoreClosed
	}
	rec, err := s.getRecordLocked(commit.Ref)
	if err != nil {
		return err
	}
	for _, j := range rec.journal {
		if j.IdempotencyKey == commit.IdempotencyKey && j.Kind == commit.JournalKind {
			if commit.JournalKind == dagv1.JournalKind_JOURNAL_KIND_COMPENSATION_COMMITTED {
				reconcileSagaPop(rec, commit)
			}
			return nil
		}
	}
	inst := rec.instance
	inst.UpdatedAt = timestamppb.Now()
	switch commit.JournalKind {
	case dagv1.JournalKind_JOURNAL_KIND_SIGNAL_RECEIVED:
		inst.Status = commit.NextStatus
		inst.CurrentNodeId = commit.NextNodeId
		rec.waiting = nil
		if commit.OutputSnapshot != nil {
			inst.Sequence++
			rec.snapshot = entity.CloneSnapshot(commit.OutputSnapshot)
			rec.snapshot.Sequence = inst.Sequence
		}
	case dagv1.JournalKind_JOURNAL_KIND_JOIN_COMMITTED:
		inst.Sequence++
		inst.Status = commit.NextStatus
		inst.CurrentNodeId = commit.NextNodeId
		if commit.OutputSnapshot != nil {
			rec.snapshot = entity.CloneSnapshot(commit.OutputSnapshot)
			rec.snapshot.Sequence = inst.Sequence
		}
	case dagv1.JournalKind_JOURNAL_KIND_COMPENSATION_COMMITTED:
		popSagaStack(rec)
	default:
		if commit.OutputSnapshot != nil {
			inst.Sequence++
			rec.snapshot = entity.CloneSnapshot(commit.OutputSnapshot)
			rec.snapshot.Sequence = inst.Sequence
		}
		inst.Status = commit.NextStatus
		inst.CurrentNodeId = commit.NextNodeId
	}
	if commit.SagaDelta != nil {
		if rec.saga == nil {
			rec.saga = &dagv1.SagaState{}
		}
		rec.saga.Stack = append(rec.saga.Stack, commit.SagaDelta.Stack...)
		rec.saga.Records = append(rec.saga.Records, commit.SagaDelta.Records...)
	}
	entry := &dagv1.LineJournalEntry{
		JournalSequence: int64(len(rec.journal) + 1),
		Kind:            commit.JournalKind,
		NodeId:          commit.NodeId,
		InputSequence:   commit.InputSequence,
		OutputSequence:  inst.Sequence,
		OutputSnapshot:  entity.CloneSnapshot(rec.snapshot),
		IdempotencyKey:  commit.IdempotencyKey,
		CommittedAt:     timestamppb.Now(),
		SignalName:      commit.SignalName,
		DeliveryId:      commit.DeliveryId,
		FailureReason:   commit.FailureReason,
	}
	if len(commit.Spawned) > 0 {
		entry.SpawnedRef = commit.Spawned[0]
		entry.SpawnedRefs = append([]*dagv1.EntityRef(nil), commit.Spawned...)
	}
	rec.journal = append(rec.journal, entry)
	if exec, ok := s.executions[commit.IdempotencyKey]; ok {
		exec.Status = dagv1.ExecutionStatus_EXECUTION_STATUS_COMMITTED
		exec.CommittedAt = timestamppb.Now()
	}
	return nil
}

func (s *LineStore) ListJournal(ctx context.Context, ref *dagv1.EntityRef) ([]*dagv1.LineJournalEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, err := s.getRecord(ref)
	if err != nil {
		return nil, err
	}
	out := make([]*dagv1.LineJournalEntry, len(rec.journal))
	copy(out, rec.journal)
	return out, nil
}

func (s *LineStore) GetSagaState(ctx context.Context, ref *dagv1.EntityRef) (*dagv1.SagaState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, err := s.getRecord(ref)
	if err != nil {
		return nil, err
	}
	return cloneSaga(rec.saga), nil
}

func (s *LineStore) RecordSignalDelivery(ctx context.Context, entityID, signalName, deliveryID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := runtime.SignalIdempotencyKey(entityID, signalName, deliveryID)
	if _, ok := s.signals[key]; ok {
		return false, nil
	}
	s.signals[key] = struct{}{}
	return true, nil
}

func (s *LineStore) ListRunnableInstances(ctx context.Context) ([]*dagv1.EntityRef, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var refs []*dagv1.EntityRef
	for _, rec := range s.instances {
		switch rec.instance.Status {
		case dagv1.InstanceStatus_INSTANCE_STATUS_RUNNING, dagv1.InstanceStatus_INSTANCE_STATUS_COMPENSATING:
			refs = append(refs, rec.instance.Ref)
		}
	}
	return refs, nil
}

func (s *LineStore) ListWaitingTimeouts(ctx context.Context, now time.Time) ([]*dag.WaitingInstance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*dag.WaitingInstance
	for _, rec := range s.instances {
		if rec.instance.Status != dagv1.InstanceStatus_INSTANCE_STATUS_WAITING || rec.waiting == nil {
			continue
		}
		if !rec.waiting.Deadline.IsZero() && !now.Before(rec.waiting.Deadline) {
			out = append(out, rec.waiting)
		}
	}
	return out, nil
}

func (s *LineStore) SetWaiting(ref *dagv1.EntityRef, nodeID string, w *dag.WaitingInstance) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.getRecordLocked(ref)
	if err != nil {
		return err
	}
	rec.waiting = w
	rec.instance.Status = dagv1.InstanceStatus_INSTANCE_STATUS_WAITING
	if nodeID != "" {
		rec.instance.CurrentNodeId = nodeID
	}
	rec.instance.UpdatedAt = timestamppb.Now()
	return nil
}

func (s *LineStore) GetWaiting(ref *dagv1.EntityRef) (*dag.WaitingInstance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, err := s.getRecord(ref)
	if err != nil {
		return nil, err
	}
	return rec.waiting, nil
}

func (s *LineStore) FindChildByCorrelation(parent *dagv1.EntityRef, correlationID string) (*dagv1.EntityRef, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, rec := range s.instances {
		if rec.instance.Parent == nil || parent == nil {
			continue
		}
		if rec.instance.Parent.EntityId == parent.EntityId && rec.instance.CorrelationId == correlationID {
			return rec.instance.Ref, nil
		}
	}
	return nil, dag.ErrInstanceNotFound
}

// ListChildrenByCorrelationPrefix 列出 parent 下 correlation_id 前缀匹配的子实例。
func (s *LineStore) ListChildrenByCorrelationPrefix(parent *dagv1.EntityRef, prefix string) ([]*dagv1.EntityInstance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*dagv1.EntityInstance
	for _, rec := range s.instances {
		if rec.instance.Parent == nil || parent == nil {
			continue
		}
		if rec.instance.Parent.EntityId != parent.EntityId {
			continue
		}
		if prefix != "" && !strings.HasPrefix(rec.instance.CorrelationId, prefix) {
			continue
		}
		out = append(out, cloneInstance(rec.instance))
	}
	return out, nil
}

// ListSpawnedFromJournal 返回最近一次 SPAWNED journal 中的子实例 ref。
func (s *LineStore) ListSpawnedFromJournal(ctx context.Context, ref *dagv1.EntityRef) ([]*dagv1.EntityRef, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, err := s.getRecord(ref)
	if err != nil {
		return nil, err
	}
	for i := len(rec.journal) - 1; i >= 0; i-- {
		entry := rec.journal[i]
		if entry.Kind != dagv1.JournalKind_JOURNAL_KIND_SPAWNED {
			continue
		}
		if len(entry.SpawnedRefs) > 0 {
			out := make([]*dagv1.EntityRef, len(entry.SpawnedRefs))
			copy(out, entry.SpawnedRefs)
			return out, nil
		}
		if entry.SpawnedRef != nil {
			return []*dagv1.EntityRef{entry.SpawnedRef}, nil
		}
	}
	return nil, nil
}

func (s *LineStore) PopSagaFrame(ref *dagv1.EntityRef) (*dagv1.CompensationFrame, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.getRecordLocked(ref)
	if err != nil {
		return nil, err
	}
	if len(rec.saga.Stack) == 0 {
		return nil, nil
	}
	idx := len(rec.saga.Stack) - 1
	frame := rec.saga.Stack[idx]
	rec.saga.Stack = rec.saga.Stack[:idx]
	return frame, nil
}

func (s *LineStore) PushSagaFrame(ref *dagv1.EntityRef, frame *dagv1.CompensationFrame) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.getRecordLocked(ref)
	if err != nil {
		return err
	}
	rec.saga.Stack = append(rec.saga.Stack, frame)
	return nil
}

func (s *LineStore) UpdateExecutionAttempt(ctx context.Context, idempotencyKey string, attempt int32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return dag.ErrStoreClosed
	}
	rec, ok := s.executions[idempotencyKey]
	if !ok {
		return nil
	}
	rec.Attempt = attempt
	return nil
}

func (s *LineStore) AdvanceInstanceNode(ctx context.Context, ref *dagv1.EntityRef, nodeID string, status dagv1.InstanceStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return dag.ErrStoreClosed
	}
	rec, err := s.getRecordLocked(ref)
	if err != nil {
		return err
	}
	rec.instance.CurrentNodeId = nodeID
	rec.instance.Status = status
	rec.instance.UpdatedAt = timestamppb.Now()
	return nil
}

func (s *LineStore) UpdateInstanceStatus(ref *dagv1.EntityRef, status dagv1.InstanceStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.getRecordLocked(ref)
	if err != nil {
		return err
	}
	rec.instance.Status = status
	rec.instance.UpdatedAt = timestamppb.Now()
	return nil
}

func (s *LineStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func (s *LineStore) getRecord(ref *dagv1.EntityRef) (*instanceRecord, error) {
	if ref == nil || ref.EntityId == "" {
		return nil, dag.ErrInstanceNotFound
	}
	rec, ok := s.instances[ref.EntityId]
	if !ok {
		return nil, dag.ErrInstanceNotFound
	}
	return rec, nil
}

func (s *LineStore) getRecordLocked(ref *dagv1.EntityRef) (*instanceRecord, error) {
	return s.getRecord(ref)
}

func cloneInstance(inst *dagv1.EntityInstance) *dagv1.EntityInstance {
	if inst == nil {
		return nil
	}
	out := *inst
	if inst.Ref != nil {
		r := *inst.Ref
		out.Ref = &r
	}
	if inst.TypeKey != nil {
		k := *inst.TypeKey
		out.TypeKey = &k
	}
	if inst.GraphVersion != nil {
		g := *inst.GraphVersion
		out.GraphVersion = &g
	}
	if inst.Parent != nil {
		p := *inst.Parent
		out.Parent = &p
	}
	return &out
}

func cloneExecution(rec *dagv1.ExecutionRecord) *dagv1.ExecutionRecord {
	if rec == nil {
		return nil
	}
	out := *rec
	return &out
}

func popSagaStack(rec *instanceRecord) {
	if rec.saga == nil || len(rec.saga.Stack) == 0 {
		return
	}
	rec.saga.Stack = rec.saga.Stack[:len(rec.saga.Stack)-1]
}

func reconcileSagaPop(rec *instanceRecord, commit *dagv1.HopCommit) {
	if rec.saga == nil || len(rec.saga.Stack) == 0 {
		return
	}
	top := rec.saga.Stack[len(rec.saga.Stack)-1]
	if top.NodeId == commit.NodeId && top.ForwardSequence == commit.InputSequence {
		popSagaStack(rec)
	}
}

func cloneSaga(s *dagv1.SagaState) *dagv1.SagaState {
	if s == nil {
		return &dagv1.SagaState{}
	}
	out := &dagv1.SagaState{
		Stack:   append([]*dagv1.CompensationFrame(nil), s.Stack...),
		Records: append([]*dagv1.CompensationRecord(nil), s.Records...),
	}
	return out
}
