package worktree

import (
	"context"
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
)

// SyncParams holds inputs for planning and running a cascade sync.
type SyncParams struct {
	ProjectDir string
	StateDir   string
	Config     domain.Config
	BaseBranch string
	DryRun     bool
	// KeepConflict leaves a conflicting rebase in progress in its worktree (for
	// manual resolution) instead of aborting it. Descendants are still skipped;
	// independent branches keep syncing, so several worktrees may be left
	// mid-rebase.
	KeepConflict bool
	// SelectedBranches restricts the cascade to the named worktrees (topological
	// order is preserved). An empty slice means every managed worktree.
	SelectedBranches []string
	// FastForwardParents advances the parents no step covers to origin/<parent>
	// before their children rebase onto them. Without it they are reported stale
	// but left as they are; a diverged one is never touched either way.
	FastForwardParents bool
}

// PlanSync builds the ordered cascade plan without touching any branch. Used to
// preview the operation and surface ordering errors (e.g. cycles) before acting.
func PlanSync(ctx context.Context, params SyncParams) (domain.SyncPlan, error) {
	nodes, err := buildNodes(ctx, params.ProjectDir, params.StateDir)
	if err != nil {
		return domain.SyncPlan{}, err
	}
	plan, err := rules.BuildSyncPlan(rules.BuildSyncPlanParams{
		Nodes:      nodes,
		BaseBranch: params.BaseBranch,
	})
	if err != nil {
		return domain.SyncPlan{}, err
	}
	plan.Steps = rules.FilterSyncSteps(plan.Steps, params.SelectedBranches)
	plan.BaseTargeted = rules.BaseIsTarget(rules.BaseIsTargetParams{
		Steps:      plan.Steps,
		Selected:   params.SelectedBranches,
		BaseBranch: params.BaseBranch,
	})
	return plan, nil
}

// Sync runs the cascade: it fetches and fast-forwards the base, then rebases
// every managed worktree onto its (refreshed) parent in topological order. The
// rebases are entirely local — pushing is a separate step (see PushSynced).
func Sync(ctx context.Context, params SyncParams) (domain.SyncResult, error) {
	nodes, err := buildNodes(ctx, params.ProjectDir, params.StateDir)
	if err != nil {
		return domain.SyncResult{}, err
	}
	plan, err := rules.BuildSyncPlan(rules.BuildSyncPlanParams{
		Nodes:      nodes,
		BaseBranch: params.BaseBranch,
	})
	if err != nil {
		return domain.SyncResult{}, err
	}
	plan.Steps = rules.FilterSyncSteps(plan.Steps, params.SelectedBranches)

	mainPath := mainWorktreePath(nodes, params.ProjectDir)
	oldTips := captureTips(ctx, captureTipsParams{
		MainPath:   mainPath,
		BaseBranch: params.BaseBranch,
		Steps:      plan.Steps,
	})

	baseTargeted := rules.BaseIsTarget(rules.BaseIsTargetParams{
		Steps:      plan.Steps,
		Selected:   params.SelectedBranches,
		BaseBranch: params.BaseBranch,
	})

	// An untargeted base is left out of the JSON too, not just the text recap:
	// reporting a tip for a ref the run never read would invite a consumer to act
	// on it.
	baseTip := ""
	if baseTargeted {
		baseTip = oldTips[params.BaseBranch]
	}

	result := domain.SyncResult{
		BaseBranch:       params.BaseBranch,
		BaseTargeted:     baseTargeted,
		BaseOldTip:       baseTip,
		BaseNewTip:       baseTip,
		SelectedBranches: stepBranches(plan.Steps),
	}

	if !params.DryRun && baseTargeted {
		newTip, updated := updateBase(ctx, updateBaseParams{
			MainPath:   mainPath,
			BaseBranch: params.BaseBranch,
		})
		result.BaseUpdated = updated
		result.BaseNewTip = newTip
	}

	// Parents outside the cascade are reconciled BEFORE the steps run, so a child
	// is rebased onto the refreshed parent rather than a stale ref. Dry-run stays
	// offline — it neither fetches nor moves anything — but still reports what the
	// cached remote-tracking refs already show, so the preview names the problem.
	result.ParentUpdates = reconcileParents(ctx, reconcileParentsParams{
		Nodes:       nodes,
		Steps:       plan.Steps,
		MainPath:    mainPath,
		BaseBranch:  params.BaseBranch,
		FastForward: params.FastForwardParents,
		Offline:     params.DryRun,
	})

	skipped := make(map[string]bool)
	for _, step := range plan.Steps {
		if ctx.Err() != nil {
			result.Steps = append(result.Steps, cancelledStep(cancelledStepParams{Step: step, OldTips: oldTips, Detail: domain.SyncNotReachedDetail}))
			continue
		}
		stepResult, blocks := evaluateStep(ctx, stepEval{
			Step:         step,
			OldTips:      oldTips,
			Skipped:      skipped,
			DryRun:       params.DryRun,
			KeepConflict: params.KeepConflict,
		})
		if blocks {
			skipped[step.Branch] = true
		}
		if ctx.Err() != nil && rules.SyncStepCutShort(stepResult) {
			stepResult = cancelledStep(cancelledStepParams{Step: step, OldTips: oldTips, Detail: domain.SyncCancelledDetail})
		}
		result.Steps = append(result.Steps, stepResult)
	}

	return result, nil
}

