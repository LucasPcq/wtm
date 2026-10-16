// Package provision is the unit that gives a branch its worktree: added whole, then numbered, its ports settled and its hooks run.
package provision

import (
	"context"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/rules"
	envcase "github.com/LucasPcq/wtm/internal/rules/runenv"
	"github.com/LucasPcq/wtm/internal/store"
)

type Ports struct {
	Git        git.ReadWriter
	Meta       store.Meta
	Env        store.EnvFiles
	StateDir   string
	PublicPort func(ctx context.Context, configured int) int
	Now        func() time.Time
}

type Project struct {
	MainPath string
	EnvFiles []domain.EnvFile
	OnCreate []domain.HookCommand
	Run      domain.RunConfig
	RunErr   error
	Global   domain.GlobalConfig
}

type Worktree struct {
	Branch     string
	Path       string
	Checkout   git.Checkout
	Source     string
	ParentPath string
	Strategy   domain.EnvStrategy
	Isolation  domain.Isolation
	Origin     Origin
}

type Origin struct {
	ExistingBranch bool   `json:"existing_branch"`
	OriginState    string `json:"origin_state,omitempty"`
	OriginAhead    int    `json:"origin_ahead,omitempty"`
	OriginBehind   int    `json:"origin_behind,omitempty"`
	Upstream       string `json:"upstream,omitempty"`
}

type Detail struct {
	Path     string                  `json:"path"`
	Metadata domain.WorktreeMetadata `json:"metadata"`
	Origin
	Ordinal   *int               `json:"ordinal,omitempty"`
	Allocated bool               `json:"-"`
	EnvPorts  domain.EnvPortPlan `json:"env_ports,omitzero"`
}

type UnitParams struct {
	Ports    Ports
	Project  Project
	Worktree Worktree
}

const (
	StepWorktree = "worktree"
	StepEnv      = "env"
	StepMeta     = "meta"
	PhaseOrdinal = "ordinal"
	PhasePorts   = "ports"
)

func Unit(params UnitParams) kernel.Unit[Detail] {
	u := unit{UnitParams: params}
	u.kept, u.ignored = envcase.Targets(envcase.TargetsParams{Run: params.Project.Run, Files: params.Project.EnvFiles})
	var then []kernel.Phase[Detail]
	if u.needsOrdinal() {
		then = append(then, kernel.Phase[Detail]{Name: PhaseOrdinal, Run: u.allocate})
	}
	if u.settlesPorts() {
		then = append(then, kernel.Phase[Detail]{Name: PhasePorts, Run: u.settle})
	}
	if len(params.Project.OnCreate) > 0 {
		then = append(then, kernel.Phase[Detail]{Name: string(domain.HookOnCreate), Log: u.hookLog(), Run: u.hooks})
	}
	return kernel.Unit[Detail]{Saga: u.saga(), Then: then}
}

type unit struct {
	UnitParams
	kept    domain.RunConfig
	ignored []envcase.Ignored
}

func (u unit) envCase() envcase.Case {
	return envcase.CaseOf(envcase.CaseParams{Isolation: u.isolation()})
}

func (u unit) isolation() domain.Isolation {
	return rules.EffectiveIsolation(u.Worktree.Isolation)
}

func (u unit) needsOrdinal() bool {
	if u.Project.RunErr != nil {
		return false
	}
	return envcase.PortsNeedOrdinal(envcase.PortsNeedOrdinalParams{Case: u.envCase(), Run: u.kept}) || u.hooksReadRun()
}

func (u unit) hooksReadRun() bool {
	if len(u.Project.OnCreate) == 0 || u.Project.RunErr != nil {
		return false
	}
	return rules.RunEnvReachesHooks(rules.RunEnvReachesHooksParams{Config: u.Project.Run, Recorded: u.isolation()})
}

func (u unit) settlesPorts() bool {
	if u.Project.RunErr != nil || len(u.ignored) > 0 {
		return true
	}
	if _, verbatim := u.envCase().(envcase.Verbatim); verbatim {
		return false
	}
	if len(u.kept.EnvPorts) > 0 || len(u.kept.EnvValues) > 0 {
		return true
	}
	return len(rules.OwnedEnvTargets(rules.OwnedEnvTargetsParams{Config: u.kept, EnvFiles: u.Project.EnvFiles})) > 0
}

func (u unit) hookLog() string {
	return rules.HooksLogPath(rules.HooksLogPathParams{StateDir: u.Ports.StateDir, Phase: domain.HookOnCreate, Branch: u.Worktree.Branch})
}
