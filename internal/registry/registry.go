// Package registry is the composition root of the engine: every command and query the surfaces serve, with its real ports.
package registry

import (
	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/query/operations"
)

func Production(d *dispatch.Dispatcher) dispatch.Registry {
	return dispatch.Registry{
		dispatch.RegisterQuery(dispatch.RegisterQueryParams[operations.Request, operations.View]{Query: operations.Query(d.Operations)}),
	}
}
