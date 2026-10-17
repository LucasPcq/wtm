// Package app is the API every surface calls: each use case run under its operation locks, its plan fingerprinted, its events published.
package app

import (
	"time"

	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/operation"
)

type Hold interface {
	Grow(locks []operation.Lock) (bool, error)
	Protected() bool
	Release()
}

type Deps struct {
	Create        create.Deps
	TakeLocks     func(op operation.Operation) (Hold, error)
	Publisher     func(op operation.Operation) wtm.Report
	NewOpID       func() string
	Now           func() time.Time
	CorrelationID string
}

type App struct {
	deps Deps
}

func New(deps Deps) *App {
	return &App{deps: deps}
}
