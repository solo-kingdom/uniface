package memory

import (
	"testing"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag/graph"
)

func TestResolveGraphForInstancePinOnNode(t *testing.T) {
	reg := NewRegistry()
	v1 := goldenGraphSpec()
	v2 := goldenGraphSpec()
	v2.Version = &dagv1.GraphVersion{GraphId: graphID, Version: "v2"}
	v2.Nodes["validate"].Transitions[0].TargetNodeId = "term_failure"

	if err := reg.RegisterGraph(v1); err != nil {
		t.Fatal(err)
	}
	if err := reg.RegisterGraph(v2); err != nil {
		t.Fatal(err)
	}

	latest, err := reg.GetLatestGraphVersion(graphID)
	if err != nil {
		t.Fatal(err)
	}
	if latest.Version != "v2" {
		t.Fatalf("expected latest v2, got %s", latest.Version)
	}

	pinStart := &dagv1.EntityInstance{
		GraphVersion:   &dagv1.GraphVersion{GraphId: graphID, Version: graphVersion},
		GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_START,
	}
	spec, err := reg.ResolveGraphForInstance(pinStart)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Version.Version != graphVersion {
		t.Fatalf("PIN_ON_START should use v1, got %s", spec.Version.Version)
	}

	pinNode := &dagv1.EntityInstance{
		GraphVersion:   &dagv1.GraphVersion{GraphId: graphID, Version: graphVersion},
		GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_NODE,
	}
	spec, err = reg.ResolveGraphForInstance(pinNode)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Version.Version != "v2" {
		t.Fatalf("PIN_ON_NODE should use v2, got %s", spec.Version.Version)
	}
}

func TestResolveGraphVersionOnNode(t *testing.T) {
	inst := &dagv1.EntityInstance{
		GraphVersion:   &dagv1.GraphVersion{GraphId: graphID, Version: "v1"},
		GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_NODE,
	}
	latest := &dagv1.GraphVersion{GraphId: graphID, Version: "v2"}
	got := graph.ResolveGraphVersion(inst, latest)
	if got.Version != "v2" {
		t.Fatalf("expected v2, got %s", got.Version)
	}
}
