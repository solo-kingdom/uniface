package units

import (
	"encoding/json"
	"fmt"
	"reflect"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag/graph"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// unmarshalPayload 将 snapshot.payload (Any) 反序列化为 proto.Message。
func unmarshalPayload(any *anypb.Any) (proto.Message, error) {
	if any == nil {
		return nil, fmt.Errorf("nil payload")
	}
	msg, err := anypb.UnmarshalNew(any, proto.UnmarshalOptions{DiscardUnknown: true})
	if err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}
	return msg, nil
}

// buildBody 按 BodyTemplate 构造 request body（JSON 字节）。
//   - Level 0（field_path 为空，默认）：整个 snapshot.payload（Any）按 protojson 序列化为 JSON body
//   - Level 1（field_path 非空）：取 snapshot.payload.<field_path> 子字段，序列化为 JSON
//
// snapshot.Payload 为 nil 时返回 nil body（不发请求体）。
// field_path 字段不存在时返回错误（Execute 会将其转为 mutation.fail）。
func (u *HttpUnit) buildBody(snapshot *dagv1.EntitySnapshot) ([]byte, error) {
	if snapshot == nil || snapshot.Payload == nil {
		return nil, nil
	}
	tmpl := u.config.GetRequestBody()
	fieldPath := ""
	if tmpl != nil {
		fieldPath = tmpl.GetFieldPath()
	}
	msg, err := unmarshalPayload(snapshot.Payload)
	if err != nil {
		return nil, err
	}
	if fieldPath == "" {
		// Level 0：整包 payload → protojson。
		return marshalProtoJSON(msg)
	}
	// Level 1：取子字段。
	field, err := graph.ResolveFieldPath(msg, fieldPath)
	if err != nil {
		return nil, fmt.Errorf("field path %q: %w", fieldPath, err)
	}
	return marshalFieldValue(field)
}

// marshalFieldValue 将 reflect.Value 序列化为 JSON。
// 若字段是 proto.Message 则用 protojson；否则用 encoding/json（标量场景）。
func marshalFieldValue(field reflect.Value) ([]byte, error) {
	if !field.IsValid() {
		return nil, fmt.Errorf("field value invalid")
	}
	if field.Kind() == reflect.Ptr && field.IsNil() {
		return []byte("null"), nil
	}
	iface := field.Interface()
	if pm, ok := iface.(proto.Message); ok {
		return marshalProtoJSON(pm)
	}
	out, err := json.Marshal(iface)
	if err != nil {
		return nil, fmt.Errorf("marshal scalar field: %w", err)
	}
	return out, nil
}

func marshalProtoJSON(msg proto.Message) ([]byte, error) {
	out, err := protojson.MarshalOptions{EmitUnpopulated: false}.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("protojson marshal: %w", err)
	}
	return out, nil
}
