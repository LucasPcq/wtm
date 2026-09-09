package rules

import (
	"github.com/LucasPcq/wtm/internal/domain"
)

// ClassifyPruneParams holds the pre-gathered data ClassifyPrune reasons over. All
// I/O (worktree listing, PR fetch, upstream-gone probing, current-worktree path
// resolution) is done by the caller so this stays pure.
type ClassifyPruneParams struct {
	Statuses []domain.WorktreeStatus
	Nodes    []domain.WorktreeNode
	// PRStates maps a branch to its normalized PR state (open/merged/closed).
	PRStates map[string]string
	// Gone maps a branch to whether its upstream tracking ref was deleted.
	Gone map[string]bool
	// Unpushed maps a branch to its count of local commits not on the remote.
	// A candidate with unpushed commits is unsafe to remove without Force.
	Unpushed map[string]int
	// Active reason filters.
	Merged     bool
	Closed     bool
	GoneFilter bool
	BaseBranch string
	// Force allows dirty worktrees to be selected instead of skipped.
	Force bool
}

// ClassifyPrune decides, without side effects, which worktrees a prune would
// remove, which it skips (and why), and how surviving children reparent onto
// their grandparent. A worktree is considered only when it matches an active
// filter; matches that are protected (base, main, current) or dirty (without
// Force) are recorded as skips.
func ClassifyPrune(params ClassifyPruneParams) domain.PrunePlan {
	nodeByBranch := make(map[string]domain.WorktreeNode, len(params.Nodes))
	for _, n := range params.Nodes {
		nodeByBranch[n.Branch] = n
	}

	var plan domain.PrunePlan
	selected := make(map[string]bool)

	for _, st := range params.Statuses {
		reason, matched := pruneMatchReason(st, params)
		if !matched {
			continue
		}

		node := nodeByBranch[st.Branch]
		if protectReason, protected := pruneProtectReason(pruneProtectParams{
			Status:     st,
			Node:       node,
			BaseBranch: params.BaseBranch,
		}); protected {
			plan.Skipped = append(plan.Skipped, domain.PruneSkip{Branch: st.Branch, Reason: protectReason})
			continue
		}

		// Dirty / unpushed / open-PR are unsafe to remove without --force (mirrors
		// clean). Without Force they skip here; with Force the candidate is selected
		// but tagged so the interactive confirm and FinalizePrunePlan can re-gate it.
		unsafe := pruneUnsafeReason(pruneUnsafeParams{
			Status:    st,
			Unpushed:  params.Unpushed[st.Branch],
			HasOpenPR: params.PRStates[st.Branch] == domain.PRStateOpen,
		})
		if unsafe != "" && !params.Force {
			plan.Skipped = append(plan.Skipped, domain.PruneSkip{Branch: st.Branch, Reason: unsafe})
			continue
		}

		plan.Selected = append(plan.Selected, domain.PruneCandidate{
			Branch:       st.Branch,
			Path:         st.Path,
			Reason:       reason,
			IsDirty:      st.IsDirty,
			UnsafeReason: unsafe,
			SourceBranch: node.SourceBranch,
		})
		selected[st.Branch] = true
	}

	plan.Reparents = pruneReparents(pruneReparentsParams{
		Selected:   plan.Selected,
		SelectedIn: selected,
		Nodes:      params.Nodes,
		BaseBranch: params.BaseBranch,
	})
	return plan
}

// pruneMatchReason returns the category that makes a worktree prunable under the
// active reason filters. "Done" is decided from GitHub as the source of truth,
// never from local commit topology (which can't tell a merged branch from a new
// or stale one and misses squash/rebase merges): --merged matches a merged PR,
// --closed a PR closed without merging, --gone a deleted upstream branch. A
// branch with no PR (or an open one) is never merged/closed.
func pruneMatchReason(st domain.WorktreeStatus, params ClassifyPruneParams) (string, bool) {
	switch params.PRStates[st.Branch] {
	case domain.PRStateMerged:
		if params.Merged {
			return domain.PruneReasonPRMerged, true
		}
	case domain.PRStateClosed:
		if params.Closed {
			return domain.PruneReasonPRClosed, true
		}
	}
	if params.GoneFilter && params.Gone[st.Branch] {
		return domain.PruneReasonGone, true
	}
	return "", false
}

