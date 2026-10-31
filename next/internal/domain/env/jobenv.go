package env

import (
	"cmp"
	"path"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"

	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
)

// The variables wtm sets in the environment of a worktree's jobs and hooks. ComposeProjectKey is also the key wtm owns in the .env files docker compose reads.
const (
	ComposeProjectKey = "COMPOSE_PROJECT_NAME"
	WorktreeKey       = "WTM_WORKTREE" // the slug of the worktree a job runs in
	BranchKey         = "WTM_BRANCH"   // the branch of the worktree a job runs in
	ordinalKey        = "WTM_ORDINAL"
	portOffsetKey     = "WTM_PORT_OFFSET"
	ProjectKey        = "WTM_PROJECT" // the host label of the repository a job runs in
	isolationKey      = "WTM_ISOLATION"
)

// JobKeys returns the names of every variable wtm itself sets in a job's environment, the job's port variables aside.
func JobKeys() []string {
	return []string{WorktreeKey, BranchKey, ordinalKey, portOffsetKey, ProjectKey, isolationKey, ComposeProjectKey}
}

// NotInherited returns the variables a worktree's hooks and commands never take from the caller's environment: those wtm itself sets (JobKeys), and every port run declares, which are the caller's worktree's, never theirs.
func NotInherited(run project.Run) []string {
	keys := JobKeys()
	for _, job := range run.Jobs {
		for name := range job.Ports {
			keys = append(keys, name)
		}
	}
	return keys
}

// JobParams describes the worktree a job runs in: its branch, the project's directory, the worktree's ordinal, the project's isolation and its run.toml.
type JobParams struct {
	Branch     string
	ProjectDir string
	Ordinal    int
	Isolation  project.Isolation
	Run        project.Run
}

// jobEnv returns the variables a job of the worktree runs with: the WTM_ variables, COMPOSE_PROJECT_NAME unless the isolation is verbatim, and each port declared by exactly one job, offset for this worktree unless that job is shared.
func jobEnv(params JobParams) map[string]string {
	env := map[string]string{
		WorktreeKey:   worktree.Slug(params.Branch),
		BranchKey:     params.Branch,
		ordinalKey:    strconv.Itoa(params.Ordinal),
		portOffsetKey: strconv.Itoa(params.Offset()),
		ProjectKey:    worktree.HostLabel(params.Project()),
		isolationKey:  string(params.Isolation.Effective()),
	}
	if name := params.composeProject(); name != "" {
		env[ComposeProjectKey] = name
	}
	for name, port := range params.ports() {
		env[name] = strconv.Itoa(port)
	}
	return env
}

// Offset returns how far the worktree's ports sit above their base: its ordinal times the offset block, or zero when the isolation is verbatim.
func (p JobParams) Offset() int {
	if p.Isolation.Effective() == project.Verbatim {
		return 0
	}
	return p.Ordinal * OffsetBlock(p.Run)
}

// Project returns the project's name, the base name of its directory.
func (p JobParams) Project() string {
	return filepath.Base(p.ProjectDir)
}

// composeProject returns the docker compose project name of the worktree, <project>-<branch> slugged, or empty when the isolation is verbatim.
func (p JobParams) composeProject() string {
	if p.Isolation.Effective() == project.Verbatim {
		return ""
	}
	return worktree.Slug(p.Project()) + "-" + worktree.Slug(p.Branch)
}

const defaultOffsetBlock = 10

// OffsetBlock returns how many ports apart two consecutive worktrees sit: run.toml's port_offset_block, else 10.
func OffsetBlock(cfg project.Run) int {
	if cfg.PortOffsetBlock <= 0 {
		return defaultOffsetBlock
	}
	return cfg.PortOffsetBlock
}

// resolvedPortParams describes a port: its base, the worktree's offset, and whether the job declaring it is shared by every worktree.
type resolvedPortParams struct {
	Base   int
	Offset int
	Shared bool
}

// resolvedPort returns the port the worktree uses: the base for a shared job, the base plus the offset otherwise.
func resolvedPort(params resolvedPortParams) int {
	if params.Shared {
		return params.Base
	}
	return params.Base + params.Offset
}

