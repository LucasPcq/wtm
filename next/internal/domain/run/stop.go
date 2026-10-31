package run

// A Stop asks the daemon to stop the job Name of the worktree at WorkDir, for
// the command of CorrelationID, which the job.stopped event carries.
type Stop struct {
	Name          string
	WorkDir       string
	CorrelationID string
}

// A StopAll asks the daemon to stop every job up in the worktree at WorkDir,
// every job of the machine when empty, for the command of CorrelationID.
type StopAll struct {
	WorkDir       string
	CorrelationID string
}

// Outcome is what a stop did to one job.
type Outcome string

// The outcomes of a stop.
const (
	OutcomeStopped    Outcome = "stopped"     // the job was up and is stopped
	OutcomeReleased   Outcome = "released"    // a worktree let go of a shared job other worktrees still hold, which runs on
	OutcomeNotRunning Outcome = "not_running" // nothing of that name was up in that worktree
)

// A StoppedJob is a job a stop reached, as it stood before, and what the stop did.
type StoppedJob struct {
	Job     Job
	Outcome Outcome
}
