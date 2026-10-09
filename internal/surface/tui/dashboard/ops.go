package dashboard

import "github.com/LucasPcq/wtm/internal/flow"

// operation is a flow the dashboard started and has not seen finish. Its
// targets are the worktrees it works on, known only once the session naming
// them is answered — several since the run flows became cumulative. stages are
// the latest Stage/HookPhase message per worktree — what the list shows in place
// of a locked row's state pill.
type operation struct {
	id      int
	kind    string
	mode    flow.Mode
	targets []string
	stages  map[string]string
}

// operations is what the dashboard has in flight. A run declares how it holds the
// surface (flow.Operation); this is where that declaration is enforced, once,
// instead of at every action site.
type operations struct {
	running []operation
	lastID  int
}

func (o operations) begin(op operation) (operations, int) {
	o.lastID++
	op.id = o.lastID
	o.running = append(append([]operation(nil), o.running...), op)
	return o, op.id
}

func (o operations) retarget(id int, targets []string) operations {
	running := append([]operation(nil), o.running...)
	for index := range running {
		if running[index].id == id {
			running[index].targets = targets
		}
	}
	o.running = running
	return o
}

type stageParams struct {
	ID int
	// Target is the worktree the message came from. Empty means the run said
	// nothing about which one — a flow acting on a single worktree never has to —
	// and every row the run holds then shows it.
	Target string
	Stage  string
}

// stage records a run's latest Stage/HookPhase message. A run over several
// worktrees posts one per worktree: a single string would show the last event
// received on every row it holds, whichever worktree it came from.
func (o operations) stage(params stageParams) operations {
	running := append([]operation(nil), o.running...)
	for index := range running {
		if running[index].id != params.ID {
			continue
		}
		stages := make(map[string]string, len(running[index].stages)+1)
		for target, text := range running[index].stages {
			stages[target] = text
		}
		stages[params.Target] = params.Stage
		running[index].stages = stages
	}
	o.running = running
	return o
}

// stageFor is what a locked row shows: what the run said about this worktree,
// else what it said about none.
func (o operation) stageFor(target string) string {
	if stage, said := o.stages[target]; said {
		return stage
	}
	return o.stages[""]
}

// holding reports the run that owns a worktree. A background run keeps its
// target to itself: nothing else may act on a worktree still being worked on.
func (o operations) holding(target string) (operation, bool) {
	if target == "" {
		return operation{}, false
	}
	for _, op := range o.running {
		for _, held := range op.targets {
			if held == target {
				return op, true
			}
		}
	}
	return operation{}, false
}

// active reports whether any run is still in flight. The spinner loop reads it:
// a locked row's glyph must keep advancing for the whole run, or it reads as
// hung rather than as busy.
func (o operations) active() bool { return len(o.running) > 0 }

// blocking reports a run that holds the whole surface, and with it every action.
func (o operations) blocking() (operation, bool) {
	for _, op := range o.running {
		if op.mode == flow.ModeBlocking {
			return op, true
		}
	}
	return operation{}, false
}

func (o operations) byID(id int) (operation, bool) {
	for _, op := range o.running {
		if op.id == id {
			return op, true
		}
	}
	return operation{}, false
}

func (o operations) end(id int) operations {
	running := make([]operation, 0, len(o.running))
	for _, op := range o.running {
		if op.id != id {
			running = append(running, op)
		}
	}
	o.running = running
	return o
}
