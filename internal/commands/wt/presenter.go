package wt

import (
	"errors"
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	cleanflow "github.com/LucasPcq/wtm/internal/flow/clean"
	createflow "github.com/LucasPcq/wtm/internal/flow/create"
	envflow "github.com/LucasPcq/wtm/internal/flow/env"
	execflow "github.com/LucasPcq/wtm/internal/flow/exec"
	extractflow "github.com/LucasPcq/wtm/internal/flow/extract"
	ffflow "github.com/LucasPcq/wtm/internal/flow/fastforward"
	pruneflow "github.com/LucasPcq/wtm/internal/flow/prune"
	relocateflow "github.com/LucasPcq/wtm/internal/flow/relocate"
	reparentflow "github.com/LucasPcq/wtm/internal/flow/reparent"
	syncflow "github.com/LucasPcq/wtm/internal/flow/sync"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
)

// A batch's per-item lines go on stderr with the progress; the readout that
// names every item at the end is each command's own.
func batchStarted(p shared.CLIPresenter, progress flow.Progress) {
	if !p.Human {
		return
	}
	output.BranchHeader(shared.OpenBlock(p.Cmd.ErrOrStderr(), true),
		fmt.Sprintf(domain.BatchProgressFmt, progress.Branch, progress.Position, progress.Total))
}

func batchFailed(p shared.CLIPresenter, failure domain.BatchFailure) {
	if !p.Human {
		return
	}
	output.Error(shared.OpenBlock(p.Cmd.ErrOrStderr(), false),
		fmt.Sprintf(domain.BatchFailedFmt, failure.Branch, failure.Error))
}

type createPresenter struct {
	shared.CLIPresenter
	config shared.ConfigResult
}

func (p createPresenter) BranchStarted(progress flow.Progress) {
	batchStarted(p.CLIPresenter, progress)
}

func (p createPresenter) BranchCreated(domain.CreateResult) {}

func (p createPresenter) BranchFailed(failure domain.BatchFailure) {
	batchFailed(p.CLIPresenter, failure)
}

func (p createPresenter) Created(outcome createflow.Outcome) error {
	if p.Format == domain.OutputJSON {
		return output.WriteWorktreeCreateJSON(p.Cmd.OutOrStdout(), domain.CreateBatchResult{
			Results: nonNil(outcome.Results),
			Failed:  nonNil(outcome.Failed),
		})
	}
	switch {
	case len(outcome.Results)+len(outcome.Failed) > 1:
		p.batch(outcome)
	case len(outcome.Results) == 1:
		p.single(outcome.Results[0], outcome.FromBranch)
	}
	return nil
}

func nonNil[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

func (p createPresenter) batch(outcome createflow.Outcome) {
	rows := make([]output.CreateBatchRow, 0, len(outcome.Results))
	for _, result := range outcome.Results {
		rows = append(rows, output.CreateBatchRow{
			Branch:        result.Branch,
			Path:          createDisplayPath(displayPathParams{Config: p.config.Config, ProjectDir: p.config.ProjectDir, Path: result.Path}),
			AlreadyExists: result.AlreadyExists,
		})
	}
	output.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
		output.FormatCreateBatch(w, output.CreateBatchParams{Created: rows, Failed: outcome.Failed})
	})
}

func (p createPresenter) single(result domain.CreateResult, from string) {
	// A reused branch's divergence from origin is the one thing "Created worktree x
	// on existing branch" would leave out, and a prompt-free run has no wizard to
	// have shown it.
	var reusedNote shared.ReusedBranchNoteResult
	if result.ExistingBranch {
		reusedNote = shared.ReusedBranchNote(shared.ReusedBranchNoteParams{
			Branch: result.Branch,
			Ahead:  result.OriginAhead,
			Behind: result.OriginBehind,
		})
	}

	output.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
		output.FormatCreateResult(w, output.CreateResultParams{
			Branch:        result.Branch,
			AlreadyExists: result.AlreadyExists,
			From:          from,
			EnvStrategy:   string(result.Metadata.EnvStrategy),
			EnvNote:       rules.EnvPortSettlementNote(result.EnvPorts),
			Path: createDisplayPath(displayPathParams{
				Config:     p.config.Config,
				ProjectDir: p.config.ProjectDir,
				Path:       result.Path,
			}),
			ExistingBranch:    result.ExistingBranch,
			ReusedNote:        reusedNote.Text,
			ReusedNoteWarning: reusedNote.Warning,
			GoCommand:         fmt.Sprintf(domain.GoCommandFmt, result.Branch),
		})
	})
}

type cleanPresenter struct {
	shared.CLIPresenter
}

func (p cleanPresenter) WorktreeStarted(progress flow.Progress) {
	batchStarted(p.CLIPresenter, progress)
}

func (p cleanPresenter) WorktreeCleaned(domain.CleanResult) {}

func (p cleanPresenter) WorktreeFailed(failure domain.BatchFailure) {
	batchFailed(p.CLIPresenter, failure)
}

