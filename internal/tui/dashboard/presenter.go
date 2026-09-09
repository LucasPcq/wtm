package dashboard

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	cleanflow "github.com/LucasPcq/wtm/internal/flow/clean"
	createflow "github.com/LucasPcq/wtm/internal/flow/create"
	ffflow "github.com/LucasPcq/wtm/internal/flow/fastforward"
	pruneflow "github.com/LucasPcq/wtm/internal/flow/prune"
	reparentflow "github.com/LucasPcq/wtm/internal/flow/reparent"
	downflow "github.com/LucasPcq/wtm/internal/flow/run/down"
	logsflow "github.com/LucasPcq/wtm/internal/flow/run/logs"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	stopflow "github.com/LucasPcq/wtm/internal/flow/run/stop"
	syncflow "github.com/LucasPcq/wtm/internal/flow/sync"
	"github.com/LucasPcq/wtm/internal/rules"
)

// presenter is the dashboard half of flow.Presenter: every phase of a run lands
// in the bottom output panel, one line at a time. id is the operation it runs
// under, so Stage and HookPhase can also post what a locked row shows in place
// of its state pill.
type presenter struct {
	send func(tea.Msg)
	id   int
}

func (p presenter) line(text string) { p.send(OutputLineMsg{Text: text}) }

func (p presenter) Stage(params flow.StageParams) error {
	p.line(params.Message)
	p.send(opStageMsg{id: p.id, stage: params.Message})
	return params.Work()
}

// HookPhase streams the hooks as they run: RunHooks writes from this goroutine,
// so the sink turns its bytes into lines and posts each one as a message rather
// than touching the model. The panel scrolls and cannot repaint, so it keeps the
// stream and takes the beats as two more lines around it.
func (p presenter) HookPhase(params flow.HookPhaseParams) error {
	p.line(params.Title)
	p.send(opStageMsg{id: p.id, stage: params.Title})
	sink := &flow.LineWriter{Emit: p.line}
	err := params.Run(flow.HookSink{
		Output: sink,
		OnHook: func(beat domain.HookBeat) {
			sink.Flush()
			p.line(rules.HookBeatLine(beat))
		},
	})
	sink.Flush()
	return err
}

// Notice keeps the abort to itself: on a surface where the modal closing is the
// answer, logging "Aborted." for every question the user backed out of turns the
// panel into a list of things that did not happen.
func (p presenter) Notice(notice flow.Notice) {
	if notice.IsAbort() {
		return
	}
	p.line(notice.Text)
	for _, line := range notice.Lines {
		p.line(line)
	}
}

func (p presenter) Status(notice flow.Notice) {
	p.line(notice.Text)
	for _, line := range notice.Lines {
		p.line(line)
	}
}

type createPresenter struct{ presenter }

func (p createPresenter) Created(outcome createflow.Outcome) error {
	if outcome.Aborted {
		return nil
	}
	p.line(fmt.Sprintf(domain.DashboardFinishedFmt, domain.OpKindCreate, outcome.Branch))
	if note := rules.EnvPortSettlementNote(outcome.EnvPorts); note != "" {
		p.line(note)
	}
	p.send(createdMsg{branch: outcome.Branch})
	return nil
}

type cleanPresenter struct{ presenter }

func (p cleanPresenter) Cleaned(outcome cleanflow.Outcome) error {
	if outcome.AlreadyAbsent {
		p.line(fmt.Sprintf(domain.CleanAlreadyAbsentFmt, outcome.Branch))
	} else {
		p.line(fmt.Sprintf(domain.DashboardFinishedFmt, domain.OpKindClean, outcome.Branch))
	}
	for _, child := range outcome.Reparented {
		p.line(fmt.Sprintf(domain.CleanReparentedFmt, child.Branch, child.NewParent))
	}
	for _, child := range outcome.OrphanedChildren {
		p.line(fmt.Sprintf(domain.CleanStillOrphanedFmt, child.Branch, child.OldParent))
	}
	p.send(cleanedMsg{branch: outcome.Branch})
	return nil
}

type reparentPresenter struct{ presenter }

func (p reparentPresenter) Reparented(outcome reparentflow.Outcome) error {
	for _, result := range outcome.Results {
		p.line(fmt.Sprintf(domain.ReparentedFmt, result.Branch, result.OldParent, result.NewParent))
	}
	// The change is metadata only; the rebase is a separate run the user starts.
	p.line(domain.ReparentSyncHintBare)
	p.send(reparentedMsg{})
	return nil
}

type prunePresenter struct{ presenter }

