package rules

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// SyncIncludesBaseParams holds inputs for SyncIncludesBase.
type SyncIncludesBaseParams struct {
	Selected   []string
	BaseBranch string
}

// SyncIncludesBase reports whether the selection asks to refresh the base branch.
// The --all case (nil selection) always does; an explicit selection does when it
// names the base. It lets the command run a base-only refresh with no rebase steps.
func SyncIncludesBase(params SyncIncludesBaseParams) bool {
	if params.Selected == nil {
		return true
	}
	for _, branch := range params.Selected {
		if branch == params.BaseBranch {
			return true
		}
	}
	return false
}

// SyncSelectsOnlyBase reports that a selection asks for nothing but a base
// refresh, so the cascade has no rebase step — decidable without planning it. A
// nil (or empty) selection means "every worktree", which is never base-only.
func SyncSelectsOnlyBase(params SyncIncludesBaseParams) bool {
	if len(params.Selected) == 0 {
		return false
	}
	for _, branch := range params.Selected {
		if branch != params.BaseBranch {
			return false
		}
	}
	return true
}

// BaseIsTargetParams holds inputs for BaseIsTarget.
type BaseIsTargetParams struct {
	Steps      []domain.SyncStep
	Selected   []string
	BaseBranch string
}

// BaseIsTarget reports whether the base plays any role in this run. When it does
// not, fetching and fast-forwarding it would be a side effect on a branch the run
// was never asked about — which is what a cascade hanging entirely off some other
// parent produces.
func BaseIsTarget(params BaseIsTargetParams) bool {
	for _, step := range params.Steps {
		if step.SourceBranch == params.BaseBranch {
			return true
		}
	}
	// No step rebases onto it, so only the selection can bring it in: --all covers
	// the forest including its root, an explicit selection may name it.
	return SyncIncludesBase(SyncIncludesBaseParams{
		Selected:   params.Selected,
		BaseBranch: params.BaseBranch,
	})
}

// PushableCount counts the rebased steps that have a pending push not yet done.
func PushableCount(steps []domain.SyncStepResult) int {
	count := 0
	for _, step := range steps {
		if step.PushPending && !step.Pushed {
			count++
		}
	}
	return count
}

// HasSyncFailure reports whether any step ended in a conflict or error.
func HasSyncFailure(steps []domain.SyncStepResult) bool {
	for _, step := range steps {
		if step.Status == domain.SyncStatusConflict || step.Status == domain.SyncStatusError {
			return true
		}
	}
	return false
}

// SyncStepCutShort reports a step an interrupt may have cut short: its git
// failed, or never got the answer it read divergence from. Every other status
// is a state the branch really is in.
func SyncStepCutShort(step domain.SyncStepResult) bool {
	return step.Status == domain.SyncStatusError || step.Status == domain.SyncStatusDiverged
}

// SyncInterrupted reports a run an interrupt stopped before every step was done.
func SyncInterrupted(steps []domain.SyncStepResult) bool {
	for _, step := range steps {
		if step.Status == domain.SyncStatusCancelled {
			return true
		}
	}
	return false
}

// PushDecision is the resolved push action after applying the non-interactive
// push flags and the pushable count.
type PushDecision int

const (
	// PushSkip means do not push: nothing is pushable, --no-push was given, or a
	// non-interactive run was not told to push with --push.
	PushSkip PushDecision = iota
	// PushForce means push without prompting (--push with pushable branches).
	PushForce
	// PushConfirm means an interactive run should ask before pushing.
	PushConfirm
)

// DecidePushParams holds inputs for DecidePush.
type DecidePushParams struct {
	Push          bool
	NoPush        bool
	Interactive   bool
	PushableCount int
	// Yes is --yes: run without the push prompt. Pushing force-pushes with lease,
	// so the safe default under --yes is not to push; the user opts in with --push.
	Yes bool
}

// DecidePush resolves whether to push the rebased branches. With branches to
// push and neither --push nor --no-push, an interactive run is asked (PushConfirm);
// a non-interactive or --yes run only pushes when --push is set (safe default: no push).
func DecidePush(params DecidePushParams) PushDecision {
	if params.PushableCount == 0 || params.NoPush {
		return PushSkip
	}
	if params.Push {
		return PushForce
	}
	if params.Yes || !params.Interactive {
		return PushSkip
	}
	return PushConfirm
}

// ParentDecision is the resolved action for the parents no step covers.
type ParentDecision int

const (
	ParentLeaveAsIs ParentDecision = iota
	ParentFastForward
	ParentAsk
)

// DecideParentFastForwardParams holds inputs for DecideParentFastForward.
type DecideParentFastForwardParams struct {
	FF          bool
	NoFF        bool
	Interactive bool
	StaleCount  int
	// Yes is --yes. Advancing a branch the user never named is a side effect, so
	// the unattended default is to report it and move on; --ff-parents opts in.
	Yes bool
}

// ParentFlagsDecision resolves what the flags alone say, without knowing whether
// anything is stale — so a surface learns whether it must inspect the parents at
// all. Only ParentAsk needs the count.
func ParentFlagsDecision(params DecideParentFastForwardParams) ParentDecision {
	if params.NoFF {
		return ParentLeaveAsIs
	}
	if params.FF {
		return ParentFastForward
	}
	if params.Yes || !params.Interactive {
		return ParentLeaveAsIs
	}
	return ParentAsk
}