// PruneClassifyForceParams holds inputs for PruneClassifyForce.
type PruneClassifyForceParams struct {
	// Force is the --force flag: the safety axis.
	Force bool
	// Interactive reports that someone can still answer for the selection.
	Interactive bool
	// DryRun reports a preview: it mutates nothing, so nobody is asked.
	DryRun bool
}

// PruneClassifyForce decides whether classification runs with force, which is a
// different question from whether unsafe worktrees are actually removed.
// An interactive run classifies with force so dirty, unpushed and open-PR
// worktrees surface as candidates left unchecked, the confirmation deciding
// afterwards (FinalizePrunePlan drops them again unless it was forced). A dry
// run has nobody to decide, so it honours --force as given — classifying it with
// force would make a preview list worktrees a real run would have skipped.
func PruneClassifyForce(params PruneClassifyForceParams) bool {
	if params.Force {
		return true
	}
	return params.Interactive && !params.DryRun
}

// PruneGHNotice returns the advisory to surface when prune needs GitHub PR data
// but the gh CLI is unavailable. Because merged/closed detection is GitHub-truth,
// without gh only --gone applies. show is false when the CLI is reachable.
func PruneGHNotice(conn domain.GHConnection) (title string, lines []string, show bool) {
	const dependency = "prune detects merged & closed PRs via the GitHub CLI — without it, only --gone applies."
	switch conn {
	case domain.GHConnectionNotInstalled:
		return "GitHub CLI not found", []string{dependency, "Install it: https://cli.github.com"}, true
	case domain.GHConnectionNotAuthenticated:
		return "GitHub not connected", []string{dependency, "Connect: run `gh auth login`"}, true
	default:
		return "", nil, false
	}
}

type pruneProtectParams struct {
	Status     domain.WorktreeStatus
	Node       domain.WorktreeNode
	BaseBranch string
}

// pruneProtectReason reports whether a matched worktree is unconditionally spared
// (the base branch or the main worktree), and why. These protections hold even
// under --force. The current worktree is intentionally NOT spared — like clean,
// prune removes it and redirects the shell to the base repo afterwards.
func pruneProtectReason(params pruneProtectParams) (string, bool) {
	if params.Status.Branch == params.BaseBranch {
		return domain.PruneSkipBase, true
	}
	if params.Node.IsMain || params.Status.IsParent {
		return domain.PruneSkipMain, true
	}
	return "", false
}

type pruneUnsafeParams struct {
	Status    domain.WorktreeStatus
	Unpushed  int
	HasOpenPR bool
}

// pruneUnsafeReason reports why a worktree is unsafe to remove without --force,
// mirroring CleanUnsafeReason's precedence (dirty → unpushed → open PR).
// An empty string means the worktree is safe to prune.
func pruneUnsafeReason(params pruneUnsafeParams) string {
	if params.Status.IsDirty {
		return domain.PruneSkipDirty
	}
	if params.Unpushed > 0 {
		return domain.PruneSkipUnpushed
	}
	if params.HasOpenPR {
		return domain.PruneSkipOpenPR
	}
	return ""
}

type pruneReparentsParams struct {
	Selected   []domain.PruneCandidate
	SelectedIn map[string]bool
	Nodes      []domain.WorktreeNode
	BaseBranch string
}

// pruneReparents computes the grandparent moves for children of pruned worktrees.
// A child that is itself pruned is skipped; a grandparent that is also pruned
// falls back to the base branch so no child is left pointing at a removed parent.
func pruneReparents(params pruneReparentsParams) []domain.ReparentResult {
	var reparents []domain.ReparentResult
	for _, cand := range params.Selected {
		grandparent := cand.SourceBranch
		if grandparent == "" || params.SelectedIn[grandparent] {
			grandparent = params.BaseBranch
		}
		children := ChildrenOf(ChildrenOfParams{Nodes: params.Nodes, Branch: cand.Branch})
		for _, child := range children {
			if params.SelectedIn[child.Branch] {
				continue
			}
			reparents = append(reparents, domain.ReparentResult{
				Branch:    child.Branch,
				OldParent: cand.Branch,
				NewParent: grandparent,
			})
		}
	}
	return reparents
}

// FinalizePrunePlanParams holds inputs for FinalizePrunePlan.
type FinalizePrunePlanParams struct {
	Plan       domain.PrunePlan
	Chosen     []string
	BaseBranch string
	// Force keeps unsafe worktrees (dirty/unpushed/open-PR) in the selection;
	// without it, a chosen unsafe worktree is dropped back to Skipped (the
	// interactive confirm gates this).
	Force bool
}

