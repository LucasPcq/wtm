package worktree

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	envsvc "github.com/LucasPcq/wtm/internal/service/env"
	"github.com/LucasPcq/wtm/internal/service/process"
)

type ResolveEnvPortsParams struct {
	ProjectDir   string
	StateDir     string
	Branch       string
	WorktreePath string
	// EnvFiles are the value targets .wtm.toml configures. A link may only name
	// one of them, and this is the only place both files are in hand — LoadRun
	// validates what run.toml can answer for alone and never sees .wtm.toml.
	EnvFiles []domain.EnvFile
	// Global carries the machine's [proxy] table. The project says whether it
	// wants addresses; this says whether the machine can serve them.
	Global domain.GlobalConfig
}

// ResolveEnvPorts gathers what a worktree needs to reconcile the ports written
// into its .env files: the links and bases run.toml declares, and the offset its
// ordinal binds on. A project declaring no link resolves to zero links, which
// every caller treats as nothing to do.
func ResolveEnvPorts(params ResolveEnvPortsParams) (envsvc.EnvPortsParams, error) {
	cfg, err := config.LoadRun(params.StateDir)
	if err != nil {
		return envsvc.EnvPortsParams{}, err
	}

	// Resolved from the branch and the repository alone: this value is written to
	// a file, so it must not inherit the COMPOSE_PROJECT_NAME the calling process
	// happens to carry — a `wtm env` run from inside another worktree, or from a
	// job, would stamp that worktree's name into this one's .env.
	owned := rules.OwnedEnvWrites(rules.OwnedEnvWritesParams{
		Config:   cfg,
		EnvFiles: params.EnvFiles,
		Values: map[string]string{domain.EnvComposeProjectName: rules.ComposeProjectName(rules.ComposeProjectNameParams{
			Project:  filepath.Base(params.ProjectDir),
			Worktree: rules.WorktreeSlug(params.Branch),
		})},
	})

	// The identity needs neither an ordinal nor an offset, so a project with no
	// link resolves without asking git anything — EnsureOrdinal writes, and this
	// function is on the read path of every address wtm hands out.
	if len(cfg.EnvPorts) == 0 {
		if len(owned) == 0 {
			return envsvc.EnvPortsParams{}, nil
		}
		return envsvc.EnvPortsParams{WorktreePath: params.WorktreePath, Owned: owned}, nil
	}

	if errs := rules.ValidateEnvPortTargets(cfg.EnvPorts, params.EnvFiles); len(errs) > 0 {
		return envsvc.EnvPortsParams{}, fmt.Errorf("invalid run config: %s", strings.Join(errs, "; "))
	}

	// BranchEnv rather than EnsureOrdinal: it settles the offset and the worktree
	// label in one place, so a .env and the route a job answers under can never
	// disagree on which worktree they belong to.
	env, err := BranchEnv(WorktreeRef{
		ProjectDir: params.ProjectDir,
		StateDir:   params.StateDir,
		Branch:     params.Branch,
	})
	if err != nil {
		return envsvc.EnvPortsParams{}, err
	}
	offset, err := strconv.Atoi(env[domain.EnvPortOffset])
	if err != nil {
		return envsvc.EnvPortsParams{}, fmt.Errorf("resolve port offset: %w", err)
	}

	return envsvc.EnvPortsParams{
		WorktreePath: params.WorktreePath,
		Links:        cfg.EnvPorts,
		Owned:        owned,
		Bases:        rules.EnvPortBases(cfg),
		Shared:       rules.SharedJobNames(cfg),
		Offset:       offset,
		Block:        rules.EffectivePortOffsetBlock(cfg),
		Origins: rules.OriginContext{
			Addressing: rules.EffectiveAddressing(cfg),
			Jobs:       jobsByName(cfg),
			Worktree:   env[domain.EnvWorktree],
			Project:    filepath.Base(params.ProjectDir),
			PublicPort: process.PublicProxyPort(rules.ProxyPort(params.Global)),
		},
	}, nil
}

// EnvPortPlanFor is the [[env_port]] pass this worktree would get, computed and
// not applied. A surface handing out named URLs reads it to know whether the
// .env behind them answers on those names yet; `wtm env` computes the very same
// plan before writing it, so the two can never disagree.
func EnvPortPlanFor(params ResolveEnvPortsParams) (domain.EnvPortPlan, error) {
	resolved, err := ResolveEnvPorts(params)
	if err != nil || resolved.Empty() {
		return domain.EnvPortPlan{}, err
	}
	return envsvc.ComputeEnvPorts(resolved)
}

func jobsByName(cfg domain.RunConfig) map[string]domain.JobConfig {
	byName := make(map[string]domain.JobConfig, len(cfg.Jobs))
	for _, job := range cfg.Jobs {
		byName[job.Name] = job
	}
	return byName
}
