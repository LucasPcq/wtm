package runenv

import (
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	envcase "github.com/LucasPcq/wtm/internal/rules/runenv"
)

type EnvParams struct {
	Params
	WorktreePath string
	// EnvFiles are the targets config.toml declares: a link naming any other file is left out.
	EnvFiles []domain.EnvFile
	Proxy    domain.ProxyConfig
	// PublicPort is what a named URL announces, zero when nothing serves names: the caller asks the daemon.
	PublicPort int
}

type EnvWrites struct {
	WorktreePath string
	Links        []domain.EnvPortLink
	// ValueLinks is nil on the main checkout, whose [[env]] keys are plain keys.
	ValueLinks []domain.EnvValueLink
	Owned      []domain.EnvOwnedEntry
	Bases      map[domain.PortRef]int
	Shared     map[string]bool
	Offset     int
	Block      int
	Origins    rules.OriginContext
}

func (w EnvWrites) Empty() bool {
	return len(w.Links) == 0 && len(w.Owned) == 0
}

// Env resolves a verbatim worktree to no write, which is what makes every writer leave its .env exactly as it was copied.
func Env(params EnvParams) (EnvWrites, error) {
	envCase := caseOf(params.Identity)
	if _, verbatim := envCase.(envcase.Verbatim); verbatim {
		return EnvWrites{}, nil
	}
	cfg, _ := rules.PartitionEnvTargets(rules.PartitionEnvTargetsParams{Config: params.Run, Files: params.EnvFiles})
	owned, err := ownedWrites(ownedWritesParams{Env: params, Config: cfg})
	if err != nil {
		return EnvWrites{}, err
	}

	if len(cfg.EnvPorts) == 0 && len(cfg.EnvValues) == 0 {
		if len(owned) == 0 {
			return EnvWrites{}, nil
		}
		return EnvWrites{WorktreePath: params.WorktreePath, Owned: owned}, nil
	}
	if err := rules.ValidateProxy(params.Proxy); err != nil {
		return EnvWrites{}, err
	}

	// The environment rather than the ordinal: a .env and the route a job answers under never disagree on which worktree they belong to.
	env, err := JobEnv(params.Params)
	if err != nil {
		return EnvWrites{}, err
	}
	offset, err := offsetOf(env)
	if err != nil {
		return EnvWrites{}, err
	}
	origins := rules.OriginContext{
		Addressing: rules.EffectiveAddressing(cfg),
		Jobs:       jobsByName(cfg),
		Worktree:   env[domain.EnvWorktree],
		Project:    filepath.Base(params.ProjectDir),
		PublicPort: params.PublicPort,
	}
	values, err := rules.EnvValueWrites(rules.EnvValueWritesParams{
		Config:   cfg,
		Worktree: env[domain.EnvWorktree],
		Ordinal:  params.Identity.Ordinal,
		Offset:   offset,
		Origins:  origins,
	})
	if err != nil {
		return EnvWrites{}, err
	}
	valueLinks, values, err := valuesFor(valuesForParams{Env: params, Case: envCase, Links: cfg.EnvValues, Stamps: values})
	if err != nil {
		return EnvWrites{}, err
	}

	return EnvWrites{
		WorktreePath: params.WorktreePath,
		Links:        cfg.EnvPorts,
		ValueLinks:   valueLinks,
		Owned:        append(owned, values...),
		Bases:        rules.EnvPortBases(cfg),
		Shared:       rules.SharedJobNames(cfg),
		Offset:       offset,
		Block:        rules.EffectivePortOffsetBlock(cfg),
		Origins:      origins,
	}, nil
}

type valuesForParams struct {
	Env    EnvParams
	Case   envcase.Case
	Links  []domain.EnvValueLink
	Stamps []domain.EnvOwnedEntry
}

func valuesFor(params valuesForParams) ([]domain.EnvValueLink, []domain.EnvOwnedEntry, error) {
	if _, main := params.Case.(envcase.Main); !main {
		return params.Links, params.Stamps, nil
	}
	// The main checkout's [[env]] keys stay plain: a value wtm stamped there goes back to the template's.
	repairs, err := mainValueRepairs(mainValueRepairsParams{Env: params.Env, Stamps: params.Stamps})
	return nil, repairs, err
}

type ownedWritesParams struct {
	Env    EnvParams
	Config domain.RunConfig
}

func ownedWrites(params ownedWritesParams) ([]domain.EnvOwnedEntry, error) {
	if len(rules.OwnedEnvTargets(rules.OwnedEnvTargetsParams{Config: params.Config, EnvFiles: params.Env.EnvFiles})) == 0 {
		return nil, nil
	}
	composeProject, err := ComposeProject(params.Env.Params)
	if err != nil {
		return nil, err
	}
	return rules.OwnedEnvWrites(rules.OwnedEnvWritesParams{
		Config:   params.Config,
		EnvFiles: params.Env.EnvFiles,
		Values:   map[string]string{domain.EnvComposeProjectName: composeProject},
	}), nil
}

func jobsByName(cfg domain.RunConfig) map[string]domain.JobConfig {
	byName := make(map[string]domain.JobConfig, len(cfg.Jobs))
	for _, job := range cfg.Jobs {
		byName[job.Name] = job
	}
	return byName
}
