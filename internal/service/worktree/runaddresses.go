package worktree

import (
	"path/filepath"
	"strconv"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type RunAddressesForParams struct {
	ProjectDir string
	StateDir   string
	RunConfig  domain.RunConfig
	// Branches must only name worktrees that already have a job up. BranchEnv
	// goes through EnsureOrdinal, which writes: sweeping every worktree would
	// allocate an ordinal to worktrees that never ran anything.
	Branches []string
	// EnvFiles and Global are what the [[env_port]] pass is resolved against: a
	// worktree whose .env still spells ports is served its ports, since the name
	// it publishes is the one entrance nothing behind it answers on.
	EnvFiles  []domain.EnvFile
	Global    domain.GlobalConfig
	ProxyPort int
}

// planFor is the worktree's [[env_port]] pass, computed and not applied. Empty
// when it cannot be read, which reads as settled: an address is a poor place to
// report that a config could not be loaded.
func planFor(params RunAddressesForParams, branch, path string) domain.EnvPortPlan {
	if path == "" {
		return domain.EnvPortPlan{}
	}
	plan, err := EnvPortPlanFor(ResolveEnvPortsParams{
		ProjectDir:   params.ProjectDir,
		StateDir:     params.StateDir,
		Branch:       branch,
		WorktreePath: path,
		EnvFiles:     params.EnvFiles,
		Global:       params.Global,
	})
	if err != nil {
		return domain.EnvPortPlan{}
	}
	return plan
}

// pathsByBranch lists the worktrees once rather than per branch: resolving each
// path on its own turned one address refresh into a git call per worktree.
func pathsByBranch(projectDir string) map[string]string {
	worktrees, err := ListAll(ListAllParams{ProjectDir: projectDir})
	if err != nil {
		return nil
	}
	paths := make(map[string]string, len(worktrees))
	for _, wt := range worktrees {
		paths[wt.Branch] = wt.Path
	}
	return paths
}

// RunAddressesFor is where every declared job answers, in each worktree asked
// for. A branch whose environment cannot be read is left out rather than given
// a guess: a wrong port reads as a truth.
func RunAddressesFor(params RunAddressesForParams) domain.RunAddresses {
	if len(params.RunConfig.Jobs) == 0 {
		return domain.RunAddresses{}
	}

	paths := pathsByBranch(params.ProjectDir)
	answer := domain.RunAddresses{
		ByBranch:      make(map[string]map[string]domain.JobAddress, len(params.Branches)),
		Notes:         map[string]string{},
		PortAddressed: map[string]bool{},
	}
	for _, branch := range params.Branches {
		env, err := BranchEnv(WorktreeRef{
			ProjectDir: params.ProjectDir,
			StateDir:   params.StateDir,
			Branch:     branch,
		})
		if err != nil {
			continue
		}
		offset, err := strconv.Atoi(env[domain.EnvPortOffset])
		if err != nil {
			continue
		}
		plan := planFor(params, branch, paths[branch])
		publicPort := params.ProxyPort
		if rules.AddressedByPort(plan) {
			publicPort = 0
			answer.PortAddressed[branch] = true
		}
		if note := rules.AddressingDriftLine(rules.AddressingDriftParams{Worktree: branch, Plan: plan}); note != "" {
			answer.Notes[branch] = note
		}

		answer.ByBranch[branch] = rules.WorktreeJobAddresses(rules.WorktreeJobAddressesParams{
			Config:     params.RunConfig,
			PortOffset: offset,
			Worktree:   env[domain.EnvWorktree],
			Project:    filepath.Base(params.ProjectDir),
			PublicPort: publicPort,
		})
	}
	return answer
}
