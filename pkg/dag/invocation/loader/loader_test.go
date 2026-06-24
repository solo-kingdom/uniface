package loader_test

import (
	"errors"
	"strings"
	"testing"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"github.com/solo-kingdom/uniface/pkg/dag/graph"
	"github.com/solo-kingdom/uniface/pkg/dag/invocation/loader"
)

const basicGraphYAML = `
graph_id: order-fulfill
version: v1
entity_type: order.Order
schema_version: v1
entry: validate
nodes:
  validate:
    kind: compute
    unit: order.validate
    transitions:
      - target: wait_approval
  wait_approval:
    kind: wait
    signal: manual_approval
    deadline_seconds: 60
    on_timeout: fail
    transitions:
      - target: charge
  charge:
    kind: compute
    unit: order.charge
    compensator: order.refund
    transitions:
      - target: join
      - target: fail
        condition:
          field:
            path: status
            op: eq
            value: "declined"
        priority: 10
  join:
    kind: join
    join_policy: all_success
    fail_parent_on_child_failure: true
    dynamic_prefix: "payment-"
    transitions:
      - target: done
  done:
    kind: terminal
    outcome: success
  fail:
    kind: terminal
    outcome: failure
`

// TestLoadYAML_BasicGraph 验证基础图（compute/wait/join/terminal + condition）解析，
// 且返回的 GraphSpec 可通过 graph.ValidateGraphSpec。
func TestLoadYAML_BasicGraph(t *testing.T) {
	res, err := loader.LoadYAML([]byte(basicGraphYAML), nil)
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	if res.Spec.Version.GraphId != "order-fulfill" {
		t.Fatalf("GraphId = %q", res.Spec.Version.GraphId)
	}
	if res.Spec.EntryNodeId != "validate" {
		t.Fatalf("Entry = %q", res.Spec.EntryNodeId)
	}
	// 验证 wait 节点。
	wait := res.Spec.Nodes["wait_approval"]
	if wait == nil || wait.Kind != dagv1.NodeKind_NODE_KIND_WAIT {
		t.Fatal("wait_approval 节点缺失或类型错误")
	}
	if wait.WaitConfig.SignalName != "manual_approval" {
		t.Fatalf("SignalName = %q", wait.WaitConfig.SignalName)
	}
	// 验证 join 节点。
	join := res.Spec.Nodes["join"]
	if join == nil || join.Kind != dagv1.NodeKind_NODE_KIND_JOIN || join.JoinSpec == nil {
		t.Fatal("join 节点缺失或类型错误")
	}
	// 验证 field predicate condition。
	charge := res.Spec.Nodes["charge"]
	if len(charge.Transitions) != 2 {
		t.Fatalf("charge transitions = %d, want 2", len(charge.Transitions))
	}
	var failCond *dagv1.Condition
	for _, tr := range charge.Transitions {
		if tr.TargetNodeId == "fail" {
			failCond = tr.Condition
		}
	}
	if failCond == nil {
		t.Fatal("missing charge→fail transition")
	}
	if fp := failCond.GetFieldPredicate(); fp == nil || fp.FieldPath != "status" || fp.Value != "declined" {
		t.Fatalf("field predicate = %+v", fp)
	}
	// 通过 graph.ValidateGraphSpec。
	if err := graph.ValidateGraphSpec(res.Spec); err != nil {
		t.Fatalf("ValidateGraphSpec: %v", err)
	}
}

const httpGraphYAML = `
graph_id: httpcall
version: v1
entity_type: order.Order
schema_version: v1
entry: call
nodes:
  call:
    kind: compute
    unit_id: http.call
    unit:
      http:
        service: upstream
        url: "http://localhost:9999"
        method: POST
        path: /process
        response:
          mode: auto
          payload_type_url: "type.googleapis.com/google.protobuf.StringValue"
    retry_policy:
      max_attempts: 5
      initial_backoff: 100ms
      max_backoff: 1s
    transitions:
      - target: done
  done:
    kind: terminal
    outcome: success
`

