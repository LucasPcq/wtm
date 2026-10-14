// Package operations is the query of the operations in progress.
package operations

import (
	"context"

	"github.com/LucasPcq/wtm/internal/domain/operation"
	"github.com/LucasPcq/wtm/internal/kernel"
)

const Name = "operations"

type Request struct{}

type View struct {
	Operations []operation.Operation `json:"operations"`
}

type List func(context.Context) ([]operation.Operation, error)

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
