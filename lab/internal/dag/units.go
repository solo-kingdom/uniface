package dag

import (
	"context"
	"sync/atomic"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"github.com/solo-kingdom/uniface/pkg/dag/entity"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type echoUnit struct{}

func (u *echoUnit) Execute(_ context.Context, snapshot *dagv1.EntitySnapshot) (*dagv1.EntityMutation, error) {
	msg := readString(snapshot)
	out := wrapperspb.String("echo:" + msg)
	payload, _ := anypb.New(out)
	return &dagv1.EntityMutation{
		Intent: &dagv1.EntityMutation_Update{
			Update: entity.NewSnapshot(snapshot.Ref, snapshot.TypeKey, snapshot.Sequence+1, payload),
		},
	}, nil
}

type failOnceUnit struct {
	failed atomic.Bool
}

func (u *failOnceUnit) Execute(_ context.Context, snapshot *dagv1.EntitySnapshot) (*dagv1.EntityMutation, error) {
	if u.failed.CompareAndSwap(false, true) {
		return &dagv1.EntityMutation{Intent: &dagv1.EntityMutation_Fail{Fail: &dagv1.FailIntent{
			Reason:              "fail_once first attempt",
			TriggerCompensation: true,
		}}}, nil
	}
	out := wrapperspb.String("recovered")
	payload, _ := anypb.New(out)
	return &dagv1.EntityMutation{
		Intent: &dagv1.EntityMutation_Update{
			Update: entity.NewSnapshot(snapshot.Ref, snapshot.TypeKey, snapshot.Sequence+1, payload),
		},
	}, nil
}

type rollbackCompensator struct {
	count atomic.Int32
}

func (c *rollbackCompensator) Compensate(_ context.Context, _ *dagv1.CompensationContext) error {
	c.count.Add(1)
	return nil
}

func readString(snapshot *dagv1.EntitySnapshot) string {
	if snapshot == nil || snapshot.Payload == nil {
		return ""
	}
	var s wrapperspb.StringValue
	if err := snapshot.Payload.UnmarshalTo(&s); err != nil {
		return string(snapshot.Payload.Value)
	}
	return s.GetValue()
}

// Ensure interfaces compile.
var (
	_ dag.ComputeUnit = (*echoUnit)(nil)
	_ dag.ComputeUnit = (*failOnceUnit)(nil)
	_ dag.Compensator = (*rollbackCompensator)(nil)
)
