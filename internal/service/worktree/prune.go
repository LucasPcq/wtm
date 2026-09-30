package worktree

import (
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
)

// PlanPrune gathers the data ClassifyPrune needs — worktree statuses, the parent
// graph, PR states, and (for the Gone filter) a pruning fetch plus per-branch
// upstream-gone probing — then returns the side-effect-free plan. The PR set is
// passed in so the command owns its graceful/spinner behavior.
func PlanPrune(params domain.PruneParams, prs []domain.PRInfo) (domain.PrunePlan, error) {
	if params.Gone && !params.NoFetch {
		// Best-effort, like sync's base fetch: stale local refs are better than a
		// hard failure offline. The user opted into the network cost with --gone.
		_ = infra.FetchPrune(infra.FetchPruneParams{ProjectDir: params.ProjectDir})
	}

	statuses, err := List(domain.ListParams{
		ProjectDir: params.ProjectDir,
		StateDir:   params.StateDir,
		Config:     params.Config,
	})
	if err != nil {
		return domain.PrunePlan{}, err
	}

	nodes, err := buildNodes(params.ProjectDir, params.StateDir)
	if err != nil {
		return domain.PrunePlan{}, err
	}

	gone := map[string]bool{}
	if params.Gone {
		gone = computeGone(params.ProjectDir, statuses)
	}

	// Unpushed commits make a candidate unsafe to remove (like clean), so probe
	// every listed worktree — the guard applies regardless of the active filter.
	unpushed := computeUnpushed(params.ProjectDir, statuses)

	return rules.ClassifyPrune(rules.ClassifyPruneParams{
		Statuses:   statuses,
		Nodes:      nodes,
		PRStates:   prStates(prs),
		Gone:       gone,
		Unpushed:   unpushed,
		Merged:     params.Merged,
		Closed:     params.Closed,
		GoneFilter: params.Gone,
		BaseBranch: params.BaseBranch,
		Force:      params.Force,
	}), nil
}

// prStates maps each branch to its normalized PR state, keeping the first hit
// (ListPRsAllStates returns newest-first).
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

// computeGone probes, concurrently, whether each worktree's branch has a deleted
// upstream ("[gone]"). Bounded by statusWorkers.
func computeGone(projectDir string, statuses []domain.WorktreeStatus) map[string]bool {
	result := make(map[string]bool, len(statuses))
	var mu sync.Mutex

	sem := make(chan struct{}, statusWorkers)
	var wg sync.WaitGroup
	for _, st := range statuses {
		wg.Add(1)
		sem <- struct{}{}
		go func(branch string) {
			defer wg.Done()
			defer func() { <-sem }()
			if !infra.UpstreamGone(infra.UpstreamGoneParams{ProjectDir: projectDir, Branch: branch}) {
				return
			}
			mu.Lock()
			result[branch] = true
			mu.Unlock()
		}(st.Branch)
	}
	wg.Wait()

	return result
}

// computeUnpushed probes, concurrently, how many local commits each worktree's
// branch has that are not on its remote. Bounded by statusWorkers. Branches with
// no remote tracking ref report 0 (nothing to lose on the remote).
func computeUnpushed(projectDir string, statuses []domain.WorktreeStatus) map[string]int {
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
			n, _ := infra.UnpushedCommits(infra.UnpushedCommitsParams{ProjectDir: projectDir, Branch: branch})
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
