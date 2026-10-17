package worktree

import (
	"context"
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
)

type PlanPruneParams struct {
	Prune domain.PruneParams
	// PRs is awaited only once the git side of the scan is done, so a lookup
	// started beforehand runs alongside it.
	PRs func() []domain.PRInfo
}

// PlanPrune gathers what ClassifyPrune reasons over — worktree statuses, the
// parent graph, upstream state, PR states — then returns the side-effect-free
// plan. Every git probe is per worktree, never per branch of the repository.
func PlanPrune(ctx context.Context, params PlanPruneParams) (domain.PrunePlan, error) {
	prune := params.Prune
	worktrees, err := infra.ListWorktrees(ctx, infra.ListWorktreesParams{ProjectDir: prune.ProjectDir})
	if err != nil {
		return domain.PrunePlan{}, err
	}
	branches := worktreeBranches(worktrees)

	if prune.Gone && !prune.NoFetch {
		// Best-effort, like sync's base fetch: stale local refs are better than a
		// hard failure offline.
		_ = refreshUpstreams(ctx, refreshUpstreamsParams{ProjectDir: prune.ProjectDir, Branches: branches})
	}

	statuses, err := List(ctx, domain.ListParams{
		ProjectDir: prune.ProjectDir,
		StateDir:   prune.StateDir,
		Config:     prune.Config,
	})
	if err != nil {
		return domain.PrunePlan{}, err
	}

	nodes, err := buildNodes(ctx, prune.ProjectDir, prune.StateDir)
	if err != nil {
		return domain.PrunePlan{}, err
	}

	gone := map[string]bool{}
	if prune.Gone {
		gone = computeGone(ctx, prune.ProjectDir, branches)
	}

	// Unpushed commits make a candidate unsafe to remove (like clean), so probe
	// every listed worktree — the guard applies regardless of the active filter.
	unpushed := computeUnpushed(ctx, prune.ProjectDir, statuses)
	prs := params.PRs()
	// Every probe above reads a failure as "nothing to report": cut short by an
	// interrupt, they would plan a branch with unpushed commits as safe to go.
	if err := ctx.Err(); err != nil {
		return domain.PrunePlan{}, err
	}

	return rules.ClassifyPrune(rules.ClassifyPruneParams{
		Statuses:   statuses,
		Nodes:      nodes,
		PRStates:   prStates(prs),
		Gone:       gone,
		Unpushed:   unpushed,
		Merged:     prune.Merged,
		Closed:     prune.Closed,
		GoneFilter: prune.Gone,
		BaseBranch: prune.BaseBranch,
		Force:      prune.Force,
	}), nil
}

type WorktreeBranchesParams struct {
	ProjectDir string
}

// WorktreeBranches names the branch of every worktree, the main one included.
func WorktreeBranches(ctx context.Context, params WorktreeBranchesParams) ([]string, error) {
	worktrees, err := infra.ListWorktrees(ctx, infra.ListWorktreesParams{ProjectDir: params.ProjectDir})
	if err != nil {
		return nil, err
	}
	return worktreeBranches(worktrees), nil
}

func worktreeBranches(worktrees []domain.GitWorktree) []string {
	branches := make([]string, 0, len(worktrees))
	for _, worktree := range worktrees {
		if worktree.Branch != "" {
			branches = append(branches, worktree.Branch)
		}
	}
	return branches
}

type refreshUpstreamsParams struct {
	ProjectDir string
	Branches   []string
}

// refreshUpstreams is `git fetch --prune origin` narrowed to the worktrees'
// branches: what origin no longer has loses its remote-tracking ref, the rest
// is fetched. A repository's other branches are neither fetched nor pruned,
// which is what keeps prune's cost proportional to its worktrees.
func refreshUpstreams(ctx context.Context, params refreshUpstreamsParams) error {
	upstreams, err := infra.Upstreams(ctx, infra.UpstreamsParams{ProjectDir: params.ProjectDir, Branches: params.Branches})
	if err != nil {
		return err
	}
	tracking := rules.OriginTrackingRefs(rules.OriginTrackingRefsParams{Branches: params.Branches, Upstreams: upstreams})
	remoteRefs := make([]string, 0, len(tracking))
	for remoteRef := range tracking {
		remoteRefs = append(remoteRefs, remoteRef)
	}
	existing, err := infra.ExistingRemoteRefs(ctx, infra.RemoteRefsParams{
		ProjectDir: params.ProjectDir,
		Remote:     domain.OriginRemote,
		Refs:       remoteRefs,
	})
	if err != nil {
		return err
	}

	var present, deleted []string
	for _, remoteRef := range remoteRefs {
		if existing[remoteRef] {
			present = append(present, remoteRef)
			continue
		}
		deleted = append(deleted, tracking[remoteRef])
	}
	if err := infra.DeleteRefs(ctx, infra.DeleteRefsParams{ProjectDir: params.ProjectDir, Refs: deleted}); err != nil {
		return err
	}
	return infra.FetchRemoteRefs(ctx, infra.RemoteRefsParams{
		ProjectDir: params.ProjectDir,
		Remote:     domain.OriginRemote,
		Refs:       present,
	})
}

// prStates maps each branch to its normalized PR state, keeping the first hit
// (PR lookups answer newest-first).
func prStates(prs []domain.PRInfo) map[string]string {
	states := make(map[string]string, len(prs))
	for _, pr := range prs {
		if _, seen := states[pr.Branch]; seen {
			continue
		}
		states[pr.Branch] = pr.State
	}
	return states
}

// computeGone reads whether each branch has a deleted upstream ("[gone]").
func computeGone(ctx context.Context, projectDir string, branches []string) map[string]bool {
	gone := map[string]bool{}
	upstreams, err := infra.Upstreams(ctx, infra.UpstreamsParams{ProjectDir: projectDir, Branches: branches})
	if err != nil {
		return gone
	}
	for branch, upstream := range upstreams {
		if upstream.Gone {
			gone[branch] = true
		}
	}
	return gone
}

// computeUnpushed probes, concurrently, how many local commits each worktree's
// branch has that are not on its remote. Bounded by statusWorkers. Branches with
// no remote tracking ref report 0 (nothing to lose on the remote).
func computeUnpushed(ctx context.Context, projectDir string, statuses []domain.WorktreeStatus) map[string]int {
	result := make(map[string]int, len(statuses))
	var mu sync.Mutex

	sem := make(chan struct{}, statusWorkers)
	var wg sync.WaitGroup
	for _, st := range statuses {
		wg.Add(1)
		sem <- struct{}{}
		go func(branch string) {
			defer wg.Done()
			defer func() { <-sem }()
			n, _ := infra.UnpushedCommits(ctx, infra.UnpushedCommitsParams{ProjectDir: projectDir, Branch: branch})
			if n == 0 {
				return
			}
			mu.Lock()
			result[branch] = n
			mu.Unlock()
		}(st.Branch)
	}
	wg.Wait()

	return result
}
