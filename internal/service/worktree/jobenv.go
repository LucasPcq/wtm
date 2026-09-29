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
		ComposeProject:  composeProjectOf(composeProjectParams{ProjectDir: params.ProjectDir, Config: cfg, Ordinal: ordinal}),
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

type composeProjectParams struct {
	ProjectDir string
	Config     domain.RunConfig
	Ordinal    int
}

// composeProjectOf is the COMPOSE_PROJECT_NAME a worktree's jobs run under when
// one is set on purpose, empty to let wtm derive it. A linked worktree takes it
// from this process's environment. The main checkout never does: that
// environment belongs to whichever worktree the command was launched from, and
// the main's name is read from its own .env instead.
func composeProjectOf(params composeProjectParams) string {
	if params.Ordinal != domain.MainWorktreeOrdinal {
		return os.Getenv(domain.EnvComposeProjectName)
	}
	return mainComposeProject(mainComposeProjectParams{ProjectDir: params.ProjectDir, Config: params.Config})
}

type mainComposeProjectParams struct {
	ProjectDir string
	Config     domain.RunConfig
}

// mainComposeProject is the compose project of the main checkout, where the
// shared services run. A .env that cannot be read counts as one that names
// nothing.
func mainComposeProject(params mainComposeProjectParams) string {
	dirs := rules.ComposeProjectDirs(params.Config)
	if len(dirs) == 0 {
		dirs = []string{"."}
	}
	files := make([][]domain.EnvLine, 0, len(dirs))
	for _, dir := range dirs {
		data, err := os.ReadFile(filepath.Join(params.ProjectDir, dir, domain.EnvFileName))
		if err != nil {
			continue
		}
		files = append(files, rules.ParseEnv(string(data)))
	}
	return rules.MainComposeProjectName(rules.MainComposeProjectNameParams{
		Project:  filepath.Base(params.ProjectDir),
		EnvFiles: files,
	})
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
