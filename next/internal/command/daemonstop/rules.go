package daemonstop

import (
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

// subject is the one item of a stop: the daemon.
const subject = "daemon"

// ----------------------------------------------------------------------------
// Decide

// Decide turns the daemon as it stands into the plan: what its shutdown does to each job it holds.
func Decide(daemon run.Daemon) Plan {
	shutdown := run.ShutdownOf(daemon.Jobs)
	plan := Plan{Running: daemon.Answering, Stops: run.Held(shutdown.Cut), Kept: run.Held(shutdown.Kept), Lost: run.Held(shutdown.Lost), JobsUnreadable: daemon.Answering && daemon.Skew == run.SkewProtocol}
	plan.Confirm = len(shutdown.Cut) > 0 || plan.JobsUnreadable
	plan.Recap, plan.Empty = plan.recap(), !plan.Running
	return plan
}

// ----------------------------------------------------------------------------
// The plan

// A Plan is what Apply will do, as the dry run and the JSON envelope show it and the fingerprint covers it: whether a daemon runs to stop, the jobs it runs that stop (Stops), the stacks and their claims that run on (Kept), the claims on a job that stops, which go with it (Lost), whether its jobs could not be read (another protocol), and whether a surface asks to confirm it (Confirm: something it runs stops, or may); then the recap, which the fingerprint leaves out.
type Plan struct {
	Running        bool           `json:"running"`
	Stops          []run.HeldRepo `json:"stops"`
	Kept           []run.HeldRepo `json:"kept"`
	Lost           []run.HeldRepo `json:"lost"`
	JobsUnreadable bool           `json:"jobs_unreadable,omitempty"`
	Confirm        bool           `json:"confirm"`
	domain.Recapped
}

// Locks are none: the daemon is no repository's.
func (Plan) Locks() []domain.Lock {
	return nil
}

// Fingerprinted is what the plan's fingerprint covers: the plan, its recap left out.
func (p Plan) Fingerprinted() any {
	p.Recapped = domain.Recapped{}
	return p
}

// A Detail is what became of the daemon: nothing more than its item's status says.
type Detail struct{}

// ----------------------------------------------------------------------------
// The recap

// recap is the plan row by row: what stops, what runs on, what is lost; nothing when no daemon runs.
func (p Plan) recap() []domain.RecapRow {
	if !p.Running {
		return nil
	}
	stops := run.Names(p.Stops)
	if p.JobsUnreadable {
		stops = []string{"every job it runs itself: they can't be read"}
	}
	rows := []domain.RecapRow{{Key: "stops", Label: "Stops", Values: domain.RecapValues(append(stops, "the daemon"))}}
	if kept := run.Names(p.Kept); len(kept) > 0 {
		rows = append(rows, domain.RecapRow{Key: "kept", Label: "Runs on", Values: domain.RecapValues(kept)})
	}
	if lost := run.Names(p.Lost); len(lost) > 0 {
		rows = append(rows, domain.RecapRow{Key: "lost", Label: "Lets go", Values: domain.RecapValues(lost)})
	}
	return rows
}
