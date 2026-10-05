package runjobs

import (
	"context"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// Read is the index as a surface that polls reads it: waking a sleeping daemon
// only when the caller says the question is worth it. A waking read always
// knows — it opens the daemon rather than asking whether one is listening.
func Read(wake bool) (jobs []domain.JobInfo, known bool) {
	if wake {
		return Load(), true
	}
	return Peek()
}

type PublicPortParams struct {
	StateDir string
	Global   domain.GlobalConfig
}

// PublicPort is the port this project's published names answer on. run.toml is
// read on each call, like the port itself: `wtm run addressing` may switch the
// project while a surface is open, and a daemon started after it must not leave
// every address unpublished.
func PublicPort(params PublicPortParams) int {
	run, _ := runconfig.Load(params.StateDir)
	return process.PublicProxyPort(rules.RunProxyPort(rules.RunProxyPortParams{Run: run, Global: params.Global}))
}

type TracesParams struct {
	StateDir string
	Branches []string
}

// Traces names, per branch, the jobs that left output in that worktree: the
// only read that says what a worktree ran and no longer runs.
func Traces(params TracesParams) map[string]map[string]bool {
	logged := make(map[string]map[string]bool, len(params.Branches))
	for _, branch := range params.Branches {
		logged[branch] = process.LoggedJobs(rules.WorktreeLogDir(rules.WorktreeLogDirParams{
			StateDir: params.StateDir,
			Branch:   branch,
		}))
	}
	return logged
}

type AddressesParams struct {
	ProjectDir string
	StateDir   string
	Config     domain.RunConfig
	// A branch no run has given an ordinal yet is skipped: reading an address
	// never allocates one.
	Branches []string
	EnvFiles []domain.EnvFile
	Global   domain.GlobalConfig
}

// Addresses is where the named worktrees' jobs answer, against the public port
// dialed now rather than once at startup.
func Addresses(ctx context.Context, params AddressesParams) domain.RunAddresses {
	return worktree.RunAddressesFor(ctx, worktree.RunAddressesForParams{
		ProjectDir: params.ProjectDir,
		StateDir:   params.StateDir,
		RunConfig:  params.Config,
		Branches:   params.Branches,
		EnvFiles:   params.EnvFiles,
		Global:     params.Global,
		ProxyPort:  process.PublicProxyPort(rules.RunProxyPort(rules.RunProxyPortParams{Run: params.Config, Global: params.Global})),
	})
}
