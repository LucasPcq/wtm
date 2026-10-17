// Package app is the API every surface calls: each use case run under its operation locks, its plan fingerprinted, its events published.
package app

import (
	"time"

	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/wtm"
)

type Hold interface {
	Grow(locks []wtm.Lock) (bool, error)
	Protected() bool
	Release()
}

type Deps struct {
	Create        create.Deps
	TakeLocks     func(op wtm.Operation) (Hold, error)
	Publisher     func(op wtm.Operation) wtm.Report
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
