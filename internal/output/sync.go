package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// FormatSyncPlan prints the ordered cascade preview: which branch is rebased
// onto which parent, in execution order. It emits a raw body with no outer blank
// lines; the caller's frame owns the outer vertical padding.
func FormatSyncPlan(w io.Writer, plan domain.SyncPlan) {
	SectionTitle(w, domain.SyncPlanHeader)
	if len(plan.Steps) == 0 {
		Message(w, styles.Muted.Render("No worktrees to sync."))
		return
	}
	for i, step := range plan.Steps {
		parent := step.SourceBranch
		if parent == "" {
			parent = styles.Muted.Render("unknown parent")
		}
		InfoLine(w, fmt.Sprintf("%d.", i+1), fmt.Sprintf("%s %s %s", step.Branch, styles.Muted.Render("←"), parent))
	}
}

// FormatSyncResult prints the detailed recap of a sync run: the base update and
// one line per branch (parent, target commit, before→after, replayed count).
// It is rendered BEFORE the push decision so the user sees what happened before
// being asked to push. The push summary is a separate block
// (see FormatSyncPushSummary).
//
// Raw body: no outer blank lines. The command's frame (FrameStart/FrameEnd) owns
// the top/bottom padding; the inter-section blank between the base line and the
// per-branch lines is kept as a genuine separator.
func FormatSyncResult(w io.Writer, result domain.SyncResult) {
	printBase(w, result)

	// The blank separates the base line from the per-branch lines; with no steps
	// (a base-only refresh) it would stack against the push summary's leading
	// blank, so it is only emitted when there is something to separate.
	if len(result.Steps) == 0 {
		return
	}
	if result.BaseTargeted {
		Blank(w)
	}
	printParentUpdates(w, result.ParentUpdates)
	for _, step := range result.Steps {
		printStep(w, step)
	}
	printConflictFooter(w, result.Steps)
}

// printBase says nothing when the run never involved the base: naming it would
// put a branch in the recap that was never read (see rules.BaseIsTarget).
func printBase(w io.Writer, result domain.SyncResult) {
	if !result.BaseTargeted {
		return
	}
	if result.BaseUpdated {
		InfoLine(w, "Base", fmt.Sprintf("%s  %s → %s  %s",
			result.BaseBranch,
			styles.Muted.Render(result.BaseOldTip),
			styles.Muted.Render(result.BaseNewTip),
			styles.Muted.Render(fmt.Sprintf("(fast-forwarded from origin/%s)", result.BaseBranch))))
	} else {
		InfoLine(w, "Base", fmt.Sprintf("%s  %s  %s",
			result.BaseBranch,
			styles.Muted.Render(result.BaseOldTip),
			styles.Muted.Render("(already up to date / no fast-forward)")))
	}
}

// FormatSyncPushSummary prints the trailing summary of which branches were
// pushed or are ready to push. Rendered AFTER the push decision so it reflects
// the final state. Leads with a blank, never trails.
func FormatSyncPushSummary(w io.Writer, steps []domain.SyncStepResult) {
	Blank(w)
	printPushSummary(w, steps)
}

func printParentUpdates(w io.Writer, updates []domain.ParentUpdate) {
	if len(updates) == 0 {
		return
	}
	for _, update := range updates {
		remote := domain.RemoteBranchPrefix + update.Branch
		switch update.Status {
		case domain.ParentFastForwarded:
			Success(w, fmt.Sprintf("%s fast-forwarded to %s  %s",
				update.Branch,
				styles.Muted.Render(remote),
				styles.Muted.Render(update.OldTip+" → "+update.NewTip)))
		case domain.ParentBehind:
			Warning(w, fmt.Sprintf("%s is %s behind %s — %s rebased onto it as is (--%s to refresh)",
				update.Branch, rules.CommitCountLabel(update.Behind), remote,
				strings.Join(update.Children, ", "), domain.FlagFFParents))
		case domain.ParentDiverged:
			Warning(w, fmt.Sprintf("%s has diverged from %s — left untouched; reconcile it manually",
				update.Branch, remote))
		case domain.ParentFFFailed:
			// The refresh was asked for and could not happen: name the obstacle
			// rather than suggest the flag the user already passed.
			Warning(w, fmt.Sprintf("%s could not be fast-forwarded to %s — %s; %s rebased onto it as is",
				update.Branch, remote, update.Detail, strings.Join(update.Children, ", ")))
		}
	}
	Blank(w)
}