// ClassifyParentsParams holds inputs for ClassifyParents.
type ClassifyParentsParams struct {
	ProjectDir string
	StateDir   string
	BaseBranch string
}

// ClassifyParents inspects every recorded parent against its remote, changing
// nothing. A wizard runs it before its first question: which parents a selection
// leaves uncovered is only known step by step, their state against origin is not
// — so the network work happens once and the wizard filters it locally.
func ClassifyParents(ctx context.Context, params ClassifyParentsParams) ([]domain.ParentUpdate, error) {
	nodes, err := buildNodes(ctx, params.ProjectDir, params.StateDir)
	if err != nil {
		return nil, err
	}

	mainPath := mainWorktreePath(nodes, params.ProjectDir)
	pathByBranch := make(map[string]string, len(nodes))
	for _, node := range nodes {
		pathByBranch[node.Branch] = node.Path
	}

	branches := rules.ParentBranches(rules.ParentBranchesParams{
		Nodes:      nodes,
		BaseBranch: params.BaseBranch,
	})
	if len(branches) == 0 {
		return nil, nil
	}
	infra.Fetch(ctx, infra.FetchParams{ProjectDir: mainPath})

	updates := make([]domain.ParentUpdate, 0, len(branches))
	for _, branch := range branches {
		update, ok := reconcileParent(ctx, reconcileParentParams{
			Parent:       domain.ParentUpdate{Branch: branch},
			WorktreePath: pathByBranch[branch],
			MainPath:     mainPath,
		})
		if !ok {
			continue
		}
		updates = append(updates, update)
	}
	return updates, nil
}

// StaleParentsParams holds inputs for StaleParents.
type StaleParentsParams struct {
	Sync       SyncParams
	Branches   []string
	Classified []domain.ParentUpdate
}

// StaleParents narrows a ClassifyParents inspection to what the given selection
// leaves uncovered. No network, so a wizard can call it while building a step.
func StaleParents(ctx context.Context, params StaleParentsParams) []domain.ParentUpdate {
	preview := params.Sync
	preview.SelectedBranches = params.Branches
	plan, err := PlanSync(ctx, preview)
	if err != nil {
		return nil
	}
	return rules.StaleParentsFor(rules.StaleParentsForParams{
		Uncovered: rules.ParentsOutsideCascade(rules.ParentsOutsideCascadeParams{
			Steps:      plan.Steps,
			BaseBranch: params.Sync.BaseBranch,
		}),
		Classified: params.Classified,
	})
}

