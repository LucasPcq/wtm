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
	// Addressing overrides run.toml's when set: a plan read before a switch is
	// written, to know what the switch would move.
	Addressing domain.Addressing
	// Isolation overrides the recorded one when set, for the same reason: the
	// choice is recorded only once the .env is in line with it.
	Isolation domain.Isolation
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
	if params.Addressing != "" {
		cfg.Addressing = params.Addressing
	}

	// A verbatim worktree keeps its .env exactly as it was copied: no identity,
	// no port, no slice. Resolving to nothing here is what makes every writer —
	// create, `wtm env`, an addressing switch — leave it alone alike.
	ref := WorktreeRef{ProjectDir: params.ProjectDir, StateDir: params.StateDir, Branch: params.Branch}
	isolation := params.Isolation
	if isolation == "" {
		isolation = IsolationOf(ref)
	}
	if rules.IsVerbatim(isolation) {
		return envsvc.EnvPortsParams{}, nil
	}

	owned, err := ownedEnvWrites(ownedEnvWritesParams{Resolve: params, Config: cfg})
	if err != nil {
		return envsvc.EnvPortsParams{}, err
	}

	// The identity needs neither an ordinal nor an offset, so a project with no
	// link resolves without asking git anything — EnsureOrdinal writes, and this
	// function is on the read path of every address wtm hands out.
	if len(cfg.EnvPorts) == 0 && len(cfg.EnvValues) == 0 {
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
	env, err := branchEnvAs(ref, isolation)
	if err != nil {
		return envsvc.EnvPortsParams{}, err
	}
	offset, err := strconv.Atoi(env[domain.EnvPortOffset])
	if err != nil {
		return envsvc.EnvPortsParams{}, fmt.Errorf("resolve port offset: %w", err)
	}

	origins := rules.OriginContext{
		Addressing: rules.EffectiveAddressing(cfg),
		Jobs:       jobsByName(cfg),
		Worktree:   env[domain.EnvWorktree],
		Project:    filepath.Base(params.ProjectDir),
		PublicPort: process.PublicProxyPort(rules.ProxyPort(params.Global)),
	}

	ordinal, err := strconv.Atoi(env[domain.EnvOrdinal])
	if err != nil {
		return envsvc.EnvPortsParams{}, fmt.Errorf("resolve ordinal: %w", err)
	}
	values, err := rules.EnvValueWrites(rules.EnvValueWritesParams{
		Config:   cfg,
		Worktree: env[domain.EnvWorktree],
		Ordinal:  ordinal,
		Offset:   offset,
		Origins:  origins,
	})
	if err != nil {
		return envsvc.EnvPortsParams{}, err
	}

	return envsvc.EnvPortsParams{
		WorktreePath: params.WorktreePath,
		Links:        cfg.EnvPorts,
		ValueLinks:   cfg.EnvValues,
		Owned:        append(owned, values...),
		Bases:        rules.EnvPortBases(cfg),
		Shared:       rules.SharedJobNames(cfg),
		Offset:       offset,
		Block:        rules.EffectivePortOffsetBlock(cfg),
		Origins:      origins,
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

type ownedEnvWritesParams struct {
	Resolve ResolveEnvPortsParams
	Config  domain.RunConfig
}

// ownedEnvWrites resolves the compose project name from the branch and the
// repository alone: this value is written to a file, so it must not inherit the
// COMPOSE_PROJECT_NAME the calling process happens to carry — a `wtm env` run
// from inside another worktree, or from a job, would stamp that worktree's name
// into this one's .env. The main checkout gets the name its jobs run under,
// which never follows its branch.
func ownedEnvWrites(params ownedEnvWritesParams) ([]domain.EnvOwnedEntry, error) {
	targets := rules.OwnedEnvTargets(rules.OwnedEnvTargetsParams{Config: params.Config, EnvFiles: params.Resolve.EnvFiles})
	if len(targets) == 0 {
		return nil, nil
	}

	isMain, err := isMainBranch(WorktreeRef{ProjectDir: params.Resolve.ProjectDir, Branch: params.Resolve.Branch})
	if err != nil {
		return nil, err
	}
	name := rules.ComposeProjectName(rules.ComposeProjectNameParams{
		Project:  filepath.Base(params.Resolve.ProjectDir),
		Worktree: rules.WorktreeSlug(params.Resolve.Branch),
	})
	if isMain {
		name = mainComposeProject(mainComposeProjectParams{ProjectDir: params.Resolve.ProjectDir, Config: params.Config})
	}

	return rules.OwnedEnvWrites(rules.OwnedEnvWritesParams{
		Config:   params.Config,
		EnvFiles: params.Resolve.EnvFiles,
		Values:   map[string]string{domain.EnvComposeProjectName: name},
	}), nil
}

func jobsByName(cfg domain.RunConfig) map[string]domain.JobConfig {
	byName := make(map[string]domain.JobConfig, len(cfg.Jobs))
	for _, job := range cfg.Jobs {
		byName[job.Name] = job
	}
	return byName
}

type OwnedEnvKeysParams struct {
	StateDir string
	EnvFiles []domain.EnvFile
}

// OwnedEnvKeys are the .env keys wtm writes into an isolated worktree, whatever
// this one's isolation: they are what a switch to verbatim puts back.
func OwnedEnvKeys(params OwnedEnvKeysParams) ([]domain.EnvKeyRef, error) {
	cfg, err := config.LoadRun(params.StateDir)
	if err != nil {
		return nil, err
	}
	return rules.OwnedEnvKeyRefs(rules.OwnedEnvTargetsParams{Config: cfg, EnvFiles: params.EnvFiles}), nil
}