// sharedJobs returns the set of the jobs of cfg whose scope is shared.
func sharedJobs(cfg project.Run) map[string]bool {
	shared := map[string]bool{}
	for _, job := range cfg.Jobs {
		if job.Scope == project.Shared {
			shared[job.Name] = true
		}
	}
	return shared
}

// ports returns each port variable declared by exactly one job, resolved for the worktree; a name two jobs declare is left out, since neither value would be right for both.
func (p JobParams) ports() map[string]int {
	declaring := map[string]int{}
	for _, job := range p.Run.Jobs {
		for name := range job.Ports {
			declaring[name]++
		}
	}

	ports := map[string]int{}
	for _, job := range p.Run.Jobs {
		for name, base := range job.Ports {
			if declaring[name] == 1 {
				ports[name] = resolvedPort(resolvedPortParams{Base: base, Offset: p.Offset(), Shared: job.Scope == project.Shared})
			}
		}
	}
	return ports
}

// JobPorts returns the ports job binds in the worktree, by the names it declares them under: each its base, offset for the worktree unless the job is shared; what the job reads, whichever other job declares the same name.
func (p JobParams) JobPorts(job project.Job) map[string]int {
	ports := make(map[string]int, len(job.Ports))
	for name, base := range job.Ports {
		ports[name] = resolvedPort(resolvedPortParams{Base: base, Offset: p.Offset(), Shared: job.Scope == project.Shared})
	}
	return ports
}

// composeDirs returns the directories, relative to the worktree, of the jobs that run docker compose, each once, in the order of the jobs.
func composeDirs(cfg project.Run) []string {
	var dirs []string
	seen := map[string]bool{}
	for _, job := range cfg.Jobs {
		dir := path.Clean(cmp.Or(job.Cwd, "."))
		if !job.RunsCompose() || seen[dir] {
			continue
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	return dirs
}

// ComposeEnvPaths returns the .env file docker compose reads in each of the compose directories, or the worktree's root .env when no job runs compose.
func ComposeEnvPaths(cfg project.Run) []string {
	dirs := composeDirs(cfg)
	if len(dirs) == 0 {
		dirs = []string{"."}
	}

	paths := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		paths = append(paths, filepath.Join(dir, ".env"))
	}
	return paths
}

// declaredComposeProject returns the first non-empty COMPOSE_PROJECT_NAME the files declare, empty when none does.
func declaredComposeProject(files [][]dotenv.Line) string {
	for _, lines := range files {
		at := slices.IndexFunc(lines, func(line dotenv.Line) bool { return line.Kind == dotenv.Pair && line.Key == ComposeProjectKey })
		if at >= 0 && lines[at].Value != "" {
			return lines[at].Value
		}
	}
	return ""
}

// HookEnv returns the variables a hook runs with: the job environment of the worktree, with the COMPOSE_PROJECT_NAME declared in the compose .env files, read as composeEnv, winning over wtm's own.
func HookEnv(job JobParams, composeEnv [][]dotenv.Line) map[string]string {
	env := jobEnv(job)
	if name := declaredComposeProject(composeEnv); name != "" {
		env[ComposeProjectKey] = name
	}
	return env
}

// ReadHookEnv returns the variables a hook of the worktree at dir runs with (HookEnv), each compose .env file read by read; it fails with read's error.
func ReadHookEnv(job JobParams, dir string, read func(path string) ([]dotenv.Line, error)) (map[string]string, error) {
	var compose [][]dotenv.Line
	for _, path := range ComposeEnvPaths(job.Run) {
		lines, err := read(filepath.Join(dir, path))
		if err != nil {
			return nil, err
		}
		compose = append(compose, lines)
	}
	return HookEnv(job, compose), nil
}

// HooksReadRun reports whether the hooks' environment depends on run.toml: an isolation is set and some job runs docker compose.
func HooksReadRun(run project.Run, isolation project.Isolation) bool {
	return isolation != "" && len(composeDirs(run)) > 0
}
