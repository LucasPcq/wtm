package runenv

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	envcase "github.com/LucasPcq/wtm/internal/rules/runenv"
)

type EnvParams struct {
	Params
	WorktreePath string
	// EnvFiles are the targets config.toml declares: a link naming any other
	// file is left out.
	EnvFiles []domain.EnvFile
	// Proxy is the machine's [proxy] table, PublicPort what a named URL
	// announces, zero when nothing serves names: the caller asks the daemon.
	Proxy      domain.ProxyConfig
	PublicPort int
}

// EnvWrites is what the worktree's .env files are brought in line with; the
// zero value is nothing to write.
type EnvWrites struct {
	WorktreePath string
	Links        []domain.EnvPortLink
	// ValueLinks is nil on the main checkout, whose [[env]] keys are plain keys.
	ValueLinks []domain.EnvValueLink
	// Owned are the keys wtm writes whole: COMPOSE_PROJECT_NAME and the [[env]]
	// values, or on the main checkout the template values they go back to.
	Owned   []domain.EnvOwnedEntry
	Bases   map[domain.PortRef]int
	Shared  map[string]bool
	Offset  int
	Block   int
	Origins rules.OriginContext
}

func (w EnvWrites) Empty() bool {
	return len(w.Links) == 0 && len(w.Owned) == 0
}

// Env resolves the writes; a verbatim worktree resolves to none, which is what
// makes every writer leave its .env exactly as it was copied.
func Env(params EnvParams) (EnvWrites, error) {
	envCase := caseOf(params.Identity)
	if _, verbatim := envCase.(envcase.Verbatim); verbatim {
		return EnvWrites{}, nil
	}
	cfg, _ := rules.PartitionEnvTargets(rules.PartitionEnvTargetsParams{Config: params.Run, Files: params.EnvFiles})
	owned := ownedWrites(ownedWritesParams{Env: params, Config: cfg})

	if len(cfg.EnvPorts) == 0 && len(cfg.EnvValues) == 0 {
		if len(owned) == 0 {
			return EnvWrites{}, nil
		}
		return EnvWrites{WorktreePath: params.WorktreePath, Owned: owned}, nil
	}
	if err := rules.ValidateProxy(params.Proxy); err != nil {
		return EnvWrites{}, err
	}

	// The environment rather than the ordinal alone: it settles the offset and
	// the worktree label in one place, so a .env and the route a job answers
	// under never disagree on which worktree they belong to.
	env := JobEnv(params.Params)
	offset, err := strconv.Atoi(env[domain.EnvPortOffset])
	if err != nil {
		return EnvWrites{}, fmt.Errorf("resolve port offset: %w", err)
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

// valuesFor leaves the main checkout's [[env]] keys plain: a value wtm stamped
// there goes back to the template's.
func valuesFor(params valuesForParams) ([]domain.EnvValueLink, []domain.EnvOwnedEntry, error) {
	if _, main := params.Case.(envcase.Main); !main {
		return params.Links, params.Stamps, nil
	}
	repairs, err := mainValueRepairs(mainValueRepairsParams{MainPath: params.Env.WorktreePath, Files: params.Env.EnvFiles, Stamps: params.Stamps})
	return nil, repairs, err
}

type ownedWritesParams struct {
	Env    EnvParams
	Config domain.RunConfig
}

func ownedWrites(params ownedWritesParams) []domain.EnvOwnedEntry {
	if len(rules.OwnedEnvTargets(rules.OwnedEnvTargetsParams{Config: params.Config, EnvFiles: params.Env.EnvFiles})) == 0 {
		return nil
	}
	return rules.OwnedEnvWrites(rules.OwnedEnvWritesParams{
		Config:   params.Config,
		EnvFiles: params.Env.EnvFiles,
		Values:   map[string]string{domain.EnvComposeProjectName: ComposeProject(params.Env.Params)},
	})
}

func jobsByName(cfg domain.RunConfig) map[string]domain.JobConfig {
	byName := make(map[string]domain.JobConfig, len(cfg.Jobs))
	for _, job := range cfg.Jobs {
		byName[job.Name] = job
	}
	return byName
}
