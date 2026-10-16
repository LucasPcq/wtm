package provision

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/LucasPcq/wtm/next/internal/wtm/jobenv"
	"github.com/LucasPcq/wtm/next/internal/wtm/settle"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
)

var errOrdinalUnreadable = errors.New("cannot read a live worktree's ordinal")

func (p *provisioning) needsOrdinal() bool {
	return p.Project.RunErr == nil && (p.portsNeedOrdinal() || p.hooksReadRun())
}

func (p *provisioning) portsNeedOrdinal() bool {
	return settle.NeedsOrdinal(settle.NeedsOrdinalParams{Isolation: p.Worktree.Isolation, Run: p.kept})
}

func (p *provisioning) hooksReadRun() bool {
	return len(p.Project.OnCreate) > 0 && jobenv.HooksReadRun(jobenv.HooksReadRunParams{Run: p.Project.Run, Isolation: p.Worktree.Isolation})
}

func (p *provisioning) number(ctx context.Context, provisioned *Provisioned) error {
	ordinal, err := p.allocate(ctx)
	if err != nil && ctx.Err() != nil {
		return err
	}
	if err != nil {
		if p.portsNeedOrdinal() {
			p.warn(p.notSettled(notSettled{Cause: err}))
		}
		return nil
	}
	provisioned.Ordinal = &ordinal
	return nil
}

func (p *provisioning) allocate(ctx context.Context) (int, error) {
	var ordinal int
	held, err := p.Deps.Meta.LockOrdinals(ctx, func() error {
		taken, err := p.taken(ctx)
		if err != nil {
			return err
		}
		ordinal = worktree.FreeOrdinal(taken)
		written, err := p.Deps.Meta.Update(ctx, worktree.MetaUpdate{Branch: p.Worktree.Branch, Change: func(meta *worktree.Meta) error {
			meta.Ordinal = ordinal
			return nil
		}})
		p.unlocked = p.unlocked || written.Unlocked
		return err
	})
	p.unlocked = p.unlocked || !held
	return ordinal, err
}

func (p *provisioning) taken(ctx context.Context) ([]int, error) {
	worktrees, err := p.Deps.Git.Worktrees(ctx)
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}
	var taken []int
	for _, listed := range worktrees {
		branch := listed.Branch()
		if listed.Main || branch == "" || branch == p.Worktree.Branch {
			continue
		}
		meta, err := p.Deps.Meta.Load(branch)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", errOrdinalUnreadable, branch, err)
		}
		taken = append(taken, meta.Ordinal)
	}
	return taken, nil
}
