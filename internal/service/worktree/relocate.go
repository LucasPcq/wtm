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

type PlanRelocateParams struct {
	ProjectDir     string
	StateDir       string
	TargetBasePath string
	BaseBranch     string
	Force          bool
}

func PlanRelocate(params PlanRelocateParams) (domain.RelocatePlan, error) {
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

type MoveParams struct {
	ProjectDir string
	From       string
	To         string
	Force      bool
}

func Move(params MoveParams) error {
	if err := os.MkdirAll(filepath.Dir(params.To), 0o755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}
	return infra.MoveWorktree(infra.MoveWorktreeParams{
		ProjectDir: params.ProjectDir,
		From:       params.From,
		To:         params.To,
		Force:      params.Force,
	})
}

type AdoptParams struct {
	StateDir string
	Branch   string
	Parent   string
}

// Adopt completes the record rather than writing a new one: a worktree that
// already ran jobs holds its ordinal, its isolation and the namespaces clean
// has to give back. The ordinal is left for the run module to allocate on
// first use, as for a created worktree.
func Adopt(params AdoptParams) error {
	metadata, err := loadMetadata(params.StateDir, params.Branch)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read metadata for %s: %w", params.Branch, err)
	}
	metadata.SourceBranch = params.Parent
	metadata.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	return writeMetadata(rules.WorktreeMetaDir(params.StateDir, params.Branch), metadata)
}

type SetBasePathParams struct {
	ProjectDir string
	StateDir   string
	Project    domain.ProjectConfig
	BasePath   string
}

// SetBasePath also retires the previous base_path directory once the moves
// have emptied it. os.Remove leaves a directory that still holds anything —
// a worktree a refusal kept there, or files that are not wtm's.
func SetBasePath(params SetBasePathParams) error {
	updated := params.Project
	updated.Worktrees.BasePath = params.BasePath
	if err := config.WriteProjectConfig(config.WriteProjectConfigParams{
		StateDir: params.StateDir,
		Config:   updated,
	}); err != nil {
		return fmt.Errorf("update config base_path: %w", err)
	}
	_ = os.Remove(filepath.Join(params.ProjectDir, params.Project.Worktrees.BasePath))
	return nil
}

func collectRelocateCandidates(params PlanRelocateParams) ([]rules.RelocateCandidate, error) {
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
