package rules

import (
	"fmt"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
)

// DesiredWorktreePathParams holds inputs for computing a worktree's target path.
type DesiredWorktreePathParams struct {
	ProjectDir string
	BasePath   string
	Branch     string
}

// DesiredWorktreePath returns the canonical path a worktree should live at:
// <project-dir>/<base-path>/<sanitized-branch>. It mirrors the path computed at
// creation time so relocate and create stay aligned.
func DesiredWorktreePath(params DesiredWorktreePathParams) string {
	return filepath.Join(params.ProjectDir, params.BasePath, SanitizeBranchName(params.Branch))
}

// RelocateCandidate is one worktree to classify, with the filesystem facts the
// pure planner needs (gathered by the service layer via git/os calls).
type RelocateCandidate struct {
	Branch    string
	FromPath  string
	IsMain    bool
	IsManaged bool
	IsDirty   bool
	IsLocked  bool
	// InspectErr is true when the worktree's working-tree state could not be
	// determined (e.g. git status failed). A worktree that needs moving but cannot
	// be inspected is reported as an error rather than moved blind.
	InspectErr bool
	// DestOccupied reports whether the target path already exists as an unrelated
	// directory; it only matters when the worktree needs to move.
	DestOccupied bool
	// HasJobs reports jobs running in the worktree. They are keyed on its path,
	// so it only matters when the worktree needs to move.
	HasJobs bool
	// NameClash is the live worktree an external one shares its derived name
	// with, nil when none does or the run module declares no job.
	NameClash *domain.WorktreeNameClash
}

// BuildRelocatePlanParams holds inputs for building a relocate plan.
type BuildRelocatePlanParams struct {
	Candidates []RelocateCandidate
	ProjectDir string
	BasePath   string
	BaseBranch string
	Force      bool
}

// BuildRelocatePlan classifies every worktree into a relocate action: move it to
// the target base path, adopt it in place (create metadata), leave it untouched,
// or skip/block it. An external worktree (no wtm metadata) is always adopted when
// acted upon. Dirty and locked worktrees that need moving are skipped unless Force
// is set; a target path already occupied is blocked regardless of Force.
func BuildRelocatePlan(params BuildRelocatePlanParams) domain.RelocatePlan {
	steps := make([]domain.RelocateStep, 0, len(params.Candidates))
	for _, c := range params.Candidates {
		if c.IsMain {
			continue
		}
		steps = append(steps, classifyCandidate(classifyParams{
			Candidate:  c,
			ProjectDir: params.ProjectDir,
			BasePath:   params.BasePath,
			BaseBranch: params.BaseBranch,
			Force:      params.Force,
		}))
	}

	return domain.RelocatePlan{
		BasePath:   params.BasePath,
		BaseBranch: params.BaseBranch,
		Steps:      steps,
	}
}

type classifyParams struct {
	Candidate  RelocateCandidate
	ProjectDir string
	BasePath   string
	BaseBranch string
	Force      bool
}

func classifyCandidate(params classifyParams) domain.RelocateStep {
	c := params.Candidate
	to := DesiredWorktreePath(DesiredWorktreePathParams{
		ProjectDir: params.ProjectDir,
		BasePath:   params.BasePath,
		Branch:     c.Branch,
	})
	external := !c.IsManaged

	step := domain.RelocateStep{
		Branch:   c.Branch,
		FromPath: c.FromPath,
		ToPath:   to,
		Adopt:    external,
	}
	if external {
		step.Parent = params.BaseBranch
	}
	if external && c.NameClash != nil {
		step.Status = domain.RelocateStatusBlockedName
		step.Detail = fmt.Sprintf(domain.RelocateNameClashFmt, c.Branch, c.NameClash.Branch, c.NameClash.Name)
		return step
	}

	if samePath(c.FromPath, to) {
		step.Status = inPlaceStatus(external)
		return step
	}

	step.Status = moveStatus(moveStatusParams{Candidate: c, Force: params.Force})
	return step
}

