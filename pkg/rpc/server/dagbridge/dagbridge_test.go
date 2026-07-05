package dagbridge_test

import (
	"strings"
	"testing"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag/invocation/app"
	"github.com/solo-kingdom/uniface/pkg/rpc/server/dagbridge"
)

func mkResult(status dagv1.InstanceStatus, value string) *app.StringCallResult {
	return &app.StringCallResult{
		CallResult: app.CallResult{
			Instance: &dagv1.EntityInstance{Status: status},
		},
		Value: value,
	}
}

// TestResponseForTerminalResult_COMPLETED 验证 COMPLETED → 200 + Value。
func TestResponseForTerminalResult_COMPLETED(t *testing.T) {
	resp := dagbridge.ResponseForTerminalResult(mkResult(dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED, "echo:hello"))
	if resp.StatusCode != 200 {
		t.Fatalf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if string(resp.Body) != "echo:hello" {
		t.Fatalf("Body = %q, want echo:hello", resp.Body)
	}
}

// TestResponseForTerminalResult_WAITING 验证 WAITING → 500，body 含 "WAITING"。
func TestResponseForTerminalResult_WAITING(t *testing.T) {
	resp := dagbridge.ResponseForTerminalResult(mkResult(dagv1.InstanceStatus_INSTANCE_STATUS_WAITING, "x"))
	if resp.StatusCode != 500 {
		t.Fatalf("StatusCode = %d, want 500", resp.StatusCode)
	}
	if !strings.Contains(string(resp.Body), "WAITING") {
		t.Fatalf("Body = %q, want to contain WAITING", resp.Body)
	}
}

// TestResponseForTerminalResult_FAILED 验证 FAILED → 500，body 含 "FAILED" 与 Value。
func TestResponseForTerminalResult_FAILED(t *testing.T) {
	resp := dagbridge.ResponseForTerminalResult(mkResult(dagv1.InstanceStatus_INSTANCE_STATUS_FAILED, "boom"))
	if resp.StatusCode != 500 {
		t.Fatalf("StatusCode = %d, want 500", resp.StatusCode)
	}
	body := string(resp.Body)
	if !strings.Contains(body, "FAILED") {
		t.Fatalf("Body = %q, want to contain FAILED", body)
	}
	if !strings.Contains(body, "boom") {
		t.Fatalf("Body = %q, want to contain boom", body)
	}
}

// TestResponseForTerminalResult_COMPENSATED 验证 COMPENSATED → 500，body 含状态名与 Value。
func TestResponseForTerminalResult_COMPENSATED(t *testing.T) {
	resp := dagbridge.ResponseForTerminalResult(mkResult(dagv1.InstanceStatus_INSTANCE_STATUS_COMPENSATED, "rolled-back"))
	if resp.StatusCode != 500 {
		t.Fatalf("StatusCode = %d, want 500", resp.StatusCode)
	}
	body := string(resp.Body)
	if !strings.Contains(body, "COMPENSATED") {
		t.Fatalf("Body = %q, want to contain COMPENSATED", body)
	}
	if !strings.Contains(body, "rolled-back") {
		t.Fatalf("Body = %q, want to contain rolled-back", body)
	}
}

// TestResponseForTerminalResult_CANCELLED 验证 CANCELLED → 500，body 含状态名与 Value。
func TestResponseForTerminalResult_CANCELLED(t *testing.T) {
	resp := dagbridge.ResponseForTerminalResult(mkResult(dagv1.InstanceStatus_INSTANCE_STATUS_CANCELLED, "user-cancel"))
	if resp.StatusCode != 500 {
		t.Fatalf("StatusCode = %d, want 500", resp.StatusCode)
	}
	body := string(resp.Body)
	if !strings.Contains(body, "CANCELLED") {
		t.Fatalf("Body = %q, want to contain CANCELLED", body)
	}
	if !strings.Contains(body, "user-cancel") {
		t.Fatalf("Body = %q, want to contain user-cancel", body)
	}
}

// TestResponseForTerminalResult_Nil 验证 nil 入参 → 500 + "nil dag result"，不 panic。
func TestResponseForTerminalResult_Nil(t *testing.T) {
	resp := dagbridge.ResponseForTerminalResult(nil)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.StatusCode != 500 {
		t.Fatalf("StatusCode = %d, want 500", resp.StatusCode)
	}
	if !strings.Contains(string(resp.Body), "nil dag result") {
		t.Fatalf("Body = %q, want to contain 'nil dag result'", resp.Body)
	}
}
