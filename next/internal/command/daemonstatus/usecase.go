// Package daemonstatus reads wtm's daemon, the one of the machine: whether it runs, its build against this wtm's, the jobs it holds by repository, its proxy, its socket and its index; a read, it changes nothing and never starts it.
package daemonstatus

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell; its command line is `daemon status`.
const Name = "daemon-status"

// A UseCase reads wtm's daemon through Daemon; Version is this wtm's build, and Restart the follow-up that restarts a daemon of another build.
type UseCase struct {
	Daemon  Daemon
	Version string
	Restart *domain.FollowUp
}

// An Outcome is what Run read: one Item, the daemon, and the warnings of what it holds that a reader should know.
type Outcome = domain.Read[Detail]

// An Item is the daemon as read.
type Item = domain.Unit[Detail]

// ----------------------------------------------------------------------------
// Run

// Run reads the daemon; a read, it takes no lock, runs no operation and never starts it. An error returned is a socket that answered nothing a client can read.
func (u UseCase) Run(ctx context.Context, _ Request, _ domain.RunOptions) (Outcome, error) {
	// 1. Read the daemon, or the index it left when none answers.
	daemon, err := u.Daemon(ctx)
	if err != nil {
		return Outcome{}, err
	}

	// 2. Report it, with what a reader should know of it.
	return Report(Facts{Daemon: daemon, Version: u.Version}, ReportParams{Restart: u.Restart}), nil
}

// ----------------------------------------------------------------------------
// Questions

// Questions is daemon status's form: a read of the machine, it asks nothing.
func (u UseCase) Questions(context.Context, Request) (Questions, error) {
	return Questions{}, nil
}