func printStep(w io.Writer, step domain.SyncStepResult) {
	switch step.Status {
	case domain.SyncStatusSynced:
		Success(w, syncedLine(step))
	case domain.SyncStatusUpToDate:
		Unchanged(w, fmt.Sprintf("%s %s", step.Branch, domain.SyncUpToDateSuffix))
	case domain.SyncStatusSkippedDirty:
		Warning(w, fmt.Sprintf("%s skipped — uncommitted changes (descendants skipped)", step.Branch))
	case domain.SyncStatusSkippedAncestor:
		Warning(w, fmt.Sprintf("%s skipped — ancestor %s was not synced", step.Branch, step.SourceBranch))
	case domain.SyncStatusDiverged:
		Warning(w, fmt.Sprintf("%s skipped — diverged from origin/%s (reconcile manually); descendants skipped", step.Branch, step.Branch))
	case domain.SyncStatusRebaseInProgress:
		Warning(w, fmt.Sprintf("%s skipped — rebase already in progress; finish it with git rebase --continue (or --abort); descendants skipped", step.Branch))
	case domain.SyncStatusUnknownParent:
		Warning(w, fmt.Sprintf("%s skipped — no recorded parent branch", step.Branch))
	case domain.SyncStatusConflict:
		if step.KeptInProgress {
			Danger(w, fmt.Sprintf("%s conflict on %s — left in progress (%s); descendants skipped",
				step.Branch, step.SourceBranch, conflictCount(step.ConflictFiles)))
		} else {
			Danger(w, fmt.Sprintf("%s conflict on %s — aborted, tree clean (%s); descendants skipped",
				step.Branch, step.SourceBranch, conflictCount(step.ConflictFiles)))
			printConflictFiles(w, step.ConflictFiles)
		}
	case domain.SyncStatusError:
		Error(w, fmt.Sprintf("%s failed — %s", step.Branch, step.Detail))
	}
}

func syncedLine(step domain.SyncStepResult) string {
	pushed := ""
	if step.Pushed {
		pushed = styles.Success.Render("  (pushed)")
	}
	return fmt.Sprintf("%s rebased onto %s (%s)   %s   %s → %s%s",
		step.Branch,
		step.SourceBranch,
		styles.Muted.Render(step.OntoTip),
		styles.Muted.Render(fmt.Sprintf("%d commits", step.CommitsReplayed)),
		styles.Muted.Render(step.OldTip),
		styles.Muted.Render(step.NewTip),
		pushed)
}

func printPushSummary(w io.Writer, steps []domain.SyncStepResult) {
	pushed := make([]string, 0)
	ready := make([]string, 0)
	for _, step := range steps {
		if step.Pushed {
			pushed = append(pushed, step.Branch)
			continue
		}
		if step.PushPending {
			ready = append(ready, step.Branch)
		}
	}

	if len(pushed) > 0 {
		Success(w, fmt.Sprintf("Pushed %d branch(es) (force-with-lease): %s", len(pushed), strings.Join(pushed, ", ")))
	}
	if len(ready) > 0 {
		// No branch list here: the per-branch recap just above already names them,
		// so the pending line only needs the count.
		Message(w, fmt.Sprintf("%d branch(es) ready to push (force-with-lease).", len(ready)))
	}
	if len(pushed) == 0 && len(ready) == 0 {
		Message(w, styles.Muted.Render("Everything is in sync with origin — nothing to push."))
	}
}

// maxConflictFilesShown caps how many conflicting paths are listed inline before
// the rest are collapsed into a "…+N more" suffix.
const maxConflictFilesShown = 5

// printConflictFooter lists the branches whose conflicting rebase was left in
// progress, with the worktree path and the commands to resume. It renders
// nothing in the default (auto-abort) mode. Raw body: the caller's frame owns the
// outer padding.
func printConflictFooter(w io.Writer, steps []domain.SyncStepResult) {
	kept := make([]domain.SyncStepResult, 0)
	for _, step := range steps {
		if step.KeptInProgress {
			kept = append(kept, step)
		}
	}
	if len(kept) == 0 {
		return
	}

	Blank(w)
	Message(w, styles.Bold.Render("Conflicts left in progress — resolve, then continue the rebase:"))
	for _, step := range kept {
		Blank(w)
		InfoLine(w, step.Branch, styles.Muted.Render(step.Path))
		if len(step.ConflictFiles) > 0 {
			Message(w, "   "+styles.Muted.Render("conflicting:"))
			printConflictFiles(w, step.ConflictFiles)
		}
		Message(w, "   "+styles.Muted.Render("→ resolve, then: git rebase --continue    (abort: git rebase --abort)"))
	}
}

// conflictCount renders the bare file count, e.g. "2 files" or "1 file".
func conflictCount(files []string) string {
	if len(files) == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", len(files))
}

// printConflictFiles lists the conflicting paths vertically, one indented line
// each, capping at maxConflictFilesShown and collapsing the rest into "…+N more".
func printConflictFiles(w io.Writer, files []string) {
	for _, line := range conflictFileLines(files) {
		Message(w, "      "+styles.Muted.Render(line))
	}
}

// conflictFileLines returns the conflicting paths as one line each, capping the
// list at maxConflictFilesShown and collapsing the remainder into a trailing
// "…+N more" line.
func conflictFileLines(files []string) []string {
	shown := files
	overflow := 0
	if len(files) > maxConflictFilesShown {
		shown = files[:maxConflictFilesShown]
		overflow = len(files) - maxConflictFilesShown
	}
	lines := make([]string, 0, len(shown)+1)
	lines = append(lines, shown...)
	if overflow > 0 {
		lines = append(lines, fmt.Sprintf("…+%d more", overflow))
	}
	return lines
}

// WriteSyncResultJSON writes the sync result as pretty-printed JSON.
func WriteSyncResultJSON(w io.Writer, result domain.SyncResult) error {
	return encodeJSON(w, result)
}
