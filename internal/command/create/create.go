// Package create is `wtm create` on the engine: a worktree for each branch asked, from one source, each provisioned whole.
package create

import (
	"context"
	"slices"
	"time"

	"github.com/LucasPcq/wtm/internal/command/provision"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/kernel"
	rulebranch "github.com/LucasPcq/wtm/internal/rules/branch"
	"github.com/LucasPcq/wtm/internal/store"
)

type Ports struct {
	Git        git.ReadWriter
	Meta       store.Meta
	Config     store.ProjectConfig
	Env        store.EnvFiles
	Dir        string
	StateDir   string
	PublicPort func(ctx context.Context, configured int) int
	Now        func() time.Time
}

func Command(ports Ports) kernel.Command[Request, Facts, Plan, provision.Detail] {
	return kernel.Command[Request, Facts, Plan, provision.Detail]{
		Name:    domain.CmdCreate,
		Observe: observer(ports),
		Forget:  forget,
		Fields:  fields(),
		Rules:   checks(),
		Locks:   locks,
		Plan:    plan,
		Apply:   applier(ports),
	}
}

func locks(req Request) []kernel.LockKey {
	keys := []kernel.LockKey{{Scope: kernel.LockRepo, Mode: kernel.LockShared}}
	for _, branch := range req.Branches {
		if branch != "" {
			keys = append(keys, kernel.LockKey{Scope: kernel.LockWorktree, Name: branch, Mode: kernel.LockExclusive})
		}
	}
	if fastForwardsSource(req) {
		keys = append(keys, kernel.LockKey{Scope: kernel.LockWorktree, Name: req.From, Mode: kernel.LockExclusive})
	}
	return keys
}

func fastForwardsSource(req Request) bool {
	return req.FastForward && req.From != "" && !rulebranch.IsRemote(req.From) && !slices.Contains(req.Branches, req.From)
}