type reconcileParentsParams struct {
	Nodes       []domain.WorktreeNode
	Steps       []domain.SyncStep
	MainPath    string
	BaseBranch  string
	FastForward bool
	// Offline classifies against the refs already on disk, skipping fetch and any
	// move: it is what lets --dry-run report a stale parent.
	Offline bool
}

func reconcileParents(ctx context.Context, params reconcileParentsParams) []domain.ParentUpdate {
	pathByBranch := make(map[string]string, len(params.Nodes))
	for _, node := range params.Nodes {
		pathByBranch[node.Branch] = node.Path
	}

	pending := rules.ParentsOutsideCascade(rules.ParentsOutsideCascadeParams{
		Steps:      params.Steps,
		BaseBranch: params.BaseBranch,
	})
	if len(pending) == 0 {
		return nil
	}

	// One fetch refreshes every origin ref, so the loop below is pure ref
	// arithmetic. A failure is not fatal — the cached refs still say something.
	if !params.Offline {
		infra.Fetch(ctx, infra.FetchParams{ProjectDir: params.MainPath})
	}

	updates := make([]domain.ParentUpdate, 0, len(pending))
	for _, parent := range pending {
		update, ok := reconcileParent(ctx, reconcileParentParams{
			Parent:       parent,
			WorktreePath: pathByBranch[parent.Branch],
			MainPath:     params.MainPath,
			FastForward:  params.FastForward && !params.Offline,
		})
		if !ok {
			continue
		}
		updates = append(updates, update)
	}
	return updates
}

type reconcileParentParams struct {
	Parent domain.ParentUpdate
	// WorktreePath is empty when the parent is a branch with no worktree.
	WorktreePath string
	MainPath     string
	FastForward  bool
}

// reconcileParent returns false when there is nothing worth reporting: no remote
// counterpart, or a local ref that already carries it.
func reconcileParent(ctx context.Context, params reconcileParentParams) (domain.ParentUpdate, bool) {
	update := params.Parent
	gitDir := params.MainPath
	if params.WorktreePath != "" {
		gitDir = params.WorktreePath
	}

	remoteRef := domain.RemoteBranchPrefix + update.Branch
	if _, err := infra.Tip(ctx, infra.TipParams{WorktreePath: gitDir, Ref: remoteRef}); err != nil {
		return domain.ParentUpdate{}, false
	}

	tip, err := infra.Tip(ctx, infra.TipParams{WorktreePath: gitDir, Ref: update.Branch})
	if err != nil {
		return domain.ParentUpdate{}, false
	}
	update.OldTip, update.NewTip = tip, tip

	if infra.IsAncestor(ctx, infra.IsAncestorParams{
		WorktreePath: gitDir,
		Ancestor:     remoteRef,
		Descendant:   update.Branch,
	}) {
		return domain.ParentUpdate{}, false
	}

	if !infra.IsAncestor(ctx, infra.IsAncestorParams{
		WorktreePath: gitDir,
		Ancestor:     update.Branch,
		Descendant:   remoteRef,
	}) {
		// Neither ref is an ancestor of the other, but that is a real divergence
		// only when the remote carries commits not already integrated by patch.
		// After a local rebase of the parent they are all patch-present, so there
		// is nothing to reconcile and nothing to report — the same refinement
		// integrateRemote applies to a step's own branch.
		if !infra.RemoteHasUnintegratedCommits(ctx, infra.RemoteBranchParams{
			WorktreePath: gitDir,
			Branch:       update.Branch,
		}) {
			return domain.ParentUpdate{}, false
		}
		update.Status = domain.ParentDiverged
		return update, true
	}

	update.Status = domain.ParentBehind
	if behind, behindErr := infra.Behind(ctx, infra.BehindParams{
		WorktreePath: gitDir,
		Branch:       update.Branch,
		Upstream:     remoteRef,
	}); behindErr == nil {
		update.Behind = behind
	}
	if !params.FastForward {
		return update, true
	}
	if ffErr := fastForwardParent(ctx, fastForwardParentParams{
		Branch:       update.Branch,
		WorktreePath: params.WorktreePath,
		ProjectDir:   gitDir,
	}); ffErr != nil {
		update.Status = domain.ParentFFFailed
		update.Detail = ffErr.Error()
		return update, true
	}

	update.Status = domain.ParentFastForwarded
	if newTip, tipErr := infra.Tip(ctx, infra.TipParams{WorktreePath: gitDir, Ref: update.Branch}); tipErr == nil {
		update.NewTip = newTip
	}
	return update, true
}

