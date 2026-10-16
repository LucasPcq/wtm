package create

import (
	"maps"

	"github.com/LucasPcq/wtm/internal/command/provision"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/text"
)

const (
	CodeConfigNotFound      kernel.Code = "create.config_not_found"
	CodeBranchNameInvalid   kernel.Code = "create.branch_name_invalid"
	CodeCheckedOutElsewhere kernel.Code = "create.checked_out_elsewhere"
	CodeNameClash           kernel.Code = "create.name_clash"
	CodePathExists          kernel.Code = "create.path_exists"
	CodeParentRequired      kernel.Code = "create.parent_required"
	CodeNothingToIsolate    kernel.Code = "create.nothing_to_isolate"
	CodeNothingBehind       kernel.Code = "create.nothing_behind"
	CodeSetByFlag           kernel.Code = "create.set_by_flag"
	CodeSourceDiverged      kernel.Code = "create.source_diverged"
	CodeEnvParentFallback   kernel.Code = "create.env_parent_fallback"
	CodeFastForwardFailed   kernel.Code = "create.ff_failed"
	CodeFastForwardDirty    kernel.Code = "create.ff_dirty"
	CodeRememberFailed      kernel.Code = "create.remember_failed"
	CodeBadgeExisting       kernel.Code = "create.badge_existing"
	CodeBadgeCheckedOut     kernel.Code = "create.badge_checked_out"
	CodeBadgeOnOrigin       kernel.Code = "create.badge_on_origin"
	CodeBadgeBehind         kernel.Code = "create.badge_behind"
	CodeBadgeAhead          kernel.Code = "create.badge_ahead"
	CodeBadgeDiverged       kernel.Code = "create.badge_diverged"
)

const (
	ParamAhead   = "ahead"
	ParamBehind  = "behind"
	ParamBranch  = "branch"
	ParamCause   = "cause"
	ParamName    = "name"
	ParamOther   = "other"
	ParamProblem = "problem"
)

var messages = text.Messages{
	CodeConfigNotFound:      "no wtm config found — run `wtm init` first",
	CodeBranchNameInvalid:   "{problem}",
	CodeCheckedOutElsewhere: "{branch} is checked out at {path} — run `wtm go {branch}` to jump in",
	CodeNameClash:           "{branch} would share its name with {other} ({name}) — pick another name",
	CodePathExists:          "worktree path already exists: {path}",
	CodeParentRequired:      "a branch that already exists locally needs its parent named (it can't be inferred, and `wtm sync` needs it)",
	CodeNothingToIsolate:    "run.toml declares nothing a worktree isolates",
	CodeNothingBehind:       "nothing to fast-forward: no branch it starts from is behind origin",
	CodeSetByFlag:           "set by its flag only",
	CodeSourceDiverged:      "{branch} has diverged from origin ({ahead} ahead, {behind} behind) and can't be fast-forwarded: the worktree starts from your local branch, missing commits that are on origin — you may have to rebase or resolve conflicts later",
	CodeEnvParentFallback:   "The \"parent\" env strategy needs the source branch checked out to copy its .env; without a worktree it comes from main.",
	CodeFastForwardFailed:   "couldn't fast-forward {branch}: {cause} — it starts from your local branch",
	CodeFastForwardDirty:    "couldn't fast-forward {branch}: its worktree has uncommitted changes — it starts from your local branch",
	CodeRememberFailed:      "could not remember the answer: {cause}",
	CodeBadgeExisting:       "existing",
	CodeBadgeCheckedOut:     "worktree exists",
	CodeBadgeOnOrigin:       "on origin",
	CodeBadgeBehind:         "{behind} behind",
	CodeBadgeAhead:          "{ahead} ahead",
	CodeBadgeDiverged:       "{ahead} ahead, {behind} behind",
}

var Messages = withProvision()

func withProvision() text.Messages {
	all := maps.Clone(messages)
	maps.Copy(all, provision.Messages)
	return all
}

var ExitCodes = map[kernel.Code]int{
	CodeConfigNotFound: domain.ExitCodeConfigNotFound,
}
