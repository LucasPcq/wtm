package worktree

import (
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/github"
)

// BuildTreeParams holds inputs for assembling the worktree forest.
type BuildTreeParams struct {
	ProjectDir string
	StateDir   string
	Config     domain.Config
	// WithPRs annotates each worktree with its newest pull request, whatever
	// its state. Without gh, the annotations are omitted.
	WithPRs bool
}

// BuildTree assembles the rendered forest of worktrees: it loads the parent
// links and per-worktree status (reusing List), computes the "needs sync"
// signal per node, attaches any matching PR, and hands the enriched nodes to
// rules.BuildForest. It performs no rendering and never fails on a bad parent
// chain (cycles are broken by the rules layer).
func BuildTree(params BuildTreeParams) (domain.Forest, error) {
	nodes, err := buildNodes(params.ProjectDir, params.StateDir)
	if err != nil {
		return domain.Forest{}, err
	}
	awaitPRs := startTreePRLookup(treePRLookupParams{ProjectDir: params.ProjectDir, Nodes: nodes, WithPRs: params.WithPRs})

	statuses, err := List(domain.ListParams{
		ProjectDir: params.ProjectDir,
		StateDir:   params.StateDir,
		Config:     params.Config,
	})
	if err != nil {
		return domain.Forest{}, err
	}
	statusByBranch := make(map[string]domain.WorktreeStatus, len(statuses))
	for _, s := range statuses {
		statusByBranch[s.Branch] = s
	}

	needsSync := computeNeedsSync(nodes)
	prs := awaitPRs()

	forestNodes := make([]rules.ForestNode, 0, len(nodes))
	for _, n := range nodes {
		st := statusByBranch[n.Branch]
		forestNodes = append(forestNodes, rules.ForestNode{
			Branch:       n.Branch,
			Path:         n.Path,
			SourceBranch: n.SourceBranch,
			IsMain:       n.IsMain,
			CreatedAt:    st.CreatedAt,
			Status: domain.TreeNodeStatus{
				CommitsAhead:     st.CommitsAhead,
				IsDirty:          st.IsDirty,
				IsLocked:         st.IsLocked,
				RebaseInProgress: st.RebaseInProgress,
				NeedsSync:        needsSync[n.Branch],
				OriginAhead:      st.OriginAhead,
				OriginBehind:     st.OriginBehind,
				OriginState:      st.OriginState,
				PR:               matchTreePR(n.Branch, prs),
			},
		})
	}

	return rules.BuildForest(rules.BuildForestParams{
		Nodes: forestNodes,
	}), nil
}

type treePRLookupParams struct {
	ProjectDir string
	Nodes      []domain.WorktreeNode
	WithPRs    bool
}

// startTreePRLookup asks GitHub while the tree's git probes run.
func startTreePRLookup(params treePRLookupParams) func() []domain.PRInfo {
	if !params.WithPRs {
		return func() []domain.PRInfo { return nil }
	}
	branches := make([]string, 0, len(params.Nodes))
	for _, node := range params.Nodes {
		if node.Branch != "" {
			branches = append(branches, node.Branch)
		}
	}
	done := make(chan []domain.PRInfo, 1)
	go func() {
		prs, _ := github.ListPRsOfBranches(github.ListPRsOfBranchesParams{ProjectDir: params.ProjectDir, Branches: branches})
		done <- prs
	}()
	return sync.OnceValue(func() []domain.PRInfo { return <-done })
}

// computeNeedsSync probes, per non-main worktree, whether its parent tip has
// advanced past it: the parent ref must resolve locally and not be an ancestor
// of the child branch. Probes run concurrently (two git calls each). A parent
// branch that does not exist locally yields no signal (false).
func computeNeedsSync(nodes []domain.WorktreeNode) map[string]bool {
	result := make(map[string]bool, len(nodes))
	var mu sync.Mutex

	sem := make(chan struct{}, statusWorkers)
	var wg sync.WaitGroup
	for _, n := range nodes {
		if n.IsMain || n.SourceBranch == "" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(node domain.WorktreeNode) {
			defer wg.Done()
			defer func() { <-sem }()
			if !parentAdvanced(node) {
				return
			}
			mu.Lock()
			result[node.Branch] = true
			mu.Unlock()
		}(n)
	}
	wg.Wait()

	return result
}

// parentAdvanced reports whether the node's parent tip is NOT an ancestor of the
// node's branch — i.e. the parent moved and the child needs a rebase. A parent
// ref that cannot be resolved from the child worktree returns false (no signal).
func parentAdvanced(node domain.WorktreeNode) bool {
	if _, err := infra.Tip(infra.TipParams{
		WorktreePath: node.Path,
		Ref:          node.SourceBranch,
	}); err != nil {
		return false
	}
	return !infra.IsAncestor(infra.IsAncestorParams{
		WorktreePath: node.Path,
		Ancestor:     node.SourceBranch,
		Descendant:   node.Branch,
	})
}

// matchTreePR returns the PR summary for a branch, or nil when none matches.
func matchTreePR(branch string, prs []domain.PRInfo) *domain.WorktreeListPR {
	for _, pr := range prs {
		if pr.Branch == branch {
			return &domain.WorktreeListPR{Number: pr.Number, URL: pr.URL, State: pr.State}
		}
	}
	return nil
}
