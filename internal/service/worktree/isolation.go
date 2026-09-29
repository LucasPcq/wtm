package worktree

import (
	"fmt"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/config"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
)

// IsolationOf is the choice recorded for this worktree. A worktree with no
// metadata — main, or one wtm never touched — is isolated.
func IsolationOf(ref WorktreeRef) domain.Isolation {
	meta, err := loadMetadata(ref.StateDir, ref.Branch)
	if err != nil {
		return domain.IsolationIsolated
	}
	return rules.EffectiveIsolation(meta.Isolation)
}

// RecordedIsolation is the choice meta.json holds, empty for a worktree that
// never made one — created before the choice existed, or never by wtm.
func RecordedIsolation(ref WorktreeRef) domain.Isolation {
	meta, err := loadMetadata(ref.StateDir, ref.Branch)
	if err != nil {
		return ""
	}
	return meta.Isolation
}

type SetIsolationParams struct {
	Ref       WorktreeRef
	Isolation domain.Isolation
}

// SetIsolation records a new choice for an existing worktree. It writes the
// record only: bringing the .env in line with it is the caller's next step.
func SetIsolation(params SetIsolationParams) error {
	isMain, err := isMainBranch(params.Ref)
	if err != nil {
		return err
	}
	if isMain {
		if rules.IsVerbatim(params.Isolation) {
			return domain.ErrIsolationMain
		}
		return nil
	}

	meta, err := loadMetadata(params.Ref.StateDir, params.Ref.Branch)
	if err != nil {
		meta = domain.WorktreeMetadata{}
	}
	meta.Isolation = rules.EffectiveIsolation(params.Isolation)
	return writeMetadata(rules.WorktreeMetaDir(params.Ref.StateDir, params.Ref.Branch), meta)
}

func isMainBranch(ref WorktreeRef) (bool, error) {
	worktrees, err := infra.ListWorktrees(infra.ListWorktreesParams{ProjectDir: ref.ProjectDir})
	if err != nil {
		return false, fmt.Errorf("list worktrees: %w", err)
	}
	for _, wt := range worktrees {
		if wt.IsMain && wt.Branch == ref.Branch {
			return true, nil
		}
	}
	return false, nil
}

type IsolationAdoptionParams struct {
	Ref          WorktreeRef
	WorktreePath string
}

// IsolationAdoptionFor says whether a worktree still has to adopt its isolation
// and what adopting it changes. It resolves nothing and allocates nothing: the
// answer is asked before the worktree is touched. A run.toml that cannot be
// read leaves nothing to adopt — the port pass is skipped for that reason, and
// says so itself.
func IsolationAdoptionFor(params IsolationAdoptionParams) (domain.IsolationAdoptionPlan, error) {
	cfg, err := config.LoadRun(params.Ref.StateDir)
	if err != nil {
		return domain.IsolationAdoptionPlan{}, nil
	}
	isMain, err := isMainBranch(params.Ref)
	if err != nil {
		return domain.IsolationAdoptionPlan{}, err
	}
	if !rules.IsolationAdoptionPending(rules.IsolationAdoptionPendingParams{IsMain: isMain, Recorded: RecordedIsolation(params.Ref), Config: cfg}) {
		return domain.IsolationAdoptionPlan{}, nil
	}

	plan := domain.IsolationAdoptionPlan{Pending: true}
	dirs := rules.ComposeProjectDirs(cfg)
	if len(dirs) == 0 {
		return plan, nil
	}
	plan.ComposeProject = rules.ComposeProjectName(rules.ComposeProjectNameParams{
		Project:  filepath.Base(params.Ref.ProjectDir),
		Worktree: rules.WorktreeSlug(params.Ref.Branch),
	})
	plan.CurrentComposeProject = rules.DeclaredComposeProject(composeEnvFiles(composeEnvFilesParams{Dir: params.WorktreePath, Config: cfg}))
	if plan.CurrentComposeProject == "" {
		plan.CurrentComposeProject = rules.DefaultComposeProjectName(filepath.Base(filepath.Join(params.WorktreePath, dirs[0])))
	}
	return plan, nil
}
