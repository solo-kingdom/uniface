package dag

import (
	"context"
	"path/filepath"
	"testing"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
)

// TestLoadFixtures 通过真实 Runtime 加载内置 fixture，确保升级后的 YAML 全部通过校验与注册。
func TestLoadFixtures(t *testing.T) {
	rt, err := NewRuntime(filepath.Join("..", "fixtures", "graphs"))
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	defer rt.Close()
	for _, name := range []string{"echo", "approval_branch", "http_call", "saga_compensate", "fork_join", "dynamic_join"} {
		if _, err := rt.LoadFixture(name); err != nil {
			t.Fatalf("LoadFixture(%q): %v", name, err)
		}
	}
}

// TestEndToEnd_HttpUnitViaMockServer 端到端验证：mock HTTP server + http_call fixture + RunOnce → COMPLETED。
func TestEndToEnd_HttpUnitViaMockServer(t *testing.T) {
	rt, err := NewRuntime(filepath.Join("..", "fixtures", "graphs"))
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	defer rt.Close()
	if _, err := rt.LoadFixture("http_call"); err != nil {
		t.Fatalf("LoadFixture(http_call): %v", err)
	}
	// 启动内置 mock HTTP 服务（http_call fixture 指向 127.0.0.1:18099）。
	if err := rt.StartMockHTTPServer("127.0.0.1:18099"); err != nil {
		t.Fatalf("StartMockHTTPServer: %v", err)
	}
	ctx := context.Background()
	if _, err := rt.Start(ctx, "http_call", "e2e-1", "hello"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	for i := 0; i < 10; i++ {
		if err := rt.RunOnce(ctx); err != nil {
			t.Fatalf("RunOnce(%d): %v", i, err)
		}
	}
	inst, err := rt.GetInstance(ctx, "e2e-1")
	if err != nil {
		t.Fatalf("GetInstance: %v", err)
	}
	if inst.Status != dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED {
		t.Fatalf("expected COMPLETED, got %v (node=%s)", inst.Status, inst.CurrentNodeId)
	}
}



func TestParseGraphYAML_StringUnit(t *testing.T) {
	yaml := []byte(`
graph_id: t
version: v1
entity_type: lab.Generic
schema_version: v1
entry: a
nodes:
  a:
    kind: compute
    unit: lab.echo
    transitions:
      - target: done
  done:
    kind: terminal
    outcome: success
`)
	spec, defs, err := parseGraphYAML(yaml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if spec.Nodes["a"].UnitId != "lab.echo" {
		t.Fatalf("expected unit_id lab.echo, got %q", spec.Nodes["a"].UnitId)
	}
	if len(defs) != 0 {
		t.Fatalf("expected no inline defs for string unit, got %d", len(defs))
	}
}

func TestParseGraphYAML_HttpUnitObject(t *testing.T) {
	yaml := []byte(`
graph_id: t
version: v1
entity_type: lab.Generic
schema_version: v1
entry: call
nodes:
  call:
    kind: compute
    unit:
      http:
        url: http://example:8080
        path: /charge
        method: PUT
        headers:
          Authorization: Bearer x
        request_body:
          field_path: Order
        response:
          mode: mutation
          payload_type_url: type.googleapis.com/x.Y
          payload_field: Z
          on_success: success
        timeout: 5s
        retry_on:
          retry_status_codes: [503]
          fail_status_codes: [404]
    retry_policy:
      max_attempts: 5
      initial_backoff: 200ms
    transitions:
      - target: done
  done:
    kind: terminal
    outcome: success
`)
	spec, defs, err := parseGraphYAML(yaml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 inline def, got %d", len(defs))
	}
	def := defs[0]
	if def.UnitId != "t.call" {
		t.Fatalf("expected auto unit_id t.call, got %q", def.UnitId)
	}
	http := def.GetHttp()
	if http == nil {
		t.Fatal("expected http implementation")
	}
	if http.Url != "http://example:8080" || http.Path != "/charge" || http.Method != "PUT" {
		t.Fatalf("unexpected http config: %+v", http)
	}
	if http.Headers["Authorization"] != "Bearer x" {
		t.Fatalf("expected Authorization header, got %+v", http.Headers)
	}
	if http.RequestBody.GetFieldPath() != "Order" {
		t.Fatalf("expected request body field Order, got %q", http.RequestBody.GetFieldPath())
	}
	if http.Response.GetMode() != dagv1.ResponseMapping_MODE_MUTATION {
		t.Fatalf("expected mode MUTATION, got %v", http.Response.GetMode())
	}
	if http.Response.GetPayloadTypeUrl() != "type.googleapis.com/x.Y" {
		t.Fatalf("unexpected payload type url %q", http.Response.GetPayloadTypeUrl())
	}
	if http.Response.GetOnSuccess() != dagv1.TerminalOutcome_TERMINAL_OUTCOME_SUCCESS {
		t.Fatalf("expected on_success SUCCESS, got %v", http.Response.GetOnSuccess())
	}
	if http.RetryOn.GetRetryStatusCodes()[0] != 503 {
		t.Fatalf("expected retry 503, got %v", http.RetryOn.GetRetryStatusCodes())
	}
	if def.RetryPolicy.GetMaxAttempts() != 5 {
		t.Fatalf("expected retry policy max_attempts 5, got %d", def.RetryPolicy.GetMaxAttempts())
	}
	if spec.Nodes["call"].UnitId != "t.call" {
		t.Fatalf("node should reference auto unit_id t.call, got %q", spec.Nodes["call"].UnitId)
	}
}

func TestParseGraphYAML_ConditionKinds(t *testing.T) {
	yaml := []byte(`
graph_id: t
version: v1
entity_type: lab.Generic
schema_version: v1
entry: decide
nodes:
  decide:
    kind: compute
    unit: lab.echo
    transitions:
      - target: yes
        priority: 10
        condition:
          field: {path: Value, op: eq, value: "approved"}
      - target: via_signal
        priority: 5
        condition:
          signal:
            name: ok
            payload_predicate: {path: Value, op: eq, value: "true"}
      - target: no
        condition:
          always: true
  yes:
    kind: terminal
    outcome: success
  via_signal:
    kind: terminal
    outcome: success
  no:
    kind: terminal
    outcome: failure
`)
	spec, _, err := parseGraphYAML(yaml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	trs := spec.Nodes["decide"].Transitions
	if len(trs) != 3 {
		t.Fatalf("expected 3 transitions, got %d", len(trs))
	}
	if trs[0].GetPriority() != 10 {
		t.Fatalf("expected priority 10, got %d", trs[0].GetPriority())
	}
	if _, ok := trs[0].Condition.GetKind().(*dagv1.Condition_FieldPredicate); !ok {
		t.Fatalf("expected field predicate, got %T", trs[0].Condition.GetKind())
	}
	if _, ok := trs[1].Condition.GetKind().(*dagv1.Condition_SignalPredicate); !ok {
		t.Fatalf("expected signal predicate, got %T", trs[1].Condition.GetKind())
	}
	if _, ok := trs[2].Condition.GetKind().(*dagv1.Condition_Always); !ok {
		t.Fatalf("expected always, got %T", trs[2].Condition.GetKind())
	}
}

func TestParseGraphYAML_WaitExtensions(t *testing.T) {
	yaml := []byte(`
graph_id: t
version: v1
entity_type: lab.Generic
schema_version: v1
entry: w
nodes:
  w:
    kind: wait
    signal: approval
    deadline_seconds: 60
    on_timeout: fail
    transitions:
      - target: ok
  ok:
    kind: terminal
    outcome: success
  fail:
    kind: terminal
    outcome: failure
`)
	spec, _, err := parseGraphYAML(yaml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	wc := spec.Nodes["w"].WaitConfig
	if wc.GetDefaultDeadlineSeconds() != 60 {
		t.Fatalf("expected deadline 60, got %d", wc.GetDefaultDeadlineSeconds())
	}
	if wc.GetOnTimeoutTargetNodeId() != "fail" {
		t.Fatalf("expected on_timeout fail, got %q", wc.GetOnTimeoutTargetNodeId())
	}
}

func TestParseGraphYAML_JoinFailParent(t *testing.T) {
	yaml := []byte(`
graph_id: t
version: v1
entity_type: lab.Generic
schema_version: v1
entry: j
nodes:
  j:
    kind: join
    join_policy: all_success
    fail_parent_on_child_failure: true
    dynamic_prefix: child-
    transitions:
      - target: done
  done:
    kind: terminal
    outcome: success
`)
	spec, _, err := parseGraphYAML(yaml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	js := spec.Nodes["j"].JoinSpec
	if !js.GetFailParentOnChildFailure() {
		t.Fatal("expected fail_parent_on_child_failure true")
	}
}

func TestParseGraphYAML_PriorityDefault(t *testing.T) {
	yaml := []byte(`
graph_id: t
version: v1
entity_type: lab.Generic
schema_version: v1
entry: a
nodes:
  a:
    kind: compute
    unit: lab.echo
    transitions:
      - target: done
  done:
    kind: terminal
    outcome: success
`)
	spec, _, err := parseGraphYAML(yaml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if spec.Nodes["a"].Transitions[0].GetPriority() != 0 {
		t.Fatalf("expected default priority 0, got %d", spec.Nodes["a"].Transitions[0].GetPriority())
	}
}