type fastForwardParentParams struct {
	Branch       string
	WorktreePath string
	ProjectDir   string
}

// fastForwardParent advances a parent to origin/<parent>, returning why it could
// not when it could not. A branch with no worktree is moved by fetching straight
// into its ref; a checked-out one must be advanced inside its own worktree, and
// only while that worktree is clean — the same guard updateBase applies to the
// base.
func fastForwardParent(ctx context.Context, params fastForwardParentParams) error {
	if params.WorktreePath == "" {
		return infra.FastForwardRef(ctx, infra.FastForwardRefParams{
			ProjectDir: params.ProjectDir,
			Branch:     params.Branch,
		})
	}

	dirty, err := infra.IsDirty(ctx, infra.IsDirtyParams{WorktreePath: params.WorktreePath})
	if err != nil {
		return fmt.Errorf("cannot check %s worktree: %w", params.Branch, err)
	}
	if dirty {
		return fmt.Errorf("worktree has uncommitted changes")
	}
	return infra.FastForwardBranch(ctx, infra.FastForwardParams{
		WorktreePath: params.WorktreePath,
		Onto:         domain.RemoteBranchPrefix + params.Branch,
	})
}

// stepBranches lists the branches covered by a cascade's steps — the explicit
// selection or, for --all, every managed worktree. Always non-nil so the JSON
// field marshals as [] rather than null.
func stepBranches(steps []domain.SyncStep) []string {
	branches := make([]string, 0, len(steps))
	for _, step := range steps {
		branches = append(branches, step.Branch)
	}
	return branches
}

// PushSyncedParams holds inputs for pushing the successfully-rebased branches.
type PushSyncedParams struct {
	ProjectDir string
	Result     domain.SyncResult
}

// PushSynced force-pushes (with lease) every branch that was rebased in the run.
// It mutates and returns a copy of the result with Pushed flags set.
func PushSynced(ctx context.Context, params PushSyncedParams) domain.SyncResult {
	worktrees, err := infra.ListWorktrees(ctx, infra.ListWorktreesParams{ProjectDir: params.ProjectDir})
	if err != nil {
		return params.Result
	}

	pathByBranch := make(map[string]string, len(worktrees))
	for _, w := range worktrees {
		pathByBranch[w.Branch] = w.Path
	}

	for i := range params.Result.Steps {
		step := &params.Result.Steps[i]
		if !step.PushPending || step.Pushed {
			continue
		}
		path := pathByBranch[step.Branch]
		if path == "" {
			continue
		}
		if pushErr := infra.PushForceWithLease(ctx, infra.PushForceParams{
			WorktreePath: path,
			Branch:       step.Branch,
		}); pushErr != nil {
			step.Detail = pushErr.Error()
			continue
		}
		step.Pushed = true
	}

	return params.Result
}

type NodesParams struct {
	ProjectDir string
	StateDir   string
}

// Nodes is the managed worktrees with their recorded parents: the graph the
// reparent and sync rules are validated against. flow/ cannot reach infra/, so
// this is where it reads it from.
func Nodes(ctx context.Context, params NodesParams) ([]domain.WorktreeNode, error) {
	return buildNodes(ctx, params.ProjectDir, params.StateDir)
}

