package worktree

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type JobEnvParams struct {
	ProjectDir string
	StateDir   string
	// Dir is the directory the command was launched from, which may be any
	// subdirectory of the worktree.
	Dir string
}

// JobEnv resolves what a worktree's jobs and hooks learn about it. Only a
// client can build this: it takes git to name the branch, and the daemon must
// never run git.
//
// A user-defined COMPOSE_PROJECT_NAME is read from this process's environment
// and passed through untouched — a project name set on purpose is an answer.
func JobEnv(params JobEnvParams) (map[string]string, error) {
	branch, err := CurrentBranch(CurrentBranchParams{Dir: params.Dir})
	if err != nil {
		return nil, err
	}
	return BranchEnv(WorktreeRef{
		ProjectDir: params.ProjectDir,
		StateDir:   params.StateDir,
		Branch:     branch,
	})
}

// BranchEnv is JobEnv for a caller that already knows the branch — the
// lifecycle hooks, which are handed one rather than a directory to ask git
// about.
func BranchEnv(params WorktreeRef) (map[string]string, error) {
	ordinal, err := EnsureOrdinal(params)
	if err != nil {
		return nil, err
	}

	cfg := runConfig(params.StateDir)
	env := rules.WorktreeJobEnv(rules.WorktreeJobEnvParams{
		Branch:          params.Branch,
		Project:         filepath.Base(params.ProjectDir),
		Ordinal:         ordinal,
		PortOffsetBlock: rules.EffectivePortOffsetBlock(cfg),
		ComposeProject:  os.Getenv(domain.EnvComposeProjectName),
		Isolation:       IsolationOf(params),
	})

	// The offset is read back from the environment just resolved rather than
	// recomputed, so a hook and a job of the same worktree can never disagree
	// on it.
	offset, err := strconv.Atoi(env[domain.EnvPortOffset])
	if err != nil {
		offset = 0
	}
	return rules.WithPortEnv(env, rules.LifecyclePorts(rules.LifecyclePortsParams{
		Config:     cfg,
		PortOffset: offset,
	})), nil
}

// runConfig reads what run.toml says about ports — the spacing between two
// worktrees, and the declarations a hook can be given. A file that cannot be
// read or is refused degrades to an empty config instead of failing: the
// command that actually needs that run.toml will report the refusal itself, and
// a lifecycle hook must not lose its worktree identity over it.
func runConfig(stateDir string) domain.RunConfig {
	cfg, _ := config.LoadRun(stateDir)
	return cfg
}

// hookEnv resolves the worktree variables a lifecycle hook runs with, degrading
// to nothing rather than to another worktree's values.
func hookEnv(params WorktreeRef) map[string]string {
	env, err := BranchEnv(params)
	if err != nil {
		return nil
	}
	return env
}
