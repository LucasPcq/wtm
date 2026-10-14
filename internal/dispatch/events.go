package dispatch

import (
	"context"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/domain/operation"
	"github.com/LucasPcq/wtm/internal/kernel"
)

type EventOf[F, M any] struct {
	Facts     F
	Milestone M
}

type Events[F, D any] struct {
	Committed func(EventOf[F, kernel.Committed[D]]) []domain.Event
	PhaseDone func(EventOf[F, kernel.PhaseDone[D]]) []domain.Event
	Finished  func(EventOf[F, kernel.Item[D]]) []domain.Event
}

type announceParams[F, M any] struct {
	Bus   operationBus
	Facts F
	Of    func(EventOf[F, M]) []domain.Event
}

func announce[F, M any](params announceParams[F, M]) func(M) {
	if params.Of == nil {
		return nil
	}
	return func(milestone M) {
		for _, event := range params.Of(EventOf[F, M]{Facts: params.Facts, Milestone: milestone}) {
			params.Bus.publish(event)
		}
	}
}

type milestonesParams[F, D any] struct {
	Bus    operationBus
	Facts  F
	Events Events[F, D]
}

func milestones[F, D any](params milestonesParams[F, D]) kernel.Milestones[D] {
	return kernel.Milestones[D]{
		Committed: announce(announceParams[F, kernel.Committed[D]]{Bus: params.Bus, Facts: params.Facts, Of: params.Events.Committed}),
		PhaseDone: announce(announceParams[F, kernel.PhaseDone[D]]{Bus: params.Bus, Facts: params.Facts, Of: params.Events.PhaseDone}),
		Finished:  announce(announceParams[F, kernel.Item[D]]{Bus: params.Bus, Facts: params.Facts, Of: params.Events.Finished}),
	}
}

type operationBus struct {
	ctx       context.Context
	publisher Publisher
	now       func() time.Time
	operation operation.Operation
}

// Without cancellation: a consumer must learn how an interrupted operation ended.
func (d *Dispatcher) bus(ctx context.Context, operation operation.Operation) operationBus {
	return operationBus{ctx: context.WithoutCancel(ctx), publisher: d.params.Publisher, now: d.params.Now, operation: operation}
}

func (b operationBus) started() {
	b.publish(domain.Event{Type: domain.EventOperationStarted, Operation: b.described()})
}

func (b operationBus) unitFinished(finished *kernel.UnitFinished) {
	b.publish(domain.Event{Type: domain.EventUnitFinished, Unit: &domain.EventUnit{
		Subject: finished.Subject,
		Status:  string(finished.Status),
	}})
}

func (b operationBus) finished(failure kernel.Error) {
	ok := failure == nil
	event := domain.Event{Type: domain.EventOperationFinished, Operation: b.described(), OK: &ok}
	if failure != nil {
		event.Code = string(failure.Base().Code)
	}
	b.publish(event)
}

func (b operationBus) described() *domain.EventOperation {
	return &domain.EventOperation{Command: b.operation.Command, Surface: b.operation.Surface}
}

func (b operationBus) publish(event domain.Event) {
	if b.publisher == nil {
		return
	}
	event.TS = b.now().UTC().Format(time.RFC3339Nano)
	event.OpID = b.operation.ID
	event.CorrelationID = b.operation.CorrelationID
	b.publisher.Publish(b.ctx, event)
}