func buildNodes(ctx context.Context, projectDir, stateDir string) ([]domain.WorktreeNode, error) {
	worktrees, err := infra.ListWorktrees(ctx, infra.ListWorktreesParams{ProjectDir: projectDir})
	if err != nil {
		return nil, err
	}

	nodes := make([]domain.WorktreeNode, 0, len(worktrees))
	for _, w := range worktrees {
		source := ""
		if !w.IsMain {
			source = loadSourceBranch(stateDir, w.Branch)
		}
		nodes = append(nodes, domain.WorktreeNode{
			Branch:           w.Branch,
			Path:             w.Path,
			SourceBranch:     source,
			IsMain:           w.IsMain,
			RebaseInProgress: w.RebaseInProgress,
		})
	}

	return nodes, nil
}

func mainWorktreePath(nodes []domain.WorktreeNode, projectDir string) string {
	for _, node := range nodes {
		if node.IsMain {
			return node.Path
		}
	}
	return projectDir
}

type captureTipsParams struct {
	MainPath   string
	BaseBranch string
	Steps      []domain.SyncStep
}

// captureTips records every branch tip BEFORE any rebase. The recorded parent
// tip is later used as the rebase upstream so only a child's own commits replay.
func captureTips(ctx context.Context, params captureTipsParams) map[string]string {
	tips := make(map[string]string, len(params.Steps)+1)

	if tip, err := infra.Tip(ctx, infra.TipParams{
		WorktreePath: params.MainPath,
		Ref:          params.BaseBranch,
	}); err == nil {
		tips[params.BaseBranch] = tip
	}

	for _, step := range params.Steps {
		if tip, err := infra.Tip(ctx, infra.TipParams{
			WorktreePath: step.Path,
			Ref:          step.Branch,
		}); err == nil {
			tips[step.Branch] = tip
		}
	}

	return tips
}

type updateBaseParams struct {
	MainPath   string
	BaseBranch string
}

// updateBase fetches origin and fast-forwards the local base branch. A diverged
// base (no fast-forward), missing remote, or a dirty main worktree is non-fatal:
// the cascade then rebases onto the local base as-is. The dirty guard mirrors the
// per-worktree behaviour — the base is only advanced when its worktree is clean.
func updateBase(ctx context.Context, params updateBaseParams) (string, bool) {
	currentTip, _ := infra.Tip(ctx, infra.TipParams{
		WorktreePath: params.MainPath,
		Ref:          params.BaseBranch,
	})

	if dirty, err := infra.IsDirty(ctx, infra.IsDirtyParams{WorktreePath: params.MainPath}); err != nil || dirty {
		return currentTip, false
	}

	if fetchErr := infra.FetchBranch(ctx, infra.FetchBranchParams{
		ProjectDir: params.MainPath,
		Branch:     params.BaseBranch,
	}); fetchErr != nil {
		return currentTip, false
	}

	if ffErr := infra.FastForwardBranch(ctx, infra.FastForwardParams{
		WorktreePath: params.MainPath,
		Onto:         domain.RemoteBranchPrefix + params.BaseBranch,
	}); ffErr != nil {
		return currentTip, false
	}

	newTip, _ := infra.Tip(ctx, infra.TipParams{
		WorktreePath: params.MainPath,
		Ref:          params.BaseBranch,
	})
	return newTip, newTip != currentTip
}

type integrateOutcome int

const (
	integrateProceed integrateOutcome = iota
	integrateDiverged
)