func (p cleanPresenter) Cleaned(outcome cleanflow.Outcome) error {
	result := domain.CleanBatchResult{
		Results:          nonNil(outcome.Results),
		Failed:           nonNil(outcome.Failed),
		Skipped:          nonNil(outcome.Skipped),
		Reparented:       nonNil(outcome.Reparented),
		OrphanedChildren: nonNil(outcome.Orphaned),
		Namespaces:       nonNil(outcome.Namespaces),
	}
	if p.Format == domain.OutputJSON {
		return output.WriteCleanJSON(p.Cmd.OutOrStdout(), result)
	}
	switch {
	case len(outcome.Results)+len(outcome.Failed)+len(outcome.Skipped) > 1:
		output.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) { output.FormatCleanBatch(w, result) })
	case len(outcome.Results) == 1:
		p.single(outcome)
	}
	return nil
}

func (p cleanPresenter) single(outcome cleanflow.Outcome) {
	cleaned := outcome.Results[0]
	output.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
		if cleaned.AlreadyAbsent {
			output.Unchanged(w, fmt.Sprintf(domain.CleanAlreadyAbsentFmt, cleaned.Branch))
			return
		}
		output.Success(w, fmt.Sprintf(domain.CleanedFmt, cleaned.Branch))
		for _, child := range outcome.Reparented {
			output.Success(w, fmt.Sprintf(domain.CleanReparentedFmt, child.Branch, child.NewParent))
		}
		for _, child := range outcome.Orphaned {
			output.Warning(w, fmt.Sprintf(domain.CleanStillOrphanedFmt, child.Branch, child.OldParent))
		}
	})
}

type prunePresenter struct {
	shared.CLIPresenter
}

// Pruned renders the three shapes a prune run concludes in: nothing matched, a
// dry-run preview, or a result. The frame goes on exactly once per branch, and
// never in JSON.
func (p prunePresenter) Pruned(outcome pruneflow.Outcome) error {
	if outcome.Empty {
		if p.Format == domain.OutputJSON {
			return output.WritePruneResultJSON(p.Cmd.OutOrStdout(), outcome.Result)
		}
		output.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
			output.Unchanged(w, domain.PruneNothingToPrune)
		})
		return nil
	}

	if p.Format == domain.OutputJSON {
		return output.WritePruneResultJSON(p.Cmd.OutOrStdout(), outcome.Result)
	}

	if outcome.Result.DryRun {
		output.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
			output.FormatPrunePlan(w, outcome.Plan)
		})
		return nil
	}

	output.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
		output.FormatPruneResult(w, outcome.Result)
	})
	return nil
}

type syncPresenter struct {
	shared.CLIPresenter
}

// sync writes across two streams — the plan and the spinners on stderr, the
// recap and the push on stdout — so its frame cannot be a closure. It joins the
// run's block the way every other section does, and Synced closes it.
func (p syncPresenter) section(w io.Writer) io.Writer {
	return shared.OpenBlock(w, true)
}

// Planned prints the cascade a run that could not ask never saw in a recap.
func (p syncPresenter) Planned(plan domain.SyncPlan) {
	if !p.Human {
		return
	}
	output.FormatSyncPlan(p.section(p.Cmd.ErrOrStderr()), plan)
}

// Rebased is the recap the user reads BEFORE being asked to push.
func (p syncPresenter) Rebased(result domain.SyncResult) {
	if !p.Human {
		return
	}
	output.FormatSyncResult(p.section(p.Cmd.OutOrStdout()), result)
}

func (p syncPresenter) Synced(outcome syncflow.Outcome) error {
	if p.Format == domain.OutputJSON {
		return output.WriteSyncResultJSON(p.Cmd.OutOrStdout(), outcome.Result)
	}
	if outcome.Empty {
		output.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
			output.Unchanged(w, domain.SyncNothingToSync)
		})
		return nil
	}
	output.FormatSyncPushSummary(output.Barred(p.Cmd.OutOrStdout()), outcome.Result.Steps)
	output.FrameEnd(p.Cmd.OutOrStdout())
	return nil
}

type reparentPresenter struct {
	shared.CLIPresenter
}

func (p reparentPresenter) Reparented(outcome reparentflow.Outcome) error {
	if p.Format == domain.OutputJSON {
		return output.WriteReparentJSON(p.Cmd.OutOrStdout(), outcome.Results)
	}

	output.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
		for _, result := range outcome.Results {
			output.Success(w, fmt.Sprintf(domain.ReparentedFmt, result.Branch, result.OldParent, result.NewParent))
		}
		output.NextStep(w, output.NextStepParams{
			Command: reparentSyncHint(outcome.Results),
			Note:    domain.ReparentSyncHintNote,
		})
	})
	return nil
}

// reparentSyncHint tells the user how to apply the recorded change. A single
// worktree names it in the suggested command; several point at a bare `wtm sync`.
func reparentSyncHint(results []domain.ReparentResult) string {
	if len(results) == 1 {
		return fmt.Sprintf(domain.ReparentSyncHintFmt, results[0].Branch)
	}
	return domain.ReparentSyncHintBare
}

type ffPresenter struct {
	shared.CLIPresenter
}

