package graph

import (
	"context"
	"fmt"
	"sort"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
)

// ValidateGraphSpec 静态校验图规格。
func ValidateGraphSpec(spec *dagv1.GraphSpec) error {
	if spec == nil || spec.EntryNodeId == "" || len(spec.Nodes) == 0 {
		return dag.ErrInvalidGraph
	}
	if _, ok := spec.Nodes[spec.EntryNodeId]; !ok {
		return fmt.Errorf("%w: entry node %q not found", dag.ErrInvalidGraph, spec.EntryNodeId)
	}
	for id, node := range spec.Nodes {
		if node.NodeId == "" {
			node.NodeId = id
		}
		if node.NodeId != id {
			return fmt.Errorf("%w: node id mismatch %q vs %q", dag.ErrInvalidGraph, id, node.NodeId)
		}
		for _, tr := range node.Transitions {
			if tr.TargetNodeId == "" {
				return fmt.Errorf("%w: empty target on node %q", dag.ErrInvalidGraph, id)
			}
			if _, ok := spec.Nodes[tr.TargetNodeId]; !ok {
				return fmt.Errorf("%w: target %q not found from %q", dag.ErrInvalidGraph, tr.TargetNodeId, id)
			}
		}
		switch node.Kind {
		case dagv1.NodeKind_NODE_KIND_TERMINAL:
			if len(node.Transitions) > 0 {
				return fmt.Errorf("%w: terminal node %q has transitions", dag.ErrInvalidGraph, id)
			}
			if node.TerminalOutcome == dagv1.TerminalOutcome_TERMINAL_OUTCOME_UNSPECIFIED {
				return fmt.Errorf("%w: terminal node %q missing outcome", dag.ErrInvalidGraph, id)
			}
		case dagv1.NodeKind_NODE_KIND_JOIN:
			if node.JoinSpec == nil || len(node.JoinSpec.Barriers) == 0 {
				return fmt.Errorf("%w: join node %q missing barriers", dag.ErrInvalidGraph, id)
			}
		}
	}
	if !reachableTerminal(spec, spec.EntryNodeId, map[string]bool{}) {
		return fmt.Errorf("%w: no path to terminal from entry", dag.ErrInvalidGraph)
	}
	return nil
}

func reachableTerminal(spec *dagv1.GraphSpec, nodeID string, visiting map[string]bool) bool {
	node, ok := spec.Nodes[nodeID]
	if !ok {
		return false
	}
	if node.Kind == dagv1.NodeKind_NODE_KIND_TERMINAL {
		return true
	}
	if visiting[nodeID] {
		return false
	}
	visiting[nodeID] = true
	for _, tr := range node.Transitions {
		if reachableTerminal(spec, tr.TargetNodeId, visiting) {
			return true
		}
	}
	if node.Kind == dagv1.NodeKind_NODE_KIND_WAIT && node.WaitConfig != nil && node.WaitConfig.OnTimeoutTargetNodeId != "" {
		if reachableTerminal(spec, node.WaitConfig.OnTimeoutTargetNodeId, visiting) {
			return true
		}
	}
	return false
}

// ResolveGraphVersion 根据 pin policy 选择图版本。
func ResolveGraphVersion(inst *dagv1.EntityInstance, latest *dagv1.GraphVersion) *dagv1.GraphVersion {
	if inst == nil {
		return latest
	}
	switch inst.GraphPinPolicy {
	case dagv1.GraphPinPolicy_GRAPH_PIN_ON_START, dagv1.GraphPinPolicy_GRAPH_PIN_POLICY_UNSPECIFIED:
		return inst.GraphVersion
	default:
		return latest
	}
}

// Resolver 默认 GraphResolver 实现。
type Resolver struct{}

func NewResolver() *Resolver {
	return &Resolver{}
}

func (r *Resolver) Resolve(ctx context.Context, graph *dagv1.GraphSpec, nodeID string, snapshot *dagv1.EntitySnapshot) (string, error) {
	node, ok := graph.Nodes[nodeID]
	if !ok {
		return "", fmt.Errorf("%w: node %q not found", dag.ErrInvalidGraph, nodeID)
	}
	switch node.Kind {
	case dagv1.NodeKind_NODE_KIND_TERMINAL:
		return "", nil
	case dagv1.NodeKind_NODE_KIND_WAIT, dagv1.NodeKind_NODE_KIND_JOIN:
		return nodeID, nil
	}
	return ResolveTransitions(node, snapshot)
}

// ResolveTransitions 按 priority 评估节点出边。
func ResolveTransitions(node *dagv1.NodeDef, snapshot *dagv1.EntitySnapshot) (string, error) {
	if node == nil {
		return "", dag.ErrInvalidGraph
	}
	transitions := append([]*dagv1.Transition(nil), node.Transitions...)
	sort.SliceStable(transitions, func(i, j int) bool {
		return transitions[i].Priority > transitions[j].Priority
	})
	for _, tr := range transitions {
		if tr.Condition == nil {
			continue
		}
		match, err := evalCondition(tr.Condition, snapshot)
		if err != nil {
			return "", err
		}
		if match {
			return tr.TargetNodeId, nil
		}
	}
	return "", dag.ErrNoTransition
}

func evalCondition(cond *dagv1.Condition, snapshot *dagv1.EntitySnapshot) (bool, error) {
	if cond == nil {
		return false, nil
	}
	switch k := cond.Kind.(type) {
	case *dagv1.Condition_Always:
		return k.Always, nil
	case *dagv1.Condition_FieldPredicate:
		return EvalFieldPredicate(k.FieldPredicate, snapshot)
	default:
		return false, nil
	}
}

// GetNode 获取节点定义。
func GetNode(spec *dagv1.GraphSpec, nodeID string) (*dagv1.NodeDef, bool) {
	if spec == nil {
		return nil, false
	}
	n, ok := spec.Nodes[nodeID]
	return n, ok
}
