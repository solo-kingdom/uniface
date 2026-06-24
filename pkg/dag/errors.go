package dag

import (
	"errors"
	"fmt"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
)

var (
	ErrInvalidEntityType      = errors.New("invalid entity type")
	ErrInstanceAlreadyExists  = errors.New("instance already exists")
	ErrInstanceNotFound       = errors.New("instance not found")
	ErrInvalidSpawn           = errors.New("invalid spawn spec")
	ErrIncompatibleSchema     = errors.New("incompatible schema")
	ErrInvalidGraph           = errors.New("invalid graph")
	ErrNoTransition           = errors.New("no matching transition")
	ErrTypeMismatch           = errors.New("type mismatch")
	ErrUnsupportedSideEffect  = errors.New("unsupported side effect class")
	ErrSignalMismatch         = errors.New("signal name mismatch")
	ErrInstanceCancelled      = errors.New("instance cancelled")
	ErrExecutionAlreadyExists = errors.New("execution already committed")
	ErrStoreClosed            = errors.New("store closed")
)

// DAGError 带上下文的 DAG 错误。
type DAGError struct {
	Op  string
	Ref *dagv1.EntityRef
	Err error
}

func (e *DAGError) Error() string {
	if e.Ref != nil && e.Ref.EntityId != "" {
		return fmt.Sprintf("dag %s entity=%q: %v", e.Op, e.Ref.EntityId, e.Err)
	}
	return fmt.Sprintf("dag %s: %v", e.Op, e.Err)
}

func (e *DAGError) Unwrap() error {
	return e.Err
}

// NewDAGError 创建 DAGError。
func NewDAGError(op string, ref *dagv1.EntityRef, err error) error {
	return &DAGError{Op: op, Ref: ref, Err: err}
}
