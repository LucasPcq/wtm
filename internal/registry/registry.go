// Package registry is the composition root of the engine: every command and query the surfaces serve, with its real ports.
package registry

import (
	"time"

	"github.com/LucasPcq/wtm/internal/command/create"
	"github.com/LucasPcq/wtm/internal/command/provision"
	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/query/operations"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/store"
)

func Production(d *dispatch.Dispatcher) dispatch.Registry {
	return dispatch.Registry{
		dispatch.Register(dispatch.RegisterParams[create.Request, create.Facts, create.Plan, provision.Detail]{
			Dispatcher: d,
			Command: create.Command(create.Ports{
				Git:        git.Repo{Dir: d.Dir(), Remote: domain.OriginRemote},
				Meta:       store.MetaDisk{StateDir: d.StateDir()},
				Config:     store.ConfigDisk{StateDir: d.StateDir()},
				Env:        store.EnvDisk{},
				Dir:        d.Dir(),
				StateDir:   d.StateDir(),
				PublicPort: process.PublicProxyPort,
				Now:        time.Now,
			}),
			Messages:  create.Messages,
			ExitCodes: create.ExitCodes,
			Events:    createEvents(),
		}),
		dispatch.RegisterQuery(dispatch.RegisterQueryParams[operations.Request, operations.View]{Query: operations.Query(d.Operations)}),
	}
}

func createEvents() dispatch.Events[create.Facts, provision.Detail] {
	return dispatch.Events[create.Facts, provision.Detail]{
		Committed: func(e dispatch.EventOf[create.Facts, kernel.Committed[provision.Detail]]) []domain.Event {
			return create.Created(e.Facts, e.Milestone)
		},
		PhaseDone: func(e dispatch.EventOf[create.Facts, kernel.PhaseDone[provision.Detail]]) []domain.Event {
			return create.Updated(e.Facts, e.Milestone)
		},
		Finished: func(e dispatch.EventOf[create.Facts, kernel.Item[provision.Detail]]) []domain.Event {
			return create.Provisioned(e.Facts, e.Milestone)
		},
	}
}
