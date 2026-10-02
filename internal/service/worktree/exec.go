package worktree

import (
	"os"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
)

type ExecCandidatesParams struct {
	ProjectDir string
}

// ExecCandidates drops a detached worktree (no branch to name it by) and one
// whose directory is gone: there is nowhere to run in it.
func ExecCandidates(params ExecCandidatesParams) ([]domain.GitWorktree, error) {
	all, err := infra.ListWorktrees(infra.ListWorktreesParams{ProjectDir: params.ProjectDir})
	if err != nil {
		return nil, err
	}
	candidates := make([]domain.GitWorktree, 0, len(all))
	for _, candidate := range all {
		if candidate.Branch == "" {
			continue
		}
		if _, statErr := os.Stat(candidate.Path); statErr != nil {
			continue
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

type ExecEnvParams struct {
	Ref          WorktreeRef
	WorktreePath string
}

// ExecEnv strips the worktree variables even when the target has none of its
// own, unlike a hook: an exec command runs in worktrees the caller is not in.
func ExecEnv(params ExecEnvParams) []string {
	return rules.MergeEnv(rules.MergeEnvParams{
		Env:       os.Environ(),
		Clear:     domain.WorktreeScopedEnv,
		Overrides: hookEnv(hookEnvParams(params)),
	})
}
