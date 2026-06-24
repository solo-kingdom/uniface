package daghttp

import (
	"context"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"github.com/solo-kingdom/uniface/pkg/dag/entity"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type helloUnit struct{}

func (u *helloUnit) Execute(_ context.Context, snapshot *dagv1.EntitySnapshot) (*dagv1.EntityMutation, error) {
	msg := readString(snapshot)
	out := wrapperspb.String("hello, " + msg)
	payload, _ := anypb.New(out)
	return &dagv1.EntityMutation{
		Intent: &dagv1.EntityMutation_Update{
			Update: entity.NewSnapshot(snapshot.Ref, snapshot.TypeKey, snapshot.Sequence+1, payload),
		},
	}, nil
}

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

var (
	_ dag.ComputeUnit = (*helloUnit)(nil)
	_ dag.ComputeUnit = (*echoUnit)(nil)
)
