package worktree

import (
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
	envsvc "github.com/LucasPcq/wtm/internal/service/env"
)

type DetailParams struct {
	ProjectDir string
	StateDir   string
	Config     domain.Config
	Status     domain.WorktreeStatus
	Parent     string
	// ParentPath is the parent's worktree path, which cannot be derived from Parent
	// (the branch name) alone. The "parent" env strategy reads values from it; empty
	// means the parent has no local worktree, a legitimate fallback to main, not an
	// error.
	ParentPath string
	Children   []string
	PRs        []domain.PRInfo
	Commits    int
}

// Detail gathers what the detail panel displays beyond the status. It never
// returns an error: each family fails on its own, into Failures, so the rest of
// the panel stays readable.
func Detail(params DetailParams) domain.WorktreeDetail {
	detail := domain.WorktreeDetail{
		Branch:   params.Status.Branch,
		Children: params.Children,
		LoadedAt: time.Now(),
		Failures: map[domain.DetailFamily]error{},
	}

	commits, err := infra.RecentCommits(infra.RecentCommitsParams{
		WorktreePath: params.Status.Path,
		Limit:        params.Commits,
	})
	if err != nil {
		detail.Failures[domain.DetailFamilyCommits] = err
	}
	detail.Commits = commits

	detail.Changes, err = readChanges(params.Status.Path)
	if err != nil {
		detail.Failures[domain.DetailFamilyChanges] = err
	}

	if !params.Status.IsParent {
		detail.BranchDiff, err = infra.BranchDiffShortstat(infra.BranchDiffShortstatParams{
			WorktreePath: params.Status.Path,
			Base:         params.Config.Project.Worktrees.BaseBranch,
			Branch:       params.Status.Branch,
		})
		if err != nil {
			detail.Failures[domain.DetailFamilyBranchDiff] = err
		}
	}

	detail.Blockers = rules.CleanBlockers(domain.CleanCheckResult{
		WorktreePath:    params.Status.Path,
		Branch:          params.Status.Branch,
		IsDirty:         params.Status.IsDirty,
		IsParent:        params.Status.IsParent,
		UnpushedCommits: unpushed(params),
		HasOpenPR:       openPR(params) != nil,
		PRUrl:           openPRURL(params),
	})

	detail.EnvDrift, err = readEnvDrift(params)
	if err != nil {
		detail.Failures[domain.DetailFamilyEnv] = err
	}

	return detail
}

func readChanges(worktreePath string) (domain.WorkingChanges, error) {
	entries, err := infra.ListModifiedFiles(infra.ListModifiedFilesParams{WorktreePath: worktreePath})
	if err != nil {
		return domain.WorkingChanges{}, err
	}

	changes := rules.CountChanges(entries)
	stat, err := infra.DiffShortstat(infra.DiffShortstatParams{WorktreePath: worktreePath})
	if err != nil {
		return changes, err
	}
	changes.Insertions, changes.Deletions = stat.Insertions, stat.Deletions
	return changes, nil
}

// unpushed treats a branch with no remote as nothing to push, not a failure.
func unpushed(params DetailParams) int {
	count, err := infra.UnpushedCommits(infra.UnpushedCommitsParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Status.Branch,
	})
	if err != nil {
		return 0
	}
	return count
}

func openPR(params DetailParams) *domain.PRInfo {
	for index, pr := range params.PRs {
		if pr.Branch == params.Status.Branch {
			return &params.PRs[index]
		}
	}
	return nil
}

func openPRURL(params DetailParams) string {
	pr := openPR(params)
	if pr == nil {
		return ""
	}
	return pr.URL
}

// readEnvDrift distinguishes "no env files configured" (legitimate absence, not a
// failure) from "drift computed": the renderer must not present the former as a
// success.
func readEnvDrift(params DetailParams) (domain.EnvDriftSummary, error) {
	files := params.Config.Project.Env.Files
	if len(files) == 0 {
		return domain.EnvDriftSummary{Configured: false}, nil
	}

	meta, _ := Metadata(ParentBranchParams{
		StateDir: params.StateDir,
		Branch:   params.Status.Branch,
	})

	results, err := envsvc.ComputeEnvDiff(envsvc.ComputeEnvParams{
		Branch:             params.Status.Branch,
		MainPath:           params.ProjectDir,
		WorktreePath:       params.Status.Path,
		ParentWorktreePath: params.ParentPath,
		ParentBranch:       params.Parent,
		Files:              files,
		Strategy:           meta.EnvStrategy,
		Mode:               domain.EnvModeAdd,
	})
	if err != nil {
		return domain.EnvDriftSummary{Configured: true}, err
	}

	drift := domain.EnvDriftSummary{Configured: true}
	for _, result := range results {
		drift.Missing += len(rules.EnvKeysWithStatus(rules.EnvDiffFilter{Diff: result.Diff, Status: domain.EnvKeyMissing}))
		drift.Conflicting += len(rules.EnvKeysWithStatus(rules.EnvDiffFilter{Diff: result.Diff, Status: domain.EnvKeyConflict}))
		drift.Orphan += len(rules.EnvKeysWithStatus(rules.EnvDiffFilter{Diff: result.Diff, Status: domain.EnvKeyOrphan}))
	}
	return drift, nil
}
