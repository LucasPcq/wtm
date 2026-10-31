package run

// IndexVersion is the version of the daemon's index this wtm writes, the
// legacy wtm's too: the two share the file, so a daemon of either takes over
// the jobs the other left.
const IndexVersion = 2

// IndexAccess says what a daemon may do with an index of version file: read it
// only when it is its own version, freeze it read-only when a newer wtm wrote
// it (writing over it would lose that wtm's record of what it started), and
// replace an older one, which it cannot read, with its own.
func IndexAccess(file int) (read, frozen bool) {
	return file == IndexVersion, file > IndexVersion
}

// A Group is what the daemon knows of a process group a record names, as its
// recovery probes it: whether any member is alive, and whether one started
// when the record says, which proves the group is still the job's and not a
// number the system gave to another process since.
type Group struct {
	Alive      bool
	Identified bool
}

// ReconcileParams is a record of the index a starting daemon takes over, the
// group a foreground one names, and whether its worktree is still there.
type ReconcileParams struct {
	Record        Record
	Group         Group
	WorkDirExists bool
}

// A Decision is what a starting daemon does with a record: keep it, at State,
// and reap its group first.
type Decision struct {
	Keep   bool
	State  State
	Reaped bool
}

// Reconcile decides what becomes of a record a daemon left behind. A
// foreground service's process died with its daemon, or survived it as an
// orphan: one whose group is still identifiably the job's is reaped, one whose
// group is gone crashed, one whose group now belongs to another process is
// dropped (signalling it would kill a stranger). This comes before the work
// dir check, since a signal needs no directory. A detached stack and a claim
// own no process of the daemon's: they are kept while their worktree is there.
func Reconcile(params ReconcileParams) Decision {
	record := params.Record
	if record.Foreground() {
		switch {
		case params.Group.Alive && params.Group.Identified:
			return Decision{Keep: true, State: Stopped, Reaped: true}
		case params.Group.Alive:
			return Decision{}
		case params.WorkDirExists:
			return Decision{Keep: true, State: Crashed}
		}
		return Decision{}
	}
	if !params.WorkDirExists || record.Hold() == HoldOwn {
		return Decision{}
	}
	if record.Hold() == HoldClaim {
		return Decision{Keep: true}
	}

	return Decision{Keep: true, State: Running}
}
