package worktree

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
)

type ResolveSyncBranchesParams struct {
	ProjectDir string
	Queries    []string
}

// ResolveSyncBranches maps each query (a branch name or unambiguous substring) to
// a concrete worktree branch. An unknown query returns domain.ErrBranchNotFound;
// an ambiguous substring returns a descriptive error listing the candidates.
func ResolveSyncBranches(params ResolveSyncBranchesParams) ([]string, error) {
	worktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{
		ProjectDir: params.ProjectDir,
	})
	if err != nil {
		return nil, err
	}

	branches := make([]string, 0, len(params.Queries))
	for _, query := range params.Queries {
		branch, matchErr := matchSyncBranch(worktrees, query)
		if matchErr != nil {
			return nil, matchErr
		}
		branches = append(branches, branch)
	}
	return branches, nil
}

func matchSyncBranch(worktrees []domain.GitWorktree, query string) (string, error) {
	for _, wt := range worktrees {
		if wt.Branch == query {
			return wt.Branch, nil
		}
	}

	var matches []string
	for _, wt := range worktrees {
		if strings.Contains(wt.Branch, query) {
			matches = append(matches, wt.Branch)
		}
	}

	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("%q is ambiguous: matches %s", query, strings.Join(matches, ", "))
	}
	return "", fmt.Errorf("%w: %s", domain.ErrBranchNotFound, query)
}

// Resolve returns a direct path on an exact match, the candidates when ambiguous,
// and every worktree for an empty query (the picker).
func Resolve(params domain.ResolveParams) (domain.ResolveResult, error) {
	worktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{
		ProjectDir: params.ProjectDir,
	})
	if err != nil {
		return domain.ResolveResult{}, err
	}

	result := resolveWorktree(worktrees, params.Query)
	if !result.Ambiguous && result.Path == "" {
		return domain.ResolveResult{}, domain.ErrWorktreeNotFound
	}
	return result, nil
}

// resolveWorktree maps a query to a worktree against a known list. An empty query
// lists all worktrees (Ambiguous) for the picker; an exact branch match wins;
// otherwise a unique substring match returns its path and multiple matches are
// Ambiguous. No match yields a zero result, which Resolve maps to
// domain.ErrWorktreeNotFound.
func resolveWorktree(worktrees []domain.GitWorktree, query string) domain.ResolveResult {
	if query == "" {
		return domain.ResolveResult{Ambiguous: true, Matches: worktrees}
	}

	for _, wt := range worktrees {
		if wt.Branch == query {
			return domain.ResolveResult{Path: wt.Path, Branch: wt.Branch}
		}
	}

	var matches []domain.GitWorktree
	for _, wt := range worktrees {
		if strings.Contains(wt.Branch, query) {
			matches = append(matches, wt)
		}
	}

	if len(matches) == 1 {
		return domain.ResolveResult{Path: matches[0].Path, Branch: matches[0].Branch}
	}
	if len(matches) > 1 {
		return domain.ResolveResult{Ambiguous: true, Matches: matches}
	}
	return domain.ResolveResult{}
}

type FindByBranchParams struct {
	ProjectDir string
	Branch     string
}

// FindByBranch never matches a substring, unlike Resolve: a caller already holding
// a branch name must not land on a different worktree.
func FindByBranch(params FindByBranchParams) (domain.GitWorktree, error) {
	return infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	})
}

type CurrentBranchParams struct {
	Dir string
}

// CurrentBranch names the branch checked out in the worktree containing Dir,
// which may be any of its subdirectories. A detached HEAD returns an error.
func CurrentBranch(params CurrentBranchParams) (string, error) {
	return infra.CurrentBranch(params.Dir)
}

// Root is the worktree containing dir, spelled the way git spells it. It exists
// so a flow can ask without reaching into infra/: the directory a command was
// launched from doubles as the daemon's key for a job (name + WorkDir) and as
// the job's own working directory, which run.toml's `cwd` is resolved against.
// A subdirectory — or macOS's /var where git says /private/var — would both
// mis-resolve that `cwd` and split one worktree into two keys.
func Root(dir string) (string, error) {
	return infra.Toplevel(dir)
}
