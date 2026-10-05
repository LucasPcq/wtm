package worktree

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/branch"
)

// statusWorkers bounds concurrent git probes when building worktree status.
const statusWorkers = 8

// List returns all worktrees enriched with git status, sorted with parent first
// then children by creation date (oldest first).
func List(ctx context.Context, params domain.ListParams) ([]domain.WorktreeStatus, error) {
	gitWorktrees, err := infra.ListWorktrees(ctx, infra.ListWorktreesParams{
		ProjectDir: params.ProjectDir,
	})
	if err != nil {
		return nil, err
	}

	baseBranch := params.Config.Project.Worktrees.BaseBranch
	statuses := make([]domain.WorktreeStatus, len(gitWorktrees))

	sem := make(chan struct{}, statusWorkers)
	var wg sync.WaitGroup
	for i, gitWorktree := range gitWorktrees {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, wt domain.GitWorktree) {
			defer wg.Done()
			defer func() { <-sem }()
			statuses[idx] = buildStatus(ctx, buildStatusParams{
				GitWorktree: wt,
				BaseBranch:  baseBranch,
				StateDir:    params.StateDir,
				ProjectDir:  params.ProjectDir,
			})
		}(i, gitWorktree)
	}
	wg.Wait()

	rules.SortStatuses(statuses)

	return statuses, nil
}

// Refresh fetches origin then re-lists so the origin-divergence badges reflect
// the latest remote state. A failed fetch is ignored (best effort): the list
// still refreshes from whatever remote-tracking refs are present. The dirty and
// base-ahead fields are local and recompute harmlessly.
func Refresh(ctx context.Context, params domain.ListParams) ([]domain.WorktreeStatus, error) {
	_ = infra.Fetch(ctx, infra.FetchParams{ProjectDir: params.ProjectDir})
	return List(ctx, params)
}

type buildStatusParams struct {
	GitWorktree domain.GitWorktree
	BaseBranch  string
	StateDir    string
	ProjectDir  string
}

func buildStatus(ctx context.Context, params buildStatusParams) domain.WorktreeStatus {
	gitWorktree := params.GitWorktree
	dirty, _ := infra.IsDirty(ctx, infra.IsDirtyParams{WorktreePath: gitWorktree.Path})

	ahead := 0
	if !gitWorktree.IsMain {
		ahead, _ = infra.CommitsAhead(ctx, infra.CommitsAheadParams{
			WorktreePath: gitWorktree.Path,
			BaseBranch:   params.BaseBranch,
			Branch:       gitWorktree.Branch,
		})
	}

	originState, originAB := branch.Divergence(ctx, branch.BranchParams{
		ProjectDir: params.ProjectDir,
		Branch:     gitWorktree.Branch,
	})

	return domain.WorktreeStatus{
		Branch:           gitWorktree.Branch,
		Path:             gitWorktree.Path,
		IsParent:         gitWorktree.IsMain,
		IsDirty:          dirty,
		IsLocked:         gitWorktree.Locked,
		RebaseInProgress: gitWorktree.RebaseInProgress,
		CommitsAhead:     ahead,
		CreatedAt:        worktreeCreatedAt(params.StateDir, gitWorktree.Branch, gitWorktree.Path),
		OriginAhead:      originAB.Ahead,
		OriginBehind:     originAB.Behind,
		OriginState:      originState,
	}
}

// worktreeCreatedAt reads the recorded creation time from the per-worktree
// meta.json. Falls back to the worktree directory mtime when meta is absent
// (e.g. worktrees created outside wtm).
func worktreeCreatedAt(stateDir, branch, fallbackPath string) time.Time {
	metaPath := filepath.Join(rules.WorktreeMetaDir(stateDir, branch), domain.MetaFileName)
	data, err := os.ReadFile(metaPath)
	if err == nil {
		var meta domain.WorktreeMetadata
		if json.Unmarshal(data, &meta) == nil && meta.CreatedAt != "" {
			t, parseErr := time.Parse(time.RFC3339, meta.CreatedAt)
			if parseErr == nil {
				return t
			}
		}
	}

	info, err := os.Stat(fallbackPath)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

type ListAllParams struct {
	ProjectDir string
}

// ListAll returns the raw worktrees; List enriches them with metadata, divergence
// and dirtiness.
func ListAll(ctx context.Context, params ListAllParams) ([]domain.GitWorktree, error) {
	return infra.ListWorktrees(ctx, infra.ListWorktreesParams{ProjectDir: params.ProjectDir})
}

type LastFetchAtParams struct {
	ProjectDir string
}

// LastFetchAt dates the last successful fetch, zero when the repository has
// never fetched. A thin wrapper so callers above service/ (the dashboard's
// header, in particular) never reach into infra/ directly.
func LastFetchAt(ctx context.Context, params LastFetchAtParams) time.Time {
	return infra.LastFetchAt(ctx, infra.LastFetchAtParams{ProjectDir: params.ProjectDir})
}

type MainCheckoutParams struct {
	ProjectDir string
}

// MainCheckout is where a shared job runs: the one worktree guaranteed to live
// as long as the repository does, and the one at ordinal 0 — so a shared job's
// declared port is the port it binds. A bare clone has none, and a shared job
// then has nowhere to run rather than silently taking a linked worktree that
// `clean` may remove under it.
func MainCheckout(ctx context.Context, params MainCheckoutParams) (string, error) {
	worktrees, err := infra.ListWorktrees(ctx, infra.ListWorktreesParams{ProjectDir: params.ProjectDir})
	if err != nil {
		return "", err
	}
	for _, candidate := range worktrees {
		if candidate.IsMain {
			return candidate.Path, nil
		}
	}
	return "", domain.ErrNoMainCheckout
}
