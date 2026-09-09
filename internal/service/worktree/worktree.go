// Package worktree implements git worktree operations (create, list, remove, detect parent).
package worktree

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/branch"
	"github.com/LucasPcq/wtm/internal/service/env"
	"github.com/LucasPcq/wtm/internal/service/hooks"
)

// Create orchestrates worktree creation: git worktree add, env copy, metadata, hooks.
func Create(params domain.CreateParams) (domain.CreateResult, error) {
	sanitized := rules.SanitizeBranchName(params.Branch)
	worktreePath := filepath.Join(params.ProjectDir, params.Config.Project.Worktrees.BasePath, sanitized)

	if _, err := os.Stat(worktreePath); err == nil {
		// Idempotent path: with --if-not-exists, an existing worktree is a
		// no-op success so agents can safely retry.
		if params.IfNotExists {
			return domain.CreateResult{
				Branch:        params.Branch,
				Path:          worktreePath,
				AlreadyExists: true,
			}, nil
		}
		return domain.CreateResult{}, fmt.Errorf("%w: %s", domain.ErrWorktreePathExists, worktreePath)
	}

	// The branch is a separate axis from the directory: it may already exist (the
	// worktree then checks it out as-is, FromBranch unused) or already be checked
	// out somewhere else, which git allows only once.
	target := branch.Target(branch.BranchParams{ProjectDir: params.ProjectDir, Branch: params.Branch})
	if target.State == domain.BranchTargetCheckedOut {
		if params.IfNotExists {
			return domain.CreateResult{
				Branch:        params.Branch,
				Path:          target.WorktreePath,
				AlreadyExists: true,
			}, nil
		}
		return domain.CreateResult{}, fmt.Errorf("%w: "+domain.BranchCheckedOutElsewhereFmt,
			domain.ErrWorktreeExists, params.Branch, target.WorktreePath, params.Branch)
	}

	reuseBranch := target.State == domain.BranchTargetExisting
	if err := infra.CreateWorktree(infra.CreateWorktreeParams{
		ProjectDir:  params.ProjectDir,
		Path:        worktreePath,
		Branch:      params.Branch,
		FromBranch:  params.FromBranch,
		ReuseBranch: reuseBranch,
	}); err != nil {
		return domain.CreateResult{}, err
	}

	strategy := rules.ResolveEnvStrategy(params.Config.Project.Env.Strategy, params.EnvFromOverride)

	mainPath, err := infra.FindMainWorktreePath(infra.FindMainWorktreeParams{
		ProjectDir: params.ProjectDir,
	})
	if err != nil {
		return domain.CreateResult{}, fmt.Errorf("find main worktree: %w", err)
	}

	sourceBranch := params.SourceBranch
	if sourceBranch == "" {
		sourceBranch = params.FromBranch
	}

	envFiles := params.Config.Project.Env.Files
	if len(envFiles) > 0 {
		copyErr := env.CopyEnvFiles(env.CopyEnvFilesParams{
			Strategy:           strategy,
			Files:              envFiles,
			TargetDir:          worktreePath,
			MainWorktreePath:   mainPath,
			ParentWorktreePath: parentWorktreePath(params.ProjectDir, sourceBranch),
		})
		if copyErr != nil {
			return domain.CreateResult{}, fmt.Errorf("copy env files: %w", copyErr)
		}
	}

	ordinal, err := EnsureOrdinal(WorktreeRef{
		ProjectDir: params.ProjectDir,
		StateDir:   params.StateDir,
		Branch:     params.Branch,
	})
	if err != nil {
		return domain.CreateResult{}, fmt.Errorf("allocate ordinal: %w", err)
	}

	metadata := domain.WorktreeMetadata{
		SourceBranch: sourceBranch,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		EnvStrategy:  strategy,
		Ordinal:      ordinal,
	}

	metaDir := rules.WorktreeMetaDir(params.StateDir, params.Branch)
	if err := writeMetadata(metaDir, metadata); err != nil {
		return domain.CreateResult{}, err
	}

	// on_create hooks run inline unless the caller opts to run them as a separate
	// phase (create's phased output) via SkipHooks.
	if !params.SkipHooks {
		if err := RunCreateHooks(domain.CreateHooksParams{
			ProjectDir:   params.ProjectDir,
			StateDir:     params.StateDir,
			WorktreePath: worktreePath,
			Branch:       params.Branch,
			FromBranch:   params.FromBranch,
			Hooks:        params.Config.Project.Hooks.OnCreate,
		}); err != nil {
			return domain.CreateResult{}, err
		}
	}

	result := domain.CreateResult{
		Branch:   params.Branch,
		Path:     worktreePath,
		Metadata: metadata,
	}
	if reuseBranch {
		result.ExistingBranch = true
		result.OriginState = rules.DivergenceStateString(target.Origin)
		result.OriginAhead = target.AheadBehind.Ahead
		result.OriginBehind = target.AheadBehind.Behind
	}
	return result, nil
}

