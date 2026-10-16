package runenv

import (
	"maps"
	"path/filepath"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
)

const (
	envWorktree       = "WTM_WORKTREE"
	envBranch         = "WTM_BRANCH"
	envOrdinal        = "WTM_ORDINAL"
	envPortOffset     = "WTM_PORT_OFFSET"
	envProject        = "WTM_PROJECT"
	envIsolation      = "WTM_ISOLATION"
	envComposeProject = "COMPOSE_PROJECT_NAME"
)

type Params struct {
	Branch     string
	ProjectDir string
	Ordinal    int
	Isolation  run.Isolation
	Run        run.Config
}

func JobEnv(params Params) map[string]string {
	slug := branch.Slug(params.Branch)
	isolation := params.Isolation.Effective()
	offset := params.offset()
	env := map[string]string{
		envWorktree:   slug,
		envBranch:     params.Branch,
		envOrdinal:    strconv.Itoa(params.Ordinal),
		envPortOffset: strconv.Itoa(offset),
		envProject:    branch.HostLabel(params.project()),
		envIsolation:  string(isolation),
	}
	if isolation == run.Isolated {
		env[envComposeProject] = composeProjectName(composeNameParams{Project: params.project(), Worktree: slug})
	}
	ports := lifecyclePorts(lifecyclePortsParams{Run: params.Run, Offset: offset})
	merged := maps.Clone(env)
	for name, port := range ports {
		merged[name] = strconv.Itoa(port)
	}
	return merged
}

func (p Params) offset() int {
	if p.Isolation.Effective() == run.Verbatim {
		return 0
	}
	return p.Ordinal * offsetBlock(p.Run)
}

func (p Params) project() string {
	return filepath.Base(p.ProjectDir)
}