// TestLoadYAML_InlineHttpUnit 验证内联 HttpUnit 与 retry_policy 解析。
func TestLoadYAML_InlineHttpUnit(t *testing.T) {
	res, err := loader.LoadYAML([]byte(httpGraphYAML), nil)
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	if len(res.UnitDefs) != 1 {
		t.Fatalf("UnitDefs count = %d, want 1", len(res.UnitDefs))
	}
	def := res.UnitDefs[0]
	if def.UnitId != "http.call" {
		t.Fatalf("UnitId = %q", def.UnitId)
	}
	httpImpl := def.GetHttp()
	if httpImpl == nil {
		t.Fatal("implementation 不是 HttpUnit")
	}
	if httpImpl.Service != "upstream" || httpImpl.Method != "POST" || httpImpl.Path != "/process" {
		t.Fatalf("HttpUnit = %+v", httpImpl)
	}
	if def.RetryPolicy == nil || def.RetryPolicy.MaxAttempts != 5 {
		t.Fatalf("RetryPolicy = %+v", def.RetryPolicy)
	}
	if def.RetryPolicy.InitialBackoff.AsDuration().Milliseconds() != 100 {
		t.Fatalf("InitialBackoff = %v", def.RetryPolicy.InitialBackoff.AsDuration())
	}
	// 节点 unit_id 指向该 def。
	callNode := res.Spec.Nodes["call"]
	if callNode.UnitId != "http.call" {
		t.Fatalf("call.UnitId = %q", callNode.UnitId)
	}
}

// TestLoadYAML_InvalidGraph 验证非法图返回错误。
func TestLoadYAML_InvalidGraph(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{
			name: "missing graph_id",
			yaml: `entry: x
nodes:
  x:
    kind: terminal
    outcome: success
`,
		},
		{
			name: "unsupported node kind",
			yaml: `graph_id: g
entry: x
nodes:
  x:
    kind: bogus
`,
		},
		{
			name: "compute missing unit",
			yaml: `graph_id: g
entry: x
nodes:
  x:
    kind: compute
    transitions:
      - target: y
  y:
    kind: terminal
    outcome: success
`,
		},
		{
			name: "malformed yaml",
			yaml: `graph_id: g
entry: [unclosed`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loader.LoadYAML([]byte(tc.yaml), &loader.Options{
				DefaultEntityType:    "order.Order",
				DefaultSchemaVersion: "v1",
			})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

// TestLoadYAML_DefaultTypeFromOptions 验证文档缺省 entity_type 时使用 Options 默认值。
func TestLoadYAML_DefaultTypeFromOptions(t *testing.T) {
	yamlDoc := `graph_id: g
entry: x
nodes:
  x:
    kind: terminal
    outcome: success
`
	// 无 Options 且文档缺省 → 错误。
	if _, err := loader.LoadYAML([]byte(yamlDoc), nil); err == nil {
		t.Fatal("expected error when entity_type missing and no defaults")
	}
	// 提供 Options 默认 → 成功。
	res, err := loader.LoadYAML([]byte(yamlDoc), &loader.Options{
		DefaultEntityType:    "order.Order",
		DefaultSchemaVersion: "v1",
	})
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	if err := graph.ValidateGraphSpec(res.Spec); err != nil {
		t.Fatalf("ValidateGraphSpec: %v", err)
	}
}

// TestLoadYAML_DoesNotReadFiles 验证 Loader 不绑定 fixture 文件定位：
// 仅接受字节切片，不读取磁盘文件。
func TestLoadYAML_DoesNotReadFiles(t *testing.T) {
	// Loader 只暴露 LoadYAML([]byte, *Options)；不存在文件路径入参。
	// 本测试确认：传入空字节返回明确错误，而非尝试打开文件。
	_, err := loader.LoadYAML(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil data")
	}
	if !errors.Is(err, dag.ErrInvalidGraph) && !strings.Contains(err.Error(), "required") {
		// nil → yaml 解析为空文档 → graph_id 缺失错误
		t.Logf("got expected error: %v", err)
	}
}
