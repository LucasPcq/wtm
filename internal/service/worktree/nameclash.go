package worktree

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
)

type checkNameFreeParams struct {
	ProjectDir string
	StateDir   string
	Branch     string
}

// checkNameFree refuses a branch whose derived name a live worktree carries.
// The names only exist for the run module, so a project declaring no job is
// never refused: the core creates what it always created. A run.toml that
// cannot be read still counts as declaring some — it says nothing about what it
// will run once fixed.
func checkNameFree(params checkNameFreeParams) error {
	if !runDeclaresJobs(params.StateDir) {
		return nil
	}
	worktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{ProjectDir: params.ProjectDir})
	if err != nil {
		return err
	}
	clash, found := rules.WorktreeNameClash(rules.WorktreeNameClashParams{Branch: params.Branch, Live: branchesOf(worktrees)})
	if !found {
		return nil
	}
	return fmt.Errorf("%w: "+domain.WorktreeNameClashFmt, domain.ErrWorktreeNameTaken, params.Branch, clash.Branch, clash.Name)
}

func runDeclaresJobs(stateDir string) bool {
	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		return true
	}
	return rules.IsRunInitialized(cfg)
}

func branchesOf(worktrees []domain.GitWorktree) []string {
	branches := make([]string, 0, len(worktrees))
	for _, wt := range worktrees {
		branches = append(branches, wt.Branch)
	}
	return branches
}

type nameClashesParams struct {
	StateDir  string
	Worktrees []domain.GitWorktree
}

// liveNames answers the clash question for every worktree of one listing,
// reading run.toml once.
type liveNames struct {
	checked  bool
	branches []string
}

func nameClashes(params nameClashesParams) liveNames {
	if !runDeclaresJobs(params.StateDir) {
		return liveNames{}
	}
	return liveNames{checked: true, branches: branchesOf(params.Worktrees)}
}

type nameClashOfParams struct {
	Branch  string
	Managed bool
}

// of is only asked for a worktree about to be adopted: two worktrees wtm
// already manages under one name are clean's to report, not relocate's.
func (n liveNames) of(params nameClashOfParams) *domain.WorktreeNameClash {
	if !n.checked || params.Managed {
		return nil
	}
	clash, found := rules.WorktreeNameClash(rules.WorktreeNameClashParams{Branch: params.Branch, Live: n.branches})
	if !found {
		return nil
	}
	return &clash
}
