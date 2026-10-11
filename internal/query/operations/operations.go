// Package operations is the query of the operations in progress.
package operations

import (
	"context"
	"time"

	"github.com/LucasPcq/wtm/internal/kernel"
)

const Name = "operations"

// Operation is one run in progress, as dispatch records it in its holder file.
type Operation struct {
	ID            kernel.OpID      `json:"op_id"`
	PID           int              `json:"pid"`
	Command       string           `json:"command"`
	Surface       string           `json:"surface"`
	CorrelationID string           `json:"correlation_id,omitempty"`
	StartedAt     time.Time        `json:"started_at"`
	Locks         []kernel.LockKey `json:"locks"`
}

type Request struct{}

type View struct {
	Operations []Operation `json:"operations"`
}

type List func(context.Context) ([]Operation, error)

func Query(list List) kernel.Query[Request, View] {
	return kernel.Query[Request, View]{
		Name: Name,
		Run: func(ctx context.Context, _ Request) (View, error) {
			operations, err := list(ctx)
			if err != nil {
				return View{}, err
			}
			return View{Operations: operations}, nil
		},
	}
}
