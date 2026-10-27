package worktree

// An Action is what may be done next with a worktree, one entry of the menu every surface shows of it.
type Action string

// The actions on a worktree, in the order a read offers them.
const (
	ActionGo          Action = "go"
	ActionOpenPR      Action = "open_pr"
	ActionRunUp       Action = "run_up"
	ActionRunDown     Action = "run_down"
	ActionLogs        Action = "logs"
	ActionFastForward Action = "fast_forward"
	ActionSync        Action = "sync"
	ActionReparent    Action = "reparent"
	ActionClean       Action = "clean"
)

// Actions returns every Action, in the order a read offers them.
func Actions() []Action {
	return []Action{ActionGo, ActionOpenPR, ActionRunUp, ActionRunDown, ActionLogs, ActionFastForward, ActionSync, ActionReparent, ActionClean}
}

// An Unoffered is why a worktree does not offer an action, as a code each surface words its own way.
type Unoffered string

// Why a worktree does not offer an action.
const (
	UnofferedMissing       Unoffered = "missing"         // its directory is gone: only removing what is left applies
	UnofferedNoBranch      Unoffered = "no_branch"       // it is on no branch
	UnofferedMainWorktree  Unoffered = "main_worktree"   // the main worktree is never removed nor synced
	UnofferedBaseBranch    Unoffered = "base_branch"     // the base branch's worktree is never removed
	UnofferedNoPullRequest Unoffered = "no_pull_request" // no pull request is known: there is none, or GitHub was not asked
	UnofferedNotOnOrigin   Unoffered = "not_on_origin"   // origin has no branch of its name to fast-forward from
	UnofferedUnrecorded    Unoffered = "unrecorded"      // wtm did not create it: it records no parent to change
	UnofferedUnreadable    Unoffered = "meta_unreadable" // its meta record exists but does not read
)

// Unoffereds returns every Unoffered.
func Unoffereds() []Unoffered {
	return []Unoffered{UnofferedMissing, UnofferedNoBranch, UnofferedMainWorktree, UnofferedBaseBranch, UnofferedNoPullRequest, UnofferedNotOnOrigin, UnofferedUnrecorded, UnofferedUnreadable}
}

// OfferParams are what decides the actions a worktree offers.
type OfferParams struct {
	Main        bool
	Base        bool // it is on the base branch
	Missing     bool
	Detached    bool
	PullRequest bool // a pull request of its branch is known
	OnOrigin    bool // origin has a branch of its name
	Record      RecordState
}

// A RecordState is what a read knows of a worktree's meta record: none, one that reads, or one that exists but does not.
type RecordState int

// The states of a worktree's meta record.
const (
	RecordNone RecordState = iota
	RecordRead
	RecordUnreadable
)

// RecordOf is the state of the meta record of branch, as a read's records (by branch) and the errors of those that do not read say it.
func RecordOf(records map[string]Meta, errs map[string]error, branch string) RecordState {
	switch _, read := records[branch]; {
	case read:
		return RecordRead
	case errs[branch] != nil:
		return RecordUnreadable
	}

	return RecordNone
}

// Unoffer is why the worktree of p does not offer action, "" when it offers it.
func Unoffer(p OfferParams, action Action) Unoffered {
	if action == ActionClean {
		return unofferedClean(p)
	}

	switch {
	case p.Missing:
		return UnofferedMissing
	case action == ActionGo:
		return ""
	case p.Detached:
		return UnofferedNoBranch
	case action == ActionOpenPR && !p.PullRequest:
		return UnofferedNoPullRequest
	case action == ActionFastForward && !p.OnOrigin:
		return UnofferedNotOnOrigin
	case action == ActionSync && p.Main:
		return UnofferedMainWorktree
	case action == ActionReparent:
		return unofferedReparent(p)
	}

	return ""
}

func unofferedReparent(p OfferParams) Unoffered {
	switch {
	case p.Main:
		return UnofferedMainWorktree
	case p.Base:
		return UnofferedBaseBranch
	case p.Record == RecordUnreadable:
		return UnofferedUnreadable
	case p.Record == RecordNone:
		return UnofferedUnrecorded
	}

	return ""
}

func unofferedClean(p OfferParams) Unoffered {
	switch {
	case p.Main:
		return UnofferedMainWorktree
	case p.Base:
		return UnofferedBaseBranch
	}

	return ""
}