// integrateRemote fast-forwards a branch from its own origin/<branch> before it
// is rebased. Returns integrateDiverged when local and remote have both moved
// (no fast-forward possible). A missing remote branch is a no-op.
func integrateRemote(ctx context.Context, step domain.SyncStep) integrateOutcome {
	if fetchErr := infra.FetchBranch(ctx, infra.FetchBranchParams{
		ProjectDir: step.Path,
		Branch:     step.Branch,
	}); fetchErr != nil {
		return integrateProceed
	}

	remoteRef := domain.RemoteBranchPrefix + step.Branch
	localHasRemote := infra.IsAncestor(ctx, infra.IsAncestorParams{
		WorktreePath: step.Path,
		Ancestor:     remoteRef,
		Descendant:   step.Branch,
	})
	if localHasRemote {
		return integrateProceed
	}

	remoteHasLocal := infra.IsAncestor(ctx, infra.IsAncestorParams{
		WorktreePath: step.Path,
		Ancestor:     step.Branch,
		Descendant:   remoteRef,
	})
	if remoteHasLocal {
		infra.FastForwardBranch(ctx, infra.FastForwardParams{
			WorktreePath: step.Path,
			Onto:         remoteRef,
		})
		return integrateProceed
	}

	// Neither ref is an ancestor of the other. This is a genuine divergence only
	// when the remote carries commits not already integrated (by patch) into the
	// local branch. After a prior local rebase the remote's commits are
	// patch-present locally (just rewritten), so there is nothing to reconcile —
	// proceed and let the rebase see behind == 0 (up_to_date).
	if infra.RemoteHasUnintegratedCommits(ctx, infra.RemoteBranchParams{
		WorktreePath: step.Path,
		Branch:       step.Branch,
	}) {
		return integrateDiverged
	}
	return integrateProceed
}

type stepEval struct {
	Step         domain.SyncStep
	OldTips      map[string]string
	Skipped      map[string]bool
	DryRun       bool
	KeepConflict bool
}

// evaluateStep computes the outcome of one cascade step. The second return value
// reports whether this branch must block its descendants (skip/conflict/error).
type cancelledStepParams struct {
	Step    domain.SyncStep
	OldTips map[string]string
	Detail  string
}

func cancelledStep(params cancelledStepParams) domain.SyncStepResult {
	return domain.SyncStepResult{
		Branch:       params.Step.Branch,
		SourceBranch: params.Step.SourceBranch,
		Path:         params.Step.Path,
		Status:       domain.SyncStatusCancelled,
		OldTip:       params.OldTips[params.Step.Branch],
		NewTip:       params.OldTips[params.Step.Branch],
		Detail:       params.Detail,
	}
}

