package api

import (
	"errors"
	"strings"
	"testing"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
)

// stubSentinel 是 ResultSentinel 的最小测试实现（不带 Err）。
type stubSentinel struct {
	completed bool
	status    dagv1.InstanceStatus
}

func (s *stubSentinel) IsCompleted() bool         { return s.completed }
func (s *stubSentinel) Status() dagv1.InstanceStatus { return s.status }

// stubSentinelWithErr 是 ResultSentinelWithErr 的测试实现。
type stubSentinelWithErr struct {
	stubSentinel
	err error
}

func (s *stubSentinelWithErr) Err() error { return s.err }

// TestRecordResult_Completed 验证 IsCompleted=true → ok=true, Error 空。
func TestRecordResult_Completed(t *testing.T) {
	rec := NewOpRecorder(10)
	res := &stubSentinel{completed: true, status: dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED}

	rec.RecordResult("echo", "e1", res)
	ops := rec.Snapshot()
	if len(ops) != 1 {
		t.Fatalf("len(ops) = %d, want 1", len(ops))
	}
	if !ops[0].OK {
		t.Fatal("OK = false, want true")
	}
	if ops[0].Error != "" {
		t.Fatalf("Error = %q, want empty", ops[0].Error)
	}
	if ops[0].Op != "echo" || ops[0].Detail != "e1" {
		t.Fatalf("op/detail = %q/%q", ops[0].Op, ops[0].Detail)
	}
}

// TestRecordResult_Failed_WithErr 验证 IsCompleted=false + Err()!=nil → ok=false, Error=Err。
func TestRecordResult_Failed_WithErr(t *testing.T) {
	rec := NewOpRecorder(10)
	res := &stubSentinelWithErr{
		stubSentinel: stubSentinel{completed: false, status: dagv1.InstanceStatus_INSTANCE_STATUS_FAILED},
		err:          errors.New("unit failed"),
	}

	rec.RecordResult("echo", "e1", res)
	ops := rec.Snapshot()
	if len(ops) != 1 {
		t.Fatalf("len(ops) = %d, want 1", len(ops))
	}
	if ops[0].OK {
		t.Fatal("OK = true, want false")
	}
	if ops[0].Error != "unit failed" {
		t.Fatalf("Error = %q, want 'unit failed'", ops[0].Error)
	}
}

// TestRecordResult_Nil 验证 nil 入参 → ok=false, Error 含 "nil result"。
func TestRecordResult_Nil(t *testing.T) {
	rec := NewOpRecorder(10)

	rec.RecordResult("echo", "e1", nil)
	ops := rec.Snapshot()
	if len(ops) != 1 {
		t.Fatalf("len(ops) = %d, want 1", len(ops))
	}
	if ops[0].OK {
		t.Fatal("OK = true, want false")
	}
	if !strings.Contains(ops[0].Error, "nil result") {
		t.Fatalf("Error = %q, want to contain 'nil result'", ops[0].Error)
	}
}

// TestRecord_CompatWithOriginal 验证原 Record 行为不变（向后兼容）。
func TestRecord_CompatWithOriginal(t *testing.T) {
	rec := NewOpRecorder(10)

	rec.Record("echo", "e1", true, nil)
	rec.Record("echo", "e2", false, errors.New("boom"))
	ops := rec.Snapshot()
	if len(ops) != 2 {
		t.Fatalf("len(ops) = %d, want 2", len(ops))
	}
	if !ops[0].OK || ops[0].Error != "" {
		t.Fatalf("ops[0] = %+v", ops[0])
	}
	if ops[1].OK || ops[1].Error != "boom" {
		t.Fatalf("ops[1] = %+v", ops[1])
	}
}

// TestRecordResult_Failed_FallbackToStatus 验证无 Err() 时回退到 "status=<Status>"。
func TestRecordResult_Failed_FallbackToStatus(t *testing.T) {
	rec := NewOpRecorder(10)
	res := &stubSentinel{completed: false, status: dagv1.InstanceStatus_INSTANCE_STATUS_FAILED}

	rec.RecordResult("echo", "e1", res)
	ops := rec.Snapshot()
	if ops[0].OK {
		t.Fatal("OK = true, want false")
	}
	if ops[0].Error != "status=INSTANCE_STATUS_FAILED" {
		t.Fatalf("Error = %q, want 'status=FAILED'", ops[0].Error)
	}
}
