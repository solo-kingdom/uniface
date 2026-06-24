package entity

import (
	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// ShouldMergeSignalPayload 判断 WAIT 节点是否合并信号 payload（默认 true）。
func ShouldMergeSignalPayload(cfg *dagv1.WaitNodeConfig) bool {
	if cfg == nil || cfg.MergeSignalPayload == nil {
		return true
	}
	return cfg.GetMergeSignalPayload()
}

// MergeSignalPayload 将信号 payload 合并入 snapshot。
func MergeSignalPayload(snapshot *dagv1.EntitySnapshot, signalName string, payload *anypb.Any) (*dagv1.EntitySnapshot, error) {
	if snapshot == nil || payload == nil {
		return snapshot, nil
	}
	out := CloneSnapshot(snapshot)
	if out.Payload == nil {
		out.Payload = &anypb.Any{TypeUrl: payload.TypeUrl, Value: append([]byte(nil), payload.Value...)}
		return out, nil
	}
	if out.Payload.TypeUrl == payload.TypeUrl {
		base, err := anypb.UnmarshalNew(out.Payload, proto.UnmarshalOptions{DiscardUnknown: true})
		if err != nil {
			return nil, err
		}
		incoming, err := anypb.UnmarshalNew(payload, proto.UnmarshalOptions{DiscardUnknown: true})
		if err != nil {
			return nil, err
		}
		proto.Merge(base, incoming)
		merged, err := anypb.New(base)
		if err != nil {
			return nil, err
		}
		out.Payload = merged
		return out, nil
	}
	wrapper := &dagv1.SignalPayload{SignalName: signalName, Payload: payload}
	merged, err := anypb.New(wrapper)
	if err != nil {
		return nil, err
	}
	out.Payload = merged
	return out, nil
}