func evaluateStep(ctx context.Context, e stepEval) (domain.SyncStepResult, bool) {
	result := domain.SyncStepResult{
		Branch:       e.Step.Branch,
		SourceBranch: e.Step.SourceBranch,
		Path:         e.Step.Path,
		OldTip:       e.OldTips[e.Step.Branch],
		NewTip:       e.OldTips[e.Step.Branch],
	}

	if e.Step.SourceBranch == "" {
		result.Status = domain.SyncStatusUnknownParent
		result.Detail = "no recorded parent branch"
		return result, true
	}

	if e.Skipped[e.Step.SourceBranch] {
		result.Status = domain.SyncStatusSkippedAncestor
		result.Detail = "ancestor " + e.Step.SourceBranch + " was not synced"
		return result, true
	}

	// A worktree with a rebase already paused mid-way (e.g. a prior keep-conflict
	// run) cannot be rebased again until it is resolved. Report it distinctly rather
	// than as a generic "uncommitted changes" skip.
	if e.Step.RebaseInProgress {
		result.Status = domain.SyncStatusRebaseInProgress
		result.Detail = "rebase already in progress — finish it with git rebase --continue (or --abort)"
		return result, true
	}

	dirty, dirtyErr := infra.IsDirty(ctx, infra.IsDirtyParams{WorktreePath: e.Step.Path})
	if dirtyErr != nil {
		// Could not determine cleanliness — do not rebase blind. Block the
		// branch (and its descendants) so a real git failure is surfaced rather
		// than silently treated as a clean worktree.
		result.Status = domain.SyncStatusError
		result.Detail = "cannot check working tree: " + dirtyErr.Error()
		return result, true
	}
	if dirty {
		result.Status = domain.SyncStatusSkippedDirty
		result.Detail = "uncommitted changes"
		return result, true
	}

	// Pull the branch's own remote (fast-forward only) before rebasing, so
	// commits merged into origin/<branch> elsewhere are included. A true
	// committed divergence (no fast-forward) is left for the user to reconcile.
	// Skipped in dry-run, which stays fully offline.
	if !e.DryRun && integrateRemote(ctx, e.Step) == integrateDiverged {
		result.Status = domain.SyncStatusDiverged
		result.Detail = "local and origin/" + e.Step.Branch + " have diverged"
		return result, true
	}

	if tip, err := infra.Tip(ctx, infra.TipParams{
		WorktreePath: e.Step.Path,
		Ref:          e.Step.SourceBranch,
	}); err == nil {
		result.OntoTip = tip
	}

	behind, behindErr := infra.Behind(ctx, infra.BehindParams{
		WorktreePath: e.Step.Path,
		Branch:       e.Step.Branch,
		Upstream:     e.Step.SourceBranch,
	})
	if behindErr != nil {
		// A failed behind-count must not be read as "0 → up to date", which
		// would skip the rebase and report a stale branch as synced.
		result.Status = domain.SyncStatusError
		result.Detail = behindErr.Error()
		return result, true
	}
	if behind == 0 {
		result.Status = domain.SyncStatusUpToDate
		if tip, err := infra.Tip(ctx, infra.TipParams{
			WorktreePath: e.Step.Path,
			Ref:          e.Step.Branch,
		}); err == nil {
			result.NewTip = tip
		}
		if !e.DryRun {
			result.PushPending = infra.AheadOfRemote(ctx, infra.RemoteBranchParams{
				WorktreePath: e.Step.Path,
				Branch:       e.Step.Branch,
			})
		}
		return result, false
	}

	upstream := e.OldTips[e.Step.SourceBranch]
	if upstream == "" {
		upstream = e.Step.SourceBranch
	}
	result.CommitsReplayed, _ = infra.CommitCount(ctx, infra.CommitCountParams{
		WorktreePath: e.Step.Path,
		Range:        upstream + ".." + e.Step.Branch,
	})

	if e.DryRun {
		result.Status = domain.SyncStatusSynced
		result.PushPending = true
		return result, false
	}

	rebaseResult, err := infra.RebaseOnto(ctx, infra.RebaseOntoParams{
		WorktreePath: e.Step.Path,
		NewBase:      e.Step.SourceBranch,
		Upstream:     upstream,
		Branch:       e.Step.Branch,
		KeepConflict: e.KeepConflict,
	})
	if err != nil {
		result.Status = domain.SyncStatusError
		result.Detail = err.Error()
		return result, true
	}
	if rebaseResult.Conflicted {
		result.Status = domain.SyncStatusConflict
		result.ConflictFiles = rebaseResult.Files
		result.KeptInProgress = rebaseResult.Kept
		if rebaseResult.Kept {
			result.Detail = "rebase conflict (left in progress)"
		} else {
			result.Detail = "rebase conflict (aborted, working tree clean)"
		}
		return result, true
	}

	// A rebase that went through is reported as it is, interrupt or not.
	settled := context.WithoutCancel(ctx)
	result.Status = domain.SyncStatusSynced
	if tip, tipErr := infra.Tip(settled, infra.TipParams{
		WorktreePath: e.Step.Path,
		Ref:          e.Step.Branch,
	}); tipErr == nil {
		result.NewTip = tip
	}
	result.PushPending = infra.AheadOfRemote(settled, infra.RemoteBranchParams{
		WorktreePath: e.Step.Path,
		Branch:       e.Step.Branch,
	})
	return result, false
}
