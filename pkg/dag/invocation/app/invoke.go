package app

import (
	"context"
	"errors"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag/invocation"
	"google.golang.org/protobuf/proto"
)

// StringCall 描述一次 string payload 请求式调用。
type StringCall struct {
	GraphID      string
	GraphVersion string
	EntityID     string
	Payload      string
	TypeKey      *dagv1.EntityTypeKey
}

// MessageCall 描述一次 protobuf message payload 请求式调用。
type MessageCall struct {
	GraphID      string
	GraphVersion string
	EntityID     string
	Input        proto.Message
	TypeKey      *dagv1.EntityTypeKey
}

// CallResult 封装底层 InvokeResult 的实例与 snapshot 信息。
type CallResult struct {
	Instance *dagv1.EntityInstance
	Snapshot *dagv1.EntitySnapshot
}

// Status 返回实例状态；Instance 为 nil 时返回 UNSPECIFIED。
func (r *CallResult) Status() dagv1.InstanceStatus {
	if r == nil || r.Instance == nil {
		return dagv1.InstanceStatus_INSTANCE_STATUS_UNSPECIFIED
	}
	return r.Instance.Status
}

// IsCompleted 报告实例是否处于 COMPLETED 终态。
func (r *CallResult) IsCompleted() bool {
	return r.Status() == dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED
}

// IsWaiting 报告实例是否处于 WAITING（同步调用不会继续等待外部 signal）。
func (r *CallResult) IsWaiting() bool {
	return r.Status() == dagv1.InstanceStatus_INSTANCE_STATUS_WAITING
}

// IsTerminal 报告实例是否处于终态。
func (r *CallResult) IsTerminal() bool {
	return invocation.IsTerminal(r.Status())
}

// TerminalErr 在失败终态时返回错误；COMPLETED 与 WAITING 等非失败状态返回 nil。
func (r *CallResult) TerminalErr() error {
	if r == nil {
		return errors.New("app: nil call result")
	}
	return invocation.TerminalError(r.Instance)
}

// StringCallResult 是 string payload 调用的结果；Value 仅在 COMPLETED 且解码成功时设置。
type StringCallResult struct {
	CallResult
	Value string
}

// InvokeString 发起 string payload 请求式调用，复用底层 Invoker 与 StringValue codec。
//
// Drain 错误透传；FAILED/COMPENSATED/CANCELLED 终态不填充 Value；WAITING 显式暴露且不继续等待。
func (rt *Runtime) InvokeString(ctx context.Context, call *StringCall) (*StringCallResult, error) {
	req, err := buildStringInvokeRequest(call)
	if err != nil {
		return nil, err
	}
	raw, err := rt.rt.Invoker().Invoke(ctx, req)
	cr := callResultFrom(raw)
	result := &StringCallResult{CallResult: *cr}
	if raw != nil && result.IsCompleted() && raw.Snapshot != nil {
		result.Value, _ = invocation.UnmarshalString(raw.Snapshot)
	}
	return result, err
}

// InvokeMessage 发起 protobuf message 请求式调用，完成后将终态 payload 解码到 out。
//
// out 不可为 nil；仅在 COMPLETED 时尝试解码；失败终态与 WAITING 语义同 InvokeString。
func (rt *Runtime) InvokeMessage(ctx context.Context, call *MessageCall, out proto.Message) (*CallResult, error) {
	if out == nil {
		return nil, errors.New("app: output message must not be nil")
	}
	req, err := buildMessageInvokeRequest(call)
	if err != nil {
		return nil, err
	}
	raw, err := rt.rt.Invoker().Invoke(ctx, req)
	result := callResultFrom(raw)
	if raw != nil && result.IsCompleted() && raw.Snapshot != nil {
		if decErr := invocation.UnmarshalSnapshot(raw.Snapshot, out); decErr != nil {
			return result, decErr
		}
	}
	return result, err
}

func buildStringInvokeRequest(call *StringCall) (*invocation.InvokeRequest, error) {
	if call == nil || call.EntityID == "" {
		return nil, errors.New("app: StringCall.EntityID is required")
	}
	if call.TypeKey == nil {
		return nil, errors.New("app: StringCall.TypeKey is required")
	}
	if call.GraphID == "" {
		return nil, errors.New("app: StringCall.GraphID is required")
	}
	version := call.GraphVersion
	if version == "" {
		version = "v1"
	}
	req := &invocation.InvokeRequest{
		Ref:            &dagv1.EntityRef{EntityId: call.EntityID},
		TypeKey:        call.TypeKey,
		GraphVersion:   &dagv1.GraphVersion{GraphId: call.GraphID, Version: version},
		GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_START,
	}
	if call.Payload != "" {
		payload, err := invocation.MarshalString(call.Payload)
		if err != nil {
			return nil, err
		}
		req.InitialPayload = payload
	}
	return req, nil
}

func buildMessageInvokeRequest(call *MessageCall) (*invocation.InvokeRequest, error) {
	if call == nil || call.EntityID == "" {
		return nil, errors.New("app: MessageCall.EntityID is required")
	}
	if call.TypeKey == nil {
		return nil, errors.New("app: MessageCall.TypeKey is required")
	}
	if call.GraphID == "" {
		return nil, errors.New("app: MessageCall.GraphID is required")
	}
	version := call.GraphVersion
	if version == "" {
		version = "v1"
	}
	req := &invocation.InvokeRequest{
		Ref:            &dagv1.EntityRef{EntityId: call.EntityID},
		TypeKey:        call.TypeKey,
		GraphVersion:   &dagv1.GraphVersion{GraphId: call.GraphID, Version: version},
		GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_START,
	}
	if call.Input != nil {
		payload, err := invocation.MarshalAny(call.Input)
		if err != nil {
			return nil, err
		}
		req.InitialPayload = payload
	}
	return req, nil
}

func callResultFrom(raw *invocation.InvokeResult) *CallResult {
	if raw == nil {
		return &CallResult{}
	}
	return &CallResult{Instance: raw.Instance, Snapshot: raw.Snapshot}
}
