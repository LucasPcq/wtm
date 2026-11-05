package run

import "slices"

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

// Fate is what a stop does to one job of a worktree.
type Fate string

// The fates of a job a stop reaches.
const (
	FateStop    Fate = "stop"    // the job stops: a process of the daemon's, a stack by its stop command, or a shared service whose every hold the stop lets go of
	FateRelease Fate = "release" // the worktree lets go of a shared service other worktrees still hold, which runs on for them
	FateClear   Fate = "clear"   // a job that crashed: nothing is left to stop, the stop clears it from the listing
)

// Fates returns every Fate.
func Fates() []Fate {
	return []Fate{FateStop, FateRelease, FateClear}
}

// A Stopping is a job a stop reaches in a worktree, as the daemon lists it, its Fate, and for a hold on a shared service the worktrees, by path, that keep holding it (KeptBy, none when the service stops).
type Stopping struct {
	Job    Job
	Fate   Fate
	KeptBy []string
}

// StoppingIn is what a stop of the worktrees at dirs does to each of their jobs, worktree by worktree in the order of dirs, each one's as jobs list them, those of names only when names is not nil: a job up the worktree runs for itself stops, one that crashed is cleared, and each hold on a shared service is let go of, the service stopping when every hold on it is among those. A shared service a main checkout runs for the others without holding it is theirs, and left out.
func StoppingIn(jobs []Job, dirs, names []string) []Stopping {
	reached := func(job Job) bool {
		return slices.Contains(dirs, job.WorkDir) && (names == nil || slices.Contains(names, job.Name))
	}
	var stopping []Stopping
	for _, dir := range dirs {
		for _, job := range jobs {
			if job.WorkDir != dir || !reached(job) {
				continue
			}
			switch {
			case Own(job) && job.Up:
				stopping = append(stopping, Stopping{Job: job, Fate: FateStop})
			case Own(job) && job.State == Crashed:
				stopping = append(stopping, Stopping{Job: job, Fate: FateClear})
			// A daemon of another build that cannot tell whether main holds it: taken as held, as RunningIn offers it (the plan warns fates_approximate).
			case job.Up && (job.Hold == HoldClaim || job.MainHolds || job.MainHoldsUnknown):
				kept := slices.DeleteFunc(Holders(jobs, job), func(holder string) bool {
					return slices.ContainsFunc(jobs, func(j Job) bool { return j.WorkDir == holder && j.Name == job.Name && reached(j) })
				})
				fate := FateRelease
				if len(kept) == 0 {
					fate, kept = FateStop, nil
				}
				stopping = append(stopping, Stopping{Job: job, Fate: fate, KeptBy: kept})
			}
		}
	}
	return stopping
}

// Holders are the worktrees, by path, holding the shared service a hold of hold reaches (a claim on it, or the service itself): its main checkout when it holds it and each worktree claiming it, sorted.
func Holders(jobs []Job, hold Job) []string {
	var holders []string
	for _, job := range jobs {
		switch {
		case job.Name != hold.Name || job.SharedDir != hold.SharedDir || !job.Up:
		case job.Hold == HoldClaim:
			holders = append(holders, job.WorkDir)
		case job.MainHolds && job.WorkDir == job.SharedDir:
			holders = append(holders, job.WorkDir)
		}
	}
	slices.Sort(holders)
	return holders
}
