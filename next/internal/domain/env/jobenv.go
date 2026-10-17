package env

import (
	"cmp"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"

	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
)

const (
	ComposeProjectKey = "COMPOSE_PROJECT_NAME"
	worktreeKey       = "WTM_WORKTREE"
	branchKey         = "WTM_BRANCH"
	ordinalKey        = "WTM_ORDINAL"
	portOffsetKey     = "WTM_PORT_OFFSET"
	projectKey        = "WTM_PROJECT"
	isolationKey      = "WTM_ISOLATION"
)

func JobKeys() []string {
	return []string{worktreeKey, branchKey, ordinalKey, portOffsetKey, projectKey, isolationKey, ComposeProjectKey}
}

type JobParams struct {
	Branch     string
	ProjectDir string
	Ordinal    int
	Isolation  project.Isolation
	Run        project.Run
}

func JobEnv(params JobParams) map[string]string {
	env := map[string]string{
		worktreeKey:   worktree.Slug(params.Branch),
		branchKey:     params.Branch,
		ordinalKey:    strconv.Itoa(params.Ordinal),
		portOffsetKey: strconv.Itoa(params.Offset()),
		projectKey:    worktree.HostLabel(params.Project()),
		isolationKey:  string(params.Isolation.Effective()),
	}
	if name := params.ComposeProject(); name != "" {
		env[ComposeProjectKey] = name
	}
	for name, port := range params.ports() {
		env[name] = strconv.Itoa(port)
	}
	return env
}

func (p JobParams) Offset() int {
	if p.Isolation.Effective() == project.Verbatim {
		return 0
	}
	return p.Ordinal * OffsetBlock(p.Run)
}

func (p JobParams) Project() string {
	return filepath.Base(p.ProjectDir)
}

func (p JobParams) ComposeProject() string {
	if p.Isolation.Effective() == project.Verbatim {
		return ""
	}
	return worktree.Slug(p.Project()) + "-" + worktree.Slug(p.Branch)
}

const defaultOffsetBlock = 10

func OffsetBlock(cfg project.Run) int {
	if cfg.PortOffsetBlock <= 0 {
		return defaultOffsetBlock
	}
	return cfg.PortOffsetBlock
}

type ResolvedPortParams struct {
	Base   int
	Offset int
	Shared bool
}

func ResolvedPort(params ResolvedPortParams) int {
	if params.Shared {
		return params.Base
	}
	return params.Base + params.Offset
}

func SharedJobs(cfg project.Run) map[string]bool {
	shared := map[string]bool{}
	for _, job := range cfg.Jobs {
		if job.Scope == project.Shared {
			shared[job.Name] = true
		}
	}
	return shared
}

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
				ports[name] = ResolvedPort(ResolvedPortParams{Base: base, Offset: p.Offset(), Shared: job.Scope == project.Shared})
			}
		}
	}
	return ports
}

func ComposeDirs(cfg project.Run) []string {
	var dirs []string
	seen := map[string]bool{}
	for _, job := range cfg.Jobs {
		dir := cmp.Or(job.Cwd, ".")
		if !job.RunsCompose() || seen[dir] {
			continue
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	return dirs
}

func ComposeEnvPaths(cfg project.Run) []string {
	dirs := ComposeDirs(cfg)
	if len(dirs) == 0 {
		dirs = []string{"."}
	}
	paths := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		paths = append(paths, filepath.Join(dir, ".env"))
	}
	return paths
}

func declaredComposeProject(files [][]dotenv.Line) string {
	for _, lines := range files {
		at := slices.IndexFunc(lines, func(line dotenv.Line) bool { return line.Kind == dotenv.Pair && line.Key == ComposeProjectKey })
		if at >= 0 && lines[at].Value != "" {
			return lines[at].Value
		}
	}
	return ""
}

type HookEnvParams struct {
	JobParams
	ComposeEnv [][]dotenv.Line
}

func HookEnv(params HookEnvParams) map[string]string {
	env := JobEnv(params.JobParams)
	if name := declaredComposeProject(params.ComposeEnv); name != "" {
		env[ComposeProjectKey] = name
	}
	return env
}

type HooksReadRunParams struct {
	Run       project.Run
	Isolation project.Isolation
}

func HooksReadRun(params HooksReadRunParams) bool {
	return params.Isolation != "" && len(ComposeDirs(params.Run)) > 0
}
