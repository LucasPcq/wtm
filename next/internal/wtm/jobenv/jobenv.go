// Package jobenv is the environment a job or a hook of a worktree runs with: its identity, its ports shifted by its ordinal and its compose project.
package jobenv

import (
	"path/filepath"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
)

const ComposeProjectKey = "COMPOSE_PROJECT_NAME"

type Params struct {
	Branch     string
	ProjectDir string
	Ordinal    int
	Isolation  run.Isolation
	Run        run.Config
}

func JobEnv(params Params) map[string]string {
	env := map[string]string{
		"WTM_WORKTREE":    branch.Slug(params.Branch),
		"WTM_BRANCH":      params.Branch,
		"WTM_ORDINAL":     strconv.Itoa(params.Ordinal),
		"WTM_PORT_OFFSET": strconv.Itoa(params.Offset()),
		"WTM_PROJECT":     branch.HostLabel(params.Project()),
		"WTM_ISOLATION":   string(params.Isolation.Effective()),
	}
	if name := params.ComposeProject(); name != "" {
		env[ComposeProjectKey] = name
	}
	for name, port := range params.ports() {
		env[name] = strconv.Itoa(port)
	}
	return env
}

func (p Params) Offset() int {
	if p.Isolation.Effective() == run.Verbatim {
		return 0
	}
	return p.Ordinal * OffsetBlock(p.Run)
}

func (p Params) Project() string {
	return filepath.Base(p.ProjectDir)
}

func (p Params) ComposeProject() string {
	if p.Isolation.Effective() == run.Verbatim {
		return ""
	}
	return branch.Slug(p.Project()) + "-" + branch.Slug(p.Branch)
}
