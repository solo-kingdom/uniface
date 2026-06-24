package memory

import (
	"context"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag"
)

const (
	defaultDrainFactor      = 4
	defaultDrainAbsoluteMax = 1000
	defaultDrainMinHops     = 1
	drainFallbackMaxHops    = 100
)

// DrainInstance 循环 RunOnce 直至实例终态、WAITING、ctx 取消或 hop 上限耗尽。
func (e *Engine) DrainInstance(ctx context.Context, ref *dagv1.EntityRef, opts ...dag.Option) (*dagv1.EntityInstance, error) {
	if e.closed {
		return nil, dag.ErrStoreClosed
	}
	if ref == nil || ref.EntityId == "" {
		return nil, dag.ErrInstanceNotFound
	}

	o := e.drainOptions(opts...)

	maxHops := o.DrainMaxHops
	if maxHops <= 0 {
		maxHops = e.deriveDrainMaxHops(ctx, ref, o)
	}

	hops := 0
	for {
		inst, err := e.store.GetInstance(ctx, ref)
		if err != nil {
			return nil, err
		}
		if drainDone(inst.Status) {
			return inst, nil
		}

		if err := ctx.Err(); err != nil {
			return inst, err
		}

		if hops >= maxHops {
			return inst, dag.NewDAGError("DrainInstance", ref, dag.ErrDrainExceeded)
		}

		if err := e.RunOnce(ctx); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				if latest, getErr := e.store.GetInstance(ctx, ref); getErr == nil {
					inst = latest
				}
				return inst, ctxErr
			}
			return inst, err
		}
		hops++
	}
}

func drainDone(status dagv1.InstanceStatus) bool {
	switch status {
	case dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED,
		dagv1.InstanceStatus_INSTANCE_STATUS_FAILED,
		dagv1.InstanceStatus_INSTANCE_STATUS_COMPENSATED,
		dagv1.InstanceStatus_INSTANCE_STATUS_CANCELLED,
		dagv1.InstanceStatus_INSTANCE_STATUS_WAITING:
		return true
	default:
		return false
	}
}

func (e *Engine) deriveDrainMaxHops(ctx context.Context, ref *dagv1.EntityRef, o *dag.Options) int {
	absoluteMax := o.DrainAbsoluteMax
	if absoluteMax <= 0 {
		absoluteMax = defaultDrainAbsoluteMax
	}
	factor := o.DrainFactor
	if factor <= 0 {
		factor = defaultDrainFactor
	}
	minHops := o.DrainMinHops
	if minHops <= 0 {
		minHops = defaultDrainMinHops
	}

	fallback := drainFallbackMaxHops
	if fallback > absoluteMax {
		fallback = absoluteMax
	}

	inst, err := e.store.GetInstance(ctx, ref)
	if err != nil {
		return fallback
	}
	spec, err := e.reg.ResolveGraphForInstance(inst)
	if err != nil {
		return fallback
	}
	nodeCount := len(spec.Nodes)
	if nodeCount <= 0 {
		return fallback
	}

	derived := nodeCount * factor
	if derived < minHops {
		derived = minHops
	}
	if derived > absoluteMax {
		derived = absoluteMax
	}
	return derived
}

func (e *Engine) drainOptions(opts ...dag.Option) *dag.Options {
	base := dag.DefaultOptions()
	if e.opts != nil {
		*base = *e.opts
	}
	return base.Apply(opts...)
}
