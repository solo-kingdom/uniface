package units

import (
	"fmt"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag/entity"
	"github.com/solo-kingdom/uniface/pkg/dag/graph"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/anypb"
)

// mapResponse 按 ResponseMapping 将 2xx response body 转换为 EntityMutation。
//
// MODE_AUTO（默认）：
//   - response body 反序列化为 payload_type_url（空则复用输入 snapshot type_url）对应 proto message
//   - payload_field 非空时从反序列化结果取子字段作为 payload
//   - 包装为 Any，产出 mutation（on_success 覆盖：UPDATE 默认 / COMPLETE / FAIL）
//   - 反序列化失败 → mutation.fail{reason: "response decode failed: ..."}
//
// MODE_MUTATION：response body 视为 EntityMutation JSON，直接 apply。反序列化失败 → mutation.fail。
func (u *HttpUnit) mapResponse(body []byte, snapshot *dagv1.EntitySnapshot) (*dagv1.EntityMutation, error) {
	respCfg := u.config.GetResponse()
	mode := dagv1.ResponseMapping_MODE_AUTO
	if respCfg != nil {
		mode = respCfg.GetMode()
	}
	if mode == dagv1.ResponseMapping_MODE_MUTATION {
		return u.mapMutation(body)
	}
	return u.mapAuto(body, snapshot, respCfg)
}

// mapAuto 实现 MODE_AUTO 映射。
func (u *HttpUnit) mapAuto(body []byte, snapshot *dagv1.EntitySnapshot, respCfg *dagv1.ResponseMapping) (*dagv1.EntityMutation, error) {
	typeURL := ""
	payloadField := ""
	onSuccess := dagv1.TerminalOutcome_TERMINAL_OUTCOME_UNSPECIFIED
	if respCfg != nil {
		typeURL = respCfg.GetPayloadTypeUrl()
		payloadField = respCfg.GetPayloadField()
		onSuccess = respCfg.GetOnSuccess()
	}
	if typeURL == "" && snapshot != nil && snapshot.Payload != nil {
		typeURL = snapshot.Payload.TypeUrl
	}

	msg, err := decodeResponseAsMessage(body, typeURL)
	if err != nil {
		return failMutation(fmt.Sprintf("response decode failed: %v", err), false), nil
	}

	payload := msg
	if payloadField != "" {
		sub, err := projectField(msg, payloadField)
		if err != nil {
			return failMutation(fmt.Sprintf("response decode failed: %v", err), false), nil
		}
		payload = sub
	}

	// on_success 覆盖：COMPLETE / FAIL 终止。
	switch onSuccess {
	case dagv1.TerminalOutcome_TERMINAL_OUTCOME_SUCCESS:
		return &dagv1.EntityMutation{Intent: &dagv1.EntityMutation_Complete{Complete: dagv1.TerminalOutcome_TERMINAL_OUTCOME_SUCCESS}}, nil
	case dagv1.TerminalOutcome_TERMINAL_OUTCOME_FAILURE:
		return &dagv1.EntityMutation{Intent: &dagv1.EntityMutation_Complete{Complete: dagv1.TerminalOutcome_TERMINAL_OUTCOME_FAILURE}}, nil
	}

	// 默认：update（payload 包装为 Any）。
	anyPayload, err := anypb.New(payload)
	if err != nil {
		return failMutation(fmt.Sprintf("response decode failed: pack any: %v", err), false), nil
	}
	outSnap := entity.NewSnapshot(snapshot.Ref, snapshot.TypeKey, snapshot.Sequence+1, anyPayload)
	return &dagv1.EntityMutation{Intent: &dagv1.EntityMutation_Update{Update: outSnap}}, nil
}

// mapMutation 实现 MODE_MUTATION 映射。
func (u *HttpUnit) mapMutation(body []byte) (*dagv1.EntityMutation, error) {
	if len(body) == 0 {
		return failMutation("response decode failed: empty mutation body", false), nil
	}
	mutation := &dagv1.EntityMutation{}
	opts := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := opts.Unmarshal(body, mutation); err != nil {
		return failMutation(fmt.Sprintf("response decode failed: %v", err), false), nil
	}
	if mutation.Intent == nil {
		return failMutation("response decode failed: empty mutation intent", false), nil
	}
	return mutation, nil
}

// decodeResponseAsMessage 按 typeURL 从全局类型表查 proto message，用 protojson 反序列化 body。
func decodeResponseAsMessage(body []byte, typeURL string) (proto.Message, error) {
	mt, err := resolveMessageType(typeURL)
	if err != nil {
		return nil, err
	}
	msg := mt.New().Interface()
	if len(body) == 0 {
		return msg, nil
	}
	opts := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := opts.Unmarshal(body, msg); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	return msg, nil
}

// resolveMessageType 从全局类型表按 type_url 解析 message 类型（protoregistry 自行剥离 type host 前缀）。
func resolveMessageType(typeURL string) (protoreflect.MessageType, error) {
	if typeURL == "" {
		return nil, fmt.Errorf("empty type url")
	}
	mt, err := protoregistry.GlobalTypes.FindMessageByURL(typeURL)
	if err != nil {
		return nil, fmt.Errorf("resolve type %q: %w", typeURL, err)
	}
	return mt, nil
}

// projectField 从反序列化结果按 payload_field 取子字段作为新 payload。
func projectField(msg proto.Message, fieldPath string) (proto.Message, error) {
	field, err := graph.ResolveFieldPath(msg, fieldPath)
	if err != nil {
		return nil, err
	}
	if pm, ok := field.Interface().(proto.Message); ok {
		return pm, nil
	}
	return nil, fmt.Errorf("payload field %q is not a message", fieldPath)
}
