package rules

import "github.com/LucasPcq/wtm/internal/domain"

type ReconcileJobParams struct {
	Record domain.JobRecord
	// WorkDirExists says whether the worktree the job was started in is still
	// there. Its stop command runs in that directory, so an entry pointing at a
	// deleted one can only be dropped.
	WorkDirExists bool
	// GroupAlive says the record's process group still has members this process
	// may signal. A group whose leader is gone still answers yes while a child
	// holds a port, which is the whole case this ticket exists for.
	GroupAlive bool
	// IdentityConfirmed says a member of that group started when the record says
	// the job did. Group ids come from the same space as PIDs and are recycled,
	// so without it a twelve-day-old entry would aim at a stranger.
	IdentityConfirmed bool
}

// ReconcileDecision is what becomes of one indexed job when a daemon reads the
// index back. Adopt false drops the entry; Reap is the one decision with an
// effect, and it is confined to a foreground service whose group was both found
// alive and identified.
type ReconcileDecision struct {
	Status domain.JobStatus
	Adopt  bool
	Reap   bool
}

// ReconcileJob decides what a daemon makes of an indexed job at start-up. For
// everything but a foreground service it verifies nothing: the truth belongs to
// whoever owns the process — Docker for a detached stack — and a detached entry
// says what wtm actually knows, which is that it launched the job and has not
// seen it since.
func ReconcileJob(params ReconcileJobParams) ReconcileDecision {
	if IsForegroundService(params.Record) {
		return reconcileForeground(params)
	}
	if !params.WorkDirExists {
		return ReconcileDecision{}
	}
	// A claim on a shared service owns no process, so nothing about it can have
	// died with the daemon: it comes back exactly as it was, which is what keeps
	// the job table a usable reference count across a restart.
	if params.Record.Attached {
		return ReconcileDecision{Status: domain.JobStatusAttached, Adopt: true}
	}
	if params.Record.Config.Kind != domain.JobKindService {
		return ReconcileDecision{}
	}
	return ReconcileDecision{Status: domain.JobStatusDetached, Adopt: true}
}

// IsForegroundService is the one kind of indexed job whose process wtm actually
// owns: a service with no stop command of its own, drained through a PTY. A
// claim owns nothing and a detached stack belongs to Docker, so neither is ever
// probed or reaped.
func IsForegroundService(record domain.JobRecord) bool {
	return !record.Attached &&
		record.Config.Kind == domain.JobKindService &&
		!IsDetached(record.Config)
}

// reconcileForeground runs before the WorkDirExists guard, and that ordering is
// the fix: a signal needs no directory, unlike a stop command, and a deleted
// worktree is the worst case rather than a reason to look away — no `run down`
// can name the process any more, so nothing but this pass will ever reach it.
func reconcileForeground(params ReconcileJobParams) ReconcileDecision {
	if params.GroupAlive && params.IdentityConfirmed {
		return ReconcileDecision{Status: domain.JobStatusReaped, Adopt: true, Reap: true}
	}
	// Alive but unrecognized: the group id has been handed to someone else.
	// Dropped in silence, because the alternative is killing a stranger.
	if params.GroupAlive {
		return ReconcileDecision{}
	}
	if !params.WorkDirExists {
		return ReconcileDecision{}
	}
	// Nothing left of the group: it did die with the daemon. Reported rather
	// than hidden, since its log is on disk and `run logs` reads it.
	return ReconcileDecision{Status: domain.JobStatusCrashed, Adopt: true}
}

// IsJobUp reports whether a job in this state is still up — running in the
// daemon, or detached and left to whoever owns it. It is also exactly what the
// durable index keeps, which is why a stop needs no explicit purge: the entry
// leaves with the state.
func IsJobUp(status domain.JobStatus) bool {
	return status == domain.JobStatusRunning ||
		status == domain.JobStatusDetached ||
		status == domain.JobStatusAttached
}
