package dispatch

import (
	"context"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
)

// operationBus publishes an operation's events. They are sent even once the
// run is interrupted: a consumer must learn how the operation ended.
type operationBus struct {
	ctx       context.Context
	publisher Publisher
	operation kernel.Operation
}

func (d *Dispatcher) bus(ctx context.Context, operation kernel.Operation) operationBus {
	return operationBus{ctx: context.WithoutCancel(ctx), publisher: d.params.Publisher, operation: operation}
}

func (b operationBus) started() {
	b.publish(domain.Event{Type: domain.EventOperationStarted, Operation: b.described()})
}

func (b operationBus) unitFinished(progress kernel.Progress) {
	b.publish(domain.Event{Type: domain.EventUnitFinished, Unit: &domain.EventUnit{
		Subject: progress.Subject,
		Status:  progress.Params[kernel.ParamStatus],
	}})
}

// finished carries the code of the failure that ended the operation, if any;
// the items' own failures are in their unit.finished.
func (b operationBus) finished(failure kernel.Error) {
	ok := failure == nil
	event := domain.Event{Type: domain.EventOperationFinished, Operation: b.described(), OK: &ok}
	if failure != nil {
		event.Code = string(failure.Base().Code)
	}
	b.publish(event)
}

func (b operationBus) described() *domain.EventOperation {
	return &domain.EventOperation{Command: b.operation.Command, Surface: string(b.operation.Surface)}
}

func (b operationBus) publish(event domain.Event) {
	if b.publisher == nil {
		return
	}
	event.OpID = string(b.operation.ID)
	event.CorrelationID = b.operation.CorrelationID
	b.publisher.Publish(b.ctx, event)
}
