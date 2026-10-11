// Package operations is the query of the operations in progress: who runs
// what, from which surface, since when, holding which locks.
package operations

import (
	"context"

	"github.com/LucasPcq/wtm/internal/kernel"
)

const Name = "operations"

type Request struct{}

type View struct {
	Operations []kernel.Operation `json:"operations"`
}

// List reads the operations in progress; dispatch provides it.
type List func(context.Context) ([]kernel.Operation, error)

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