// FinalizePrunePlan restricts a full prune plan to the user-chosen subset of
// candidates (after an interactive deselect) and recomputes reparenting so no
// surviving worktree is left pointing at a removed parent. It works purely from
// the plan — each candidate carries its SourceBranch, so parent chains among
// pruned worktrees are reconstructable without the node graph. Without Force, a
// chosen unsafe worktree (dirty/unpushed/open-PR) is moved to Skipped rather than
// removed. Skips carry through unchanged.
func FinalizePrunePlan(params FinalizePrunePlanParams) domain.PrunePlan {
	chosen := make(map[string]bool, len(params.Chosen))
	for _, b := range params.Chosen {
		chosen[b] = true
	}

	candByBranch := make(map[string]domain.PruneCandidate, len(params.Plan.Selected))
	for _, c := range params.Plan.Selected {
		candByBranch[c.Branch] = c
	}

	out := domain.PrunePlan{Skipped: params.Plan.Skipped}

	// A chosen-but-unsafe worktree (dirty/unpushed/open-PR) is only removed with
	// Force; otherwise drop it from the selection and record it as skipped with
	// the reason that made it unsafe.
	if !params.Force {
		for _, c := range params.Plan.Selected {
			if c.UnsafeReason != "" && chosen[c.Branch] {
				chosen[c.Branch] = false
				out.Skipped = append(out.Skipped, domain.PruneSkip{Branch: c.Branch, Reason: c.UnsafeReason})
			}
		}
	}

	for _, c := range params.Plan.Selected {
		if chosen[c.Branch] {
			out.Selected = append(out.Selected, c)
		}
	}

	// Surviving children of a still-chosen pruned parent keep their move (its
	// target was already resolved to a survivor or the base).
	for _, r := range params.Plan.Reparents {
		if chosen[r.OldParent] {
			out.Reparents = append(out.Reparents, r)
		}
	}

	// A candidate the user deselected whose parent is still being pruned now
	// survives and must reparent onto its first surviving ancestor.
	for _, c := range params.Plan.Selected {
		if chosen[c.Branch] || !chosen[c.SourceBranch] {
			continue
		}
		out.Reparents = append(out.Reparents, domain.ReparentResult{
			Branch:    c.Branch,
			OldParent: c.SourceBranch,
			NewParent: survivingAncestor(candByBranch, chosen, c.SourceBranch, params.BaseBranch),
		})
	}

	return out
}

// survivingAncestor walks up the pruned-candidate parent chain from start until it
// reaches a branch that is not being pruned, falling back to base.
func survivingAncestor(cands map[string]domain.PruneCandidate, chosen map[string]bool, start, base string) string {
	cur := start
	for chosen[cur] {
		next := cands[cur].SourceBranch
		if next == "" {
			return base
		}
		cur = next
	}
	if cur == "" {
		return base
	}
	return cur
}

// PruneReasonLabel turns a prune reason or skip constant into the short phrase
// both surfaces show. It maps, it does not decide — a reason it does not know is
// shown as it is stored rather than swallowed.
func PruneReasonLabel(reason string) string {
	switch reason {
	case domain.PruneReasonPRMerged:
		return domain.PruneLabelPRMerged
	case domain.PruneReasonPRClosed:
		return domain.PruneLabelPRClosed
	case domain.PruneReasonGone:
		return domain.PruneLabelGone
	case domain.PruneSkipBase:
		return domain.PruneLabelBase
	case domain.PruneSkipMain:
		return domain.PruneLabelMain
	case domain.PruneSkipDirty:
		return domain.PruneLabelDirty
	case domain.PruneSkipUnpushed:
		return domain.PruneLabelUnpushed
	case domain.PruneSkipOpenPR:
		return domain.PruneLabelOpenPR
	default:
		return reason
	}
}

// PrunedBranches names what a prune removed, in the order it removed them. It is
// the one list a destructive result keeps: what is gone is actionable, even when
// the picker showed it twice already.
func PrunedBranches(result domain.PruneResult) []string {
	names := make([]string, 0, len(result.Pruned))
	for _, c := range result.Pruned {
		names = append(names, c.Branch)
	}
	return names
}