// RunCreateHooks executes the on_create hooks in the new worktree, streaming their
// output. It is a no-op when no hooks are configured. Exposed so `create` can run
// them as a distinct, titled phase after the silent creation spinner.
func RunCreateHooks(params domain.CreateHooksParams) error {
	if len(params.Hooks) == 0 {
		return nil
	}
	mainPath, err := infra.FindMainWorktreePath(infra.FindMainWorktreeParams{ProjectDir: params.ProjectDir})
	if err != nil {
		return fmt.Errorf("find main worktree: %w", err)
	}
	if err := hooks.RunHooks(hooks.RunHooksParams{
		Hooks:   params.Hooks,
		WorkDir: params.WorktreePath,
		Vars: rules.TemplateVars{
			Worktree:   params.WorktreePath,
			Branch:     params.Branch,
			Root:       mainPath,
			FromBranch: params.FromBranch,
		},
		Env: hookEnv(WorktreeRef{
			ProjectDir: params.ProjectDir,
			StateDir:   params.StateDir,
			Branch:     params.Branch,
		}),
		Output: params.Output,
		OnHook: params.OnHook,
	}); err != nil {
		return fmt.Errorf("on_create hooks: %w", err)
	}
	return nil
}

// parentWorktreePath resolves the on-disk worktree of the parent branch, used by
// the env "parent" strategy to copy .env from the worktree the new one was
// branched off. Returns "" when the parent has no local worktree (e.g. a remote
// start-point like origin/x), letting env provisioning fall back to the main
// worktree instead of copying from the wrong directory.
func parentWorktreePath(projectDir, parentBranch string) string {
	wt, err := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
		ProjectDir: projectDir,
		Branch:     parentBranch,
	})
	if err != nil {
		return ""
	}
	return wt.Path
}

// EnvFallbackParams holds inputs for EnvParentFallsBackToMain.
type EnvFallbackParams struct {
	ProjectDir  string
	Source      string
	Config      domain.Config
	EnvOverride string
}

// EnvParentFallsBackToMain reports whether provisioning the new worktree's .env
// will silently fall back to the main worktree: the resolved strategy is "parent"
// but the source branch has no local worktree to copy from. Lets a command warn
// before creating.
func EnvParentFallsBackToMain(params EnvFallbackParams) bool {
	strategy := rules.ResolveEnvStrategy(params.Config.Project.Env.Strategy, params.EnvOverride)
	return rules.ParentEnvFallsBackToMain(rules.ParentEnvFallbackParams{
		Strategy:          strategy,
		HasCopyFiles:      len(params.Config.Project.Env.Files) > 0,
		SourceHasWorktree: parentWorktreePath(params.ProjectDir, params.Source) != "",
	})
}

func writeMetadata(metaDir string, metadata domain.WorktreeMetadata) error {
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", metaDir, err)
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	// Written aside and renamed over: several commands read-modify-write this
	// file, and a reader landing on a half-written one would see a worktree that
	// holds no ordinal rather than one whose ordinal it could not read.
	metaPath := filepath.Join(metaDir, domain.MetaFileName)
	tmp, err := os.CreateTemp(metaDir, domain.MetaFileName+".*")
	if err != nil {
		return fmt.Errorf("write %s: %w", metaPath, err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", metaPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", metaPath, err)
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", metaPath, err)
	}
	if err := os.Rename(tmp.Name(), metaPath); err != nil {
		return fmt.Errorf("write %s: %w", metaPath, err)
	}

	return nil
}
