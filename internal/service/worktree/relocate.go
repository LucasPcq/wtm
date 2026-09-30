package worktree

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// RelocateParams holds inputs for planning and running a relocate.
type RelocateParams struct {
	ProjectDir     string
	StateDir       string
	Config         domain.Config
	TargetBasePath string
	BaseBranch     string
	Force          bool
	DryRun         bool
	// Parents maps a branch to the parent chosen for adoption. A missing or empty
	// entry falls back to BaseBranch.
	Parents map[string]string
}

// PlanRelocate computes the relocate plan without moving anything. Used to
// preview the operation and prompt for adoption parents before acting.
func PlanRelocate(params RelocateParams) (domain.RelocatePlan, error) {
	candidates, err := collectRelocateCandidates(params)
	if err != nil {
		return domain.RelocatePlan{}, err
	}
	return rules.BuildRelocatePlan(rules.BuildRelocatePlanParams{
		Candidates: candidates,
		ProjectDir: params.ProjectDir,
		BasePath:   params.TargetBasePath,
		BaseBranch: params.BaseBranch,
		Force:      params.Force,
	}), nil
}

// Relocate moves every managed worktree to the target base path, adopts any
// worktree created outside wtm (writing its meta.json), and rewrites the config
// base_path when it changed. Dirty/locked/blocked worktrees are reported as
// skipped without aborting the run.
func Relocate(params RelocateParams) (domain.RelocateResult, error) {
	plan, err := PlanRelocate(params)
	if err != nil {
		return domain.RelocateResult{}, err
	}

	result := domain.RelocateResult{BasePath: params.TargetBasePath}
	for _, step := range plan.Steps {
		result.Steps = append(result.Steps, executeRelocateStep(executeRelocateStepParams{
			Step:   step,
			Params: params,
		}))
	}

	if params.TargetBasePath != params.Config.Project.Worktrees.BasePath {
		if updateErr := updateBasePath(params); updateErr != nil {
			return result, updateErr
		}
		result.BasePathUpdated = !params.DryRun
	}

	return result, nil
}

func updateBasePath(params RelocateParams) error {
	if params.DryRun {
		return nil
	}
	updated := params.Config.Project
	updated.Worktrees.BasePath = params.TargetBasePath
	if err := config.WriteProjectConfig(config.WriteProjectConfigParams{
		StateDir: params.StateDir,
		Config:   updated,
	}); err != nil {
		return fmt.Errorf("update config base_path: %w", err)
	}
	return nil
}

func collectRelocateCandidates(params RelocateParams) ([]rules.RelocateCandidate, error) {
	worktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{ProjectDir: params.ProjectDir})
	if err != nil {
		return nil, err
	}

	names := nameClashes(nameClashesParams{StateDir: params.StateDir, Worktrees: worktrees})
	candidates := make([]rules.RelocateCandidate, 0, len(worktrees))
	for _, w := range worktrees {
		if w.IsMain {
			candidates = append(candidates, rules.RelocateCandidate{
				Branch:   w.Branch,
				FromPath: w.Path,
				IsMain:   true,
			})
			continue
		}

		dirty, dirtyErr := infra.IsDirty(infra.IsDirtyParams{WorktreePath: w.Path})
		to := rules.DesiredWorktreePath(rules.DesiredWorktreePathParams{
			ProjectDir: params.ProjectDir,
			BasePath:   params.TargetBasePath,
			Branch:     w.Branch,
		})

		managed := isManaged(params.StateDir, w.Branch)
		candidates = append(candidates, rules.RelocateCandidate{
			Branch:       w.Branch,
			FromPath:     w.Path,
			IsManaged:    managed,
			NameClash:    names.of(nameClashOfParams{Branch: w.Branch, Managed: managed}),
			IsDirty:      dirty,
			InspectErr:   dirtyErr != nil,
			IsLocked:     w.Locked,
			DestOccupied: !samePath(w.Path, to) && pathExists(to),
			HasJobs:      !samePath(w.Path, to) && process.WorktreeHasJobs(w.Path),
		})
	}

	return candidates, nil
}

type executeRelocateStepParams struct {
	Step   domain.RelocateStep
	Params RelocateParams
}

