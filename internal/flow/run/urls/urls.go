// Package urls is where every address the run module hands out is computed
// from. It never contacts the daemon: a job's address is a property of its
// worktree's offset, known whether or not anything is running.
package urls

import (
	"context"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
)

type Params struct {
	Context flow.Context
	Config  domain.RunConfig
	// Raw asks for the job's own port instead of the name the proxy serves it
	// under — the address the .env answers on whatever the addressing says.
	Raw bool
}

// Reader answers where each job is reachable, worktree by worktree.
type Reader struct {
	ctx       flow.Context
	config    domain.RunConfig
	proxyPort int
}

func Open(ctx context.Context, params Params) Reader {
	proxyPort := 0
	if !params.Raw {
		proxyPort = process.PublicProxyPort(ctx, rules.RunProxyPort(rules.RunProxyPortParams{Run: params.Config, Global: params.Context.Config.Global}))
	}
	return Reader{ctx: params.Context, config: params.Config, proxyPort: proxyPort}
}

// Serving reports whether a name is published at all: with no proxy, or under
// --raw, every address this hands out is a plain port.
func (r Reader) Serving() bool { return r.proxyPort > 0 }

// In lists the jobs reachable in one worktree. The worktree is what makes the
// addresses differ: its ordinal decides every port.
func (r Reader) In(ctx context.Context, dir string) ([]domain.JobURLEntry, error) {
	env, err := seam.JobEnv(ctx, seam.JobEnvParams{ProjectDir: r.ctx.ProjectDir, StateDir: r.ctx.StateDir, WorkDir: dir, Publisher: r.ctx.Publisher})
	if err != nil {
		return nil, err
	}
	addresses := rules.WorktreeJobAddresses(rules.WorktreeJobAddressesParams{
		Config:     r.config,
		PortOffset: rules.PortOffsetFromEnv(env),
		Worktree:   env[domain.EnvWorktree],
		Project:    filepath.Base(r.ctx.ProjectDir),
		PublicPort: r.proxyPort,
	})

	var entries []domain.JobURLEntry
	for _, job := range r.config.Jobs {
		url := addresses[job.Name].URL
		if url == "" {
			continue
		}
		entries = append(entries, domain.JobURLEntry{Job: job.Name, URL: url})
	}
	return entries, nil
}