// CommitCountLabel renders a commit distance for prose ("1 commit" / "3 commits").
// It lives here because output/ and tui/ both need it and cannot import each other.
func CommitCountLabel(n int) string {
	if n == 1 {
		return "1 commit"
	}
	return fmt.Sprintf("%d commits", n)
}

// SyncStatusLabel says in a few words what became of one step, for a surface
// that has one line per branch rather than a paragraph. An unrecognized status
// reads as itself: a blank line would hide a step that did happen.
func SyncStatusLabel(status domain.SyncStepStatus) string {
	switch status {
	case domain.SyncStatusSynced:
		return domain.SyncLabelSynced
	case domain.SyncStatusUpToDate:
		return domain.SyncLabelUpToDate
	case domain.SyncStatusSkippedDirty:
		return domain.SyncLabelSkippedDirty
	case domain.SyncStatusSkippedAncestor:
		return domain.SyncLabelSkippedAncestor
	case domain.SyncStatusDiverged:
		return domain.SyncLabelDiverged
	case domain.SyncStatusRebaseInProgress:
		return domain.SyncLabelRebaseInProgress
	case domain.SyncStatusConflict:
		return domain.SyncLabelConflict
	case domain.SyncStatusUnknownParent:
		return domain.SyncLabelUnknownParent
	case domain.SyncStatusError:
		return domain.SyncLabelError
	case domain.SyncStatusCancelled:
		return domain.SyncLabelCancelled
	default:
		return string(status)
	}
}

// SyncStepLabel is SyncStatusLabel plus what the status alone cannot carry: the
// cause of a failure, and which of the two conflict modes ran — a run that says
// only "conflict" leaves the user unsure whether a worktree is still mid-rebase.
func SyncStepLabel(step domain.SyncStepResult) string {
	if step.Status == domain.SyncStatusError && step.Detail != "" {
		return fmt.Sprintf(domain.SyncLabelErrorFmt, step.Detail)
	}
	if step.Status != domain.SyncStatusConflict {
		return SyncStatusLabel(step.Status)
	}
	if step.KeptInProgress {
		return domain.SyncLabelConflictKept
	}
	return domain.SyncLabelConflictAborted
}

// SyncBaseLabel says what became of the base branch a cascade fetched.
func SyncBaseLabel(updated bool) string {
	if updated {
		return domain.SyncBaseLabelFastForwarded
	}
	return domain.SyncBaseLabelUpToDate
}

// SyncParentStatusLabel says what became of a parent no step covered.
func SyncParentStatusLabel(status domain.ParentStatus) string {
	switch status {
	case domain.ParentFastForwarded:
		return domain.SyncParentLabelFastForwarded
	case domain.ParentBehind:
		return domain.SyncParentLabelBehind
	case domain.ParentDiverged:
		return domain.SyncParentLabelDiverged
	case domain.ParentFFFailed:
		return domain.SyncParentLabelFFFailed
	default:
		return string(status)
	}
}

// SyncAllBranches lists what --all covers: every worktree but the base, which is
// fast-forwarded from its remote rather than rebased. It excludes nothing else —
// a dirty or half-rebased worktree stays in so the run reports why it skipped it.
func SyncAllBranches(statuses []domain.WorktreeStatus) []string {
	branches := make([]string, 0, len(statuses))
	for _, status := range statuses {
		if status.IsParent {
			continue
		}
		branches = append(branches, status.Branch)
	}
	return branches
}

// SyncReadyBranches lists the worktrees a cascade can act on today: the base is
// among them — it is fast-forwarded from its remote, not rebased, and every chain
// below it wants it fresh — while a dirty or half-rebased worktree is left out,
// base included, because the run would skip it anyway. It is what a surface
// offering "sync everything" pre-checks; the rest stays listed, and checkable,
// rather than dropped. It is deliberately narrower than SyncAllBranches: a
// pre-check is an offer, --all is an answer.
func SyncReadyBranches(statuses []domain.WorktreeStatus) []string {
	branches := make([]string, 0, len(statuses))
	for _, status := range statuses {
		if status.IsDirty || status.RebaseInProgress {
			continue
		}
		branches = append(branches, status.Branch)
	}
	return branches
}

// WorktreeNodesParams holds inputs for WorktreeNodes.
type WorktreeNodesParams struct {
	Statuses []domain.WorktreeStatus
	// Parents maps a branch to the branch it was created from.
	Parents map[string]string
}

// WorktreeNodes pairs each worktree with its recorded parent: the forest the
// sync and reparent rules read. A surface that already holds both builds it from
// them rather than reading the repository again.
func WorktreeNodes(params WorktreeNodesParams) []domain.WorktreeNode {
	nodes := make([]domain.WorktreeNode, 0, len(params.Statuses))
	for _, status := range params.Statuses {
		// The root hangs off nothing: leftover metadata naming a parent for it would
		// otherwise close a loop through the whole forest.
		source := ""
		if !status.IsParent {
			source = params.Parents[status.Branch]
		}
		nodes = append(nodes, domain.WorktreeNode{
			Branch:           status.Branch,
			Path:             status.Path,
			SourceBranch:     source,
			IsMain:           status.IsParent,
			RebaseInProgress: status.RebaseInProgress,
		})
	}
	return nodes
}
