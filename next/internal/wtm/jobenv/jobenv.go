// Package jobenv is the environment a job or a hook of a worktree runs with: its identity, its ports shifted by its ordinal and its compose project.
package jobenv

import (
	"path/filepath"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
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

func Keys() []string {
	return []string{worktreeKey, branchKey, ordinalKey, portOffsetKey, projectKey, isolationKey, ComposeProjectKey}
}

type Params struct {
	Branch     string
	ProjectDir string
	Ordinal    int
	Isolation  run.Isolation
	Run        run.Config
}

func JobEnv(params Params) map[string]string {
	env := map[string]string{
		worktreeKey:   branch.Slug(params.Branch),
		branchKey:     params.Branch,
		ordinalKey:    strconv.Itoa(params.Ordinal),
		portOffsetKey: strconv.Itoa(params.Offset()),
		projectKey:    branch.HostLabel(params.Project()),
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
