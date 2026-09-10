package wt

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	cleanflow "github.com/LucasPcq/wtm/internal/flow/clean"
	createflow "github.com/LucasPcq/wtm/internal/flow/create"
	ffflow "github.com/LucasPcq/wtm/internal/flow/fastforward"
	pruneflow "github.com/LucasPcq/wtm/internal/flow/prune"
	reparentflow "github.com/LucasPcq/wtm/internal/flow/reparent"
	syncflow "github.com/LucasPcq/wtm/internal/flow/sync"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
)

type createPresenter struct {
	shared.CLIPresenter
	config shared.ConfigResult
}

func (p createPresenter) Created(outcome createflow.Outcome) error {
	if p.Format == domain.OutputJSON {
		return output.WriteWorktreeCreateJSON(p.Cmd.OutOrStdout(), outcome.Result)
	}

	// A reused branch's divergence from origin is the one thing "Created worktree x
	// on existing branch" would leave out, and a prompt-free run has no wizard to
	// have shown it.
	var reusedNote shared.ReusedBranchNoteResult
	if outcome.Result.ExistingBranch {
		reusedNote = shared.ReusedBranchNote(shared.ReusedBranchNoteParams{
			Branch: outcome.Branch,
			Ahead:  outcome.Result.OriginAhead,
			Behind: outcome.Result.OriginBehind,
		})
	}

	output.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
		output.FormatCreateResult(w, output.CreateResultParams{
			Branch:        outcome.Branch,
			AlreadyExists: outcome.Result.AlreadyExists,
			From:          outcome.FromBranch,
			EnvStrategy:   string(outcome.Result.Metadata.EnvStrategy),
			EnvNote:       rules.EnvPortSettlementNote(outcome.EnvPorts),
			Path: createDisplayPath(displayPathParams{
				Config:     p.config.Config,
				ProjectDir: p.config.ProjectDir,
				Path:       outcome.Result.Path,
			}),
			ExistingBranch:    outcome.Result.ExistingBranch,
			ReusedNote:        reusedNote.Text,
			ReusedNoteWarning: reusedNote.Warning,
			GoCommand:         fmt.Sprintf(domain.GoCommandFmt, outcome.Branch),
		})
	})
	return nil
}

type cleanPresenter struct {
	shared.CLIPresenter
}

func (p cleanPresenter) Cleaned(outcome cleanflow.Outcome) error {
	if outcome.AlreadyAbsent {
		if p.Format == domain.OutputJSON {
			return output.WriteWorktreeCleanJSON(p.Cmd.OutOrStdout(), output.WriteWorktreeCleanJSONParams{
				Branch:        outcome.Branch,
				AlreadyAbsent: true,
			})
		}
		output.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
			output.Unchanged(w, fmt.Sprintf(domain.CleanAlreadyAbsentFmt, outcome.Branch))
		})
		return nil
	}

	if p.Format == domain.OutputJSON {
		return output.WriteWorktreeCleanJSON(p.Cmd.OutOrStdout(), output.WriteWorktreeCleanJSONParams{
			Branch:           outcome.Branch,
			Path:             outcome.Path,
			Reparented:       outcome.Reparented,
			OrphanedChildren: outcome.OrphanedChildren,
		})
	}

	output.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
		output.Success(w, fmt.Sprintf(domain.CleanedFmt, outcome.Branch))
		for _, child := range outcome.Reparented {
			output.Success(w, fmt.Sprintf(domain.CleanReparentedFmt, child.Branch, child.NewParent))
		}
		for _, child := range outcome.OrphanedChildren {
			output.Warning(w, fmt.Sprintf(domain.CleanStillOrphanedFmt, child.Branch, child.OldParent))
		}
	})
	return nil
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
			return output.WritePruneResultJSON(p.Cmd.OutOrStdout(), domain.PruneResult{})
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