func (p prunePresenter) Pruned(outcome pruneflow.Outcome) error {
	if outcome.Empty {
		p.line(domain.PruneNothingToPrune)
		return nil
	}
	for _, candidate := range outcome.Result.Pruned {
		p.line(fmt.Sprintf(domain.DashboardFinishedFmt, domain.OpKindPrune, candidate.Branch))
	}
	for _, child := range outcome.Result.Reparented {
		p.line(fmt.Sprintf(domain.CleanReparentedFmt, child.Branch, child.NewParent))
	}
	for _, child := range outcome.Result.Orphaned {
		p.line(fmt.Sprintf(domain.CleanStillOrphanedFmt, child.Branch, child.OldParent))
	}
	// A worktree the user checked and the safety re-gate then dropped has to say
	// so: without this it simply vanishes from a run the user asked for.
	for _, skip := range outcome.Result.Skipped {
		p.line(fmt.Sprintf(domain.PruneSkippedFmt, skip.Branch, rules.PruneReasonLabel(skip.Reason)))
	}
	p.send(prunedMsg{})
	return nil
}

type syncPresenter struct{ presenter }

// Planned is what a run that could not ask prints instead of a recap. The
// dashboard always asks, so it never reaches this; the plan the user reads is
// the recap of the session itself.
func (p syncPresenter) Planned(domain.SyncPlan) {}

func (p syncPresenter) Rebased(result domain.SyncResult) {
	// A base-only refresh rebases nothing: without this line the run would report
	// nothing at all.
	if result.BaseTargeted {
		p.line(fmt.Sprintf(domain.DashboardSyncBaseFmt, result.BaseBranch, rules.SyncBaseLabel(result.BaseUpdated)))
	}
	for _, update := range result.ParentUpdates {
		p.line(fmt.Sprintf(domain.DashboardSyncParentFmt, update.Branch, rules.SyncParentStatusLabel(update.Status)))
	}
	for _, step := range result.Steps {
		p.line(fmt.Sprintf(domain.DashboardSyncStepFmt, step.Branch, rules.SyncStepLabel(step)))
	}
}

func (p syncPresenter) Synced(outcome syncflow.Outcome) error {
	if outcome.Empty {
		p.line(domain.SyncNothingToSync)
		return nil
	}
	for _, step := range outcome.Result.Steps {
		if step.Pushed {
			p.line(fmt.Sprintf(domain.DashboardSyncPushedFmt, step.Branch))
		}
		// A rebase left in progress cannot be resolved from this surface, so the
		// worktree it was left in and the two commands that end it are named — the
		// same gesture as the privileged removal.
		if step.KeptInProgress {
			p.line(fmt.Sprintf(domain.SyncKeepConflictHintFmt, step.Branch, step.Path))
		}
	}
	p.send(syncedMsg{})
	return nil
}

// runPresenter reports a `run up` the dashboard started: the phases in the
// output panel like any flow, and the start sequence through whichever watcher
// the action installed — detached for a start, the terminal hand-over for
// `View logs`, which takes the screen on purpose.
type runPresenter struct {
	presenter
	seam.Watcher
}

// downPresenter reports a stop. It has no view to open — nothing is attached to
// — so the jobs it stopped are named in the output panel.
type downPresenter struct{ presenter }

func (p downPresenter) Downed(outcome downflow.Outcome) error {
	stopped := outcome.Stopped()
	if outcome.NoDaemon || len(stopped) == 0 {
		p.line(domain.RunNoJobsHere)
		return nil
	}
	for _, result := range stopped {
		if result.Status == domain.JobActionError {
			p.line(fmt.Sprintf("%s: %s", result.Name, result.Message))
			continue
		}
		p.line(fmt.Sprintf(domain.RunStoppedFmt, result.Name))
	}
	return nil
}

// stopPresenter reports a single job stopped. Like downPresenter it has no view
// to open, so what became of the job is named in the output panel.
type stopPresenter struct{ presenter }

func (p stopPresenter) Stopped(outcome stopflow.Outcome) error {
	if outcome.NoDaemon {
		p.line(domain.RunNoJobsHere)
		return nil
	}
	p.line(fmt.Sprintf(domain.RunStoppedFmt, outcome.Job))
	return nil
}

// logsPresenter opens the run view on what a worktree already has, starting
// nothing: the same hand-over as a run up, with no start sequence to drive.
type logsPresenter struct {
	presenter
	watcher
}

func (p logsPresenter) Show(show logsflow.ShowParams) error {
	_, err := p.Sequence(seam.SequenceParams{Board: show.Board, Job: show.Job, Warnings: show.Warnings})
	return err
}

type ffPresenter struct{ presenter }

func (p ffPresenter) FastForwarded(outcome ffflow.Outcome) error {
	if outcome.Empty {
		p.line(domain.FastForwardNothingToDo)
		return nil
	}
	for _, result := range outcome.Results {
		if result.Status == domain.FFFailed {
			p.line(fmt.Sprintf(domain.FastForwardFailedFmt, result.Branch, result.Detail))
			continue
		}
		p.line(fmt.Sprintf(domain.FastForwardResultFmt, result.Branch, rules.FastForwardStatusLabel(result.Status)))
	}
	p.send(fastForwardedMsg{})
	return nil
}
