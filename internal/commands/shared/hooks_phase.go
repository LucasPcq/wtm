package shared

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// CreateHooksPhaseParams holds inputs for running on_create hooks as a distinct,
// titled phase, shared by the worktree-creating commands that have not migrated
// to flow/ (extract, checkout) so hooks read the same way in both.
type CreateHooksPhaseParams struct {
	Cmd *cobra.Command
	// ShowHeader prints the "Running on_create hooks" title before the streamed
	// output; set it only on human-facing runs (never JSON).
	ShowHeader   bool
	ProjectDir   string
	StateDir     string
	WorktreePath string
	Branch       string
	FromBranch   string
	Hooks        []domain.HookCommand
}

// RunCreateHooksPhase runs the on_create hooks under a titled section. No-op when
// no hooks are configured. It draws them the way CLIPresenter.HookPhase does —
// a bounded tail on a terminal, the raw stream anywhere else — so a hook reads
// the same whichever command ran it.
func RunCreateHooksPhase(p CreateHooksPhaseParams) error {
	if len(p.Hooks) == 0 {
		return nil
	}

	params := domain.CreateHooksParams{
		ProjectDir:   p.ProjectDir,
		StateDir:     p.StateDir,
		WorktreePath: p.WorktreePath,
		Branch:       p.Branch,
		FromBranch:   p.FromBranch,
		Hooks:        p.Hooks,
	}

	stderr := p.Cmd.ErrOrStderr()
	if !p.ShowHeader {
		return worktree.RunCreateHooks(params)
	}

	output.HooksSection(stderr, domain.HooksTitleOnCreate)
	if !output.IsTerminal(stderr) {
		return worktree.RunCreateHooks(params)
	}

	view := output.NewHookView(output.HookViewParams{W: stderr, LogPath: rules.HooksLogPath(rules.HooksLogPathParams{StateDir: p.StateDir, Phase: domain.HookOnCreate, Branch: p.Branch})})
	defer view.Close()
	params.Output, params.OnHook = view, view.OnHook
	return worktree.RunCreateHooks(params)
}
