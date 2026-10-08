package worktree

import (
	"context"
	"errors"
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
func JobEnv(ctx context.Context, params JobEnvParams) (map[string]string, error) {
	branch, err := CurrentBranch(ctx, CurrentBranchParams{Dir: params.Dir})
	if err != nil {
		return nil, err
	}
	return BranchEnv(ctx, WorktreeRef{
		ProjectDir: params.ProjectDir,
		StateDir:   params.StateDir,
		Branch:     branch,
	})
}

// BranchEnv is JobEnv for a caller that already knows the branch — the
// lifecycle hooks, which are handed one rather than a directory to ask git
// about.
func BranchEnv(ctx context.Context, params WorktreeRef) (map[string]string, error) {
	return branchEnvAs(ctx, params, IsolationOf(params))
}

// branchEnvAs resolves the environment under a given isolation: `wtm env
// --isolation` settles the .env for the choice before recording it.
func branchEnvAs(ctx context.Context, params WorktreeRef, isolation domain.Isolation) (map[string]string, error) {
	ordinal, err := Ordinal(ctx, params)
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
		Isolation:       isolation,
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
// one is set on purpose, empty to let wtm derive it from the branch. Only the
// main checkout sets one, read from its own .env: the environment of this
// process belongs to whichever worktree the command was launched from.
func composeProjectOf(params composeProjectParams) string {
	if params.Ordinal != domain.MainWorktreeOrdinal {
		return ""
	}
	return mainComposeProject(mainComposeProjectParams{ProjectDir: params.ProjectDir, Config: params.Config})
}

type mainComposeProjectParams struct {
	ProjectDir string
	Config     domain.RunConfig
}

func mainComposeProject(params mainComposeProjectParams) string {
	return rules.MainComposeProjectName(rules.MainComposeProjectNameParams{
		Project:  filepath.Base(params.ProjectDir),
		EnvFiles: composeEnvFiles(composeEnvFilesParams{Dir: params.ProjectDir, Config: params.Config}),
	})
}

type composeEnvFilesParams struct {
	Dir    string
	Config domain.RunConfig
}

// composeEnvFiles reads the .env of each directory a compose stack starts from.
// A .env that cannot be read counts as one that names nothing.
func composeEnvFiles(params composeEnvFilesParams) [][]domain.EnvLine {
	dirs := rules.ComposeProjectDirs(params.Config)
	if len(dirs) == 0 {
		dirs = []string{"."}
	}
	files := make([][]domain.EnvLine, 0, len(dirs))
	for _, dir := range dirs {
		data, err := os.ReadFile(filepath.Join(params.Dir, dir, domain.EnvFileName))
		if err != nil {
			continue
		}
		files = append(files, rules.ParseEnv(string(data)))
	}
	return files
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

type hookEnvParams struct {
	Ref          WorktreeRef
	WorktreePath string
}

// hookEnv resolves the run variables a lifecycle hook gets, nil when they do not
// apply or the worktree has no number yet (HookEnvPending: the flow allocates it
// before the hooks run). A
// COMPOSE_PROJECT_NAME the worktree's own .env sets wins: it is the stack a
// `docker compose` typed there reaches. Resolving nothing degrades to the
// hook's own environment rather than to another worktree's values.
func hookEnv(ctx context.Context, params hookEnvParams) map[string]string {
	cfg := runConfig(params.Ref.StateDir)
	if !rules.RunEnvReachesHooks(rules.RunEnvReachesHooksParams{Config: cfg, Recorded: RecordedIsolation(params.Ref)}) {
		return nil
	}
	env, err := BranchEnv(ctx, params.Ref)
	if err != nil {
		return nil
	}
	if name := rules.DeclaredComposeProject(composeEnvFiles(composeEnvFilesParams{Dir: params.WorktreePath, Config: cfg})); name != "" {
		env[domain.EnvComposeProjectName] = name
	}
	return env
}

// HookEnvPending says whether this worktree's hooks would read its run
// environment while it has no number yet: the flow allocates one first, so the
// allocation is published like any other change to the worktree.
func HookEnvPending(ctx context.Context, ref WorktreeRef) bool {
	cfg := runConfig(ref.StateDir)
	if !rules.RunEnvReachesHooks(rules.RunEnvReachesHooksParams{Config: cfg, Recorded: RecordedIsolation(ref)}) {
		return false
	}
	_, err := Ordinal(ctx, ref)
	return errors.Is(err, domain.ErrOrdinalUnallocated)
}