func inPlaceStatus(external bool) domain.RelocateStatus {
	if external {
		return domain.RelocateStatusAdopt
	}
	return domain.RelocateStatusNoop
}

type moveStatusParams struct {
	Candidate RelocateCandidate
	Force     bool
}

func moveStatus(params moveStatusParams) domain.RelocateStatus {
	if params.Candidate.DestOccupied {
		return domain.RelocateStatusBlockedDest
	}
	if params.Candidate.HasJobs {
		return domain.RelocateStatusBlockedJobs
	}
	if params.Force {
		return domain.RelocateStatusMove
	}
	if params.Candidate.InspectErr {
		return domain.RelocateStatusError
	}
	if params.Candidate.IsDirty {
		return domain.RelocateStatusSkippedDirty
	}
	if params.Candidate.IsLocked {
		return domain.RelocateStatusSkippedLocked
	}
	return domain.RelocateStatusMove
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

// ReprojectRelocatePlanParams holds inputs for ReprojectRelocatePlan.
type ReprojectRelocatePlanParams struct {
	Plan       domain.RelocatePlan
	ProjectDir string
	BasePath   string
}

// ReprojectRelocatePlan re-targets an existing plan onto a different base_path
// without touching the filesystem: every worktree's destination becomes
// <base_path>/<branch>, and a worktree that was already at its old destination but
// now moves is reclassified from in-place (noop/adopt) to a move. Dirty and locked
// skips carry through — those facts are base_path-independent. An occupied-destination
// block is NOT re-evaluated (that needs a filesystem check), so a worktree whose new
// target happens to be dirty/occupied may read optimistically here; execution, which
// re-plans against the filesystem, remains the source of truth for those edges. Used
// to preview a base_path change in the interactive recap.
func ReprojectRelocatePlan(params ReprojectRelocatePlanParams) domain.RelocatePlan {
	steps := make([]domain.RelocateStep, 0, len(params.Plan.Steps))
	for _, s := range params.Plan.Steps {
		to := DesiredWorktreePath(DesiredWorktreePathParams{
			ProjectDir: params.ProjectDir,
			BasePath:   params.BasePath,
			Branch:     s.Branch,
		})
		next := s
		next.ToPath = to
		switch {
		case samePath(s.FromPath, to):
			next.Status = inPlaceStatus(s.Adopt)
		case s.Status == domain.RelocateStatusNoop || s.Status == domain.RelocateStatusAdopt:
			next.Status = domain.RelocateStatusMove
		}
		steps = append(steps, next)
	}
	return domain.RelocatePlan{
		BasePath:   params.BasePath,
		BaseBranch: params.Plan.BaseBranch,
		Steps:      steps,
	}
}

// PlanHasWork reports whether a relocate plan contains any step that is not a
// no-op (i.e. something will move, be adopted, be skipped, or be blocked).
func PlanHasWork(plan domain.RelocatePlan) bool {
	for _, step := range plan.Steps {
		if step.Status != domain.RelocateStatusNoop {
			return true
		}
	}
	return false
}

// PlanAdoptions returns the actionable steps that will create metadata for a
// worktree (external worktrees being moved or adopted in place), so the caller
// can prompt for each one's parent branch.
func PlanAdoptions(plan domain.RelocatePlan) []domain.RelocateStep {
	out := make([]domain.RelocateStep, 0)
	for _, step := range plan.Steps {
		if !step.Adopt {
			continue
		}
		if step.Status == domain.RelocateStatusMove || step.Status == domain.RelocateStatusAdopt {
			out = append(out, step)
		}
	}
	return out
}

// RelocateHasFailure reports whether a relocate result contains a hard failure
// (an execution error or a blocked target path). Skips are not failures.
func RelocateHasFailure(result domain.RelocateResult) bool {
	for _, step := range result.Steps {
		switch step.Status {
		case domain.RelocateStatusError, domain.RelocateStatusBlockedDest, domain.RelocateStatusBlockedJobs, domain.RelocateStatusBlockedName:
			return true
		}
	}
	return false
}
