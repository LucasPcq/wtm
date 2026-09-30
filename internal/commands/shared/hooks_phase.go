package shared

import (
	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// CreateHooksPhaseParams holds inputs for running on_create hooks as a distinct,
// titled phase, shared by the worktree-creating commands that have not migrated
// to flow/ (extract, checkout) so hooks read the same way in both.
type CreateHooksPhaseParams struct {
	Cmd *cobra.Command
	// ShowHeader prints the phase title before the streamed output; set it only
	// on human-facing runs (never JSON).
	ShowHeader   bool
	ProjectDir   string
	StateDir     string
	WorktreePath string
	Branch       string
	FromBranch   string
	Hooks        []domain.HookCommand
}

// RunCreateHooksPhase runs the on_create hooks under a titled section. No-op when
// no hooks are configured.
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

	return DrawHookPhase(DrawHookPhaseParams{
		Stderr: p.Cmd.ErrOrStderr(),
		Human:  p.ShowHeader,
		Title:  domain.HooksTitleOnCreate,
		LogPath: rules.HooksLogPath(rules.HooksLogPathParams{
			StateDir: p.StateDir,
			Phase:    domain.HookOnCreate,
			Branch:   p.Branch,
		}),
		Run: func(sink flow.HookSink) error {
			params.Output, params.OnHook = sink.Output, sink.OnHook
			return worktree.RunCreateHooks(params)
		},
	})
}
