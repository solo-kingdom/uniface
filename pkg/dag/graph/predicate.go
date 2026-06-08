package graph

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// EvalSignalPredicate 对信号上下文与 snapshot 求值 SignalPredicate。
func EvalSignalPredicate(pred *dagv1.SignalPredicate, snapshot *dagv1.EntitySnapshot, sigCtx *SignalContext) (bool, error) {
	if pred == nil || sigCtx == nil {
		return false, nil
	}
	if pred.SignalName != "" && pred.SignalName != sigCtx.SignalName {
		return false, nil
	}
	if pred.PayloadPredicate == nil {
		return true, nil
	}
	return EvalFieldPredicate(pred.PayloadPredicate, snapshot)
}

// EvalFieldPredicate 对 snapshot payload 求值 FieldPredicate。
func EvalFieldPredicate(pred *dagv1.FieldPredicate, snapshot *dagv1.EntitySnapshot) (bool, error) {
	if pred == nil || snapshot == nil || snapshot.Payload == nil {
		return false, nil
	}
	msg, err := anypb.UnmarshalNew(snapshot.Payload, proto.UnmarshalOptions{DiscardUnknown: true})
	if err != nil {
		return false, err
	}
	val, err := resolveFieldPath(msg, pred.FieldPath)
	if err != nil {
		return false, err
	}
	return compareValue(val, pred.Op, pred.Value)
}

// EvalFieldPredicateWithMessage 使用已知 message 类型求值。
func EvalFieldPredicateWithMessage(pred *dagv1.FieldPredicate, snapshot *dagv1.EntitySnapshot, msg proto.Message) (bool, error) {
	if pred == nil {
		return false, nil
	}
	if msg == nil {
		return EvalFieldPredicate(pred, snapshot)
	}
	val, err := resolveFieldPath(msg, pred.FieldPath)
	if err != nil {
		return false, err
	}
	return compareValue(val, pred.Op, pred.Value)
}

func resolveFieldPath(msg proto.Message, path string) (reflect.Value, error) {
	if path == "" {
		return reflect.Value{}, fmt.Errorf("empty field path")
	}
	parts := strings.Split(path, ".")
	cur := reflect.ValueOf(msg)
	if cur.Kind() == reflect.Ptr {
		cur = cur.Elem()
	}
	for _, part := range parts {
		name, idx, hasIdx := parseIndexed(part)
		field := cur.FieldByName(name)
		if !field.IsValid() {
			return reflect.Value{}, fmt.Errorf("field %q not found", part)
		}
		if hasIdx {
			if field.Kind() != reflect.Slice && field.Kind() != reflect.Array {
				return reflect.Value{}, fmt.Errorf("field %q is not repeated", name)
			}
			if idx < 0 || idx >= field.Len() {
				return reflect.Value{}, fmt.Errorf("index out of range for %q", part)
			}
			field = field.Index(idx)
		}
		cur = field
	}
	return cur, nil
}

func parseIndexed(part string) (string, int, bool) {
	i := strings.Index(part, "[")
	if i < 0 {
		return part, 0, false
	}
	if !strings.HasSuffix(part, "]") {
		return part, 0, false
	}
	name := part[:i]
	idxStr := part[i+1 : len(part)-1]
	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		return part, 0, false
	}
	return name, idx, true
}

func compareValue(val reflect.Value, op dagv1.CompareOp, expected string) (bool, error) {
	s := valueToString(val)
	switch op {
	case dagv1.CompareOp_COMPARE_OP_EQ:
		return s == expected, nil
	case dagv1.CompareOp_COMPARE_OP_NE:
		return s != expected, nil
	case dagv1.CompareOp_COMPARE_OP_GT, dagv1.CompareOp_COMPARE_OP_GTE, dagv1.CompareOp_COMPARE_OP_LT, dagv1.CompareOp_COMPARE_OP_LTE:
		lhs, err1 := strconv.ParseFloat(s, 64)
		rhs, err2 := strconv.ParseFloat(expected, 64)
		if err1 != nil || err2 != nil {
			return false, nil
		}
		switch op {
		case dagv1.CompareOp_COMPARE_OP_GT:
			return lhs > rhs, nil
		case dagv1.CompareOp_COMPARE_OP_GTE:
			return lhs >= rhs, nil
		case dagv1.CompareOp_COMPARE_OP_LT:
			return lhs < rhs, nil
		case dagv1.CompareOp_COMPARE_OP_LTE:
			return lhs <= rhs, nil
		}
	}
	return false, nil
}

func valueToString(v reflect.Value) string {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64)
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	case reflect.String:
		return v.String()
	default:
		return fmt.Sprint(v.Interface())
	}
}
