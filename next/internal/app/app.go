// Package app is the API every surface calls: each use case run under its operation locks, its plan fingerprinted, its events published.
package app

import (
	"time"

	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/operation"
)

type Hold interface {
	Grow(op operation.Operation) (bool, error)
	Protected() bool
	Release()
}

type Publisher interface {
	Publish(event wtm.Event)
}

type Deps struct {
	Create        create.Deps
	TakeLocks     func(op operation.Operation) (Hold, error)
	Publisher     func(op operation.Operation) Publisher
	NewOpID       func() string
	Now           func() time.Time
	Surface       string
	CorrelationID string
}

type App struct {
	deps Deps
}

func New(deps Deps) *App {
	deps.Create.Provision.Now = deps.Now
	return &App{deps: deps}
}