func executeRelocateStep(p executeRelocateStepParams) domain.RelocateStepResult {
	res := domain.RelocateStepResult{
		Branch:   p.Step.Branch,
		FromPath: p.Step.FromPath,
		ToPath:   p.Step.ToPath,
		Parent:   resolveParent(p.Params, p.Step),
	}

	switch p.Step.Status {
	case domain.RelocateStatusMove:
		return runMoveStep(p, res)
	case domain.RelocateStatusAdopt:
		return runAdoptStep(p, res)
	case domain.RelocateStatusError:
		// The planner flags an error when a worktree needs moving but its
		// working-tree state could not be determined (do not move blind).
		res.Status = domain.RelocateStatusError
		res.Detail = "could not determine working-tree state; re-run after resolving, or with --force to move anyway"
		return res
	default:
		// Noop and every skip/block status carry through unchanged.
		res.Status = p.Step.Status
		res.Detail = p.Step.Detail
		return res
	}
}

func runMoveStep(p executeRelocateStepParams, res domain.RelocateStepResult) domain.RelocateStepResult {
	if !p.Params.DryRun {
		if err := os.MkdirAll(filepath.Dir(res.ToPath), 0o755); err != nil {
			return failStep(res, fmt.Errorf("create target dir: %w", err))
		}
		if err := infra.MoveWorktree(infra.MoveWorktreeParams{
			ProjectDir: p.Params.ProjectDir,
			From:       res.FromPath,
			To:         res.ToPath,
			Force:      p.Params.Force,
		}); err != nil {
			return failStep(res, err)
		}
	}

	if !p.Step.Adopt {
		res.Status = domain.RelocateStatusMoved
		return res
	}

	if !p.Params.DryRun {
		if err := adoptWorktree(adoptWorktreeParams{
			ProjectDir: p.Params.ProjectDir,
			StateDir:   p.Params.StateDir,
			Branch:     res.Branch,
			Parent:     res.Parent,
		}); err != nil {
			return failStep(res, err)
		}
	}
	res.Status = domain.RelocateStatusMovedAdopted
	return res
}

func runAdoptStep(p executeRelocateStepParams, res domain.RelocateStepResult) domain.RelocateStepResult {
	if !p.Params.DryRun {
		if err := adoptWorktree(adoptWorktreeParams{
			ProjectDir: p.Params.ProjectDir,
			StateDir:   p.Params.StateDir,
			Branch:     res.Branch,
			Parent:     res.Parent,
		}); err != nil {
			return failStep(res, err)
		}
	}
	res.Status = domain.RelocateStatusAdopted
	return res
}

func failStep(res domain.RelocateStepResult, err error) domain.RelocateStepResult {
	res.Status = domain.RelocateStatusError
	res.Detail = err.Error()
	return res
}

type adoptWorktreeParams struct {
	ProjectDir string
	StateDir   string
	Branch     string
	Parent     string
}

// adoptWorktree completes the record rather than writing a new one: a worktree
// that already ran jobs holds its ordinal, its isolation and the namespaces
// clean has to give back. The ordinal is left for the run module to allocate
// on first use, as for a created worktree.
func adoptWorktree(params adoptWorktreeParams) error {
	metadata, err := loadMetadata(params.StateDir, params.Branch)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read metadata for %s: %w", params.Branch, err)
	}
	metadata.SourceBranch = params.Parent
	metadata.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	return writeMetadata(rules.WorktreeMetaDir(params.StateDir, params.Branch), metadata)
}

func resolveParent(params RelocateParams, step domain.RelocateStep) string {
	if !step.Adopt {
		return ""
	}
	if parent, ok := params.Parents[step.Branch]; ok && parent != "" {
		return parent
	}
	if step.Parent != "" {
		return step.Parent
	}
	return params.BaseBranch
}

// isManaged reports whether wtm ever created or adopted this worktree, which is
// what CreatedAt records. The file alone no longer answers it: a worktree that
// merely ran a job has a meta.json holding its ordinal and nothing else, and it
// is still external — relocate must keep offering to adopt it, and reparent must
// keep refusing it a parent it never had.
func isManaged(stateDir, branch string) bool {
	meta, err := loadMetadata(stateDir, branch)
	if err != nil {
		return false
	}
	return meta.CreatedAt != ""
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}