func (p ffPresenter) FastForwarded(outcome ffflow.Outcome) error {
	if p.Format == domain.OutputJSON {
		return output.WriteFastForwardJSON(p.Cmd.OutOrStdout(), outcome.Results)
	}
	if outcome.Empty {
		output.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
			output.Unchanged(w, domain.FastForwardNothingToDo)
		})
		return nil
	}
	output.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
		output.FormatFastForwardResults(w, outcome.Results)
	})
	return nil
}

type execPresenter struct {
	shared.CLIPresenter
	print bool
	view  *output.ExecView
}

// Progress draws the live region on a terminal this process may repaint; a
// pipe or a CI log gets one line per finished worktree instead.
func (p *execPresenter) Progress(progress execflow.ExecProgress) {
	if !p.Human {
		return
	}
	stderr := p.Cmd.ErrOrStderr()
	if !output.IsTerminal(stderr) {
		if !progress.Beat.Started {
			output.ExecResultLine(stderr, progress.Beat.Result)
		}
		return
	}
	if p.view == nil {
		p.view = output.NewExecView(output.ExecViewParams{W: stderr, Branches: progress.Branches})
	}
	p.view.OnBeat(progress.Beat)
}

func (p *execPresenter) Executed(outcome execflow.Outcome) error {
	if p.view != nil {
		p.view.Close()
	}
	if p.Format == domain.OutputJSON {
		results := outcome.Results
		if p.print {
			results = withoutTails(results)
		}
		return output.WriteExecJSON(p.Cmd.OutOrStdout(), output.ExecJSONParams{Command: outcome.Command, Results: results})
	}
	output.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
		if p.print {
			output.FormatExecPrint(w, outcome.Results)
			output.Blank(w)
		}
		output.FormatExecConclusion(w, output.ExecConclusionParams{Command: outcome.Command, Results: outcome.Results, Elapsed: outcome.Elapsed})
	})
	return nil
}

func withoutTails(results []domain.ExecResult) []domain.ExecResult {
	stripped := make([]domain.ExecResult, len(results))
	for i, result := range results {
		result.Tail = nil
		stripped[i] = result
	}
	return stripped
}

type relocatePresenter struct {
	shared.CLIPresenter
}

// Relocated renders the three shapes a relocate run concludes in: nothing to
// change, a dry-run preview, or a result.
func (p relocatePresenter) Relocated(outcome relocateflow.Outcome) error {
	if outcome.Empty {
		if !p.Human {
			return output.WriteRelocateResultJSON(p.Cmd.OutOrStdout(), outcome.Result)
		}
		output.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
			output.Unchanged(w, domain.RelocateAlignedMessage)
		})
		return nil
	}

	if !p.Human {
		return output.WriteRelocateResultJSON(p.Cmd.OutOrStdout(), outcome.Result)
	}

	// A preview is what the caller asked for, so it goes to stdout as a result.
	if outcome.DryRun {
		output.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
			output.FormatRelocatePreview(w, output.RelocatePreviewParams{Plan: outcome.Plan, FromBasePath: outcome.FromBasePath})
			output.Blank(w)
			output.Unchanged(w, domain.DryRunNoChanges)
		})
		return nil
	}

	output.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
		output.FormatRelocateResult(w, outcome.Result)
	})
	return nil
}

type envPresenter struct {
	shared.CLIPresenter
	showValues bool
}

func (p envPresenter) Reconciled(outcome envflow.Outcome) error {
	report := output.EnvReportParams{Result: outcome.Result, ShowValues: p.showValues}
	if p.Format == domain.OutputJSON {
		return output.WriteEnvJSON(p.Cmd.OutOrStdout(), report)
	}
	output.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
		output.PrintEnvReport(w, report)
	})
	return nil
}

type extractPresenter struct {
	shared.CLIPresenter
	config shared.ConfigResult
}

func (p extractPresenter) Extracted(outcome extractflow.Outcome) error {
	result := outcome.Result
	if p.Format == domain.OutputJSON {
		if outcome.Nothing != nil {
			result = domain.ExtractResult{Files: []domain.ExtractFile{}}
		}
		return output.WriteExtractJSON(p.Cmd.OutOrStdout(), result)
	}
	path := createDisplayPath(displayPathParams{Config: p.config.Config, ProjectDir: p.config.ProjectDir, Path: result.TargetPath})
	output.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
		switch {
		case errors.Is(outcome.Nothing, domain.ErrNoDirtyWorktrees):
			output.Unchanged(w, domain.ExtractNothingAnywhere)
		case outcome.Nothing != nil:
			output.Unchanged(w, fmt.Sprintf(domain.ExtractNothingInSourceFmt, result.SourceBranch))
		case len(result.Conflicts) > 0:
			output.PrintExtractConflicts(w, output.ExtractConflictsParams{Result: result, Path: path})
		default:
			output.PrintExtractResult(w, output.ExtractResultParams{
				Result:  result,
				Path:    path,
				EnvNote: rules.EnvPortSettlementNote(result.EnvPorts),
			})
		}
	})
	return nil
}
