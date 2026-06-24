package invocation

import (
	"errors"
	"fmt"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// MarshalAny 将 protobuf message 编码为 *anypb.Any。
//
// 等价于 anypb.New，但作为 Codec 边界显式暴露，便于调用方统一编解码入口。
// 传入 nil message 返回错误，而不是 panic。
func MarshalAny(msg proto.Message) (*anypb.Any, error) {
	if msg == nil {
		return nil, errors.New("invocation: cannot marshal nil message")
	}
	return anypb.New(msg)
}

// UnmarshalSnapshot 将 EntitySnapshot.Payload 解码到目标 protobuf message。
//
// snapshot 为 nil 或其 Payload 为 nil 时返回错误；类型不匹配时透传 anypb 错误。
// 目标 message 不可为 nil。
func UnmarshalSnapshot(snapshot *dagv1.EntitySnapshot, dst proto.Message) error {
	if dst == nil {
		return errors.New("invocation: destination message must not be nil")
	}
	if snapshot == nil {
		return errors.New("invocation: snapshot is nil")
	}
	if snapshot.Payload == nil {
		return errors.New("invocation: snapshot payload is nil")
	}
	if err := snapshot.Payload.UnmarshalTo(dst); err != nil {
		return fmt.Errorf("invocation: decode payload to %T: %w", dst, err)
	}
	return nil
}

// MarshalString 将字符串封装为 google.protobuf.StringValue 再编码为 Any。
//
// 便于以 StringValue 作为 payload 的业务场景（lab echo、HTTP echo 等）。
func MarshalString(s string) (*anypb.Any, error) {
	return anypb.New(wrapperspb.String(s))
}

// UnmarshalString 将 snapshot payload 解码为 StringValue 并返回其字符串值。
//
// snapshot/payload 缺失或类型不匹配时返回错误。
func UnmarshalString(snapshot *dagv1.EntitySnapshot) (string, error) {
	var sv wrapperspb.StringValue
	if err := UnmarshalSnapshot(snapshot, &sv); err != nil {
		return "", err
	}
	return sv.GetValue(), nil
}

// UnmarshalAny 将 *anypb.Any 解码到目标 protobuf message。
//
// any 为 nil 时返回错误。用于调用方持有裸 Any（而非 snapshot）的场景。
func UnmarshalAny(any *anypb.Any, dst proto.Message) error {
	if dst == nil {
		return errors.New("invocation: destination message must not be nil")
	}
	if any == nil {
		return errors.New("invocation: any is nil")
	}
	if err := any.UnmarshalTo(dst); err != nil {
		return fmt.Errorf("invocation: decode any to %T: %w", dst, err)
	}
	return nil
}
